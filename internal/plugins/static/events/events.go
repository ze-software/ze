// Design: docs/architecture/static-routes.md -- redistribution event types

package staticevents

import (
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
)

const Namespace = "static"

var ProtocolID = redistevents.RegisterProtocol(Namespace)

var _ = registerProducer()

func registerProducer() bool {
	redistevents.RegisterProducer(ProtocolID)
	return true
}

var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)
