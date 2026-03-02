// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin event types
//
// Type aliases importing from shared package.
// bgp-rib was the original home for these types; they moved to
// internal/plugin/bgp/shared/ so bgp-adj-rib-in and future plugins can reuse them.
package bgp_rib

import "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/shared"

// Type aliases — these are the same types, not wrappers.
type (
	Route           = shared.Route
	Event           = shared.Event
	FamilyOperation = shared.FamilyOperation
	MessageInfo     = shared.MessageInfo
	PeerInfoFlat    = shared.PeerInfoFlat
	PeerInfoNested  = shared.PeerInfoNested
)

// Function aliases for package-internal callers.
var (
	parseEvent         = shared.ParseEvent
	formatRouteCommand = shared.FormatAnnounceCommand
	parseNLRIValue     = shared.ParseNLRIValue
	routeKey           = shared.RouteKey
)
