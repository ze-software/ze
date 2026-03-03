// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin event types
//
// Type aliases importing from component/bgp package.
// bgp-rib was the original home for these types; they moved to
// internal/component/bgp/ so bgp-adj-rib-in and future plugins can reuse them.
package bgp_rib

import bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"

// Type aliases — these are the same types, not wrappers.
type (
	Route           = bgp.Route
	Event           = bgp.Event
	FamilyOperation = bgp.FamilyOperation
	MessageInfo     = bgp.MessageInfo
	PeerInfoFlat    = bgp.PeerInfoFlat
	PeerInfoNested  = bgp.PeerInfoNested
)

// Function aliases for package-internal callers.
var (
	parseEvent         = bgp.ParseEvent
	formatRouteCommand = bgp.FormatAnnounceCommand
	parseNLRIValue     = bgp.ParseNLRIValue
	routeKey           = bgp.RouteKey
)
