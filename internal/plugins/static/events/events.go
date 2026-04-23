package staticevents

import (
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
)

const Namespace = "static"

var ProtocolID = redistevents.RegisterProtocol(Namespace)

var _ = registerProducer()

func registerProducer() bool {
	redistevents.RegisterProducer(ProtocolID)
	return true
}

var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)
