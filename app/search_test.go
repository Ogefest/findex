package app

import (
	"testing"
	"time"
)

func TestSearch_BasicQuery(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	tests := []struct {
		name          string
		query         string
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "search for report",
			query:         "report",
			expectedCount: 1,
			expectedNames: []string{"report.pdf"},
		},
		{
			name:          "search for photo",
			query:         "photo",
			expectedCount: 1,
			expectedNames: []string{"photo.jpg"},
		},
		{
			name:          "search in documents path",
			query:         "documents",
			expectedCount: 3, // directory + 2 files
		},
		{
			name:          "search for non-existent file",
			query:         "nonexistent12345",
			expectedCount: 0,
		},
		{
			name:          "search for multiple terms",
			query:         "screenshot png",
			expectedCount: 1,
			expectedNames: []string{"screenshot.png"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := searcher.Search(tt.query, nil, 100)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			if tt.expectedNames != nil {
				for _, expectedName := range tt.expectedNames {
					found := false
					for _, r := range results {
						if r.Name == expectedName {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected to find %s in results", expectedName)
					}
				}
			}
		})
	}
}

func TestSearch_WithExcludeTerms(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	// Search for images but exclude screenshot
	results, err := searcher.Search("images -screenshot", nil, 100)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	for _, r := range results {
		if r.Name == "screenshot.png" {
			t.Error("screenshot.png should be excluded from results")
		}
	}
}

func TestSearch_FilterBySize(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	tests := []struct {
		name          string
		filter        *FileFilter
		expectedCount int
	}{
		{
			name: "min size 1MB",
			filter: &FileFilter{
				MinSize: 1024 * 1024, // 1 MB
			},
			expectedCount: 4, // report.pdf (1MB), photo.jpg (5MB), screenshot.png (2MB), movie.mp4 (500MB)
		},
		{
			name: "max size 1MB",
			filter: &FileFilter{
				MaxSize: 1024 * 1024, // 1 MB
			},
			expectedCount: 5, // notes.txt (512B), report.pdf (1MB), 3 directories
		},
		{
			name: "size range 1MB to 10MB",
			filter: &FileFilter{
				MinSize: 1024 * 1024,      // 1 MB
				MaxSize: 10 * 1024 * 1024, // 10 MB
			},
			expectedCount: 3, // report.pdf (1MB), photo.jpg (5MB), screenshot.png (2MB)
		},
		{
			name: "size greater than 100MB",
			filter: &FileFilter{
				MinSize: 100 * 1024 * 1024, // 100 MB
			},
			expectedCount: 1, // movie.mp4 (500MB)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use empty query with filter only
			results, err := searcher.Search("", tt.filter, 100)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
				for _, r := range results {
					t.Logf("  - %s (%d bytes)", r.Name, r.Size)
				}
			}
		})
	}
}

func TestSearch_FilterByExtension(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	tests := []struct {
		name          string
		extensions    []string
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "filter by pdf",
			extensions:    []string{"pdf"},
			expectedCount: 1,
			expectedNames: []string{"report.pdf"},
		},
		{
			name:          "filter by jpg and png",
			extensions:    []string{"jpg", "png"},
			expectedCount: 2,
			expectedNames: []string{"photo.jpg", "screenshot.png"},
		},
		{
			name:          "filter by txt",
			extensions:    []string{".txt"}, // with dot prefix
			expectedCount: 1,
			expectedNames: []string{"notes.txt"},
		},
		{
			name:          "filter by non-existent extension",
			extensions:    []string{"xyz"},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &FileFilter{
				Exts: tt.extensions,
			}

			results, err := searcher.Search("", filter, 100)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			for _, expectedName := range tt.expectedNames {
				found := false
				for _, r := range results {
					if r.Name == expectedName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find %s in results", expectedName)
				}
			}
		})
	}
}

func TestSearch_FilterByDate(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)
	oneYearAgo := now.AddDate(-1, 0, 0)

	tests := []struct {
		name          string
		modTimeFrom   int64
		modTimeTo     int64
		minResults    int
		maxResults    int
	}{
		{
			name:        "files from last 30 days",
			modTimeFrom: thirtyDaysAgo.Unix(),
			minResults:  2, // screenshot.png, notes.txt, and directories
		},
		{
			name:        "files older than 6 months",
			modTimeTo:   now.AddDate(0, -6, 0).Unix(),
			minResults:  1, // photo.jpg (1 year old)
		},
		{
			name:        "files from last year",
			modTimeFrom: oneYearAgo.Unix(),
			minResults:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &FileFilter{
				ModTimeFrom: tt.modTimeFrom,
				ModTimeTo:   tt.modTimeTo,
			}

			results, err := searcher.Search("", filter, 100)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}

			if len(results) < tt.minResults {
				t.Errorf("expected at least %d results, got %d", tt.minResults, len(results))
			}
		})
	}
}

func TestSearch_FilterByType(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	t.Run("only files", func(t *testing.T) {
		filter := &FileFilter{
			OnlyFiles: true,
		}

		results, err := searcher.Search("", filter, 100)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		for _, r := range results {
			if r.IsDir {
				t.Errorf("expected only files, but got directory: %s", r.Name)
			}
		}

		if len(results) != 5 { // 5 files in test data
			t.Errorf("expected 5 files, got %d", len(results))
		}
	})

	t.Run("only directories", func(t *testing.T) {
		filter := &FileFilter{
			OnlyDirs: true,
		}

		results, err := searcher.Search("", filter, 100)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		for _, r := range results {
			if !r.IsDir {
				t.Errorf("expected only directories, but got file: %s", r.Name)
			}
		}

		if len(results) != 3 { // 3 directories in test data
			t.Errorf("expected 3 directories, got %d", len(results))
		}
	})
}

func TestSearch_CombinedFilters(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	t.Run("query with size filter", func(t *testing.T) {
		filter := &FileFilter{
			MinSize: 1024 * 1024, // 1 MB
		}

		results, err := searcher.Search("images", filter, 100)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		// Should find photo.jpg (5MB) and screenshot.png (2MB) in images folder
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("extension with size filter", func(t *testing.T) {
		filter := &FileFilter{
			Exts:    []string{"jpg", "png"},
			MinSize: 3 * 1024 * 1024, // 3 MB
		}

		results, err := searcher.Search("", filter, 100)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		// Should only find photo.jpg (5MB)
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
		if len(results) > 0 && results[0].Name != "photo.jpg" {
			t.Errorf("expected photo.jpg, got %s", results[0].Name)
		}
	})

	t.Run("files only with extension filter", func(t *testing.T) {
		filter := &FileFilter{
			Exts:      []string{"pdf", "txt"},
			OnlyFiles: true,
		}

		results, err := searcher.Search("", filter, 100)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})
}

func TestSearch_EmptyQueryAndFilter(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	// Empty query and no filter should return no results
	results, err := searcher.Search("", nil, 100)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query and filter, got %d", len(results))
	}
}

func TestSearch_Limit(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	filter := &FileFilter{
		OnlyFiles: true,
	}

	results, err := searcher.Search("", filter, 2)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}
}

func TestGetFileByID(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	files := createTestFiles(t, db, "test-index")

	searcher := createSearcher(t, dbPath, "test-index")
	defer searcher.Close()

	t.Run("existing file", func(t *testing.T) {
		file, err := searcher.GetFileByID("test-index", files[0].ID)
		if err != nil {
			t.Fatalf("GetFileByID failed: %v", err)
		}

		if file == nil {
			t.Fatal("expected file, got nil")
		}

		if file.Name != files[0].Name {
			t.Errorf("expected name %s, got %s", files[0].Name, file.Name)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		file, err := searcher.GetFileByID("test-index", 99999)
		if err != nil {
			t.Fatalf("GetFileByID failed: %v", err)
		}

		if file != nil {
			t.Errorf("expected nil for non-existent file, got %+v", file)
		}
	})
}

func TestPrepareFTSQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "hello world",
			expected: "hello AND world",
		},
		{
			input:    "hello -world",
			expected: "hello NOT world",
		},
		{
			input:    "foo bar -baz",
			expected: "foo AND bar NOT baz",
		},
		{
			input:    "-excluded",
			expected: "NOT excluded",
		},
		{
			input:    "single",
			expected: "single",
		},
		{
			input:    "  spaced   terms  ",
			expected: "spaced AND terms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := prepareFTSQuery(tt.input)
			if result != tt.expected {
				t.Errorf("prepareFTSQuery(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
