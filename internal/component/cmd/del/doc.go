// Design: docs/architecture/api/commands.md — del/delete verb command handlers
//
// Package del provides the top-level "del" (and "delete") CLI verb for
// removing configuration (delete peers).
//
// Detail: del.go — RPC registration for del verb handlers
package del

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/del/schema" // init() registers YANG module
)
