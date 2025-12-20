package webapp

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"

	"github.com/ogefest/findex/app"
	"github.com/ogefest/findex/models"
)

type WebApp struct {
	Router        http.Handler
	Templates     *template.Template
	IndexConfig   []*models.IndexConfig
	ActiveIndexes []string
}

func (webapp *WebApp) ReloadIndexConfiguration() {
	cfg, err := app.LoadConfig("index_config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	activeIndexes := make(map[string]bool)
	for i := range cfg.Indexes {
		webapp.IndexConfig = append(webapp.IndexConfig, &cfg.Indexes[i])
		webapp.ActiveIndexes = append(webapp.ActiveIndexes, cfg.Indexes[i].Name)
		activeIndexes[cfg.Indexes[i].Name] = true // All active by default
	}
	sort.Strings(webapp.ActiveIndexes)

	log.Printf("Indexes loaded %v\n", webapp.ActiveIndexes)
}

func (webapp *WebApp) GetRouter() http.Handler {
	return router(webapp)
}

func (webapp *WebApp) InitTemplates() {
	webapp.Templates = template.Must(
		template.New("").Funcs(template.FuncMap{
			"humanizeBytes": humanizeBytes,
		}).ParseGlob("web/templates/*.html"),
	)
}

func (webapp *WebApp) getIndexByName(name string) *models.IndexConfig {
	for _, idx := range webapp.IndexConfig {
		if idx.Name == name {
			return idx
		}
	}
	log.Printf("Unable to find index configuration by name %s\n", name)
	return nil
}
