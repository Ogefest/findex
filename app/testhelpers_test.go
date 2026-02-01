package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ogefest/findex/models"
	_ "modernc.org/sqlite"
)

// setupTestDB creates a temporary SQLite database for testing
func setupTestDB(t *testing.T) (*sql.DB, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "findex_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open db: %v", err)
	}

	// Run migrations
	schema := `
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY,
			index_name TEXT,
			path TEXT NOT NULL UNIQUE,
			name TEXT,
			dir TEXT,
			dir_index INTEGER,
			ext TEXT,
			size INTEGER,
			mod_time INTEGER,
			is_dir INTEGER,
			is_searchable INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT
		);

		CREATE TABLE IF NOT EXISTS dir_sizes (
			path TEXT PRIMARY KEY,
			total_size INTEGER,
			file_count INTEGER
		);

		CREATE TABLE IF NOT EXISTS scan_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scan_time INTEGER NOT NULL,
			stats_json TEXT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_scan_history_time ON scan_history(scan_time DESC);

		CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(name, path, tokenize = 'unicode61');

		CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
		CREATE INDEX IF NOT EXISTS idx_dir_index ON files(dir_index);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to run migrations: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, dbPath, cleanup
}

// insertTestFile inserts a test file record into the database
func insertTestFile(t *testing.T, db *sql.DB, f models.FileRecord) int64 {
	t.Helper()

	isDir := 0
	if f.IsDir {
		isDir = 1
	}

	result, err := db.Exec(`
		INSERT INTO files(path, name, dir, ext, size, mod_time, is_dir, is_searchable, index_name, dir_index)
		VALUES (?, ?, ?, ?, ?, ?, ?, 2, ?, ?)
	`, f.Path, f.Name, f.Dir, f.Ext, f.Size, f.ModTime.Unix(), isDir, f.IndexName, f.DirIndex)
	if err != nil {
		t.Fatalf("failed to insert test file: %v", err)
	}

	id, _ := result.LastInsertId()

	// Insert into FTS
	_, err = db.Exec(`INSERT INTO files_fts(rowid, name, path) VALUES (?, ?, ?)`, id, f.Name, f.Path)
	if err != nil {
		t.Fatalf("failed to insert into FTS: %v", err)
	}

	return id
}

// createTestFiles creates a set of test files with various properties
func createTestFiles(t *testing.T, db *sql.DB, indexName string) []models.FileRecord {
	t.Helper()

	now := time.Now()
	files := []models.FileRecord{
		{
			IndexName: indexName,
			Path:      "documents/report.pdf",
			Name:      "report.pdf",
			Dir:       "documents",
			Ext:       ".pdf",
			Size:      1024 * 1024, // 1 MB
			ModTime:   now.AddDate(0, -1, 0),
			IsDir:     false,
		},
		{
			IndexName: indexName,
			Path:      "documents/notes.txt",
			Name:      "notes.txt",
			Dir:       "documents",
			Ext:       ".txt",
			Size:      512, // 512 B
			ModTime:   now.AddDate(0, 0, -7),
			IsDir:     false,
		},
		{
			IndexName: indexName,
			Path:      "images/photo.jpg",
			Name:      "photo.jpg",
			Dir:       "images",
			Ext:       ".jpg",
			Size:      5 * 1024 * 1024, // 5 MB
			ModTime:   now.AddDate(-1, 0, 0),
			IsDir:     false,
		},
		{
			IndexName: indexName,
			Path:      "images/screenshot.png",
			Name:      "screenshot.png",
			Dir:       "images",
			Ext:       ".png",
			Size:      2 * 1024 * 1024, // 2 MB
			ModTime:   now,
			IsDir:     false,
		},
		{
			IndexName: indexName,
			Path:      "videos/movie.mp4",
			Name:      "movie.mp4",
			Dir:       "videos",
			Ext:       ".mp4",
			Size:      500 * 1024 * 1024, // 500 MB
			ModTime:   now.AddDate(0, -6, 0),
			IsDir:     false,
		},
		{
			IndexName: indexName,
			Path:      "documents",
			Name:      "documents",
			Dir:       "",
			Ext:       "",
			Size:      0,
			ModTime:   now,
			IsDir:     true,
		},
		{
			IndexName: indexName,
			Path:      "images",
			Name:      "images",
			Dir:       "",
			Ext:       "",
			Size:      0,
			ModTime:   now,
			IsDir:     true,
		},
		{
			IndexName: indexName,
			Path:      "videos",
			Name:      "videos",
			Dir:       "",
			Ext:       "",
			Size:      0,
			ModTime:   now,
			IsDir:     true,
		},
	}

	for i := range files {
		files[i].ID = insertTestFile(t, db, files[i])
	}

	return files
}

// createSearcher creates a Searcher with a test database
func createSearcher(t *testing.T, dbPath string, indexName string) *Searcher {
	t.Helper()

	cfg := &models.IndexConfig{
		Name:   indexName,
		DBPath: dbPath,
	}

	searcher, err := NewSearcher([]*models.IndexConfig{cfg})
	if err != nil {
		t.Fatalf("failed to create searcher: %v", err)
	}

	return searcher
}
