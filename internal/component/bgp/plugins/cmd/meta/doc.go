// Design: docs/architecture/api/commands.md — BGP command discovery and plugin configuration
//
// Package meta provides BGP command discovery, help, completion,
// and plugin process configuration handlers.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: help.go — BGP command discovery and completion handlers
// Detail: plugin_config.go — BGP plugin process configuration handlers
package meta

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/meta/schema" // init() registers YANG module
)

// Command source constants (mirrored from plugin package for use in handler output).
const (
	sourceBuiltin = "builtin"
	argVerbose    = "verbose"
)
