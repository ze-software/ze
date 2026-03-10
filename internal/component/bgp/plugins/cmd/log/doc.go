// Design: docs/architecture/api/commands.md — BGP log level command handlers
//
// Package log provides BGP log level command handlers for the plugin server.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: log.go — BGP log show and set handlers
package log

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/log/schema" // init() registers YANG module
)
