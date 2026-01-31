package app

import (
	"time"

	"github.com/ogefest/findex/models"
)

func (s *Searcher) GetIndexStats(indexName string) (*models.IndexStats, error) {
	db := s.dbs[indexName]
	stats := &models.IndexStats{Name: indexName}

	// Total files and dirs
	err := db.QueryRow(`SELECT COUNT(*) FROM files WHERE is_dir = 0`).Scan(&stats.TotalFiles)
	if err != nil {
		return nil, err
	}

	err = db.QueryRow(`SELECT COUNT(*) FROM files WHERE is_dir = 1`).Scan(&stats.TotalDirs)
	if err != nil {
		return nil, err
	}

	// Total size
	err = db.QueryRow(`SELECT COALESCE(SUM(size), 0) FROM files WHERE is_dir = 0`).Scan(&stats.TotalSize)
	if err != nil {
		return nil, err
	}

	// Average file size
	if stats.TotalFiles > 0 {
		stats.AvgFileSize = stats.TotalSize / stats.TotalFiles
	}

	// Oldest and newest file
	var oldestMod, newestMod int64
	err = db.QueryRow(`SELECT COALESCE(MIN(mod_time), 0) FROM files WHERE is_dir = 0`).Scan(&oldestMod)
	if err == nil && oldestMod > 0 {
		stats.OldestFile = time.Unix(oldestMod, 0)
	}

	err = db.QueryRow(`SELECT COALESCE(MAX(mod_time), 0) FROM files WHERE is_dir = 0`).Scan(&newestMod)
	if err == nil && newestMod > 0 {
		stats.NewestFile = time.Unix(newestMod, 0)
	}

	// Last scan time
	var lastScanStr string
	err = db.QueryRow(`SELECT value FROM metadata WHERE key = 'last_scan'`).Scan(&lastScanStr)
	if err == nil {
		stats.LastScan, _ = time.Parse(time.RFC3339, lastScanStr)
	}

	// Top 10 largest files
	rows, err := db.Query(`
		SELECT id, path, name, dir, ext, size, mod_time, is_dir, index_name
		FROM files
		WHERE is_dir = 0
		ORDER BY size DESC
		LIMIT 10
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
		FROM files
		WHERE is_dir = 0 AND ext != ''
		GROUP BY ext
		ORDER BY cnt DESC
		LIMIT 15
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
		FROM files
		WHERE is_dir = 0 AND ext != ''
		GROUP BY ext
		ORDER BY total_size DESC
		LIMIT 15
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
		label    string
		minSize  int64
		maxSize  int64
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
			err = db.QueryRow(query, sr.minSize).Scan(&count, &size)
		} else {
			query = `SELECT COUNT(*), COALESCE(SUM(size), 0) FROM files WHERE is_dir = 0 AND size >= ? AND size < ?`
			err = db.QueryRow(query, sr.minSize, sr.maxSize).Scan(&count, &size)
		}
		if err == nil {
			stats.SizeDistribution = append(stats.SizeDistribution, models.SizeRange{
				Label: sr.label,
				Count: count,
				Size:  size,
			})
		}
	}

	// Year distribution
	rows, err = db.Query(`
		SELECT
			strftime('%Y', mod_time, 'unixepoch') as year,
			COUNT(*) as cnt,
			COALESCE(SUM(size), 0) as total_size
		FROM files
		WHERE is_dir = 0 AND mod_time > 0
		GROUP BY year
		ORDER BY year DESC
		LIMIT 10
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
		FROM files
		WHERE is_dir = 0
		ORDER BY mod_time DESC
		LIMIT 10
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

	return stats, nil
}

func (s *Searcher) GetGlobalStats() (*models.GlobalStats, error) {
	global := &models.GlobalStats{
		IndexCount: len(s.dbs),
	}

	extMapByCount := make(map[string]*models.ExtensionStats)
	extMapBySize := make(map[string]*models.ExtensionStats)
	sizeDistMap := make(map[string]*models.SizeRange)
	yearDistMap := make(map[int]*models.YearStats)

	for indexName := range s.dbs {
		indexStats, err := s.GetIndexStats(indexName)
		if err != nil {
			continue
		}

		global.IndexStats = append(global.IndexStats, *indexStats)
		global.TotalFiles += indexStats.TotalFiles
		global.TotalDirs += indexStats.TotalDirs
		global.TotalSize += indexStats.TotalSize

		// Aggregate extensions by count
		for _, ext := range indexStats.TopExtensions {
			if existing, ok := extMapByCount[ext.Extension]; ok {
				existing.Count += ext.Count
				existing.Size += ext.Size
			} else {
				extMapByCount[ext.Extension] = &models.ExtensionStats{
					Extension: ext.Extension,
					Count:     ext.Count,
					Size:      ext.Size,
				}
			}
		}

		// Aggregate extensions by size
		for _, ext := range indexStats.TopExtBySize {
			if existing, ok := extMapBySize[ext.Extension]; ok {
				existing.Count += ext.Count
				existing.Size += ext.Size
			} else {
				extMapBySize[ext.Extension] = &models.ExtensionStats{
					Extension: ext.Extension,
					Count:     ext.Count,
					Size:      ext.Size,
				}
			}
		}

		// Aggregate size distribution
		for _, sd := range indexStats.SizeDistribution {
			if existing, ok := sizeDistMap[sd.Label]; ok {
				existing.Count += sd.Count
				existing.Size += sd.Size
			} else {
				sizeDistMap[sd.Label] = &models.SizeRange{
					Label: sd.Label,
					Count: sd.Count,
					Size:  sd.Size,
				}
			}
		}

		// Aggregate year distribution
		for _, yd := range indexStats.YearDistribution {
			if existing, ok := yearDistMap[yd.Year]; ok {
				existing.Count += yd.Count
				existing.Size += yd.Size
			} else {
				yearDistMap[yd.Year] = &models.YearStats{
					Year:  yd.Year,
					Count: yd.Count,
					Size:  yd.Size,
				}
			}
		}
	}

	// Convert maps to slices and sort

	// Extensions by count
	for _, ext := range extMapByCount {
		global.TopExtensions = append(global.TopExtensions, *ext)
	}
	sortExtensionsByCount(global.TopExtensions)
	if len(global.TopExtensions) > 15 {
		global.TopExtensions = global.TopExtensions[:15]
	}

	// Extensions by size
	for _, ext := range extMapBySize {
		global.TopExtBySize = append(global.TopExtBySize, *ext)
	}
	sortExtensionsBySize(global.TopExtBySize)
	if len(global.TopExtBySize) > 15 {
		global.TopExtBySize = global.TopExtBySize[:15]
	}

	// Size distribution (preserve order)
	sizeOrder := []string{"< 1 KB", "1 KB - 100 KB", "100 KB - 1 MB", "1 MB - 10 MB", "10 MB - 100 MB", "100 MB - 1 GB", "> 1 GB"}
	for _, label := range sizeOrder {
		if sd, ok := sizeDistMap[label]; ok {
			global.SizeDistribution = append(global.SizeDistribution, *sd)
		}
	}

	// Year distribution
	for _, yd := range yearDistMap {
		global.YearDistribution = append(global.YearDistribution, *yd)
	}
	sortYearStats(global.YearDistribution)

	return global, nil
}

func sortExtensionsByCount(exts []models.ExtensionStats) {
	for i := 0; i < len(exts); i++ {
		for j := i + 1; j < len(exts); j++ {
			if exts[j].Count > exts[i].Count {
				exts[i], exts[j] = exts[j], exts[i]
			}
		}
	}
}

func sortExtensionsBySize(exts []models.ExtensionStats) {
	for i := 0; i < len(exts); i++ {
		for j := i + 1; j < len(exts); j++ {
			if exts[j].Size > exts[i].Size {
				exts[i], exts[j] = exts[j], exts[i]
			}
		}
	}
}

func sortYearStats(years []models.YearStats) {
	for i := 0; i < len(years); i++ {
		for j := i + 1; j < len(years); j++ {
			if years[j].Year > years[i].Year {
				years[i], years[j] = years[j], years[i]
			}
		}
	}
}
