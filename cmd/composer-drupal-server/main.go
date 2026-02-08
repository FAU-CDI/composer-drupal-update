package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
	"github.com/swaggest/swgui/v5emb"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	mux := http.NewServeMux()

	// API routes (register each route explicitly to avoid method conflicts)
	client := drupalupdate.NewClient()
	api := drupalupdate.NewServer(client)
	mux.Handle("POST /api/parse", api)
	mux.Handle("GET /api/releases", api)
	mux.Handle("POST /api/update", api)

	// Serve the OpenAPI spec (embedded in the drupalupdate package)
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Write(drupalupdate.OpenAPISpec)
	})

	// Swagger UI
	mux.Handle("GET /doc/", v5emb.New(
		"Composer Drupal Update API",
		"/openapi.yaml",
		"/doc/",
	))

	// Redirect root to docs
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/doc/", http.StatusFound)
	})

	fmt.Printf("Starting server on %s\n", *addr)
	fmt.Printf("  API:     http://localhost%s/api/\n", *addr)
	fmt.Printf("  Docs:    http://localhost%s/doc/\n", *addr)
	fmt.Printf("  Spec:    http://localhost%s/openapi.yaml\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
