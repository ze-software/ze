// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- OSPF redistevents producer wiring.
// Related: internal/plugins/ospf/spf -- ProtocolID() (the single "ospf" identity).
//
// Package ospfredistevents is the redistevents PRODUCER wiring for OSPF. It is the
// four mandatory parts of producer registration (spec-ospf-10 Producer side),
// mirroring the IS-IS producer (internal/plugins/isis/redistribute/events):
//
//  1. the OSPF ProtocolID -- allocated ONCE by spec-ospf-8 (spf.ProtocolID(), which
//     calls redistevents.RegisterProtocol("ospf") for the Loc-RIB install Source).
//     Redistribution reuses the SAME identity rather than allocating a second one:
//     the registry is idempotent on name, but a single accessor keeps the
//     single-source contract explicit (umbrella "Redistribution source").
//  2. RegisterProducer(ProtocolID) -- so OSPF appears in redistevents.Producers()
//     and the redistribute-orchestrator subscribes. Registering only the config
//     RouteSource (source.go) is NOT enough: the orchestrator subscribes solely to
//     the IDs returned by Producers() (spec AC-14).
//  3. the typed event handle RouteChange = events.Register[*RouteChangeBatch](
//     "ospf", "route-change") -- a LOCAL handle built in THIS package; no handle
//     pointer crosses a boundary (redistevents package doc).
//  4. EMIT on the handle when SPF route changes -- done by source.go, which
//     references RouteChange and ProtocolID from here.
//
// Importing this package runs the producer registration via the package-level var
// initializers below. register.go (root ospf) imports source.go (via the
// ospfredistribute package), which imports this package, so the producer wiring
// loads.
package ospfredistevents

import (
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/plugins/ospf/spf"
)

// Namespace is the OSPF redistribute protocol/source name -- the SINGLE name
// "ospf" (no per-area names). It matches the config RouteSource name (source.go),
// the Loc-RIB Source (spf.ProtocolID), and the single admin distance (umbrella
// "Redistribution source").
const Namespace = "ospf"

// ProtocolID is the OSPF redistevents identity. It is the SAME ID spec-ospf-8
// allocated for the Loc-RIB install Source (spf.ProtocolID()); redistribution
// reuses it rather than allocating a second one, so a single "ospf" source feeds
// both the producer batch's Protocol field and the orchestrator's source
// resolution (ProtocolName(Protocol) == "ospf").
var ProtocolID = spf.ProtocolID()

// registerProducer marks OSPF as a producer so it appears in
// redistevents.Producers() and the orchestrator subscribes. Run once via the
// package-level var initializer below (so merely importing this package wires the
// producer). Idempotent on the redistevents side.
func registerProducer() bool {
	redistevents.RegisterProducer(ProtocolID)
	return true
}

// producerRegistered forces registerProducer to run at package init (var-init
// order guarantees it runs before the orchestrator reads Producers() in its
// OnStarted). The bool is read by the wiring test as a registration assertion.
var producerRegistered = registerProducer()

// RouteChange is the LOCAL typed handle bound to ("ospf", "route-change"). The
// source (source.go) emits OSPF SPF route changes on it; the orchestrator builds
// its OWN handle for the same tuple and subscribes (redistevents idempotency). No
// handle pointer crosses a boundary.
var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)
