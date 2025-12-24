package webapp

import (
	"log"
	"net/http"

	"github.com/ogefest/findex/app"
	"github.com/ogefest/findex/models"
)

func (webapp *WebApp) startPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		indexesSearch := r.URL.Query()["index[]"]

		data := map[string]any{
			"Title": "Start",
			"Query": query,
		}
		data["Indexes"] = webapp.ActiveIndexes

		if len(query) > 0 {
			var idxPtrs []*models.IndexConfig

			for _, name := range indexesSearch {
				idxPtrs = append(idxPtrs, webapp.getIndexByName(name))
			}

			searcher, err := app.NewSearcher(idxPtrs)
			if err != nil {
				log.Printf("Unable to create searcher %v\n", err)
				// return errpage
			}
			result, err := searcher.Search(query, &app.FileFilter{}, 10)
			log.Printf("Found %v\n", result)
			data["Results"] = result
		}

		err := webapp.TemplateCache["startpage.html"].Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
