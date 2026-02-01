package app

import (
	"database/sql"
	_ "embed"
	"log"

	_ "modernc.org/sqlite"
)

//go:embed init.sql
var initSQL string

func RunMigrations(db *sql.DB) error {
	_, err := db.Exec(initSQL)
	if err != nil {
		return err
	}
	log.Println("Migrations applied successfully")
	return nil
}
