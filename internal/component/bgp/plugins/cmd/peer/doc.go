// Package bgpcmdpeer provides BGP peer lifecycle and introspection
// command handlers for the plugin server.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: peer.go — BGP introspection and peer operation handlers
// Detail: summary.go — BGP summary and capabilities handlers
// Detail: session.go — BGP peer session handlers
package bgpcmdpeer

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-cmd-peer/schema" // init() registers YANG module
)
