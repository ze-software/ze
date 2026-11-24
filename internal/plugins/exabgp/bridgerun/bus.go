// Design: docs/architecture/exabgp-bridge.md -- bus declarations derived from the ExaBGP translator tables.
// Related: fleet.go -- script lifecycle for the ExaBGP wrapper.

package bridgerun

import (
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// BridgeSends returns the CLI command prefixes the ExaBGP wrapper translates
// from script stdout and dispatches to the daemon, derived directly from the
// translator's single command declaration table.
func BridgeSends() []rpc.MessageSendDecl {
	prefixes := bridge.DeclaredCommandPrefixes()
	decls := make([]rpc.MessageSendDecl, len(prefixes))
	for i, prefix := range prefixes {
		decls[i] = rpc.MessageSendDecl{Message: prefix}
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
