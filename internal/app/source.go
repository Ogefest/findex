package app

import "github.com/ogefest/findex/pkg/models"

// FileSource to abstrakcja źródła plików
type FileSource interface {
	Name() string
	Walk() <-chan models.FileRecord
}
