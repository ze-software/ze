// Design: docs/architecture/api/commands.md -- show verb command handlers
//
// Package show provides the top-level "show" CLI verb for read-only
// introspection (peer details, warnings).
//
// Detail: show.go -- RPC registration for show verb handlers
package show

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/show/schema" // init() registers YANG module
)
