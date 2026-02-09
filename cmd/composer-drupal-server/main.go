//spellchecker:words main
package main

//spellchecker:words flag http time github composer drupal update drupalupdate swaggest swgui
import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
	"github.com/swaggest/swgui/v5emb"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	mux := http.NewServeMux()

	// API routes
	client := drupalupdate.NewClient()
	api := drupalupdate.NewServer(client)
	mux.Handle("POST /api/parse", api)
	mux.Handle("GET /api/releases", api)
	mux.Handle("POST /api/update", api)

	// Serve the OpenAPI spec
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		if _, err := w.Write(drupalupdate.OpenAPISpec); err != nil {
			log.Printf("openapi.yaml: write failed: %v", err)
		}
	})

	// Swagger UI at /doc/
	mux.Handle("GET /doc/", v5emb.New(
		"Composer Drupal Update API",
		"/openapi.yaml",
		"/doc/",
	))

	// Frontend at /
	frontendFiles, err := drupalupdate.FrontendFS()
	if err != nil {
		log.Fatalf("failed to load frontend: %v", err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(frontendFiles)))

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	fmt.Printf("Starting server on %s\n", *addr)
	log.Fatal(srv.ListenAndServe())
}
