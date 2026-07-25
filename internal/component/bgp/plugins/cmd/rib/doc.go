// Design: docs/architecture/api/commands.md — RIB CLI proxy handlers
//
// Package rib registers CLI proxy handlers that forward RIB commands to
// the bgp-rib plugin process.
//
// Each handler file self-registers via init() + pluginserver.RegisterRPCs().
//
// Detail: rib.go — RIB CLI proxy handler forwarding
package rib

import (
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/rib/yang" // init() registers YANG module
)
