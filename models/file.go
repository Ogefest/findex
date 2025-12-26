package models

import "time"

type FileRecord struct {
	ID        int64     `db:"id"`
	IndexName string    `db:"index_name"`
	Path      string    `db:"path"`
	Name      string    `db:"name"`
	Dir       string    `db:"dir"`
	DirIndex  int64     `db:"dir_index"`
	Ext       string    `db:"ext"`
	Size      int64     `db:"size"`
	ModTime   time.Time `db:"mod_time"`
	IsDir     bool      `db:"is_dir"`
	Checksum  string    `db:"checksum"`
	MetaJSON  string    `db:"meta_json"`
}
