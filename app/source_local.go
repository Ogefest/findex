package app

import (
	"archive/zip"
	"hash/crc32"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ogefest/findex/models"
)

type LocalSource struct {
	IndexName       string
	RootPaths       []string
	ExcludePaths    []string
	NumWorkers      int
	ScanZipContents bool
}

func NewLocalSource(indexName string, rootPaths []string, excludePaths []string, numWorkers int, scanZipContents bool) *LocalSource {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU() * 2
	}
	return &LocalSource{
		IndexName:       indexName,
		RootPaths:       rootPaths,
		ExcludePaths:    excludePaths,
		NumWorkers:      numWorkers,
		ScanZipContents: scanZipContents,
	}
}

func (l *LocalSource) getDirDeep(path string) uint32 {
	dir := filepath.Dir(path)
	normalized := filepath.Clean(dir)
	result := crc32.ChecksumIEEE([]byte(normalized))
	return result
}

func (l *LocalSource) Name() string {
	return "local"
}

func (l *LocalSource) Walk() <-chan models.FileRecord {
	filesCh := make(chan models.FileRecord, 10000)

	go func() {
		defer close(filesCh)

		for _, root := range l.RootPaths {
			// Normalize root path for consistency
			cleanRoot := filepath.Clean(root)
			l.walkRootParallel(cleanRoot, filesCh)
		}
	}()

	return filesCh
}

func (l *LocalSource) walkRootParallel(root string, filesCh chan<- models.FileRecord) {
	dirQueue := make(chan string, 100000)
	var wg sync.WaitGroup
	var activeWorkers int32

	// Initialize - add root to queue
	dirQueue <- root
	atomic.AddInt32(&activeWorkers, 1)

	// Start workers
	for i := 0; i < l.NumWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.dirWorker(root, dirQueue, filesCh, &activeWorkers)
		}()
	}

	wg.Wait()
}

func (l *LocalSource) dirWorker(
	root string,
	dirQueue chan string,
	filesCh chan<- models.FileRecord,
	activeWorkers *int32,
) {
	for {
		select {
		case dir, ok := <-dirQueue:
			if !ok {
				return
			}
			l.processDirectory(root, dir, dirQueue, filesCh, activeWorkers)

			// Decrease active counter
			if atomic.AddInt32(activeWorkers, -1) == 0 {
				// Last worker - close queue
				close(dirQueue)
				return
			}
		}
	}
}

func (l *LocalSource) processDirectory(
	root, dir string,
	dirQueue chan string,
	filesCh chan<- models.FileRecord,
	activeWorkers *int32,
) {
	// Check exclude for directory
	for _, exclude := range l.ExcludePaths {
		if matched, _ := filepath.Match(exclude, dir); matched {
			return
		}
		if strings.HasPrefix(dir, exclude) {
			return
		}
	}

	// Open directory and read without sorting
	f, err := os.Open(dir)
	if err != nil {
		log.Printf("Error opening %s: %v", dir, err)
		return
	}

	entries, err := f.ReadDir(-1) // -1 = all entries
	f.Close()
	if err != nil {
		log.Printf("Error reading %s: %v", dir, err)
		return
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		// Check exclude for file/subdirectory
		excluded := false
		for _, exclude := range l.ExcludePaths {
			if matched, _ := filepath.Match(exclude, path); matched {
				excluded = true
				break
			}
			if strings.HasPrefix(path, exclude) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		if entry.IsDir() {
			// Add subdirectory to queue (non-blocking to avoid deadlock)
			atomic.AddInt32(activeWorkers, 1)
			select {
			case dirQueue <- path:
				// Successfully queued
			default:
				// Queue full - process synchronously to avoid deadlock
				atomic.AddInt32(activeWorkers, -1)
				l.processDirectory(root, path, dirQueue, filesCh, activeWorkers)
			}
		}

		// Send file/directory to channel
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Use full absolute path for uniqueness across multiple root_paths
		filesCh <- models.FileRecord{
			Path:      path,
			Name:      entry.Name(),
			Dir:       root,
			DirIndex:  int64(l.getDirDeep(path)),
			Ext:       filepath.Ext(entry.Name()),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			IsDir:     entry.IsDir(),
			IndexName: l.IndexName,
		}

		// Scan inside zip files if enabled
		if l.ScanZipContents && !entry.IsDir() && strings.ToLower(filepath.Ext(entry.Name())) == ".zip" {
			log.Printf("Scanning zip contents: %s", path)
			l.scanZipContents(path, root, filesCh)
		}
	}
}

func (l *LocalSource) scanZipContents(zipPath, root string, filesCh chan<- models.FileRecord) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		log.Printf("Error opening zip %s: %v", zipPath, err)
		return
	}
	defer reader.Close()

	// Track directories we've already added
	addedDirs := make(map[string]bool)

	// Add virtual root directory for zip contents (archive.zip!)
	// Use full path: /path/to/archive.zip!
	zipRootPath := zipPath + "!"
	filesCh <- models.FileRecord{
		Path:      zipRootPath,
		Name:      filepath.Base(zipPath) + "!",
		Dir:       root,
		DirIndex:  int64(l.getDirDeep(zipRootPath)),
		Ext:       "",
		Size:      0,
		ModTime:   time.Time{},
		IsDir:     true,
		IndexName: l.IndexName,
	}
	addedDirs[zipRootPath] = true

	// Helper to add a directory and all parent directories
	addDir := func(dirPath string) {
		parts := strings.Split(dirPath, "/")
		current := ""
		for _, part := range parts {
			if part == "" {
				continue
			}
			if current == "" {
				current = part
			} else {
				current = current + "/" + part
			}

			fullPath := zipPath + "!/" + current
			if addedDirs[fullPath] {
				continue
			}
			addedDirs[fullPath] = true

			filesCh <- models.FileRecord{
				Path:      fullPath,
				Name:      part,
				Dir:       root,
				DirIndex:  int64(l.getDirDeep(fullPath)),
				Ext:       "",
				Size:      0,
				ModTime:   time.Time{},
				IsDir:     true,
				IndexName: l.IndexName,
			}
		}
	}

	for _, file := range reader.File {
		// Path format: /full/path/to/archive.zip!/internal/path/file.txt
		innerPath := zipPath + "!/" + file.Name

		// Remove trailing slash for directories
		innerPath = strings.TrimSuffix(innerPath, "/")
		name := filepath.Base(file.Name)
		if name == "" || name == "." {
			continue
		}

		// Add parent directories first
		parentDir := filepath.Dir(file.Name)
		if parentDir != "." && parentDir != "" {
			addDir(parentDir)
		}

		if file.FileInfo().IsDir() {
			// Add the directory itself
			if !addedDirs[innerPath] {
				addedDirs[innerPath] = true
				filesCh <- models.FileRecord{
					Path:      innerPath,
					Name:      name,
					Dir:       root,
					DirIndex:  int64(l.getDirDeep(innerPath)),
					Ext:       "",
					Size:      0,
					ModTime:   file.Modified,
					IsDir:     true,
					IndexName: l.IndexName,
				}
			}
			continue
		}

		filesCh <- models.FileRecord{
			Path:      innerPath,
			Name:      name,
			Dir:       root,
			DirIndex:  int64(l.getDirDeep(innerPath)),
			Ext:       filepath.Ext(name),
			Size:      int64(file.UncompressedSize64),
			ModTime:   file.Modified,
			IsDir:     false,
			IndexName: l.IndexName,
		}
	}
}
