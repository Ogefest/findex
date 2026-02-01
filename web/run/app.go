package webapp

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/ogefest/findex/app"
	"github.com/ogefest/findex/models"
	"github.com/ogefest/findex/web"
)

type WebApp struct {
	Router        http.Handler
	TemplateCache map[string]*template.Template
	AppConfig     *models.AppConfig
	IndexConfig   []*models.IndexConfig
	ActiveIndexes []string
	ConfigPath    string
}

func (webapp *WebApp) ReloadIndexConfiguration() {
	configPath := webapp.ConfigPath
	if configPath == "" {
		configPath = "index_config.yaml"
	}
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	webapp.AppConfig = cfg

	activeIndexes := make(map[string]bool)
	for i := range cfg.Indexes {
		webapp.IndexConfig = append(webapp.IndexConfig, &cfg.Indexes[i])
		webapp.ActiveIndexes = append(webapp.ActiveIndexes, cfg.Indexes[i].Name)
		activeIndexes[cfg.Indexes[i].Name] = true // All active by default
	}
	sort.Strings(webapp.ActiveIndexes)

	log.Printf("Indexes loaded %v\n", webapp.ActiveIndexes)
}

func (webapp *WebApp) GetListenAddr() string {
	port := 8080
	if webapp.AppConfig != nil && webapp.AppConfig.Server.Port > 0 {
		port = webapp.AppConfig.Server.Port
	}
	return fmt.Sprintf(":%d", port)
}

func (webapp *WebApp) GetRouter() http.Handler {
	return router(webapp)
}

func (webapp *WebApp) InitTemplates() {
	webapp.TemplateCache = make(map[string]*template.Template)

	funcMap := template.FuncMap{
		"humanizeBytes":        humanizeBytes,
		"displayPath":          displayPath,
		"split":                strings.Split,
		"urlquery":             url.QueryEscape,
		"addTrailingSlash":     addTrailingSlash,
		"add":                  func(a, b int) int { return a + b },
		"sub":                  func(a, b int) int { return a - b },
		"percent":              func(part, total int64) int64 { if total == 0 { return 0 }; return (part * 100) / total },
		"buildQueryString":     buildQueryString,
		"buildQueryStringPage": buildQueryStringPage,
	}

	// Read layout template from embedded filesystem
	layoutContent, err := fs.ReadFile(web.Templates, "templates/layout.html")
	if err != nil {
		log.Fatalf("failed to read layout template: %v", err)
	}

	// Get all template files from embedded filesystem
	entries, err := fs.ReadDir(web.Templates, "templates")
	if err != nil {
		log.Fatalf("failed to read templates directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "layout.html" || !strings.HasSuffix(name, ".html") {
			continue
		}

		pageContent, err := fs.ReadFile(web.Templates, "templates/"+name)
		if err != nil {
			log.Fatalf("failed to read template %s: %v", name, err)
		}

		var ts *template.Template

		// Error template is standalone (no layout)
		if name == "error.html" {
			ts, err = template.New(name).Funcs(funcMap).Parse(string(pageContent))
		} else {
			ts, err = template.New(name).Funcs(funcMap).Parse(string(pageContent))
			if err == nil {
				ts, err = ts.Parse(string(layoutContent))
			}
		}

		if err != nil {
			log.Fatalf("failed to parse template %s: %v", name, err)
		}
		webapp.TemplateCache[name] = ts
	}
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
