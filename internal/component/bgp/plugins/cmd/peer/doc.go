// Package peer provides BGP peer lifecycle and introspection
// command handlers for the plugin server.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: peer.go — BGP introspection and peer operation handlers
// Detail: summary.go — BGP summary and capabilities handlers
// Detail: session.go — BGP peer session handlers
package peer

import (
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer/yang" // init() registers YANG module
)
