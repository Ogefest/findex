package app

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ogefest/findex/pkg/models"
	_ "modernc.org/sqlite"
)

// FileFilter pozwala filtrować wyniki wyszukiwania
type FileFilter struct {
	MinSize int64
	MaxSize int64
	Exts    []string
}

// Searcher trzyma otwarte połączenia do wszystkich indeksów
type Searcher struct {
	dbs map[string]*sql.DB // key: indeks name
}

// NewSearcher otwiera wszystkie bazy i zwraca Searcher
func NewSearcher(indexes []*models.IndexConfig) (*Searcher, error) {
	dbs := make(map[string]*sql.DB)
	for _, idx := range indexes {
		db, err := sql.Open("sqlite", idx.DBPath)
		if err != nil {
			// zamykamy już otwarte przed błędem
			for _, d := range dbs {
				d.Close()
			}
			return nil, fmt.Errorf("failed to open db %s: %w", idx.DBPath, err)
		}
		dbs[idx.Name] = db
	}
	return &Searcher{dbs: dbs}, nil
}

// Close zamyka wszystkie połączenia
func (s *Searcher) Close() {
	for _, db := range s.dbs {
		db.Close()
	}
}

// Search wykonuje zapytanie we wszystkich indeksach i zwraca wyniki
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

// searchIndex wykonuje zapytanie FTS5 + dodatkowe filtry w jednej bazie
func (s *Searcher) searchIndex(db *sql.DB, query string, filter *FileFilter, limit int) ([]models.FileRecord, error) {
	if query == "" {
		return nil, nil
	}

	querySafe := strings.ReplaceAll(query, `"`, `""`) // zabezpieczenie FTS

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
        SELECT f.path, f.name, f.dir, f.ext, f.size, f.mod_time, f.is_dir
        FROM files f
        JOIN files_fts ft ON ft.rowid = f.id
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
		if err := rows.Scan(&f.Path, &f.Name, &f.Dir, &f.Ext, &f.Size, &mod, &isDir); err != nil {
			continue
		}
		f.ModTime = time.Unix(mod, 0)
		f.IsDir = isDir != 0
		results = append(results, f)
	}

	return results, nil
}
