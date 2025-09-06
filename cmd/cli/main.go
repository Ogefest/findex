package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ogefest/findex/internal/app"
	"github.com/ogefest/findex/pkg/models"
)

func main() {
	// Parse command-line arguments
	query := flag.String("q", "", "Search query")
	flag.Parse()

	if *query == "" {
		fmt.Fprintln(os.Stderr, "Error: search query is required. Use -q <query>")
		os.Exit(1)
	}

	// Load configuration
	cfg, err := app.LoadConfig("index_config.yaml")
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

	// Perform search
	results, err := searcher.Search(*query, nil, 50) // No filter, limit 50 per index
	if err != nil {
		log.Fatalf("Search error: %v", err)
	}

	// Print file paths
	for _, result := range results {
		fmt.Println(result.Path)
	}
}
