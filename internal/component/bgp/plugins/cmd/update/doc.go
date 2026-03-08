// Package update provides BGP route announcement parsing command handlers.
// This includes text-based UPDATE parsing and wire-format (hex/b64) UPDATE parsing.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: update_text.go — text UPDATE parsing and NLRI dispatch
// Detail: update_wire.go — hex/b64 wire UPDATE parsing
package update

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update/schema" // init() registers YANG module
)
