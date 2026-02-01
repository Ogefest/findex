package main

import (
	"flag"
	"log"

	"github.com/ogefest/findex/app"
)

func main() {
	configPath := flag.String("config", "index_config.yaml", "Path to index configuration file")
	flag.Parse()

	if err := app.Run(*configPath); err != nil {
		log.Fatalf("error: %v", err)
	}
}
