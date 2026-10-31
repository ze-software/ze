// Design: docs/guide/redistribution.md -- sources and destinations
// Related: consumer.go -- BGPConsumer, what the factory builds
// Related: bgp.go -- the bgp, ibgp and ebgp sources, which need the same root

package redistribute

import (
	"github.com/ze-software/ze/internal/component/config/redistribute"
)

// bgpConfigRoot is the config root the BGP subsystem runs from. A rule naming
// BGP as a destination or a source in a config without it is refused, because
// no reactor exists to receive or produce the routes.
const bgpConfigRoot = "bgp"

func init() {
	// The BGP destination is declared here, with BGP, so the protocol-neutral
	// redistribute orchestrator names no destination: it builds this consumer
	// over its own engine connection, and a build without BGP has no factory
	// and no BGP destination. The destination declaration carries the root the
	// config doors check; the factory carries only the constructor.
	if err := redistribute.RegisterDestination(redistribute.Destination{
		Name:       bgpConsumerName,
		ConfigRoot: bgpConfigRoot,
	}); err != nil {
		panic("BUG: BGP redistribute destination registration failed: " + err.Error())
	}
	if err := redistribute.RegisterConsumerFactory(redistribute.ConsumerFactory{
		Name: bgpConsumerName,
		New: func(updater redistribute.RouteUpdater) redistribute.RedistConsumer {
			return NewBGPConsumer(updater)
		},
	}); err != nil {
		panic("BUG: BGP redistribute consumer factory registration failed: " + err.Error())
	}
}
