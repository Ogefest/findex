package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ogefest/findex/web/startpage"
)

func router(webapp *WebApp) http.Handler {
	r := chi.NewRouter()

	r.Get("/", startpage.StartPage(webapp.Templates))

	fs := http.FileServer(http.Dir("web/assets"))
	r.Handle("/assets/*", http.StripPrefix("/assets/", fs))

	return r
}
