// Design: docs/architecture/api/commands.md — delete verb command handlers
//
// Package delete provides the top-level "delete" CLI verb for removing
// configuration (delete peers). Typing "del" prefix-completes to "delete".
//
// Detail: delete.go — RPC registration for delete verb handlers
package delete

import (
	_ "github.com/ze-software/ze/internal/component/cmd/delete/yang" // init() registers YANG module
)
