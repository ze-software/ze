// Design: docs/architecture/l2tp/subscriber-session-model.md -- PPPoE
// subscriber routes are a distinct redistribution source.
package pppoe

import (
	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
)

var (
	routeProtocol = redistevents.RegisterProtocol(SubsystemName)
	routeChange   = events.Register[*redistevents.RouteChangeBatch](SubsystemName, redistevents.EventType)
)

func init() {
	if err := redistribute.RegisterSource(redistribute.RouteSource{
		Name:        SubsystemName,
		Protocol:    SubsystemName,
		Description: "subscriber routes from PPPoE sessions",
		ConfigRoot:  SubsystemName,
	}); err != nil {
		panic("BUG: register PPPoE redistribute source: " + err.Error())
	}
	redistevents.RegisterProducer(routeProtocol)
}
