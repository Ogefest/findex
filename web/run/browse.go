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
		if index == "" {
			webapp.renderError(w, http.StatusBadRequest, "Index name is required.")
			return
		}

		// Check if index exists
		if webapp.getIndexByName(index) == nil {
			webapp.renderError(w, http.StatusNotFound, "The requested index does not exist.")
			return
		}

		searcher, err := app.NewSearcher(webapp.IndexConfig)
		if err != nil {
			log.Printf("Unable to create searcher: %v\n", err)
			webapp.renderError(w, http.StatusInternalServerError, "")
			return
		}
		defer searcher.Close()

		path := r.URL.Query().Get("path")

		itemList, err := searcher.GetDirectoryContent(index, path)
		if err != nil {
			log.Printf("Unable to get dir content for path %s in index %s: %v", path, index, err)
			webapp.renderError(w, http.StatusInternalServerError, "")
			return
		}

		currentDirInfo, err := searcher.GetDirectorySize(index, path)
		if err != nil {
			log.Printf("Unable to get current dir info %s %s: %v\n", index, path, err)
			webapp.renderError(w, http.StatusInternalServerError, "")
			return
		}

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

		data := webapp.newTplData()
		data["Items"] = itemList
		data["Path"] = path
		data["Index"] = index
		data["Breadcrumbs"] = breadcrumbs
		data["DirInfo"] = currentDirInfo

		err = webapp.TemplateCache["browse.html"].Execute(w, data)
		if err != nil {
			log.Printf("Template error: %v", err)
			webapp.renderError(w, http.StatusInternalServerError, "")
		}
	}
}
