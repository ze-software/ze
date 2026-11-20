// Design: docs/architecture/web-interface.md -- LG embedded assets
// Overview: server.go -- LG server and route registration
// Related: docs/architecture/third-party-credits.md -- the works this file declares present

package lg

import (
	"embed"

	"github.com/ze-software/ze/internal/core/credits"
)

// The markup is no longer embedded. templ compiles each .templ file into Go
// (see layout.templ and its siblings), so the templates are in the binary as
// code rather than as an embedded file tree.

//go:embed assets
var assetsFS embed.FS

// The asset directory carries htmx and uPlot with their licence files, and the
// looking glass serves it on its own listener, so a build with the looking
// glass credits both even without the web interface. The names are the works
// of website/data/dependencies.json whose carried-by lists this package.
func init() {
	credits.DeclarePresent("htmx")
	credits.DeclarePresent("uPlot")
}
