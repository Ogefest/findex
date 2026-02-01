package app

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// CalculateDirSizesBackground calculates directory sizes in the background
// after index scan completes. It opens a separate database connection with
// WAL mode to avoid blocking the web server.
func CalculateDirSizesBackground(dbPath, indexName string) error {
	log.Printf("Starting background directory size calculation for %s", indexName)

	db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return err
	}

	// Get all directories
	rows, err := db.Query(`SELECT path FROM files WHERE is_dir = 1`)
	if err != nil {
		return err
	}

	var dirs []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			rows.Close()
			return err
		}
		dirs = append(dirs, path)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return err
	}

	log.Printf("Calculating sizes for %d directories in %s", len(dirs), indexName)

	const batchSize = 500
	processed := 0

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO dir_sizes (path, total_size, file_count) VALUES (?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for i, dirPath := range dirs {
		var size, count int64
		err := db.QueryRow(`
			SELECT COALESCE(SUM(size), 0), COUNT(*)
			FROM files
			WHERE path LIKE ? AND is_dir = 0
		`, dirPath+"/%").Scan(&size, &count)
		if err != nil {
			tx.Rollback()
			return err
		}

		if _, err := stmt.Exec(dirPath, size, count); err != nil {
			tx.Rollback()
			return err
		}

		processed++

		// Commit batch and start new transaction
		if (i+1)%batchSize == 0 {
			if err := tx.Commit(); err != nil {
				return err
			}
			log.Printf("Processed %d/%d directories for %s", processed, len(dirs), indexName)

			// Small delay to allow web server concurrent access
			time.Sleep(10 * time.Millisecond)

			tx, err = db.Begin()
			if err != nil {
				return err
			}
			stmt, err = tx.Prepare(`INSERT OR REPLACE INTO dir_sizes (path, total_size, file_count) VALUES (?, ?, ?)`)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	// Commit remaining
	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("Background directory size calculation completed for %s: %d directories processed", indexName, processed)
	return nil
}
