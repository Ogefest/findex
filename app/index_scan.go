package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ogefest/findex/models"

	_ "modernc.org/sqlite"
)

func ScanIndexes(cfg *models.AppConfig, migrationsPath string) error {
	for _, idx := range cfg.Indexes {
		absDBPath, err := filepath.Abs(idx.DBPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for index %s: %w", idx.Name, err)
		}

		// Check refresh interval using main database
		mainDB, err := sql.Open("sqlite", absDBPath)
		if err != nil {
			return fmt.Errorf("failed to open db: %w", err)
		}

		lastScan, err := getLastScan(mainDB)
		if err != nil {
			mainDB.Close()
			return fmt.Errorf("failed to get last scan for index %s: %w", idx.Name, err)
		}
		mainDB.Close()

		if !lastScan.IsZero() && idx.RefreshInterval > 0 {
			nextScan := lastScan.Add(time.Duration(idx.RefreshInterval) * time.Second)
			if time.Now().Before(nextScan) {
				log.Printf("Skipping index %s, last scan at %s, refresh interval %d sec", idx.Name, lastScan.Format(time.RFC3339), idx.RefreshInterval)
				continue
			}
		}

		var source models.FileSource

		switch idx.SourceEngine {
		case "local":
			source = NewLocalSource(idx.Name, idx.RootPaths, idx.ExcludePaths, idx.ScanWorkers, idx.ScanZipContents)
		default:
			log.Printf("Skipping unsupported source_engine %s for index %s\n", idx.SourceEngine, idx.Name)
			continue
		}

		log.Printf("Scanning index %s using %s engine (scan_zip_contents=%v)\n", idx.Name, source.Name(), idx.ScanZipContents)

		// Atomic database swap: scan into temp DB, then rename
		tempDBPath := absDBPath + ".new"

		// Clean up any leftover temp files from previous crash
		os.Remove(tempDBPath)
		os.Remove(tempDBPath + "-wal")
		os.Remove(tempDBPath + "-shm")

		// Initialize temp database with schema
		tempDB, err := initTempDB(tempDBPath, migrationsPath)
		if err != nil {
			return fmt.Errorf("failed to init temp db for index %s: %w", idx.Name, err)
		}

		if err := scanSource(context.Background(), tempDB, source, idx.Name); err != nil {
			tempDB.Close()
			// Clean up temp files on error
			os.Remove(tempDBPath)
			os.Remove(tempDBPath + "-wal")
			os.Remove(tempDBPath + "-shm")
			return fmt.Errorf("failed to scan index %s: %w", idx.Name, err)
		}

		// WAL checkpoint before rename to ensure all data is in main file
		log.Println("Checkpointing WAL...")
		if _, err := tempDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			tempDB.Close()
			os.Remove(tempDBPath)
			os.Remove(tempDBPath + "-wal")
			os.Remove(tempDBPath + "-shm")
			return fmt.Errorf("failed to checkpoint temp db for index %s: %w", idx.Name, err)
		}
		tempDB.Close()

		// Atomic rename: replace main database with temp database
		log.Println("Swapping database...")
		if err := os.Rename(tempDBPath, absDBPath); err != nil {
			os.Remove(tempDBPath)
			os.Remove(tempDBPath + "-wal")
			os.Remove(tempDBPath + "-shm")
			return fmt.Errorf("failed to rename temp db for index %s: %w", idx.Name, err)
		}

		// Clean up any leftover WAL/SHM files from temp
		os.Remove(tempDBPath + "-wal")
		os.Remove(tempDBPath + "-shm")

		log.Printf("Index %s scan completed and atomically swapped\n", idx.Name)

		// Start background goroutine to calculate directory sizes
		go func(dbPath string, indexName string) {
			if err := CalculateDirSizesBackground(dbPath, indexName); err != nil {
				log.Printf("Warning: background dir size calculation failed for %s: %v", indexName, err)
			}
		}(absDBPath, idx.Name)
	}
	return nil
}

func initTempDB(tempPath, migrationsPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", tempPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set journal_mode = WAL: %w", err)
	}
	if err := RunMigrations(db, migrationsPath); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	return db, nil
}

func scanSource(ctx context.Context, db *sql.DB, source models.FileSource, indexName string) error {
	if err := resetSearchableFlag(db); err != nil {
		return err
	}

	log.Println("Scanning files...")

	count := 0
	batch := 100000
	var batchFiles []models.FileRecord

	for f := range source.Walk() {
		batchFiles = append(batchFiles, f)
		count++

		if len(batchFiles) >= batch {
			log.Printf("Inserting batch of %d files...", len(batchFiles))
			if err := upsertFilesBatch(ctx, db, batchFiles); err != nil {
				return fmt.Errorf("failed to upsert batch at %d files: %w", count, err)
			}
			batchFiles = batchFiles[:0]
			log.Printf("Saved %d files to database", count)
		}
	}
	if len(batchFiles) > 0 {
		log.Printf("Inserting final batch of %d files...", len(batchFiles))
		if err := upsertFilesBatch(ctx, db, batchFiles); err != nil {
			return fmt.Errorf("failed to upsert final batch: %w", err)
		}
	}

	log.Printf("Scanning completed. Total files scanned: %d", count)
	log.Println("Finalizing index (this may take a while)...")
	if err := finalizeIndex(db, indexName); err != nil {
		return err
	}

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
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO files(path, name, dir, ext, size, mod_time, is_dir, is_searchable, index_name, dir_index)
        VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(path) DO NOTHING;
    `)
	if err != nil {
		return err
	}
	defer stmt.Close()

	progressInterval := 25000
	for i, f := range files {
		_, err = stmt.ExecContext(ctx,
			f.Path, f.Name, f.Dir, f.Ext, f.Size, f.ModTime.Unix(), boolToInt(f.IsDir), f.IndexName, f.DirIndex)
		if err != nil {
			return err
		}
		if (i+1)%progressInterval == 0 {
			log.Printf("  Inserted %d/%d files...", i+1, len(files))
		}
	}

	log.Println("  Committing transaction...")
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true

	return nil
}

func resetSearchableFlag(db *sql.DB) error {
	_, err := db.Exec(`UPDATE files SET is_searchable = 0`)
	return err
}

func finalizeIndex(db *sql.DB, indexName string) error {
	log.Println("  Marking files as searchable...")
	if _, err := db.Exec(`UPDATE files SET is_searchable = 2 WHERE is_searchable = 1`); err != nil {
		return err
	}
	log.Println("  Removing old files...")
	if _, err := db.Exec(`DELETE FROM files WHERE is_searchable = 0`); err != nil {
		return err
	}
	log.Println("  Clearing FTS index...")
	if _, err := db.Exec(`DELETE FROM files_fts`); err != nil {
		return err
	}
	log.Println("  Rebuilding FTS index...")
	if _, err := db.Exec(`
		INSERT INTO files_fts(rowid, name, path)
		SELECT id, name, path
		FROM files
		WHERE is_searchable = 2
	`); err != nil {
		return err
	}

	log.Println("  Optimizing FTS index...")
	if _, err := db.Exec(`INSERT INTO files_fts(files_fts) VALUES('optimize')`); err != nil {
		return err
	}

	log.Println("  Calculating and caching statistics...")
	if err := calculateAndCacheStats(db, indexName); err != nil {
		log.Printf("Warning: failed to cache stats: %v", err)
	}

	return nil
}

func calculateAndCacheStats(db *sql.DB, indexName string) error {
	stats := &models.IndexStats{Name: indexName}

	// Total files and dirs
	if err := db.QueryRow(`SELECT COUNT(*) FROM files WHERE is_dir = 0`).Scan(&stats.TotalFiles); err != nil {
		return err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM files WHERE is_dir = 1`).Scan(&stats.TotalDirs); err != nil {
		return err
	}

	// Total size
	if err := db.QueryRow(`SELECT COALESCE(SUM(size), 0) FROM files WHERE is_dir = 0`).Scan(&stats.TotalSize); err != nil {
		return err
	}

	// Average file size
	if stats.TotalFiles > 0 {
		stats.AvgFileSize = stats.TotalSize / stats.TotalFiles
	}

	// Oldest and newest file
	var oldestMod, newestMod int64
	if err := db.QueryRow(`SELECT COALESCE(MIN(mod_time), 0) FROM files WHERE is_dir = 0`).Scan(&oldestMod); err == nil && oldestMod > 0 {
		stats.OldestFile = time.Unix(oldestMod, 0)
	}
	if err := db.QueryRow(`SELECT COALESCE(MAX(mod_time), 0) FROM files WHERE is_dir = 0`).Scan(&newestMod); err == nil && newestMod > 0 {
		stats.NewestFile = time.Unix(newestMod, 0)
	}

	// Last scan time
	var lastScanStr string
	if err := db.QueryRow(`SELECT value FROM metadata WHERE key = 'last_scan'`).Scan(&lastScanStr); err == nil {
		stats.LastScan, _ = time.Parse(time.RFC3339, lastScanStr)
	}

	// Top 10 largest files
	rows, err := db.Query(`
		SELECT id, path, name, dir, ext, size, mod_time, is_dir, index_name
		FROM files WHERE is_dir = 0
		ORDER BY size DESC LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f models.FileRecord
			var mod int64
			var isDir int
			if err := rows.Scan(&f.ID, &f.Path, &f.Name, &f.Dir, &f.Ext, &f.Size, &mod, &isDir, &f.IndexName); err == nil {
				f.ModTime = time.Unix(mod, 0)
				f.IsDir = isDir != 0
				stats.LargestFiles = append(stats.LargestFiles, f)
			}
		}
	}

	// Top extensions by count
	rows, err = db.Query(`
		SELECT ext, COUNT(*) as cnt, COALESCE(SUM(size), 0) as total_size
		FROM files WHERE is_dir = 0 AND ext != ''
		GROUP BY ext ORDER BY cnt DESC LIMIT 15
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ext models.ExtensionStats
			if err := rows.Scan(&ext.Extension, &ext.Count, &ext.Size); err == nil {
				stats.TopExtensions = append(stats.TopExtensions, ext)
			}
		}
	}

	// Top extensions by size
	rows, err = db.Query(`
		SELECT ext, COUNT(*) as cnt, COALESCE(SUM(size), 0) as total_size
		FROM files WHERE is_dir = 0 AND ext != ''
		GROUP BY ext ORDER BY total_size DESC LIMIT 15
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ext models.ExtensionStats
			if err := rows.Scan(&ext.Extension, &ext.Count, &ext.Size); err == nil {
				stats.TopExtBySize = append(stats.TopExtBySize, ext)
			}
		}
	}

	// Size distribution
	sizeRanges := []struct {
		label   string
		minSize int64
		maxSize int64
	}{
		{"< 1 KB", 0, 1024},
		{"1 KB - 100 KB", 1024, 100 * 1024},
		{"100 KB - 1 MB", 100 * 1024, 1024 * 1024},
		{"1 MB - 10 MB", 1024 * 1024, 10 * 1024 * 1024},
		{"10 MB - 100 MB", 10 * 1024 * 1024, 100 * 1024 * 1024},
		{"100 MB - 1 GB", 100 * 1024 * 1024, 1024 * 1024 * 1024},
		{"> 1 GB", 1024 * 1024 * 1024, -1},
	}

	for _, sr := range sizeRanges {
		var count, size int64
		var query string
		if sr.maxSize == -1 {
			query = `SELECT COUNT(*), COALESCE(SUM(size), 0) FROM files WHERE is_dir = 0 AND size >= ?`
			db.QueryRow(query, sr.minSize).Scan(&count, &size)
		} else {
			query = `SELECT COUNT(*), COALESCE(SUM(size), 0) FROM files WHERE is_dir = 0 AND size >= ? AND size < ?`
			db.QueryRow(query, sr.minSize, sr.maxSize).Scan(&count, &size)
		}
		stats.SizeDistribution = append(stats.SizeDistribution, models.SizeRange{
			Label: sr.label,
			Count: count,
			Size:  size,
		})
	}

	// Year distribution
	rows, err = db.Query(`
		SELECT strftime('%Y', mod_time, 'unixepoch') as year, COUNT(*) as cnt, COALESCE(SUM(size), 0) as total_size
		FROM files WHERE is_dir = 0 AND mod_time > 0
		GROUP BY year ORDER BY year DESC LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ys models.YearStats
			if err := rows.Scan(&ys.Year, &ys.Count, &ys.Size); err == nil {
				stats.YearDistribution = append(stats.YearDistribution, ys)
			}
		}
	}

	// Recent files
	rows, err = db.Query(`
		SELECT id, path, name, dir, ext, size, mod_time, is_dir, index_name
		FROM files WHERE is_dir = 0
		ORDER BY mod_time DESC LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f models.FileRecord
			var mod int64
			var isDir int
			if err := rows.Scan(&f.ID, &f.Path, &f.Name, &f.Dir, &f.Ext, &f.Size, &mod, &isDir, &f.IndexName); err == nil {
				f.ModTime = time.Unix(mod, 0)
				f.IsDir = isDir != 0
				stats.RecentFiles = append(stats.RecentFiles, f)
			}
		}
	}

	// Cache stats as JSON
	jsonData, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	return setMetadata(db, "stats_cache", string(jsonData))
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
	// Check if metadata table exists (handles fresh/empty databases)
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='metadata'`).Scan(&tableName)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}

	var ts string
	err = db.QueryRow(`SELECT value FROM metadata WHERE key='last_scan'`).Scan(&ts)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, nil
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
