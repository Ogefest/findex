package startpage

import (
	"html/template"
	"net/http"
)

func StartPage(t *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			"Title": "Start",
		}

		err := t.ExecuteTemplate(w, "startpage.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}