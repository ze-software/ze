// Design: docs/architecture/api/commands.md — BGP event subscription handlers
//
// Package bgpcmdsubscribe provides event subscription command handlers.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
package bgpcmdsubscribe

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-cmd-subscribe/schema" // init() registers YANG module
)
