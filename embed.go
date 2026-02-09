//spellchecker:words drupalupdate
package drupalupdate

//spellchecker:words embed
import (
	"embed"
	"fmt"
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
	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		return nil, fmt.Errorf("frontend fs: %w", err)
	}
	return sub, nil
}
