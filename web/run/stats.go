package webapp

import (
	"log"
	"net/http"

	"github.com/ogefest/findex/app"
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

		globalStats, err := searcher.GetGlobalStats()
		if err != nil {
			log.Printf("Unable to get global stats: %v\n", err)
			webapp.renderError(w, http.StatusInternalServerError, "")
			return
		}

		data := webapp.newTplData()
		data["Title"] = "Statistics"
		data["Stats"] = globalStats

		err = webapp.TemplateCache["stats.html"].Execute(w, data)
		if err != nil {
			log.Printf("Template error: %v\n", err)
			webapp.renderError(w, http.StatusInternalServerError, "")
		}
	}
}
