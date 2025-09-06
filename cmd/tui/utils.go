package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// colorPalette defines a list of readable background colors
var colorPalette = []string{
	"27",  // Blue
	"29",  // Green
	"124", // Red
	"130", // Orange
	"93",  // Purple
	"172", // Yellow
	"37",  // Cyan
	"64",  // Olive
	"166", // Dark Orange
	"97",  // Light Purple
}

// generateColorForIndex generates a deterministic color based on the index name
func generateColorForIndex(indexName string) string {
	hash := 0
	for _, char := range indexName {
		hash += int(char)
	}
	return colorPalette[hash%len(colorPalette)]
}

// formatSize converts bytes to a human-readable string
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	if bytes >= GB {
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	} else if bytes >= MB {
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	} else if bytes >= KB {
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	}
	return fmt.Sprintf("%d B", bytes)
}

// openFile opens the file with the default system application
func openFile(filePath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	case "darwin":
		cmd = exec.Command("open", filePath)
	default: // linux, bsd, etc.
		cmd = exec.Command("xdg-open", filePath)
	}
	return cmd.Start()
}
