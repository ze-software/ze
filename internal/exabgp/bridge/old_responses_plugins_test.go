//go:build ze_bgp

// Design: docs/architecture/exabgp-bridge.md -- the lines a 4.2 or 5.0 script reads
// Related: old_responses_test.go -- the judge these registrations serve

package bridge

// The daemon links every plugin that registers an attribute's JSON
// formatter, so the judge does too: without bgp-rr, ORIGINATOR_ID and
// CLUSTER_LIST reach the bridge as ze's `attr-9` and `attr-10` hex, which no
// daemon writes, and the judge would fault the bridge for the test's setup.
import (
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rr" // blank import: registers originator-id and cluster-list
)
