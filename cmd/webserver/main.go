package main

import (
	"net/http"

	app "github.com/ogefest/findex/web/run"
)

func main() {

	webapp := app.WebApp{}
	webapp.ReloadIndexConfiguration()
	webapp.InitTemplates()

	http.ListenAndServe(":8080", webapp.GetRouter())
}
