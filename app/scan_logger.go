package app

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ScanLogger handles logging to both stdout and a compressed file
type ScanLogger struct {
	file       *os.File
	gzWriter   *gzip.Writer
	logger     *log.Logger
	indexName  string
	startTime  time.Time
	logPath    string
	mu         sync.Mutex

	// Counters for statistics
	filesScanned    int64
	dirsScanned     int64
	filesExcluded   int64
	dirsExcluded    int64
	errorsCount     int64
	zipFilesScanned int64
	zipEntriesFound int64
}

// NewScanLogger creates a new logger that writes to both stdout and a gzipped log file
// The log file is created in the same directory as the database
func NewScanLogger(dbPath, indexName string, retentionDays int) (*ScanLogger, error) {
	dbDir := filepath.Dir(dbPath)

	// Create log filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFileName := fmt.Sprintf("%s_scan_%s.log.gz", indexName, timestamp)
	logPath := filepath.Join(dbDir, logFileName)

	// Ensure directory exists
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Clean up old logs before starting new scan
	if retentionDays > 0 {
		cleanupOldLogs(dbDir, indexName, retentionDays)
	}

	// Open log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Create gzip writer
	gzWriter := gzip.NewWriter(file)

	// Create multi-writer for both stdout and gzipped file
	multiWriter := io.MultiWriter(os.Stdout, gzWriter)
	logger := log.New(multiWriter, "", log.Ldate|log.Ltime)

	sl := &ScanLogger{
		file:      file,
		gzWriter:  gzWriter,
		logger:    logger,
		indexName: indexName,
		startTime: time.Now(),
		logPath:   logPath,
	}

	sl.Log("%s", repeat("=", 80))
	sl.Log("SCAN LOG STARTED")
	sl.Log("Index: %s", indexName)
	sl.Log("Database path: %s", dbPath)
	sl.Log("Log file: %s", logPath)
	sl.Log("Log retention: %d days", retentionDays)
	sl.Log("Start time: %s", sl.startTime.Format(time.RFC3339))
	sl.Log("%s", repeat("=", 80))

	return sl, nil
}

// cleanupOldLogs removes log files older than retentionDays
func cleanupOldLogs(logDir, indexName string, retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	pattern := fmt.Sprintf("%s_scan_*.log.gz", indexName)

	matches, err := filepath.Glob(filepath.Join(logDir, pattern))
	if err != nil {
		log.Printf("Warning: failed to find old logs: %v", err)
		return
	}

	for _, logFile := range matches {
		info, err := os.Stat(logFile)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(logFile); err != nil {
				log.Printf("Warning: failed to remove old log %s: %v", logFile, err)
			} else {
				log.Printf("Removed old scan log: %s (age: %d days)", filepath.Base(logFile), int(time.Since(info.ModTime()).Hours()/24))
			}
		}
	}
}

// Log writes a formatted message to the log
func (sl *ScanLogger) Log(format string, args ...interface{}) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.logger.Printf(format, args...)
}

// LogSection writes a section header
func (sl *ScanLogger) LogSection(title string) {
	sl.Log("")
	sl.Log("----- %s -----", title)
}

// LogConfig logs the scan configuration
func (sl *ScanLogger) LogConfig(rootPaths []string, excludePaths []string, numWorkers int, scanZipContents bool) {
	sl.LogSection("SCAN CONFIGURATION")
	sl.Log("Root paths (%d):", len(rootPaths))
	for i, p := range rootPaths {
		sl.Log("  [%d] %s", i+1, p)
	}
	sl.Log("Exclude patterns (%d):", len(excludePaths))
	for i, p := range excludePaths {
		sl.Log("  [%d] %s", i+1, p)
	}
	sl.Log("Number of workers: %d", numWorkers)
	sl.Log("Scan zip contents: %v", scanZipContents)
}

// LogPreviousStats logs statistics from previous scan
func (sl *ScanLogger) LogPreviousStats(totalFiles, totalDirs int64, lastScan time.Time) {
	sl.LogSection("PREVIOUS SCAN STATE")
	if lastScan.IsZero() {
		sl.Log("Last scan: Never (first scan)")
	} else {
		sl.Log("Last scan: %s (%.1f hours ago)", lastScan.Format(time.RFC3339), time.Since(lastScan).Hours())
	}
	sl.Log("Previous file count: %d", totalFiles)
	sl.Log("Previous directory count: %d", totalDirs)
}

// LogRootScanStart logs start of scanning a root path
func (sl *ScanLogger) LogRootScanStart(rootIndex, totalRoots int, rootPath string) {
	sl.LogSection(fmt.Sprintf("SCANNING ROOT %d/%d", rootIndex, totalRoots))
	sl.Log("Path: %s", rootPath)

	// Check if path exists and is accessible
	info, err := os.Stat(rootPath)
	if err != nil {
		sl.Log("WARNING: Cannot access root path: %v", err)
	} else {
		sl.Log("Root is directory: %v", info.IsDir())
		sl.Log("Root permissions: %s", info.Mode().String())
	}
}

// LogRootScanComplete logs completion of scanning a root path
func (sl *ScanLogger) LogRootScanComplete(rootIndex, totalRoots int, rootPath string, duration time.Duration, filesFound, dirsFound int64) {
	sl.Log("Root %d/%d completed: %s", rootIndex, totalRoots, rootPath)
	sl.Log("  Duration: %v", duration)
	sl.Log("  Files found in this root: %d", filesFound)
	sl.Log("  Directories found in this root: %d", dirsFound)
}

// LogExcludedDir logs when a directory is excluded
func (sl *ScanLogger) LogExcludedDir(path, pattern string) {
	atomic.AddInt64(&sl.dirsExcluded, 1)
	sl.Log("EXCLUDED DIR: %s (pattern: %s)", path, pattern)
}

// LogExcludedFile logs when a file is excluded
func (sl *ScanLogger) LogExcludedFile(path, pattern string) {
	atomic.AddInt64(&sl.filesExcluded, 1)
	sl.Log("EXCLUDED FILE: %s (pattern: %s)", path, pattern)
}

// LogError logs an error during scanning
func (sl *ScanLogger) LogError(context, path string, err error) {
	atomic.AddInt64(&sl.errorsCount, 1)
	sl.Log("ERROR [%s]: %s - %v", context, path, err)
}

// LogZipScan logs zip file scanning
func (sl *ScanLogger) LogZipScan(zipPath string, entriesFound int) {
	atomic.AddInt64(&sl.zipFilesScanned, 1)
	atomic.AddInt64(&sl.zipEntriesFound, int64(entriesFound))
	sl.Log("ZIP SCANNED: %s (%d entries)", zipPath, entriesFound)
}

// IncrementFiles increments the file counter
func (sl *ScanLogger) IncrementFiles() {
	atomic.AddInt64(&sl.filesScanned, 1)
}

// IncrementDirs increments the directory counter
func (sl *ScanLogger) IncrementDirs() {
	atomic.AddInt64(&sl.dirsScanned, 1)
}

// LogBatchInsert logs batch insertion progress
func (sl *ScanLogger) LogBatchInsert(batchSize, totalProcessed int) {
	sl.Log("BATCH INSERT: %d files (total processed: %d)", batchSize, totalProcessed)
}

// LogDatabaseStats logs current database statistics
func (sl *ScanLogger) LogDatabaseStats(totalFiles, totalDirs, totalSize int64) {
	sl.LogSection("DATABASE STATISTICS AFTER SCAN")
	sl.Log("Total files in database: %d", totalFiles)
	sl.Log("Total directories in database: %d", totalDirs)
	sl.Log("Total size: %d bytes (%.2f GB)", totalSize, float64(totalSize)/(1024*1024*1024))
}

// LogComparison logs comparison between previous and current scan
func (sl *ScanLogger) LogComparison(prevFiles, currFiles, prevDirs, currDirs int64) {
	sl.LogSection("SCAN COMPARISON")

	fileDiff := currFiles - prevFiles
	dirDiff := currDirs - prevDirs

	sl.Log("Files: %d -> %d (diff: %+d)", prevFiles, currFiles, fileDiff)
	sl.Log("Directories: %d -> %d (diff: %+d)", prevDirs, currDirs, dirDiff)

	if fileDiff < 0 {
		sl.Log("WARNING: File count DECREASED by %d files!", -fileDiff)
		sl.Log("  This may indicate:")
		sl.Log("  - Files were deleted from source")
		sl.Log("  - Root paths changed or are inaccessible")
		sl.Log("  - Exclude patterns changed")
		sl.Log("  - Permission issues during scan")
	}

	if dirDiff < 0 {
		sl.Log("WARNING: Directory count DECREASED by %d directories!", -dirDiff)
	}
}

// LogSummary logs the final summary
func (sl *ScanLogger) LogSummary() {
	duration := time.Since(sl.startTime)

	sl.LogSection("SCAN SUMMARY")
	sl.Log("Total duration: %v", duration)
	sl.Log("Files scanned: %d", atomic.LoadInt64(&sl.filesScanned))
	sl.Log("Directories scanned: %d", atomic.LoadInt64(&sl.dirsScanned))
	sl.Log("Files excluded: %d", atomic.LoadInt64(&sl.filesExcluded))
	sl.Log("Directories excluded: %d", atomic.LoadInt64(&sl.dirsExcluded))
	sl.Log("Errors encountered: %d", atomic.LoadInt64(&sl.errorsCount))
	sl.Log("Zip files scanned: %d", atomic.LoadInt64(&sl.zipFilesScanned))
	sl.Log("Zip entries found: %d", atomic.LoadInt64(&sl.zipEntriesFound))

	filesScanned := atomic.LoadInt64(&sl.filesScanned)
	if filesScanned > 0 && duration.Seconds() > 0 {
		filesPerSec := float64(filesScanned) / duration.Seconds()
		sl.Log("Scan rate: %.0f files/second", filesPerSec)
	}

	sl.Log("")
	sl.Log("%s", repeat("=", 80))
	sl.Log("SCAN COMPLETED: %s", time.Now().Format(time.RFC3339))
	sl.Log("%s", repeat("=", 80))
}

// Close closes the gzip writer and log file
func (sl *ScanLogger) Close() error {
	sl.LogSummary()

	// Flush and close gzip writer first
	if sl.gzWriter != nil {
		if err := sl.gzWriter.Close(); err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}
	}

	// Then close the file
	if sl.file != nil {
		return sl.file.Close()
	}
	return nil
}

// GetStats returns current statistics
func (sl *ScanLogger) GetStats() (files, dirs, excluded, errors int64) {
	return atomic.LoadInt64(&sl.filesScanned),
		atomic.LoadInt64(&sl.dirsScanned),
		atomic.LoadInt64(&sl.filesExcluded) + atomic.LoadInt64(&sl.dirsExcluded),
		atomic.LoadInt64(&sl.errorsCount)
}

// GetLogPath returns the path to the current log file
func (sl *ScanLogger) GetLogPath() string {
	return sl.logPath
}

// repeat returns a string with s repeated n times
func repeat(s string, n int) string {
	return strings.Repeat(s, n)
}
