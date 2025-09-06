package app

import (
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func RunMigrations(db *sql.DB, migrationsPath string) error {
	content, err := os.ReadFile(migrationsPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(string(content))
	if err != nil {
		return err
	}
	log.Println("Migrations applied successfully")
	return nil
}
