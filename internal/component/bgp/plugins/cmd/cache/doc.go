// Design: docs/architecture/api/commands.md — BGP cache command handlers
//
// Package cache provides BGP message cache command handlers for the plugin server.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: cache.go — BGP cache operation handlers
package cache

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/cache/schema" // init() registers YANG module
)
