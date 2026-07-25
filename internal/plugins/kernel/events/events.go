// Design: docs/architecture/core-design.md -- redistribute producer registration

package kernelevents

import (
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
)

const Namespace = "kernel"

var ProtocolID = redistevents.RegisterProtocol(Namespace)

var _ = registerProducer()

func registerProducer() bool {
	redistevents.RegisterProducer(ProtocolID)
	return true
}

var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)
