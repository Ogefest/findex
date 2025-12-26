package models

type FileSource interface {
	Name() string
	Walk() <-chan FileRecord
}
