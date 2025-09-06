package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/ogefest/findex/pkg/models"

	_ "modernc.org/sqlite"
)

func ScanIndexes(cfg *models.AppConfig) error {
	for _, idx := range cfg.Indexes {
		absDBPath, err := filepath.Abs(idx.DBPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for index %s: %w", idx.Name, err)
		}

		db, err := sql.Open("sqlite", absDBPath)
		if err != nil {
			return fmt.Errorf("failed to open db: %w", err)
		}
		lastScan, err := getLastScan(db)
		if err != nil {
			db.Close()
			return fmt.Errorf("failed to get last scan for index %s: %w", idx.Name, err)
		}
		if !lastScan.IsZero() && idx.RefreshInterval > 0 {
			nextScan := lastScan.Add(time.Duration(idx.RefreshInterval) * time.Second)
			if time.Now().Before(nextScan) {
				log.Printf("Skipping index %s, last scan at %s, refresh interval %d sec", idx.Name, lastScan.Format(time.RFC3339), idx.RefreshInterval)
				db.Close()
				continue
			}
		}

		var source FileSource

		switch idx.SourceEngine {
		case "local":
			source = NewLocalSource(idx.Name, idx.RootPaths)
		default:
			log.Printf("Skipping unsupported source_engine %s for index %s\n", idx.SourceEngine, idx.Name)
			db.Close()
			continue
		}

		log.Printf("Scanning index %s using %s engine\n", idx.Name, source.Name())

		if err := scanSource(context.Background(), db, source); err != nil {
			db.Close()
			return fmt.Errorf("failed to scan index %s: %w", idx.Name, err)
		}

		db.Close()
	}
	return nil
}

func scanSource(ctx context.Context, db *sql.DB, source FileSource) error {
	if err := resetSearchableFlag(db); err != nil {
		return err
	}

	log.Println("Scanning files...")

	count := 0
	batch := 1000 // logujemy co 100 plików
	var batchFiles []models.FileRecord

	for f := range source.Walk() {
		batchFiles = append(batchFiles, f)
		count++

		if len(batchFiles) >= batch {
			if err := upsertFilesBatch(ctx, db, batchFiles); err != nil {
				log.Printf("Failed to upsert batch: %v\n", err)
			}
			batchFiles = batchFiles[:0] // reset slice
			log.Printf("Scanned %d files...", count)
		}
	}
	if len(batchFiles) > 0 {
		if err := upsertFilesBatch(ctx, db, batchFiles); err != nil {
			log.Printf("Failed to upsert final batch: %v\n", err)
		}
	}

	log.Printf("Scanning completed. Total files scanned: %d", count)

	// 3. finalizacja indeksu
	if err := finalizeIndex(db); err != nil {
		return err
	}

	// 4. zapis ostatniego skanu
	if err := setLastScan(db); err != nil {
		return err
	}

	log.Println("Index finalized and metadata updated")
	return nil
}

func upsertFilesBatch(ctx context.Context, db *sql.DB, files []models.FileRecord) error {
	if len(files) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO files(path, name, dir, ext, size, mod_time, is_dir, is_searchable, index_name)
        VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
        ON CONFLICT(path) DO UPDATE SET
            name=excluded.name,
            dir=excluded.dir,
            ext=excluded.ext,
            size=excluded.size,
            mod_time=excluded.mod_time,
            is_dir=excluded.is_dir,
            is_searchable=1
    `)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range files {
		_, err = stmt.ExecContext(ctx,
			f.Path, f.Name, f.Dir, f.Ext, f.Size, f.ModTime.Unix(), boolToInt(f.IsDir), f.IndexName)
		if err != nil {
			return err
		}
	}

	return nil
}

func resetSearchableFlag(db *sql.DB) error {
	_, err := db.Exec(`UPDATE files SET is_searchable = 0`)
	return err
}

func finalizeIndex(db *sql.DB) error {
	// rekordy świeże (1) → finalne (2)
	if _, err := db.Exec(`UPDATE files SET is_searchable = 2 WHERE is_searchable = 1`); err != nil {
		return err
	}
	// rekordy nieaktualne (0) → usuwamy
	if _, err := db.Exec(`DELETE FROM files WHERE is_searchable = 0`); err != nil {
		return err
	}
	return nil
}

func setMetadata(db *sql.DB, key, value string) error {
	_, err := db.Exec(`
        INSERT INTO metadata(key, value)
        VALUES (?, ?)
        ON CONFLICT(key) DO UPDATE SET value=excluded.value
    `, key, value)
	return err
}

func setLastScan(db *sql.DB) error {
	now := time.Now().Format(time.RFC3339)
	return setMetadata(db, "last_scan", now)
}

func getLastScan(db *sql.DB) (time.Time, error) {
	var ts string
	err := db.QueryRow(`SELECT value FROM metadata WHERE key='last_scan'`).Scan(&ts)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, nil // brak rekordu = nigdy nie skanowano
		}
		return time.Time{}, err
	}

	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
