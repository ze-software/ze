// Design: docs/architecture/exabgp-bridge.md -- bus declarations derived from the ExaBGP translator tables.
// Related: fleet.go -- script lifecycle for the ExaBGP wrapper.

package bridgerun

import (
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// BridgeSends returns the CLI command prefixes and BGP wire send messages the
// ExaBGP wrapper translates from script stdout and dispatches to the daemon,
// derived directly from the translator's single command declaration table plus
// the BGP send message types that `send bgp` exercises against peers.
func BridgeSends() []rpc.MessageSendDecl {
	prefixes := bridge.DeclaredCommandPrefixes()
	wireSends := []string{
		bgpevents.Namespace + ":" + bgpevents.SendUpdate,
		bgpevents.Namespace + ":" + bgpevents.SendRefresh,
		bgpevents.Namespace + ":" + bgpevents.SendRaw,
		bgpevents.Namespace + ":" + bgpevents.SendEnhancedRefresh,
	}
	decls := make([]rpc.MessageSendDecl, 0, len(prefixes)+len(wireSends))
	for _, prefix := range prefixes {
		decls = append(decls, rpc.MessageSendDecl{Message: prefix})
	}
	for _, msg := range wireSends {
		decls = append(decls, rpc.MessageSendDecl{Message: msg})
	}
	return decls
}

// BridgeReceives returns the BGP bus messages the ExaBGP wrapper subscribes to
// and translates onto script stdin, derived directly from the translator's
// single event declaration table.
func BridgeReceives() []rpc.MessageReceiveDecl {
	messages := bridge.DeclaredEventMessages()
	decls := make([]rpc.MessageReceiveDecl, len(messages))
	for i, msg := range messages {
		decls[i] = rpc.MessageReceiveDecl{Message: msg}
	}
	return decls
}
