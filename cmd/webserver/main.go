package main

import (
	"flag"
	"log"
	"net/http"

	app "github.com/ogefest/findex/web/run"
)

func main() {
	configPath := flag.String("config", "index_config.yaml", "Path to index configuration file")
	listenAddr := flag.String("listen", "", "Address to listen on (overrides config)")
	flag.Parse()

	webapp := app.WebApp{
		ConfigPath: *configPath,
	}
	webapp.ReloadIndexConfiguration()
	webapp.InitTemplates()

	addr := webapp.GetListenAddr()
	if *listenAddr != "" {
		addr = *listenAddr
	}

	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, webapp.GetRouter()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
