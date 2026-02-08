package drupalupdate

import (
	"embed"
	"io/fs"
)

// OpenAPISpec contains the embedded OpenAPI specification.
//
//go:embed openapi.yaml
var OpenAPISpec []byte

//go:embed frontend/index.html frontend/app.js frontend/api.js
var frontendFS embed.FS

// FrontendFS returns the embedded frontend files as an fs.FS
// rooted at the frontend/ directory.
func FrontendFS() (fs.FS, error) {
	return fs.Sub(frontendFS, "frontend")
}
