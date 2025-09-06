package models

// FileSource to abstrakcja źródła plików
type FileSource interface {
	Name() string
	Walk() <-chan FileRecord
}
