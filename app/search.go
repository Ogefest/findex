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
	MinSize int64
	MaxSize int64
	Exts    []string
}

type Searcher struct {
	dbs map[string]*sql.DB
}

func NewSearcher(indexes []*models.IndexConfig) (*Searcher, error) {
	dbs := make(map[string]*sql.DB)
	for _, idx := range indexes {
		dsn := fmt.Sprintf("file:%s?mode=ro", idx.DBPath)
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			for _, d := range dbs {
				d.Close()
			}
			return nil, fmt.Errorf("failed to open db %s: %w", idx.DBPath, err)
		}
		q := `PRAGMA case_sensitive_like = ON;`
		db.Exec(q)

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
	sqlQuery := fmt.Sprintf(`
        SELECT f.id, f.path, f.name, f.dir, f.ext, f.size, f.mod_time, f.is_dir, f.index_name
        FROM files f
        JOIN files_fts ft ON ft.rowid = f.rowid
        WHERE f.id = %d
        LIMIT 1`, id)
	rows, err := s.dbs[indexName].Query(sqlQuery)
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
			dirInfo, _ := s.GetDirectorySize(indexName, f.Path)
			f.Size = dirInfo.Size
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
		sql := "SELECT SUM(size), COUNT(*) FROM files"
		err := s.dbs[indexName].QueryRow(sql).Scan(&result.Size, &result.Files)
		if err != nil {
			return models.DirInfo{}, err
		}
	} else {
		q := `PRAGMA case_sensitive_like = ON;`
		s.dbs[indexName].Exec(q)

		sql := `
		SELECT SUM(size), COUNT(*)
		FROM files
		WHERE path LIKE ?
		AND is_dir = 0
	`

		err := s.dbs[indexName].QueryRow(sql, path+"/%").Scan(&result.Size, &result.Files)
		if err != nil {
			return models.DirInfo{}, err
		}
	}

	return result, nil
}

func (s *Searcher) searchIndex(db *sql.DB, query string, filter *FileFilter, limit int) ([]models.FileRecord, error) {
	log.Printf("Index search %s %d\n", query, limit)

	if query == "" {
		return nil, nil
	}

	querySafe := strings.ReplaceAll(query, `"`, `""`)
	querySafe = strings.ReplaceAll(query, `.`, ` `)
	querySafe = prepareFTSQuery(querySafe)

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
				exts = append(exts, fmt.Sprintf("f.ext='%s'", e))
			}
			conditions = append(conditions, "("+strings.Join(exts, " OR ")+")")
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	sqlQuery := fmt.Sprintf(`
        SELECT f.id, f.path, f.name, f.dir, f.ext, f.size, f.mod_time, f.is_dir, f.index_name
        FROM files f
        JOIN files_fts ft ON ft.rowid = f.rowid
        WHERE files_fts MATCH ? %s
        LIMIT ?`, whereClause)

	rows, err := db.Query(sqlQuery, querySafe, limit)

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
