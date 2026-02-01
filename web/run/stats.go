package webapp

import (
	"log"
	"net/http"
	"strconv"

	"github.com/ogefest/findex/app"
	"github.com/ogefest/findex/models"
)

func (webapp *WebApp) stats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		searcher, err := app.NewSearcher(webapp.IndexConfig)
		if err != nil {
			log.Printf("Unable to create searcher: %v\n", err)
			webapp.renderError(w, http.StatusInternalServerError, "")
			return
		}
		defer searcher.Close()

		// Check for history view request
		selectedIndex := r.URL.Query().Get("index")
		historyIDStr := r.URL.Query().Get("history_id")

		var selectedHistory *models.ScanHistoryEntry
		if selectedIndex != "" && historyIDStr != "" {
			historyID, err := strconv.ParseInt(historyIDStr, 10, 64)
			if err == nil {
				selectedHistory, _ = searcher.GetScanHistoryEntry(selectedIndex, historyID)
			}
		}

		globalStats, err := searcher.GetGlobalStats()
		if err != nil {
			log.Printf("Unable to get global stats: %v\n", err)
			webapp.renderError(w, http.StatusInternalServerError, "")
			return
		}

		// Get scan history for each index
		scanHistories := make(map[string][]models.ScanHistoryEntry)
		for _, idx := range webapp.IndexConfig {
			history, err := searcher.GetScanHistory(idx.Name, 30)
			if err == nil && len(history) > 0 {
				scanHistories[idx.Name] = history
			}
		}

		// If viewing historical data, replace the index stats with historical stats
		if selectedHistory != nil && selectedHistory.Stats != nil {
			for i, indexStat := range globalStats.IndexStats {
				if indexStat.Name == selectedIndex {
					// Keep the name but use historical stats
					historicalStats := *selectedHistory.Stats
					historicalStats.Name = indexStat.Name
					globalStats.IndexStats[i] = historicalStats
					break
				}
			}
		}

		data := webapp.newTplData()
		data["Title"] = "Statistics"
		data["Stats"] = globalStats
		data["ScanHistories"] = scanHistories
		data["SelectedIndex"] = selectedIndex
		data["SelectedHistoryID"] = historyIDStr
		data["SelectedHistory"] = selectedHistory

		err = webapp.TemplateCache["stats.html"].Execute(w, data)
		if err != nil {
			log.Printf("Template error: %v\n", err)
			webapp.renderError(w, http.StatusInternalServerError, "")
		}
	}
}
