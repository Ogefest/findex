package app

import (
	"log"
	"os"
	"path/filepath"

	"github.com/ogefest/findex/pkg/models"
)

type LocalSource struct {
	IndexName string
	RootPaths []string
}

func NewLocalSource(indexName string, rootPaths []string) *LocalSource {
	return &LocalSource{IndexName: indexName, RootPaths: rootPaths}
}

func (l *LocalSource) Name() string {
	return "local"
}

func (l *LocalSource) Walk() <-chan models.FileRecord {
	ch := make(chan models.FileRecord)

	go func() {
		defer close(ch)
		for _, root := range l.RootPaths {
			err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					log.Printf("Error accessing %s: %v\n", path, err)
					return nil
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
		}
	}()

	return ch
}
