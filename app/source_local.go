package app

import (
	"hash/crc32"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ogefest/findex/models"
)

type LocalSource struct {
	IndexName    string
	RootPaths    []string
	ExcludePaths []string
	NumWorkers   int
}

func NewLocalSource(indexName string, rootPaths []string, excludePaths []string, numWorkers int) *LocalSource {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU() * 2
	}
	return &LocalSource{
		IndexName:    indexName,
		RootPaths:    rootPaths,
		ExcludePaths: excludePaths,
		NumWorkers:   numWorkers,
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
			l.walkRootParallel(root, filesCh)
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

		relPath, _ := filepath.Rel(root, path)

		filesCh <- models.FileRecord{
			Path:      relPath,
			Name:      entry.Name(),
			Dir:       root,
			DirIndex:  int64(l.getDirDeep(relPath)),
			Ext:       filepath.Ext(entry.Name()),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			IsDir:     entry.IsDir(),
			IndexName: l.IndexName,
		}
	}
}
