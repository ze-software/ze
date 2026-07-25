// Design: docs/architecture/api/commands.md — BGP event subscription handlers
//
// Package subscribe provides event subscription command handlers.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: subscribe.go — event subscription handlers
package subscribe

import (
	_ "github.com/ze-software/ze/internal/component/cmd/subscribe/yang" // init() registers YANG module
)
