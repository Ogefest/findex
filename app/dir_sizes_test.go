package app

import (
	"testing"
	"time"

	"github.com/ogefest/findex/models"
)

func TestCalculateDirSizesBackground(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	createTestFiles(t, db, "test-index")

	err := CalculateDirSizesBackground(dbPath, "test-index")
	if err != nil {
		t.Fatalf("CalculateDirSizesBackground failed: %v", err)
	}

	// Verify dir_sizes table has entries for all directories
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM dir_sizes`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count dir_sizes: %v", err)
	}

	if count != 3 { // documents, images, videos
		t.Errorf("expected 3 directories in dir_sizes, got %d", count)
	}

	// Verify specific directory sizes
	tests := []struct {
		path          string
		expectedSize  int64
		expectedCount int64
	}{
		{
			path:          "documents",
			expectedSize:  1024*1024 + 512, // report.pdf (1MB) + notes.txt (512B)
			expectedCount: 2,
		},
		{
			path:          "images",
			expectedSize:  5*1024*1024 + 2*1024*1024, // photo.jpg (5MB) + screenshot.png (2MB)
			expectedCount: 2,
		},
		{
			path:          "videos",
			expectedSize:  500 * 1024 * 1024, // movie.mp4 (500MB)
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var size, fileCount int64
			err := db.QueryRow(`SELECT total_size, file_count FROM dir_sizes WHERE path = ?`, tt.path).Scan(&size, &fileCount)
			if err != nil {
				t.Fatalf("failed to get dir_size for %s: %v", tt.path, err)
			}

			if size != tt.expectedSize {
				t.Errorf("expected size %d for %s, got %d", tt.expectedSize, tt.path, size)
			}

			if fileCount != tt.expectedCount {
				t.Errorf("expected file_count %d for %s, got %d", tt.expectedCount, tt.path, fileCount)
			}
		})
	}
}

func TestCalculateDirSizesBackground_NestedDirectories(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	// Create nested directory structure
	files := []models.FileRecord{
		{IndexName: "test-index", Path: "/root", Name: "root", IsDir: true, ModTime: now},
		{IndexName: "test-index", Path: "/root/sub1", Name: "sub1", Dir: "/root", IsDir: true, ModTime: now},
		{IndexName: "test-index", Path: "/root/sub2", Name: "sub2", Dir: "/root", IsDir: true, ModTime: now},
		{IndexName: "test-index", Path: "/root/sub1/deep", Name: "deep", Dir: "/root/sub1", IsDir: true, ModTime: now},
		{IndexName: "test-index", Path: "/root/file1.txt", Name: "file1.txt", Dir: "/root", Ext: ".txt", Size: 100, ModTime: now},
		{IndexName: "test-index", Path: "/root/sub1/file2.txt", Name: "file2.txt", Dir: "/root/sub1", Ext: ".txt", Size: 200, ModTime: now},
		{IndexName: "test-index", Path: "/root/sub1/deep/file3.txt", Name: "file3.txt", Dir: "/root/sub1/deep", Ext: ".txt", Size: 300, ModTime: now},
		{IndexName: "test-index", Path: "/root/sub2/file4.txt", Name: "file4.txt", Dir: "/root/sub2", Ext: ".txt", Size: 400, ModTime: now},
	}

	for _, f := range files {
		insertTestFile(t, db, f)
	}

	err := CalculateDirSizesBackground(dbPath, "test-index")
	if err != nil {
		t.Fatalf("CalculateDirSizesBackground failed: %v", err)
	}

	// Verify nested directory sizes
	tests := []struct {
		path          string
		expectedSize  int64
		expectedCount int64
	}{
		{
			path:          "/root",
			expectedSize:  100 + 200 + 300 + 400, // all files under /root
			expectedCount: 4,
		},
		{
			path:          "/root/sub1",
			expectedSize:  200 + 300, // file2.txt + file3.txt
			expectedCount: 2,
		},
		{
			path:          "/root/sub1/deep",
			expectedSize:  300, // file3.txt
			expectedCount: 1,
		},
		{
			path:          "/root/sub2",
			expectedSize:  400, // file4.txt
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var size, fileCount int64
			err := db.QueryRow(`SELECT total_size, file_count FROM dir_sizes WHERE path = ?`, tt.path).Scan(&size, &fileCount)
			if err != nil {
				t.Fatalf("failed to get dir_size for %s: %v", tt.path, err)
			}

			if size != tt.expectedSize {
				t.Errorf("expected size %d for %s, got %d", tt.expectedSize, tt.path, size)
			}

			if fileCount != tt.expectedCount {
				t.Errorf("expected file_count %d for %s, got %d", tt.expectedCount, tt.path, fileCount)
			}
		})
	}
}

func TestCalculateDirSizesBackground_EmptyDirectory(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	// Create empty directory
	insertTestFile(t, db, models.FileRecord{
		IndexName: "test-index",
		Path:      "/empty",
		Name:      "empty",
		IsDir:     true,
		ModTime:   now,
	})

	err := CalculateDirSizesBackground(dbPath, "test-index")
	if err != nil {
		t.Fatalf("CalculateDirSizesBackground failed: %v", err)
	}

	var size, fileCount int64
	err = db.QueryRow(`SELECT total_size, file_count FROM dir_sizes WHERE path = ?`, "/empty").Scan(&size, &fileCount)
	if err != nil {
		t.Fatalf("failed to get dir_size for /empty: %v", err)
	}

	if size != 0 {
		t.Errorf("expected size 0 for empty directory, got %d", size)
	}

	if fileCount != 0 {
		t.Errorf("expected file_count 0 for empty directory, got %d", fileCount)
	}
}

func TestCalculateDirSizesBackground_NoDirectories(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	// Create only files, no directories
	insertTestFile(t, db, models.FileRecord{
		IndexName: "test-index",
		Path:      "/file1.txt",
		Name:      "file1.txt",
		Ext:       ".txt",
		Size:      100,
		ModTime:   now,
	})

	err := CalculateDirSizesBackground(dbPath, "test-index")
	if err != nil {
		t.Fatalf("CalculateDirSizesBackground failed: %v", err)
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM dir_sizes`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count dir_sizes: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 entries in dir_sizes, got %d", count)
	}
}

func TestCalculateDirSizesBackground_LargeBatch(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	// Create more than 500 directories to test batching
	numDirs := 600
	for i := 0; i < numDirs; i++ {
		insertTestFile(t, db, models.FileRecord{
			IndexName: "test-index",
			Path:      "/dir" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Name:      "dir" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			IsDir:     true,
			ModTime:   now,
		})
	}

	err := CalculateDirSizesBackground(dbPath, "test-index")
	if err != nil {
		t.Fatalf("CalculateDirSizesBackground failed: %v", err)
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM dir_sizes`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count dir_sizes: %v", err)
	}

	if count != numDirs {
		t.Errorf("expected %d entries in dir_sizes, got %d", numDirs, count)
	}
}
