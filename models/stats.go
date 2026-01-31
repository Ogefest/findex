package models

import "time"

type ExtensionStats struct {
	Extension string
	Count     int64
	Size      int64
}

type SizeRange struct {
	Label string
	Count int64
	Size  int64
}

type YearStats struct {
	Year  int
	Count int64
	Size  int64
}

type IndexStats struct {
	Name             string
	TotalFiles       int64
	TotalDirs        int64
	TotalSize        int64
	LastScan         time.Time
	OldestFile       time.Time
	NewestFile       time.Time
	AvgFileSize      int64
	LargestFiles     []FileRecord
	TopExtensions    []ExtensionStats
	TopExtBySize     []ExtensionStats
	RecentFiles      []FileRecord
	SizeDistribution []SizeRange
	YearDistribution []YearStats
}

type GlobalStats struct {
	TotalFiles       int64
	TotalDirs        int64
	TotalSize        int64
	IndexCount       int
	TopExtensions    []ExtensionStats
	TopExtBySize     []ExtensionStats
	SizeDistribution []SizeRange
	YearDistribution []YearStats
	IndexStats       []IndexStats
}
