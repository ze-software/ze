// Design: docs/architecture/api/commands.md — monitor verb root (unowned)
//
// Package monitor provides the generic, unowned root of the "monitor" CLI verb.
// It holds no handlers: each monitor subcommand is owned by the component whose
// behavior it streams (BGP owns "monitor bgp"; ping/traceroute/iface/ike own
// their subtrees) and container-merges onto this root. Keeping the root here,
// outside any plugin, means removing a single plugin removes only its own
// monitor subtree, not the verb. See ai/rules/plugin-self-containment.md.
//
// Detail: schema/ze-cli-monitor-cmd.yang — monitor verb root + generic subtree.
package monitor

import (
	_ "github.com/ze-software/ze/internal/component/cmd/monitor/yang" // init() registers YANG module
)
