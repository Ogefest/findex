package main

import (
	"flag"
	"log"

	"github.com/ogefest/findex/app"
)

func main() {
	configPath := flag.String("config", "index_config.yaml", "Path to index configuration file")
	migrationsPath := flag.String("migrations", "init.sql", "Path to SQL migrations file")
	flag.Parse()

	if err := app.Run(*configPath, *migrationsPath); err != nil {
		log.Fatalf("error: %v", err)
	}
}
