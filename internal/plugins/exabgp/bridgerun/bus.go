// Design: docs/architecture/exabgp-bridge.md -- shared bus declarations for both bridge runners
// Related: fleet.go -- script lifecycle shared by internal and SDK runners

package bridgerun

import (
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// BridgeSends returns the CLI command prefixes the ExaBGP bridge translates
// from script stdout and dispatches to the daemon.
func BridgeSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: events.NamespaceCLI + ":send bgp"},
		{Message: events.NamespaceCLI + ":request peer"},
		{Message: events.NamespaceCLI + ":show bgp peer"},
		{Message: events.NamespaceCLI + ":clear bgp rib in"},
		{Message: events.NamespaceCLI + ":clear bgp rib advertised"},
		{Message: events.NamespaceCLI + ":clear bgp rib out"},
		{Message: events.NamespaceCLI + ":request bgp watchdog announce"},
		{Message: events.NamespaceCLI + ":request bgp watchdog withdraw"},
		{Message: events.NamespaceCLI + ":request bgp watchdog give"},
		{Message: events.NamespaceCLI + ":request bgp watchdog forget"},
		{Message: events.NamespaceCLI + ":create bgp peer"},
		{Message: events.NamespaceCLI + ":delete bgp peer"},
		{Message: events.NamespaceCLI + ":request shutdown"},
		{Message: events.NamespaceCLI + ":help"},
	}
}

// BridgeReceives returns the BGP bus messages the ExaBGP bridge subscribes to
// and translates onto script stdin.
func BridgeReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{Message: bgpevents.Namespace + ":" + bgpevents.EventState},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventConnected},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventNegotiated},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventFSM},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventSignal},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventOpen},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventUpdate},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventNotification},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventKeepalive},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventRefresh},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventEOR},
	}
}
