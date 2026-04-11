// Design: docs/architecture/api/architecture.md -- vendored Swagger UI assets

package rest

import _ "embed"

//go:embed assets/swagger-ui.css
var swaggerUICSS []byte

//go:embed assets/swagger-ui-bundle.js
var swaggerUIBundle []byte
