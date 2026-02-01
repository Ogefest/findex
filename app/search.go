package app

import (
	"database/sql"
	"fmt"
	"hash/crc32"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/ogefest/findex/models"
	_ "modernc.org/sqlite"
)

type FileFilter struct {
	MinSize     int64
	MaxSize     int64
	Exts        []string
	ModTimeFrom int64 // unix timestamp
	ModTimeTo   int64 // unix timestamp
	OnlyFiles   bool
	OnlyDirs    bool
}

type Searcher struct {
	dbs map[string]*sql.DB
}

func NewSearcher(indexes []*models.IndexConfig) (*Searcher, error) {
	dbs := make(map[string]*sql.DB)
	for _, idx := range indexes {
		db, err := sql.Open("sqlite", idx.DBPath)
		if err != nil {
			for _, d := range dbs {
				d.Close()
			}
			return nil, fmt.Errorf("failed to open db %s: %w", idx.DBPath, err)
		}
		db.Exec(`PRAGMA case_sensitive_like = ON`)
		db.Exec(`PRAGMA journal_mode = WAL`)
		db.Exec(`PRAGMA busy_timeout = 5000`)

		dbs[idx.Name] = db
	}
	return &Searcher{dbs: dbs}, nil
}

func (s *Searcher) Close() {
	for _, db := range s.dbs {
		db.Close()
	}
}

func (s *Searcher) Search(query string, filter *FileFilter, limitPerIndex int) ([]models.FileRecord, error) {
	var results []models.FileRecord
	for _, db := range s.dbs {
		res, err := s.searchIndex(db, query, filter, limitPerIndex)
		if err != nil {
			return nil, err
		}
		results = append(results, res...)
	}
	return results, nil
}

func (s *Searcher) GetFileByID(indexName string, id int64) (*models.FileRecord, error) {
	sqlQuery := `
        SELECT id, path, name, dir, ext, size, mod_time, is_dir, index_name
        FROM files
        WHERE id = ?
        LIMIT 1`
	rows, err := s.dbs[indexName].Query(sqlQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var f models.FileRecord
		var mod int64
		var isDir int
		if err := rows.Scan(&f.ID, &f.Path, &f.Name, &f.Dir, &f.Ext, &f.Size, &mod, &isDir, &f.IndexName); err != nil {
			continue
		}
		f.ModTime = time.Unix(mod, 0)
		f.IsDir = isDir != 0
		return &f, nil
	}

	return nil, nil
}

func (s *Searcher) GetDirectoryContent(indexName string, path string) ([]models.FileRecord, error) {
	log.Printf("List dir content %s %s\n", indexName, path)
	if path != "" {
		path = fmt.Sprintf("%s/", path)
	}

	dir := filepath.Dir(path)
	normalized := filepath.Clean(dir)
	dirIndex := int64(crc32.ChecksumIEEE([]byte(normalized)))

	sqlQuery := `
		SELECT
			f.id,
			f.path,
			f.name,
			f.dir,
			f.ext,
			f.size,
			f.mod_time,
			f.is_dir,
			f.index_name
		FROM files f
		WHERE
			dir_index = ? AND f.path LIKE ?
		ORDER BY f.is_dir DESC, f.name;
    `
	rows, err := s.dbs[indexName].Query(sqlQuery, dirIndex, fmt.Sprintf("%s%%", path))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.FileRecord

	for rows.Next() {
		var f models.FileRecord
		var mod int64
		var isDir int

		if err := rows.Scan(
			&f.ID,
			&f.Path,
			&f.Name,
			&f.Dir,
			&f.Ext,
			&f.Size,
			&mod,
			&isDir,
			&f.IndexName,
		); err != nil {
			return nil, err
		}

		f.ModTime = time.Unix(mod, 0)
		f.IsDir = isDir != 0

		if f.IsDir {
			// Use cached directory size, calculate and cache if not present
			var cachedSize int64
			err := s.dbs[indexName].QueryRow(`SELECT total_size FROM dir_sizes WHERE path = ?`, f.Path).Scan(&cachedSize)
			if err == nil {
				f.Size = cachedSize
			} else if err == sql.ErrNoRows {
				// Calculate and cache on demand
				var size int64
				var count int64
				calcErr := s.dbs[indexName].QueryRow(`
					SELECT COALESCE(SUM(size), 0), COUNT(*)
					FROM files
					WHERE path LIKE ? AND is_dir = 0
				`, f.Path+"/%").Scan(&size, &count)
				if calcErr == nil {
					f.Size = size
					s.dbs[indexName].Exec(`
						INSERT OR REPLACE INTO dir_sizes (path, total_size, file_count)
						VALUES (?, ?, ?)
					`, f.Path, size, count)
				}
			}
		}

		result = append(result, f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	log.Printf("Found %d elems in %s %s", len(result), indexName, path)

	return result, nil
}

func (s *Searcher) GetDirectorySize(indexName string, path string) (models.DirInfo, error) {
	log.Printf("Dir size for %s %s\n", indexName, path)
	var result = models.DirInfo{}

	if path == "" {
		sqlQ := "SELECT SUM(size), COUNT(*) FROM files WHERE is_dir = 0"
		err := s.dbs[indexName].QueryRow(sqlQ).Scan(&result.Size, &result.Files)
		if err != nil {
			return models.DirInfo{}, err
		}
		return result, nil
	}

	// Try cache first
	err := s.dbs[indexName].QueryRow(`
		SELECT total_size, file_count FROM dir_sizes WHERE path = ?
	`, path).Scan(&result.Size, &result.Files)
	if err == nil {
		return result, nil
	}

	// Calculate and cache on demand
	sqlQ := `
		SELECT COALESCE(SUM(size), 0), COUNT(*)
		FROM files
		WHERE path LIKE ? AND is_dir = 0
	`
	err = s.dbs[indexName].QueryRow(sqlQ, path+"/%").Scan(&result.Size, &result.Files)
	if err != nil {
		return models.DirInfo{}, err
	}

	// Cache the result
	s.dbs[indexName].Exec(`
		INSERT OR REPLACE INTO dir_sizes (path, total_size, file_count)
		VALUES (?, ?, ?)
	`, path, result.Size, result.Files)

	return result, nil
}

func (s *Searcher) searchIndex(db *sql.DB, query string, filter *FileFilter, limit int) ([]models.FileRecord, error) {
	log.Printf("Index search %s %d\n", query, limit)

	// Build filter conditions
	var conditions []string
	if filter != nil {
		if filter.MinSize > 0 {
			conditions = append(conditions, fmt.Sprintf("f.size >= %d", filter.MinSize))
		}
		if filter.MaxSize > 0 {
			conditions = append(conditions, fmt.Sprintf("f.size <= %d", filter.MaxSize))
		}
		if len(filter.Exts) > 0 {
			var exts []string
			for _, e := range filter.Exts {
				e = strings.TrimPrefix(e, ".")
				exts = append(exts, fmt.Sprintf("f.ext='.%s'", e))
			}
			conditions = append(conditions, "("+strings.Join(exts, " OR ")+")")
		}
		if filter.ModTimeFrom > 0 {
			conditions = append(conditions, fmt.Sprintf("f.mod_time >= %d", filter.ModTimeFrom))
		}
		if filter.ModTimeTo > 0 {
			conditions = append(conditions, fmt.Sprintf("f.mod_time <= %d", filter.ModTimeTo))
		}
		if filter.OnlyFiles {
			conditions = append(conditions, "f.is_dir = 0")
		}
		if filter.OnlyDirs {
			conditions = append(conditions, "f.is_dir = 1")
		}
	}

	// If no query and no filters, return empty
	if query == "" && len(conditions) == 0 {
		return nil, nil
	}

	var sqlQuery string
	var rows *sql.Rows
	var err error

	if query != "" {
		// Full-text search with optional filters
		querySafe := strings.ReplaceAll(query, `"`, `""`)
		querySafe = strings.ReplaceAll(querySafe, `.`, ` `)
		querySafe = prepareFTSQuery(querySafe)

		whereClause := ""
		if len(conditions) > 0 {
			whereClause = " AND " + strings.Join(conditions, " AND ")
		}

		sqlQuery = fmt.Sprintf(`
			SELECT f.id, f.path, f.name, f.dir, f.ext, f.size, f.mod_time, f.is_dir, f.index_name
			FROM files f
			JOIN files_fts ft ON ft.rowid = f.rowid
			WHERE files_fts MATCH ? %s
			LIMIT ?`, whereClause)

		rows, err = db.Query(sqlQuery, querySafe, limit)
	} else {
		// Filter-only search (no FTS)
		whereClause := strings.Join(conditions, " AND ")

		sqlQuery = fmt.Sprintf(`
			SELECT f.id, f.path, f.name, f.dir, f.ext, f.size, f.mod_time, f.is_dir, f.index_name
			FROM files f
			WHERE %s
			ORDER BY f.mod_time DESC
			LIMIT ?`, whereClause)

		rows, err = db.Query(sqlQuery, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.FileRecord
	for rows.Next() {
		var f models.FileRecord
		var mod int64
		var isDir int
		if err := rows.Scan(&f.ID, &f.Path, &f.Name, &f.Dir, &f.Ext, &f.Size, &mod, &isDir, &f.IndexName); err != nil {
			continue
		}
		f.ModTime = time.Unix(mod, 0)
		f.IsDir = isDir != 0
		results = append(results, f)
	}

	return results, nil
}

func prepareFTSQuery(query string) string {
	parts := strings.Fields(query)
	var include []string
	var exclude []string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "-") && len(p) > 1 {
			exclude = append(exclude, p[1:])
		} else {
			include = append(include, p)
		}
	}

	var ftsQuery []string
	if len(include) > 0 {
		ftsQuery = append(ftsQuery, strings.Join(include, " AND "))
	}
	for _, ex := range exclude {
		ftsQuery = append(ftsQuery, "NOT "+ex)
	}

	return strings.Join(ftsQuery, " ")
}
