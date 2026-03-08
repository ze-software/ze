// Design: docs/architecture/api/commands.md — BGP commit command handlers
//
// Package bgpcmdcommit provides BGP named commit workflow command handlers.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: commit.go — BGP named commit workflow handlers
package bgpcmdcommit

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-cmd-commit/schema" // init() registers YANG module
)
