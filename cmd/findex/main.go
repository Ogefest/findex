package main

import (
	"flag"
	"log"

	"github.com/ogefest/findex/app"
)

func main() {
	configPath := flag.String("config", "index_config.yaml", "Path to index configuration file")
	forceScan := flag.Bool("force", false, "Force scan ignoring refresh_interval")
	flag.Parse()

	if err := app.Run(*configPath, *forceScan); err != nil {
		log.Fatalf("error: %v", err)
	}
}
