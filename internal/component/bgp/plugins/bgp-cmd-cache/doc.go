// Design: docs/architecture/api/commands.md — BGP cache command handlers
//
// Package bgpcmdcache provides BGP message cache command handlers for the plugin server.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
package bgpcmdcache

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-cmd-cache/schema" // init() registers YANG module
)
