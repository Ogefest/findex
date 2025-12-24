package webapp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func router(webapp *WebApp) http.Handler {
	r := chi.NewRouter()

	r.Get("/", webapp.startPage())
	r.Get("/download/{index}-{id}", webapp.download())
	r.Get("/browse/{index}", webapp.browse())

	fs := http.FileServer(http.Dir("web/assets"))
	r.Handle("/assets/*", http.StripPrefix("/assets/", fs))

	return r
}
