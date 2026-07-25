// Package cmd owns the entire traceroute feature surface as a dedicated feature
// module (see ai/rules/plugin-self-containment.md "Dedicated feature modules").
//
// One feature, spread across several verbs, lives here instead of scattered
// across the central verb packages:
//
//   - show traceroute   (local + ze-show:traceroute)  sequential ICMP path trace   -- traceroute.go
//   - show probe-round  (ze-show:probe-round)         one parallel probe round     -- probe_round.go
//   - monitor traceroute(local + ze-monitor:traceroute)continuous mtr-style stream  -- monitor.go / stream.go
//   - resolve traceroute(ze-resolve:traceroute)        ICMP traceroute with options -- resolve.go
//
// All paths use the internal ICMP engine (no external traceroute binary).
// The shared low-level ICMP primitives (echo-packet building, target
// resolution) live in internal/core/probe so this module does not depend on a
// central verb package or on the ping module. The YANG command schema
// container-merges onto the show, monitor, and resolve verb roots; see
// ../yang/ze-traceroute-cmd.yang.
package cmd

import (
	// Blank import registers this module's YANG command schema (show
	// traceroute, show probe-round, monitor traceroute, resolve traceroute) via
	// container merge onto the central verbs.
	_ "github.com/ze-software/ze/internal/plugins/traceroute-cmd/yang"
)
