package webapp

import (
	"fmt"
	"path/filepath"
	"strings"
)

func humanizeBytes(s int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case s >= TB:
		return fmt.Sprintf("%.2f TB", float64(s)/TB)
	case s >= GB:
		return fmt.Sprintf("%.2f GB", float64(s)/GB)
	case s >= MB:
		return fmt.Sprintf("%.2f MB", float64(s)/MB)
	case s >= KB:
		return fmt.Sprintf("%.2f KB", float64(s)/KB)
	default:
		return fmt.Sprintf("%d B", s)
	}
}

func displayPath(dir, path, name string) string {
	// usuń nazwę pliku z path
	rel := strings.TrimSuffix(path, name)
	rel = strings.TrimSuffix(rel, "/")

	return filepath.Join(dir, rel)
}

func addTrailingSlash(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}
