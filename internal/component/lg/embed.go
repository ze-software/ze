// Design: docs/architecture/web-interface.md -- LG embedded assets
// Overview: server.go -- LG server and route registration

package lg

import "embed"

// The markup is no longer embedded. templ compiles each .templ file into Go
// (see layout.templ and its siblings), so the templates are in the binary as
// code rather than as an embedded file tree.

//go:embed assets
var assetsFS embed.FS
