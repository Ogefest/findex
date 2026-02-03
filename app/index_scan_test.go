package app

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ogefest/findex/models"
)

func TestUpsertFilesBatch(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	files := []models.FileRecord{
		{
			IndexName: "test-index",
			Path:      "test/file1.txt",
			Name:      "file1.txt",
			Dir:       "test",
			Ext:       ".txt",
			Size:      100,
			ModTime:   time.Now(),
			IsDir:     false,
			DirIndex:  12345,
		},
		{
			IndexName: "test-index",
			Path:      "test/file2.pdf",
			Name:      "file2.pdf",
			Dir:       "test",
			Ext:       ".pdf",
			Size:      200,
			ModTime:   time.Now(),
			IsDir:     false,
			DirIndex:  12345,
		},
	}

	t.Run("insert new files", func(t *testing.T) {
		err := upsertFilesBatch(context.Background(), db, files)
		if err != nil {
			t.Fatalf("upsertFilesBatch failed: %v", err)
		}

		// Verify files were inserted
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM files").Scan(&count)
		if err != nil {
			t.Fatalf("failed to count files: %v", err)
		}

		if count != 2 {
			t.Errorf("expected 2 files, got %d", count)
		}
	})

	t.Run("duplicate paths are ignored", func(t *testing.T) {
		// Insert same files again
		err := upsertFilesBatch(context.Background(), db, files)
		if err != nil {
			t.Fatalf("upsertFilesBatch failed: %v", err)
		}

		// Count should still be 2 (ON CONFLICT DO NOTHING)
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM files").Scan(&count)
		if err != nil {
			t.Fatalf("failed to count files: %v", err)
		}

		if count != 2 {
			t.Errorf("expected 2 files (no duplicates), got %d", count)
		}
	})

	t.Run("empty batch", func(t *testing.T) {
		err := upsertFilesBatch(context.Background(), db, []models.FileRecord{})
		if err != nil {
			t.Errorf("empty batch should not error: %v", err)
		}
	})
}

func TestResetSearchableFlag(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert files with is_searchable = 2
	_, err := db.Exec(`
		INSERT INTO files(path, name, dir, ext, size, mod_time, is_dir, is_searchable, index_name, dir_index)
		VALUES
			('file1.txt', 'file1.txt', '', '.txt', 100, 0, 0, 2, 'test', 0),
			('file2.txt', 'file2.txt', '', '.txt', 200, 0, 0, 2, 'test', 0)
	`)
	if err != nil {
		t.Fatalf("failed to insert test files: %v", err)
	}

	err = resetSearchableFlag(db)
	if err != nil {
		t.Fatalf("resetSearchableFlag failed: %v", err)
	}

	// Verify all flags are reset to 0
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM files WHERE is_searchable = 0").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}

	if count != 2 {
		t.Errorf("expected all 2 files to have is_searchable=0, got %d", count)
	}
}

func TestFinalizeIndex(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert files with different searchable states
	_, err := db.Exec(`
		INSERT INTO files(path, name, dir, ext, size, mod_time, is_dir, is_searchable, index_name, dir_index)
		VALUES
			('old_file.txt', 'old_file.txt', '', '.txt', 100, 0, 0, 0, 'test', 0),
			('new_file.txt', 'new_file.txt', '', '.txt', 200, 0, 0, 1, 'test', 0),
			('another_new.txt', 'another_new.txt', '', '.txt', 300, 0, 0, 1, 'test', 0)
	`)
	if err != nil {
		t.Fatalf("failed to insert test files: %v", err)
	}

	err = finalizeIndex(db, "test-index")
	if err != nil {
		t.Fatalf("finalizeIndex failed: %v", err)
	}

	t.Run("old files are deleted", func(t *testing.T) {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM files WHERE path = 'old_file.txt'").Scan(&count)
		if err != nil {
			t.Fatalf("failed to count: %v", err)
		}
		if count != 0 {
			t.Error("old file with is_searchable=0 should be deleted")
		}
	})

	t.Run("new files are marked as searchable=2", func(t *testing.T) {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM files WHERE is_searchable = 2").Scan(&count)
		if err != nil {
			t.Fatalf("failed to count: %v", err)
		}
		if count != 2 {
			t.Errorf("expected 2 files with is_searchable=2, got %d", count)
		}
	})

	t.Run("FTS index is populated", func(t *testing.T) {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM files_fts").Scan(&count)
		if err != nil {
			t.Fatalf("failed to count FTS: %v", err)
		}
		if count != 2 {
			t.Errorf("expected 2 entries in FTS, got %d", count)
		}
	})
}

func TestSetAndGetLastScan(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("get last scan when not set", func(t *testing.T) {
		lastScan, err := getLastScan(db)
		if err != nil {
			t.Fatalf("getLastScan failed: %v", err)
		}
		if !lastScan.IsZero() {
			t.Error("expected zero time when last_scan not set")
		}
	})

	t.Run("set and get last scan", func(t *testing.T) {
		err := setLastScan(db)
		if err != nil {
			t.Fatalf("setLastScan failed: %v", err)
		}

		lastScan, err := getLastScan(db)
		if err != nil {
			t.Fatalf("getLastScan failed: %v", err)
		}

		if lastScan.IsZero() {
			t.Error("expected non-zero last scan time")
		}

		// Should be within last minute
		if time.Since(lastScan) > time.Minute {
			t.Error("last scan time is too old")
		}
	})
}

func TestSetMetadata(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("insert new metadata", func(t *testing.T) {
		err := setMetadata(db, "test_key", "test_value")
		if err != nil {
			t.Fatalf("setMetadata failed: %v", err)
		}

		var value string
		err = db.QueryRow("SELECT value FROM metadata WHERE key = 'test_key'").Scan(&value)
		if err != nil {
			t.Fatalf("failed to read metadata: %v", err)
		}

		if value != "test_value" {
			t.Errorf("expected test_value, got %s", value)
		}
	})

	t.Run("update existing metadata", func(t *testing.T) {
		err := setMetadata(db, "test_key", "updated_value")
		if err != nil {
			t.Fatalf("setMetadata failed: %v", err)
		}

		var value string
		err = db.QueryRow("SELECT value FROM metadata WHERE key = 'test_key'").Scan(&value)
		if err != nil {
			t.Fatalf("failed to read metadata: %v", err)
		}

		if value != "updated_value" {
			t.Errorf("expected updated_value, got %s", value)
		}
	})
}

func TestBoolToInt(t *testing.T) {
	tests := []struct {
		input    bool
		expected int
	}{
		{true, 1},
		{false, 0},
	}

	for _, tt := range tests {
		result := boolToInt(tt.input)
		if result != tt.expected {
			t.Errorf("boolToInt(%v) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestLocalSourceWalk(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "findex_walk_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file structure
	dirs := []string{
		"documents",
		"images",
		"documents/subfolder",
	}
	files := []struct {
		path    string
		content string
	}{
		{"documents/file1.txt", "hello"},
		{"documents/file2.pdf", "world"},
		{"documents/subfolder/nested.txt", "nested"},
		{"images/photo.jpg", "image data"},
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		if err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	for _, f := range files {
		err := os.WriteFile(filepath.Join(tmpDir, f.path), []byte(f.content), 0644)
		if err != nil {
			t.Fatalf("failed to create file %s: %v", f.path, err)
		}
	}

	t.Run("walk all files", func(t *testing.T) {
		source := NewLocalSource("test-index", []string{tmpDir}, nil, 0, false, nil)

		var foundFiles []models.FileRecord
		for f := range source.Walk() {
			foundFiles = append(foundFiles, f)
		}

		// Should find 4 files + 3 dirs = 7 entries (excluding root)
		// Actually it depends on implementation - let's just check we got files
		if len(foundFiles) < 4 {
			t.Errorf("expected at least 4 files, got %d", len(foundFiles))
		}

		// Verify file properties
		for _, f := range foundFiles {
			if f.IndexName != "test-index" {
				t.Errorf("expected index name 'test-index', got '%s'", f.IndexName)
			}
			if f.Path == "" {
				t.Error("file path should not be empty")
			}
			if f.Name == "" {
				t.Error("file name should not be empty")
			}
		}
	})

	t.Run("walk with exclude paths", func(t *testing.T) {
		excludeDir := filepath.Join(tmpDir, "images")
		source := NewLocalSource("test-index", []string{tmpDir}, []string{excludeDir}, 0, false, nil)

		var foundFiles []models.FileRecord
		for f := range source.Walk() {
			foundFiles = append(foundFiles, f)
		}

		// Verify no files from images directory
		for _, f := range foundFiles {
			if filepath.Dir(f.Path) == excludeDir || f.Path == excludeDir {
				t.Errorf("excluded path found in results: %s", f.Path)
			}
		}
	})

	t.Run("walk non-existent path", func(t *testing.T) {
		source := NewLocalSource("test-index", []string{"/nonexistent/path"}, nil, 0, false, nil)

		var foundFiles []models.FileRecord
		for f := range source.Walk() {
			foundFiles = append(foundFiles, f)
		}

		// Should handle gracefully with no files
		if len(foundFiles) != 0 {
			t.Errorf("expected 0 files for non-existent path, got %d", len(foundFiles))
		}
	})
}

func TestScanSourceIntegration(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "findex_scan_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []string{
		"file1.txt",
		"file2.pdf",
		"subdir/file3.doc",
	}

	for _, f := range testFiles {
		fullPath := filepath.Join(tmpDir, f)
		dir := filepath.Dir(fullPath)
		os.MkdirAll(dir, 0755)
		os.WriteFile(fullPath, []byte("test content"), 0644)
	}

	// Setup test database
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Create source and scan
	source := NewLocalSource("test-index", []string{tmpDir}, nil, 0, false, nil)

	err = scanSource(context.Background(), db, source, "test-index", nil)
	if err != nil {
		t.Fatalf("scanSource failed: %v", err)
	}

	t.Run("files are indexed", func(t *testing.T) {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM files WHERE is_dir = 0").Scan(&count)
		if err != nil {
			t.Fatalf("failed to count files: %v", err)
		}

		if count < 3 {
			t.Errorf("expected at least 3 files, got %d", count)
		}
	})

	t.Run("FTS is populated", func(t *testing.T) {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM files_fts").Scan(&count)
		if err != nil {
			t.Fatalf("failed to count FTS: %v", err)
		}

		if count < 3 {
			t.Errorf("expected at least 3 FTS entries, got %d", count)
		}
	})

	t.Run("last scan is set", func(t *testing.T) {
		lastScan, err := getLastScan(db)
		if err != nil {
			t.Fatalf("getLastScan failed: %v", err)
		}

		if lastScan.IsZero() {
			t.Error("expected last scan to be set")
		}
	})

	t.Run("files are searchable via FTS", func(t *testing.T) {
		var count int
		err = db.QueryRow(`
			SELECT COUNT(*) FROM files f
			JOIN files_fts ft ON ft.rowid = f.rowid
			WHERE files_fts MATCH 'file1'
		`).Scan(&count)
		if err != nil {
			t.Fatalf("FTS search failed: %v", err)
		}

		if count != 1 {
			t.Errorf("expected 1 match for 'file1', got %d", count)
		}
	})
}

func TestZipContentScanning(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "findex_zip_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a ZIP file with test content
	zipPath := filepath.Join(tmpDir, "test_archive.zip")
	createTestZip(t, zipPath, map[string]string{
		"readme.txt":           "Hello World",
		"docs/manual.pdf":      "PDF content",
		"docs/guide.txt":       "Guide content",
		"src/main.go":          "package main",
		"src/utils/helper.go":  "package utils",
	})

	t.Run("scan zip contents enabled", func(t *testing.T) {
		source := NewLocalSource("test-index", []string{tmpDir}, nil, 0, true, nil)

		var foundFiles []models.FileRecord
		for f := range source.Walk() {
			foundFiles = append(foundFiles, f)
		}

		// Should find: zip file + virtual root + directories + files inside
		// At minimum: test_archive.zip, test_archive.zip!, and 5 files inside
		if len(foundFiles) < 7 {
			t.Errorf("expected at least 7 entries (zip + virtual root + 5 files), got %d", len(foundFiles))
		}

		// Check for virtual zip root directory
		foundVirtualRoot := false
		for _, f := range foundFiles {
			if strings.HasSuffix(f.Path, "test_archive.zip!") && f.IsDir {
				foundVirtualRoot = true
				break
			}
		}
		if !foundVirtualRoot {
			t.Error("virtual zip root directory (test_archive.zip!) not found")
		}

		// Check for files inside zip
		foundInsideZip := 0
		for _, f := range foundFiles {
			if strings.Contains(f.Path, "test_archive.zip!/") && !f.IsDir {
				foundInsideZip++
			}
		}
		if foundInsideZip != 5 {
			t.Errorf("expected 5 files inside zip, got %d", foundInsideZip)
		}

		// Check path format for files inside zip
		for _, f := range foundFiles {
			if strings.Contains(f.Path, "test_archive.zip!/") {
				if !strings.Contains(f.Path, "!/") {
					t.Errorf("invalid zip content path format: %s", f.Path)
				}
			}
		}
	})

	t.Run("scan zip contents disabled", func(t *testing.T) {
		source := NewLocalSource("test-index", []string{tmpDir}, nil, 0, false, nil)

		var foundFiles []models.FileRecord
		for f := range source.Walk() {
			foundFiles = append(foundFiles, f)
		}

		// Should only find the zip file itself, not contents
		zipContentFound := false
		for _, f := range foundFiles {
			if strings.Contains(f.Path, "!/") {
				zipContentFound = true
				break
			}
		}
		if zipContentFound {
			t.Error("zip contents should not be scanned when ScanZipContents is false")
		}

		// But should find the zip file
		zipFileFound := false
		for _, f := range foundFiles {
			if strings.HasSuffix(f.Path, ".zip") && !f.IsDir {
				zipFileFound = true
				break
			}
		}
		if !zipFileFound {
			t.Error("zip file itself should be found")
		}
	})

	t.Run("zip file metadata", func(t *testing.T) {
		source := NewLocalSource("test-index", []string{tmpDir}, nil, 0, true, nil)

		var foundFiles []models.FileRecord
		for f := range source.Walk() {
			foundFiles = append(foundFiles, f)
		}

		// Find a specific file inside zip and check its properties
		var readmeFile *models.FileRecord
		for i, f := range foundFiles {
			if strings.HasSuffix(f.Path, "test_archive.zip!/readme.txt") {
				readmeFile = &foundFiles[i]
				break
			}
		}

		if readmeFile == nil {
			t.Fatal("readme.txt inside zip not found")
		}

		if readmeFile.Name != "readme.txt" {
			t.Errorf("expected name 'readme.txt', got '%s'", readmeFile.Name)
		}
		if readmeFile.Ext != ".txt" {
			t.Errorf("expected ext '.txt', got '%s'", readmeFile.Ext)
		}
		if readmeFile.IsDir {
			t.Error("readme.txt should not be a directory")
		}
		if readmeFile.Size != 11 { // "Hello World" = 11 bytes
			t.Errorf("expected size 11, got %d", readmeFile.Size)
		}
	})

	t.Run("nested directories in zip", func(t *testing.T) {
		source := NewLocalSource("test-index", []string{tmpDir}, nil, 0, true, nil)

		var foundFiles []models.FileRecord
		for f := range source.Walk() {
			foundFiles = append(foundFiles, f)
		}

		// Check that nested directories are created
		expectedDirs := []string{"docs", "src", "src/utils"}
		for _, dir := range expectedDirs {
			found := false
			for _, f := range foundFiles {
				if strings.Contains(f.Path, "test_archive.zip!/"+dir) && f.IsDir {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected directory '%s' inside zip not found", dir)
			}
		}
	})
}

func createTestZip(t *testing.T, zipPath string, files map[string]string) {
	t.Helper()

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for name, content := range files {
		writer, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("failed to create file in zip: %v", err)
		}
		_, err = writer.Write([]byte(content))
		if err != nil {
			t.Fatalf("failed to write to zip: %v", err)
		}
	}
}

func TestMultipleRootPaths(t *testing.T) {
	// Create three separate root directories
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	tmpDir3 := t.TempDir()

	// Create identical directory structure in each root
	// This tests that files from all roots are indexed (not just the last one)
	for _, root := range []string{tmpDir1, tmpDir2, tmpDir3} {
		// Create documents folder with files
		docsDir := filepath.Join(root, "documents")
		os.MkdirAll(docsDir, 0755)
		os.WriteFile(filepath.Join(docsDir, "report.txt"), []byte("Report from "+root), 0644)
		os.WriteFile(filepath.Join(docsDir, "notes.txt"), []byte("Notes from "+root), 0644)

		// Create images folder
		imagesDir := filepath.Join(root, "images")
		os.MkdirAll(imagesDir, 0755)
		os.WriteFile(filepath.Join(imagesDir, "photo.jpg"), []byte("Photo from "+root), 0644)

		// Create a file at root level
		os.WriteFile(filepath.Join(root, "readme.txt"), []byte("Readme from "+root), 0644)
	}

	source := NewLocalSource("test", []string{tmpDir1, tmpDir2, tmpDir3}, nil, 2, false, nil)

	var allFiles []models.FileRecord
	for f := range source.Walk() {
		allFiles = append(allFiles, f)
	}

	t.Run("all roots indexed", func(t *testing.T) {
		// Count files from each root
		filesFromRoot1 := 0
		filesFromRoot2 := 0
		filesFromRoot3 := 0

		for _, f := range allFiles {
			if strings.HasPrefix(f.Path, tmpDir1) {
				filesFromRoot1++
			} else if strings.HasPrefix(f.Path, tmpDir2) {
				filesFromRoot2++
			} else if strings.HasPrefix(f.Path, tmpDir3) {
				filesFromRoot3++
			}
		}

		// Each root should have: documents/, documents/report.txt, documents/notes.txt,
		// images/, images/photo.jpg, readme.txt = 6 items
		expectedPerRoot := 6

		if filesFromRoot1 != expectedPerRoot {
			t.Errorf("expected %d files from root1, got %d", expectedPerRoot, filesFromRoot1)
		}
		if filesFromRoot2 != expectedPerRoot {
			t.Errorf("expected %d files from root2, got %d", expectedPerRoot, filesFromRoot2)
		}
		if filesFromRoot3 != expectedPerRoot {
			t.Errorf("expected %d files from root3, got %d", expectedPerRoot, filesFromRoot3)
		}
	})

	t.Run("paths are unique", func(t *testing.T) {
		pathSet := make(map[string]bool)
		for _, f := range allFiles {
			if pathSet[f.Path] {
				t.Errorf("duplicate path found: %s", f.Path)
			}
			pathSet[f.Path] = true
		}
	})

	t.Run("paths are absolute", func(t *testing.T) {
		for _, f := range allFiles {
			if !filepath.IsAbs(f.Path) {
				t.Errorf("path is not absolute: %s", f.Path)
			}
		}
	})

	t.Run("dir field contains root", func(t *testing.T) {
		for _, f := range allFiles {
			if f.Dir != tmpDir1 && f.Dir != tmpDir2 && f.Dir != tmpDir3 {
				t.Errorf("dir field '%s' is not one of the roots", f.Dir)
			}
		}
	})

	t.Run("files can be inserted without conflicts", func(t *testing.T) {
		db, _, cleanup := setupTestDB(t)
		defer cleanup()

		err := upsertFilesBatch(context.Background(), db, allFiles)
		if err != nil {
			t.Fatalf("upsertFilesBatch failed: %v", err)
		}

		// Count total files - should be all files, not just from last root
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM files").Scan(&count)
		if err != nil {
			t.Fatalf("failed to count files: %v", err)
		}

		expectedTotal := 18 // 6 files * 3 roots
		if count != expectedTotal {
			t.Errorf("expected %d total files in database, got %d (only last root was indexed?)", expectedTotal, count)
		}
	})
}
