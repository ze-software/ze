// Design: docs/architecture/l2tp.md -- L2TP route-change event handle
// Related: ../redistribute.go -- source registration (config layer)

// Package events defines the typed EventBus handle for L2TP subscriber
// route-change events. Producers (the route observer) and consumers
// (bgp-redistribute) each call events.Register with the same
// (namespace, eventType, T) tuple; the events registry is idempotent.
package events

import (
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
)

// Namespace is the event namespace for L2TP route-change events.
const Namespace = "l2tp"

// ProtocolID is the numeric identity allocated for L2TP by the
// redistevents registry. Used by the observer to fill
// RouteChangeBatch.Protocol.
var ProtocolID = redistevents.RegisterProtocol(Namespace)

// registerProducer marks L2TP as having a producer so
// bgp-redistribute discovers it via redistevents.Producers().
var _ = registerProducer()

func registerProducer() bool {
	redistevents.RegisterProducer(ProtocolID)
	return true
}

// RouteChange is the typed handle for (l2tp, route-change). The
// observer emits via this handle; bgp-redistribute subscribes via its
// own local handle bound to the same (namespace, eventType, T) tuple.
var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)
