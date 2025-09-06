package app

import (
	"log"
	"os"
	"path/filepath"

	"github.com/ogefest/findex/pkg/models"
)

type LocalSource struct {
	RootPaths []string
}

func NewLocalSource(rootPaths []string) *LocalSource {
	return &LocalSource{RootPaths: rootPaths}
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

				ch <- models.FileRecord{
					Path:    path,
					Name:    d.Name(),
					Dir:     filepath.Dir(path),
					Ext:     filepath.Ext(d.Name()),
					Size:    info.Size(),
					ModTime: info.ModTime(),
					IsDir:   d.IsDir(),
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
