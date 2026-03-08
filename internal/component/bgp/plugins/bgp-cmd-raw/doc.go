// Design: docs/architecture/api/commands.md — BGP raw message command handler
//
// Package bgpcmdraw provides the raw BGP message injection handler.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
package bgpcmdraw

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-cmd-raw/schema" // init() registers YANG module
)
