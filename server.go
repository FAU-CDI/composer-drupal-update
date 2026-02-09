//spellchecker:words drupalupdate
package drupalupdate

//spellchecker:words encoding json http
import (
	"encoding/json"
	"log"
	"net/http"
)

// =============================================================================
// API Request/Response Types
// =============================================================================

// parseRequest is the request body for POST /api/parse.
type parseRequest struct {
	ComposerJSON ComposerJSON `json:"composer_json"`
}

// ParseResponse is the response body for POST /api/parse.
type ParseResponse struct {
	CorePackages     []Package `json:"core_packages"`
	DrupalPackages   []Package `json:"drupal_packages"`
	ComposerPackages []Package `json:"composer_packages"`
}

// ReleasesResponse is the response body for GET /api/releases?package=...
type ReleasesResponse struct {
	Package  string    `json:"package"`
	Releases []Release `json:"releases"`
}

// UpdateRequest is the request body for POST /api/update.
type UpdateRequest struct {
	ComposerJSON ComposerJSON      `json:"composer_json"`
	Versions     map[string]string `json:"versions"` // package name -> new version
}

// ErrorResponse is returned on errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

// =============================================================================
// Server
// =============================================================================

// Server implements http.Handler and provides the JSON API.
type Server struct {
	Client *Client
	Logger *log.Logger

	mux *http.ServeMux
}

// NewServer creates a Server with the given Client for fetching releases.
func NewServer(client *Client) *Server {
	s := &Server{Client: client}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("POST /api/parse", s.handleParse)
	s.mux.HandleFunc("GET /api/releases", s.handleReleases)
	s.mux.HandleFunc("POST /api/update", s.handleUpdate)
	s.Logger = log.Default()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// =============================================================================
// Handlers
// =============================================================================

// handleParse accepts a composer.json and returns all updatable packages,
// split into Drupal and Composer (non-Drupal) categories.
func (s *Server) handleParse(w http.ResponseWriter, r *http.Request) {
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON: " + err.Error()})
		return
	}

	s.writeJSON(w, http.StatusOK, ParseResponse{
		CorePackages:     req.ComposerJSON.CorePackages(),
		DrupalPackages:   req.ComposerJSON.DrupalPackages(),
		ComposerPackages: req.ComposerJSON.ComposerPackages(),
	})
}

// handleReleases returns available releases for a given composer package.
// For drupal/* packages it queries drupal.org; for others it queries Packagist.
func (s *Server) handleReleases(w http.ResponseWriter, r *http.Request) {
	pkg := r.URL.Query().Get("package")
	if pkg == "" {
		s.writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "missing 'package' query parameter"})
		return
	}

	releases, err := s.Client.FetchReleases(r.Context(), pkg)
	if err != nil {
		s.writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "failed to fetch releases: " + err.Error()})
		return
	}

	s.writeJSON(w, http.StatusOK, ReleasesResponse{Package: pkg, Releases: releases})
}

// handleUpdate accepts a composer.json and a version map, and returns the updated composer.json.
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON: " + err.Error()})
		return
	}

	for pkg, version := range req.Versions {
		if _, exists := req.ComposerJSON.Require[pkg]; exists {
			req.ComposerJSON.Require[pkg] = version
		}
	}

	s.writeJSON(w, http.StatusOK, req.ComposerJSON)
}

// =============================================================================
// Helpers
// =============================================================================

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.Logger.Printf("writeJSON: encode failed after headers sent: %v", err)
	}
}
