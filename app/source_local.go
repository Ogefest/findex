package app

import (
	"hash/crc32"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ogefest/findex/models"
)

type LocalSource struct {
	IndexName    string
	RootPaths    []string
	ExcludePaths []string
}

func NewLocalSource(indexName string, rootPaths []string, excludePaths []string) *LocalSource {
	return &LocalSource{IndexName: indexName, RootPaths: rootPaths}
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
	ch := make(chan models.FileRecord)
	var wg sync.WaitGroup

	go func() {
		defer close(ch)

		for _, root := range l.RootPaths {
			wg.Add(1)
			go func(root string) {
				defer wg.Done()

				err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						log.Printf("Error accessing %s: %v\n", path, err)
						return nil
					}

					// sprawdzanie exclude
					for _, exclude := range l.ExcludePaths {
						matched, _ := filepath.Match(exclude, path)
						if matched || strings.HasPrefix(path, exclude) {
							if d.IsDir() {
								return filepath.SkipDir
							}
							return nil
						}
					}

					info, err := d.Info()
					if err != nil {
						return nil
					}
					relPath, err := filepath.Rel(root, path)
					if err != nil {
						relPath = path
					}

					ch <- models.FileRecord{
						Path:      relPath,
						Name:      d.Name(),
						Dir:       root,
						DirIndex:  int64(l.getDirDeep(relPath)),
						Ext:       filepath.Ext(d.Name()),
						Size:      info.Size(),
						ModTime:   info.ModTime(),
						IsDir:     d.IsDir(),
						IndexName: l.IndexName,
					}
					return nil
				})
				if err != nil {
					log.Printf("Error walking root %s: %v\n", root, err)
				}
			}(root)
		}

		wg.Wait()
	}()

	return ch
}
