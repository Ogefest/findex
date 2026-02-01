package webapp

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ogefest/findex/web"
)

func router(webapp *WebApp) http.Handler {
	r := chi.NewRouter()

	r.Get("/", webapp.startPage())
	r.Get("/stats", webapp.stats())
	r.Get("/download/{index}-{id}", webapp.download())
	r.Get("/browse/{index}", webapp.browse())

	// Serve embedded assets
	assetsFS, _ := fs.Sub(web.Assets, "assets")
	fileServer := http.FileServer(http.FS(assetsFS))
	r.Handle("/assets/*", http.StripPrefix("/assets/", fileServer))

	r.NotFound(webapp.notFoundHandler())

	return r
}
