package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ogefest/findex/app"
	"github.com/ogefest/findex/models"
)

func main() {
	configPath := flag.String("config", "index_config.yaml", "Path to index configuration file")
	query := flag.String("q", "", "Search query")
	flag.Parse()

	if *query == "" {
		fmt.Fprintln(os.Stderr, "Error: search query is required. Use -q <query>")
		os.Exit(1)
	}

	cfg, err := app.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var idxPtrs []*models.IndexConfig
	for i := range cfg.Indexes {
		idxPtrs = append(idxPtrs, &cfg.Indexes[i])
	}

	searcher, err := app.NewSearcher(idxPtrs)
	if err != nil {
		log.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	results, err := searcher.Search(*query, nil, 50)
	if err != nil {
		log.Fatalf("Search error: %v", err)
	}

	for _, result := range results {
		fmt.Println(result.Path)
	}
}
