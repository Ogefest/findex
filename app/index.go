package app

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ogefest/findex/models"
)

func InitIndexes(cfg *models.AppConfig) error {
	defined := map[string]bool{}

	for _, idx := range cfg.Indexes {
		if idx.SourceEngine != "local" {
			return fmt.Errorf("unsupported source_engine %q for index %s", idx.SourceEngine, idx.Name)
		}

		absDBPath, err := filepath.Abs(idx.DBPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", idx.DBPath, err)
		}
		defined[absDBPath] = true

		if err := ensureIndex(idx); err != nil {
			return fmt.Errorf("failed to init index %s: %w", idx.Name, err)
		}
	}

	dataDir, err := filepath.Abs("./data")
	if err != nil {
		return fmt.Errorf("failed to get absolute path for data dir: %w", err)
	}

	pattern := filepath.Join(dataDir, "*.db")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob data dir: %w", err)
	}

	for _, f := range files {
		absFile, err := filepath.Abs(f)
		if err != nil {
			return err
		}

		if !defined[absFile] {
			log.Printf("Removing unused index: %s\n", absFile)
			if err := os.Remove(absFile); err != nil {
				return err
			}
		}
	}

	return nil
}

func ensureIndex(idx models.IndexConfig) error {
	if err := os.MkdirAll(filepath.Dir(idx.DBPath), 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", idx.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	return RunMigrations(db)
}
