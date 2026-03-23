// Design: docs/architecture/api/commands.md — set verb command handlers
//
// Package set provides the top-level "set" CLI verb for creating or
// modifying configuration (add peers, save config).
//
// Detail: set.go — RPC registration for set verb handlers
package set

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/set/schema" // init() registers YANG module
)
