package webapp

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ogefest/findex/app"
	"github.com/ogefest/findex/models"
)

func (webapp *WebApp) startPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		indexesSearch := r.URL.Query()["index[]"]

		// Parse filter parameters
		filter := parseFilterParams(r)

		// Parse pagination parameters
		page := 1
		perPage := 25
		if p := r.URL.Query().Get("page"); p != "" {
			if val, err := strconv.Atoi(p); err == nil && val > 0 {
				page = val
			}
		}
		if pp := r.URL.Query().Get("per_page"); pp != "" {
			if val, err := strconv.Atoi(pp); err == nil && val > 0 && val <= 100 {
				perPage = val
			}
		}

		data := webapp.newTplData()
		data["Title"] = "Start"
		data["Query"] = query
		data["IndexConfigs"] = webapp.IndexConfig
		data["Filter"] = filter
		data["FilterParams"] = getFilterParamsForTemplate(r)
		data["CurrentPage"] = page
		data["PerPage"] = perPage
		data["SelectedIndexes"] = indexesSearch

		// Execute search if there's a query OR active filters
		if len(query) > 0 || hasActiveFilters(filter) {
			var idxPtrs []*models.IndexConfig

			for _, name := range indexesSearch {
				idxPtrs = append(idxPtrs, webapp.getIndexByName(name))
			}

			searcher, err := app.NewSearcher(idxPtrs)
			if err != nil {
				log.Printf("Unable to create searcher %v\n", err)
				webapp.renderError(w, http.StatusInternalServerError, "")
				return
			}
			defer searcher.Close()

			// Get more results for pagination
			allResults, err := searcher.Search(query, filter, 1000)
			if err != nil {
				log.Printf("Search error: %v\n", err)
			}

			totalResults := len(allResults)
			totalPages := (totalResults + perPage - 1) / perPage
			if totalPages < 1 {
				totalPages = 1
			}
			if page > totalPages {
				page = totalPages
			}

			// Paginate results
			start := (page - 1) * perPage
			end := start + perPage
			if start > totalResults {
				start = totalResults
			}
			if end > totalResults {
				end = totalResults
			}

			var paginatedResults []models.FileRecord
			if start < totalResults {
				paginatedResults = allResults[start:end]
			}

			log.Printf("Found %d total results, showing page %d (%d-%d)\n", totalResults, page, start+1, end)

			data["Results"] = paginatedResults
			data["TotalResults"] = totalResults
			data["TotalPages"] = totalPages
			data["CurrentPage"] = page
			data["HasSearch"] = true
			data["HasPrevPage"] = page > 1
			data["HasNextPage"] = page < totalPages
			data["PrevPage"] = page - 1
			data["NextPage"] = page + 1

			// Generate page numbers for pagination
			var pages []int
			for i := 1; i <= totalPages; i++ {
				if i == 1 || i == totalPages || (i >= page-2 && i <= page+2) {
					pages = append(pages, i)
				} else if len(pages) > 0 && pages[len(pages)-1] != -1 {
					pages = append(pages, -1) // -1 represents "..."
				}
			}
			data["Pages"] = pages
		}

		err := webapp.TemplateCache["startpage.html"].Execute(w, data)
		if err != nil {
			log.Printf("Template error: %v\n", err)
			webapp.renderError(w, http.StatusInternalServerError, "")
		}
	}
}

func hasActiveFilters(filter *app.FileFilter) bool {
	if filter == nil {
		return false
	}
	return filter.MinSize > 0 ||
		filter.MaxSize > 0 ||
		len(filter.Exts) > 0 ||
		filter.ModTimeFrom > 0 ||
		filter.ModTimeTo > 0 ||
		filter.OnlyFiles ||
		filter.OnlyDirs
}

func parseFilterParams(r *http.Request) *app.FileFilter {
	filter := &app.FileFilter{}

	// Parse size filters
	if minSize := r.URL.Query().Get("min_size"); minSize != "" {
		if val, err := parseSize(minSize); err == nil {
			filter.MinSize = val
		}
	}
	if maxSize := r.URL.Query().Get("max_size"); maxSize != "" {
		if val, err := parseSize(maxSize); err == nil {
			filter.MaxSize = val
		}
	}

	// Parse extension filter
	if exts := r.URL.Query().Get("ext"); exts != "" {
		extList := strings.Split(exts, ",")
		for _, ext := range extList {
			ext = strings.TrimSpace(ext)
			if ext != "" {
				filter.Exts = append(filter.Exts, ext)
			}
		}
	}

	// Parse date filters
	if dateFrom := r.URL.Query().Get("date_from"); dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			filter.ModTimeFrom = t.Unix()
		}
	}
	if dateTo := r.URL.Query().Get("date_to"); dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			// End of day
			filter.ModTimeTo = t.Add(24*time.Hour - time.Second).Unix()
		}
	}

	// Parse type filter
	fileType := r.URL.Query().Get("type")
	if fileType == "files" {
		filter.OnlyFiles = true
	} else if fileType == "dirs" {
		filter.OnlyDirs = true
	}

	return filter
}

// parseSize parses size string like "10MB", "1GB", "500KB" to bytes
func parseSize(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0, nil
	}

	// Order matters - check longer suffixes first
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"TB", 1024 * 1024 * 1024 * 1024},
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, s2 := range suffixes {
		if strings.HasSuffix(s, s2.suffix) {
			numStr := strings.TrimSuffix(s, s2.suffix)
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, err
			}
			return int64(num * float64(s2.mult)), nil
		}
	}

	// Try plain number (assume bytes)
	return strconv.ParseInt(s, 10, 64)
}

// getFilterParamsForTemplate returns filter values for form inputs
func getFilterParamsForTemplate(r *http.Request) map[string]string {
	return map[string]string{
		"min_size":  r.URL.Query().Get("min_size"),
		"max_size":  r.URL.Query().Get("max_size"),
		"ext":       r.URL.Query().Get("ext"),
		"date_from": r.URL.Query().Get("date_from"),
		"date_to":   r.URL.Query().Get("date_to"),
		"type":      r.URL.Query().Get("type"),
	}
}
