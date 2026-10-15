// Design: docs/architecture/config/syntax.md — the bgp/as-notation leaf
// Overview: config_tx.go — the config transaction that records the notation
// Related: rib_attr_format.go — asPathList, the renderer that reads it
package rib

import (
	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// parseASNotation reads the notation the bgp config subtree selects, and
// records nothing: verify parses with it, and only an applied config is
// recorded (ribConfigTx.install).
//
// The rib plugin runs in the daemon or in a process of its own, and only the
// second records the value. An in-process rib reads the value the daemon
// recorded from the same leaf, at the point a candidate became the running
// configuration (SetConfigTree, internal/component/bgp/reactor/reactor_api.go).
// That is the one writer in the daemon's process, so a reload the transaction
// rolls back never leaves the candidate's notation behind.
func parseASNotation(bgp map[string]any) (asn.Notation, error) {
	return asn.NotationFromBGP(bgp)
}
