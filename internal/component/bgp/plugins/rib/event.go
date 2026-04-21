// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin event types
//
// Type aliases importing from component/bgp package.
// bgp-rib was the original home for these types; they moved to
// internal/component/bgp/ so bgp-adj-rib-in and future plugins can reuse them.
package rib

import (
	"fmt"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
)

// Type aliases — these are the same types, not wrappers.
type (
	Route           = bgp.Route
	Event           = bgp.Event
	FamilyOperation = bgp.FamilyOperation
	MessageInfo     = bgp.MessageInfo
	PeerInfoJSON    = bgp.PeerInfoJSON
	PeerRemoteInfo  = bgp.PeerRemoteInfo
	PeerLocalInfo   = bgp.PeerLocalInfo
)

// Function aliases for package-internal callers.
var (
	parseEvent         = bgp.ParseEvent
	formatRouteCommand = bgp.FormatAnnounceCommand
	parseNLRIValue     = bgp.ParseNLRIValue
	routeKey           = bgp.RouteKey

	parseCommunityStrings      = bgp.ParseCommunityStrings
	parseLargeCommunityStrings = bgp.ParseLargeCommunityStrings
	parseExtCommunityStrings   = bgp.ParseExtCommunityStrings
)

// outRouteKey creates a ribOut-specific key without the family prefix.
// The family is redundant in ribOut because it is the outer map key.
// RFC 7911: When ADD-PATH is negotiated, pathID is part of the key.
func outRouteKey(prefix string, pathID uint32) string {
	if pathID == 0 {
		return prefix
	}
	return fmt.Sprintf("%s:%d", prefix, pathID)
}
