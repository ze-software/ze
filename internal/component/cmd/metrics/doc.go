// Design: docs/architecture/api/commands.md — BGP metrics command handlers
//
// Package metrics provides BGP metrics command handlers for the plugin server.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: metrics.go — BGP metrics show and list handlers
package metrics

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/metrics/schema" // init() registers YANG module
)
