// Design: docs/architecture/isis/isis-11-redistribution.md -- IS-IS redistevents producer wiring.
// Related: internal/plugins/isis/spf -- ProtocolID() (the single "isis" identity).
//
// Package isisredistevents is the redistevents PRODUCER wiring for IS-IS. It is
// the four mandatory parts of producer registration (spec-isis-11 Producer side),
// mirroring the connected producer (internal/plugins/connected/events):
//
//  1. the IS-IS ProtocolID -- allocated ONCE by spec-isis-9 (spf.ProtocolID(),
//     which calls redistevents.RegisterProtocol("isis") for the Loc-RIB install
//     Source). Redistribution reuses the SAME identity rather than allocating a
//     second one: the registry is idempotent on name, but a single accessor keeps
//     the single-source contract explicit (umbrella "Redistribution source").
//  2. RegisterProducer(ProtocolID) -- so IS-IS appears in redistevents.Producers()
//     and the redistribute-orchestrator subscribes. Registering only the config
//     RouteSource (source.go) is NOT enough: the orchestrator subscribes solely to
//     the IDs returned by Producers() (spec AC-11).
//  3. the typed event handle RouteChange = events.Register[*RouteChangeBatch](
//     "isis", "route-change") -- a LOCAL handle built in THIS package; no handle
//     pointer crosses a boundary (redistevents package doc).
//  4. EMIT on the handle when SPF route changes -- done by source.go, which
//     references RouteChange and ProtocolID from here.
//
// Importing this package runs the producer registration via the package-level
// var initializers below. register.go (root isis) imports source.go, which imports
// this package, so the producer wiring loads.
package isisredistevents

import (
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
)

// Namespace is the IS-IS redistribute protocol/source name -- the SINGLE name
// "isis" (no per-level isis-l1/isis-l2). It matches the config RouteSource name
// (source.go), the Loc-RIB Source (spf.ProtocolID), and the single admin
// distance (umbrella "Redistribution source").
const Namespace = "isis"

// ProtocolID is the IS-IS redistevents identity. It is the SAME ID spec-isis-9
// allocated for the Loc-RIB install Source (spf.ProtocolID()); redistribution
// reuses it rather than allocating a second one, so a single "isis" source feeds
// both the producer batch's Protocol field and the orchestrator's source
// resolution (ProtocolName(Protocol) == "isis").
var ProtocolID = spf.ProtocolID()

// registerProducer marks IS-IS as a producer so it appears in
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

// RouteChange is the LOCAL typed handle bound to ("isis", "route-change"). The
// source (source.go) emits IS-IS SPF route changes on it; the orchestrator builds
// its OWN handle for the same tuple and subscribes (redistevents idempotency). No
// handle pointer crosses a boundary.
var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)
