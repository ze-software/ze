// Design: docs/architecture/core-design.md -- redistribute producer registration

package connectedevents

import (
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
)

const Namespace = "connected"

var ProtocolID = redistevents.RegisterProtocol(Namespace)

var _ = registerProducer()

func registerProducer() bool {
	redistevents.RegisterProducer(ProtocolID)
	// The kernel creates a connected route the moment an address is assigned, so
	// Ze must never program one. The declaration is what lets a connected path
	// compete for a prefix in the shared Loc-RIB without Ze installing a second
	// entry beside the kernel's own: sysrib reads it by ID and withdraws Ze's
	// route instead of installing one.
	redistevents.RegisterOSInstalled(ProtocolID)
	return true
}

var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)
