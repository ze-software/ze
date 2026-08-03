// Package cmd owns the entire ping feature surface as a dedicated feature
// module (see ai/rules/plugins.md "Dedicated feature modules").
//
// One feature, spread across several verbs, lives here instead of scattered
// across the central verb packages:
//
//   - show ping       (local + ze-show:ping)     batch ICMP echo             -- ping.go
//   - monitor ping    (local + ze-monitor:ping)  continuous streaming ping   -- monitor.go / stream.go
//   - resolve ping    (ze-resolve:ping)           ICMP ping with source bind -- resolve.go
//
// The shared low-level ICMP primitives (echo-packet building, target
// resolution) live in internal/core/probe so this module does not depend on a
// central verb package or on the traceroute module. The YANG command schema
// container-merges onto the show, monitor, and resolve verb roots; see
// ../yang/ze-ping-cmd.yang.
package cmd

import (
	// Blank import registers this module's YANG command schema (show ping,
	// monitor ping, resolve ping) via container merge onto the central verbs.
	_ "github.com/ze-software/ze/internal/plugins/ping-cmd/yang"
)
