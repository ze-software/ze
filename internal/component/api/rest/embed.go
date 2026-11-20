// Design: docs/architecture/api/architecture.md -- vendored Swagger UI assets
// Related: docs/architecture/third-party-credits.md -- the works this file declares present

package rest

import (
	_ "embed"

	"github.com/ze-software/ze/internal/core/credits"
)

//go:embed assets/swagger-ui.css
var swaggerUICSS []byte

//go:embed assets/swagger-ui-bundle.js
var swaggerUIBundle []byte

// swaggerUILicence is swagger-ui's licence, served beside the assets it
// covers.
//
//go:embed assets/swagger-ui.LICENSE
var swaggerUILicence []byte

// swaggerUINotice is swagger-ui's NOTICE file, which its Apache-2.0 licence
// requires to travel with the work.
//
//go:embed assets/swagger-ui.NOTICE
var swaggerUINotice []byte

// swaggerUIBundleLicence is the banners of the libraries the bundle contains.
// The bundle's first line points at it by this name, so it is served beside
// the bundle.
//
//go:embed assets/swagger-ui-bundle.js.LICENSE.txt
var swaggerUIBundleLicence []byte

// The documentation page serves swagger-ui and the libraries its bundle
// contains, so a build with the REST server credits each of them. The names
// are the works of website/data/dependencies.json whose carried-by lists this
// package: normalize.css is inside the stylesheet, whose banner names it.
func init() {
	credits.DeclarePresent("swagger-ui")
	credits.DeclarePresent("normalize.css")
	credits.DeclarePresent("classnames")
	credits.DeclarePresent("deep-extend")
	credits.DeclarePresent("buffer")
	credits.DeclarePresent("fast-json-patch")
	credits.DeclarePresent("repeat-string")
	credits.DeclarePresent("dompurify")
	credits.DeclarePresent("ieee754")
	credits.DeclarePresent("js-yaml")
	credits.DeclarePresent("safe-buffer")
	credits.DeclarePresent("react")
	credits.DeclarePresent("react-dom")
	credits.DeclarePresent("scheduler")
	credits.DeclarePresent("use-sync-external-store")
}
