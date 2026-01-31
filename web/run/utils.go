package webapp

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/ogefest/findex/version"
)

func (webapp *WebApp) newTplData() map[string]any {
	data := make(map[string]any)
	data["Indexes"] = webapp.ActiveIndexes
	data["Query"] = ""
	data["FilterParams"] = map[string]string{
		"min_size":  "",
		"max_size":  "",
		"ext":       "",
		"date_from": "",
		"date_to":   "",
		"type":      "",
	}
	data["Version"] = version.Version
	data["Commit"] = version.Commit
	data["BuildDate"] = version.BuildDate
	return data
}

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
	rel := strings.TrimSuffix(path, name)
	rel = strings.TrimSuffix(rel, "/")

	return fmt.Sprintf("/%s", rel)
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

// buildQueryString builds a query string with a new per_page value
func buildQueryString(data map[string]any, perPage int) template.URL {
	params := url.Values{}

	if q, ok := data["Query"].(string); ok && q != "" {
		params.Set("q", q)
	}

	if fp, ok := data["FilterParams"].(map[string]string); ok {
		for key, val := range fp {
			if val != "" {
				params.Set(key, val)
			}
		}
	}

	// Add selected indexes
	if indexes, ok := data["SelectedIndexes"].([]string); ok {
		for _, idx := range indexes {
			params.Add("index[]", idx)
		}
	}

	params.Set("per_page", fmt.Sprintf("%d", perPage))
	params.Set("page", "1") // Reset to page 1 when changing per_page

	return template.URL(params.Encode())
}

// buildQueryStringPage builds a query string with a new page value
func buildQueryStringPage(data map[string]any, page int) template.URL {
	params := url.Values{}

	if q, ok := data["Query"].(string); ok && q != "" {
		params.Set("q", q)
	}

	if fp, ok := data["FilterParams"].(map[string]string); ok {
		for key, val := range fp {
			if val != "" {
				params.Set(key, val)
			}
		}
	}

	// Add selected indexes
	if indexes, ok := data["SelectedIndexes"].([]string); ok {
		for _, idx := range indexes {
			params.Add("index[]", idx)
		}
	}

	if pp, ok := data["PerPage"].(int); ok {
		params.Set("per_page", fmt.Sprintf("%d", pp))
	}

	params.Set("page", fmt.Sprintf("%d", page))

	return template.URL(params.Encode())
}
