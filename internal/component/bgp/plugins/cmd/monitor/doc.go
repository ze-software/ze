// Design: docs/architecture/api/commands.md — BGP monitor command handlers
//
// Package monitor provides the bgp monitor command for streaming live BGP events.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: monitor.go — monitor command handler and arg parsing
// Detail: format.go — visual text line formatting
package monitor

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/monitor/schema" // init() registers YANG module
)
