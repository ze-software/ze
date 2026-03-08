// Design: docs/architecture/api/commands.md — BGP event subscription handlers
//
// Package subscribe provides event subscription command handlers.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: subscribe.go — event subscription handlers
package subscribe

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/subscribe/schema" // init() registers YANG module
)
