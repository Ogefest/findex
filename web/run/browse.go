package webapp

import (
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ogefest/findex/app"
)

type Breadcrumb struct {
	Part string
	Path string
}

func (webapp *WebApp) browse() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		index := chi.URLParam(r, "index")
		searcher, err := app.NewSearcher(webapp.IndexConfig)
		if err != nil {
			log.Printf("Unable to create searcher: %v\n", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		path := r.URL.Query().Get("path")
		if index == "" {
			log.Printf("Invalid index:%s or path: %s\n", index, path)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		itemList, err := searcher.GetDirectoryContent(index, path)
		if err != nil {
			log.Printf("Unable to get dir content for path %s in index %s", path, index)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// log.Printf("%v", itemList)

		var breadcrumbs []Breadcrumb
		var pathParts []string
		if path != "" {
			pathParts = strings.Split(path, "/")
		}

		for i, part := range pathParts {
			if part == "" {
				continue
			}
			breadcrumbs = append(breadcrumbs, Breadcrumb{
				Part: part,
				Path: strings.Join(pathParts[:i+1], "/"),
			})
		}

		data := map[string]any{
			"Items":       itemList,
			"Path":        path,
			"Index":       index,
			"Breadcrumbs": breadcrumbs,
		}

		err = webapp.TemplateCache["browse.html"].Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

}
