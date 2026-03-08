// Package bgpcmdops provides BGP operational command handlers for the plugin server.
// This includes cache management, commit workflow, raw message sending, and route refresh.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: cache.go — BGP message cache operations
// Detail: commit.go — named commit workflow handlers
// Detail: raw.go — raw message sending handler
// Detail: refresh.go — route refresh, BoRR, and EoRR handlers
package bgpcmdops

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-cmd-ops/schema" // init() registers YANG module
)
