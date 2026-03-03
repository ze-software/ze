// Design: docs/architecture/plugin/rib-storage-design.md — BGP route types for plugins
// Related: event.go — event parsing and family operations
// Related: format.go — route command formatting
// Related: nlri.go — NLRI value parsing
//
// Package bgp provides common BGP domain types used by multiple BGP plugins
// (bgp-rib, bgp-adj-rib-in, bgp-watchdog, and future plugins).
//
// RFC 7911: ADD-PATH path-id is included in route keys when present.
// Multiple paths to the same prefix with different path-ids are stored separately.
package bgp

import (
	"fmt"
	"time"
)

// Route represents a stored route with full path attributes.
// RFC 7911: PathID is included when ADD-PATH is negotiated.
type Route struct {
	MsgID     uint64    `json:"msg-id,omitempty"`
	Family    string    `json:"family"`
	Prefix    string    `json:"prefix"`
	PathID    uint32    `json:"path-id,omitempty"` // RFC 7911: ADD-PATH path identifier
	NextHop   string    `json:"next-hop"`
	Timestamp time.Time `json:"timestamp,omitzero"`

	// Path attributes for full route resend.
	Origin              string   `json:"origin,omitempty"`
	ASPath              []uint32 `json:"as-path,omitempty"`
	MED                 *uint32  `json:"med,omitempty"`
	LocalPreference     *uint32  `json:"local-preference,omitempty"`
	Communities         []string `json:"communities,omitempty"`
	LargeCommunities    []string `json:"large-communities,omitempty"`
	ExtendedCommunities []string `json:"extended-communities,omitempty"`

	// VPN fields — used by watchdog and future VPN route replay.
	RD     string   `json:"rd,omitempty"`     // Route Distinguisher ("ASN:NN" or "IP:NN")
	Labels []uint32 `json:"labels,omitempty"` // MPLS label stack
}

// RouteKey creates a unique key for a route.
// RFC 7911: When ADD-PATH is negotiated, path-id is part of the key.
func RouteKey(family, prefix string, pathID uint32) string {
	if pathID == 0 {
		return family + ":" + prefix
	}
	return fmt.Sprintf("%s:%s:%d", family, prefix, pathID)
}
