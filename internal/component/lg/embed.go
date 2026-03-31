// Design: docs/architecture/web-interface.md -- LG embedded assets and templates
// Overview: server.go -- LG server and route registration

package lg

import "embed"

//go:embed assets
var assetsFS embed.FS

//go:embed templates
var templatesFS embed.FS
