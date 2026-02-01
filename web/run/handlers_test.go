package webapp

import (
	"database/sql"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ogefest/findex/models"
	_ "modernc.org/sqlite"
)

// setupTestWebApp creates a WebApp with a test database
func setupTestWebApp(t *testing.T) (*WebApp, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "findex_web_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open db: %v", err)
	}

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

	// Insert test data
	insertTestData(t, db, "test-index")
	db.Close()

	webapp := &WebApp{
		IndexConfig: []*models.IndexConfig{
			{
				Name:   "test-index",
				DBPath: dbPath,
			},
		},
		ActiveIndexes: []string{"test-index"},
	}
	webapp.InitTemplates()
	webapp.Router = webapp.GetRouter()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return webapp, dbPath, cleanup
}

func insertTestData(t *testing.T, db *sql.DB, indexName string) {
	t.Helper()

	now := time.Now()
	files := []struct {
		path    string
		name    string
		dir     string
		ext     string
		size    int64
		modTime time.Time
		isDir   bool
	}{
		{"documents", "documents", "", "", 0, now, true},
		{"documents/report.pdf", "report.pdf", "documents", ".pdf", 1024 * 1024, now.AddDate(0, -1, 0), false},
		{"documents/notes.txt", "notes.txt", "documents", ".txt", 512, now.AddDate(0, 0, -7), false},
		{"images", "images", "", "", 0, now, true},
		{"images/photo.jpg", "photo.jpg", "images", ".jpg", 5 * 1024 * 1024, now.AddDate(-1, 0, 0), false},
		{"images/screenshot.png", "screenshot.png", "images", ".png", 2 * 1024 * 1024, now, false},
		{"videos", "videos", "", "", 0, now, true},
		{"videos/movie.mp4", "movie.mp4", "videos", ".mp4", 500 * 1024 * 1024, now.AddDate(0, -6, 0), false},
	}

	for _, f := range files {
		isDir := 0
		if f.isDir {
			isDir = 1
		}
		// Calculate dir_index same way as in source_local.go
		normalized := filepath.Clean(f.dir)
		if normalized == "" {
			normalized = "."
		}
		dirIndex := int64(crc32.ChecksumIEEE([]byte(normalized)))

		result, err := db.Exec(`
			INSERT INTO files(path, name, dir, ext, size, mod_time, is_dir, is_searchable, index_name, dir_index)
			VALUES (?, ?, ?, ?, ?, ?, ?, 2, ?, ?)
		`, f.path, f.name, f.dir, f.ext, f.size, f.modTime.Unix(), isDir, indexName, dirIndex)
		if err != nil {
			t.Fatalf("failed to insert test file %s: %v", f.path, err)
		}

		id, _ := result.LastInsertId()
		_, err = db.Exec(`INSERT INTO files_fts(rowid, name, path) VALUES (?, ?, ?)`, id, f.name, f.path)
		if err != nil {
			t.Fatalf("failed to insert into FTS: %v", err)
		}
	}

	// Insert dir_sizes cache
	db.Exec(`INSERT INTO dir_sizes(path, total_size, file_count) VALUES (?, ?, ?)`, "documents", 1024*1024+512, 2)
	db.Exec(`INSERT INTO dir_sizes(path, total_size, file_count) VALUES (?, ?, ?)`, "images", 7*1024*1024, 2)
	db.Exec(`INSERT INTO dir_sizes(path, total_size, file_count) VALUES (?, ?, ?)`, "videos", 500*1024*1024, 1)
}

// Test search functionality
func TestStartPage_Search(t *testing.T) {
	webapp, _, cleanup := setupTestWebApp(t)
	defer cleanup()

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		shouldContain  []string
		shouldNotFind  []string
	}{
		{
			name:           "search for report",
			query:          "?q=report&index[]=test-index",
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"report.pdf"},
			shouldNotFind:  []string{"photo.jpg", "movie.mp4"},
		},
		{
			name:           "search for images",
			query:          "?q=images&index[]=test-index",
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"images", "photo.jpg", "screenshot.png"},
		},
		{
			name:           "search for non-existent",
			query:          "?q=nonexistent12345&index[]=test-index",
			expectedStatus: http.StatusOK,
			shouldNotFind:  []string{"report.pdf", "photo.jpg"},
		},
		{
			name:           "empty search",
			query:          "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.query, nil)
			rec := httptest.NewRecorder()

			webapp.Router.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			body := rec.Body.String()
			for _, s := range tt.shouldContain {
				if !strings.Contains(body, s) {
					t.Errorf("response should contain %q", s)
				}
			}
			for _, s := range tt.shouldNotFind {
				if strings.Contains(body, s) {
					t.Errorf("response should not contain %q", s)
				}
			}
		})
	}
}

// Test filter by size
func TestStartPage_FilterBySize(t *testing.T) {
	webapp, _, cleanup := setupTestWebApp(t)
	defer cleanup()

	tests := []struct {
		name          string
		query         string
		shouldContain []string
		shouldNotFind []string
	}{
		{
			name:          "min size 10MB",
			query:         "?min_size=10MB&index[]=test-index",
			shouldContain: []string{"movie.mp4"},
			shouldNotFind: []string{"report.pdf", "notes.txt"},
		},
		{
			name:          "max size 1MB",
			query:         "?max_size=1MB&index[]=test-index",
			shouldContain: []string{"notes.txt", "report.pdf"},
			shouldNotFind: []string{"movie.mp4"},
		},
		{
			name:          "size range 1MB to 10MB",
			query:         "?min_size=1MB&max_size=10MB&index[]=test-index",
			shouldContain: []string{"photo.jpg", "screenshot.png"},
			shouldNotFind: []string{"notes.txt", "movie.mp4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.query, nil)
			rec := httptest.NewRecorder()

			webapp.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rec.Code)
			}

			body := rec.Body.String()
			for _, s := range tt.shouldContain {
				if !strings.Contains(body, s) {
					t.Errorf("response should contain %q", s)
				}
			}
			for _, s := range tt.shouldNotFind {
				if strings.Contains(body, s) {
					t.Errorf("response should not contain %q", s)
				}
			}
		})
	}
}

// Test filter by extension
func TestStartPage_FilterByExtension(t *testing.T) {
	webapp, _, cleanup := setupTestWebApp(t)
	defer cleanup()

	tests := []struct {
		name          string
		query         string
		shouldContain []string
		shouldNotFind []string
	}{
		{
			name:          "filter by pdf",
			query:         "?ext=pdf&index[]=test-index",
			shouldContain: []string{"report.pdf"},
			shouldNotFind: []string{"photo.jpg", "movie.mp4"},
		},
		{
			name:          "filter by jpg,png",
			query:         "?ext=jpg,png&index[]=test-index",
			shouldContain: []string{"photo.jpg", "screenshot.png"},
			shouldNotFind: []string{"movie.mp4", "report.pdf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.query, nil)
			rec := httptest.NewRecorder()

			webapp.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rec.Code)
			}

			body := rec.Body.String()
			for _, s := range tt.shouldContain {
				if !strings.Contains(body, s) {
					t.Errorf("response should contain %q", s)
				}
			}
			for _, s := range tt.shouldNotFind {
				if strings.Contains(body, s) {
					t.Errorf("response should not contain %q", s)
				}
			}
		})
	}
}

// Test filter by type (files/dirs)
func TestStartPage_FilterByType(t *testing.T) {
	webapp, _, cleanup := setupTestWebApp(t)
	defer cleanup()

	tests := []struct {
		name          string
		query         string
		shouldContain []string
		shouldNotFind []string
	}{
		{
			name:          "only files",
			query:         "?type=files&index[]=test-index",
			shouldContain: []string{"report.pdf", "photo.jpg"},
		},
		{
			name:          "only dirs",
			query:         "?type=dirs&index[]=test-index",
			shouldContain: []string{"documents", "images", "videos"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.query, nil)
			rec := httptest.NewRecorder()

			webapp.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rec.Code)
			}

			body := rec.Body.String()
			for _, s := range tt.shouldContain {
				if !strings.Contains(body, s) {
					t.Errorf("response should contain %q", s)
				}
			}
			for _, s := range tt.shouldNotFind {
				if strings.Contains(body, s) {
					t.Errorf("response should not contain %q", s)
				}
			}
		})
	}
}

// Test pagination
func TestStartPage_Pagination(t *testing.T) {
	webapp, _, cleanup := setupTestWebApp(t)
	defer cleanup()

	tests := []struct {
		name           string
		query          string
		expectedStatus int
	}{
		{
			name:           "page 1 with per_page 2",
			query:          "?type=files&index[]=test-index&page=1&per_page=2",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "page 2 with per_page 2",
			query:          "?type=files&index[]=test-index&page=2&per_page=2",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid page number",
			query:          "?type=files&index[]=test-index&page=-1",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.query, nil)
			rec := httptest.NewRecorder()

			webapp.Router.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

// Test browse endpoint
func TestBrowse(t *testing.T) {
	webapp, _, cleanup := setupTestWebApp(t)
	defer cleanup()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		shouldContain  []string
	}{
		{
			name:           "browse root",
			path:           "/browse/test-index",
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"documents", "images", "videos"},
		},
		{
			name:           "browse documents folder",
			path:           "/browse/test-index?path=documents",
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"report.pdf", "notes.txt"},
		},
		{
			name:           "browse images folder",
			path:           "/browse/test-index?path=images",
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"photo.jpg", "screenshot.png"},
		},
		{
			name:           "browse non-existent index",
			path:           "/browse/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			webapp.Router.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				body := rec.Body.String()
				for _, s := range tt.shouldContain {
					if !strings.Contains(body, s) {
						t.Errorf("response should contain %q", s)
					}
				}
			}
		})
	}
}

// Test stats endpoint
func TestStats(t *testing.T) {
	webapp, _, cleanup := setupTestWebApp(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	webapp.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Check for expected content in stats page
	expectedContent := []string{
		"Statistics",
		"test-index",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(body, expected) {
			t.Errorf("stats page should contain %q", expected)
		}
	}
}

// Test 404 for non-existent routes
func TestNotFound(t *testing.T) {
	webapp, _, cleanup := setupTestWebApp(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent/route", nil)
	rec := httptest.NewRecorder()

	webapp.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

// Test parseSize function
func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"100", 100, false},
		{"1KB", 1024, false},
		{"1kb", 1024, false},
		{"10MB", 10 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"1.5GB", 1.5 * 1024 * 1024 * 1024, false},
		{"", 0, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseSize(tt.input)

			if tt.hasError && err == nil {
				t.Errorf("expected error for input %q", tt.input)
			}
			if !tt.hasError && err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
			}
			if !tt.hasError && result != tt.expected {
				t.Errorf("parseSize(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

// Test combined search and filters
func TestStartPage_CombinedFilters(t *testing.T) {
	webapp, _, cleanup := setupTestWebApp(t)
	defer cleanup()

	tests := []struct {
		name          string
		query         string
		shouldContain []string
		shouldNotFind []string
	}{
		{
			name:          "search + size filter",
			query:         "?q=photo&min_size=1MB&index[]=test-index",
			shouldContain: []string{"photo.jpg"},
			shouldNotFind: []string{"notes.txt"},
		},
		{
			name:          "extension + size filter",
			query:         "?ext=jpg,png&max_size=3MB&index[]=test-index",
			shouldContain: []string{"screenshot.png"},
			shouldNotFind: []string{"photo.jpg"}, // 5MB > 3MB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.query, nil)
			rec := httptest.NewRecorder()

			webapp.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rec.Code)
			}

			body := rec.Body.String()
			for _, s := range tt.shouldContain {
				if !strings.Contains(body, s) {
					t.Errorf("response should contain %q", s)
				}
			}
			for _, s := range tt.shouldNotFind {
				if strings.Contains(body, s) {
					t.Errorf("response should not contain %q", s)
				}
			}
		})
	}
}
