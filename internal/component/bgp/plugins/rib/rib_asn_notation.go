// Design: docs/architecture/config/syntax.md — the bgp/as-notation leaf
// Overview: rib.go — the OnConfigure callback that records the notation
// Related: rib_attr_format.go — asPathList, the renderer that reads it
package rib

import (
	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// configureASNotation records the notation this process writes an AS number
// in, from the bgp config subtree Stage 2 delivered.
//
// The rib plugin runs in the daemon or in a process of its own, and only the
// second case needs this call. An in-process rib reads the value the daemon
// already recorded from the same leaf, at the point a candidate became the
// running configuration (SetConfigTree,
// internal/component/bgp/reactor/reactor_api.go). Recording it here costs one
// store, and it removes the difference between the two.
func configureASNotation(bgp map[string]any) error {
	return asn.ConfigureFromBGP(bgp)
}
