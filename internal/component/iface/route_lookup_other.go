//go:build !linux

// Design: plan/spec-diag-5-active-probes.md -- platform stub for route lookup

package iface

import (
	"fmt"
	"net/netip"
)

// RouteLookup is not available on non-linux platforms.
func RouteLookup(_ netip.Addr) (map[string]any, error) {
	return nil, fmt.Errorf("route lookup not available on this platform")
}
