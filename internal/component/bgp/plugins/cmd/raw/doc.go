// Design: docs/architecture/api/commands.md — BGP raw message command handler
//
// Package raw provides the raw BGP message injection handler.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: raw.go — BGP raw message injection handler
package raw

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw/schema" // init() registers YANG module
)
