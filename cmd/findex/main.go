package main

import (
	"log"
	"os"

	"github.com/ogefest/findex/app"
)

func main() {
	configPath := "./index_config.yaml"
	migrationsPath := "init.sql"

	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if err := app.Run(configPath, migrationsPath); err != nil {
		log.Fatalf("error: %v", err)
	}
}
