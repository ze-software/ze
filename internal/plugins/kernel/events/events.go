// Design: docs/architecture/core-design.md -- redistribute producer registration

package kernelevents

import (
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
)

const Namespace = "kernel"

var ProtocolID = redistevents.RegisterProtocol(Namespace)

var _ = registerProducer()

func registerProducer() bool {
	redistevents.RegisterProducer(ProtocolID)
	return true
}

var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)
