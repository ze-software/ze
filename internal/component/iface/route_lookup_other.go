//go:build !linux

// Design: plan/spec-diag-5-active-probes.md -- platform stub for route lookup

package iface

import (
	"errors"
	"net/netip"
)

var errRouteLookupNotAvailableOnThis = errors.New("route lookup not available on this platform")

// RouteLookup is not available on non-linux platforms.
func RouteLookup(_ netip.Addr) (map[string]any, error) {
	return nil, errRouteLookupNotAvailableOnThis
}
