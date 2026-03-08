// Package bgpcmdpeer provides BGP peer lifecycle, introspection, and subscription
// command handlers for the plugin server.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: peer.go — BGP introspection and peer operation handlers
// Detail: summary.go — BGP summary, capabilities, and clear soft handlers
// Detail: subscribe.go — event subscription handlers
// Detail: session.go — BGP peer session handlers
package bgpcmdpeer

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-cmd-peer/schema" // init() registers YANG module
)

// Command source constants (mirrored from plugin package for use in handler output).
const (
	sourceBuiltin = "builtin"
	argVerbose    = "verbose"
)
