// Design: docs/architecture/rib-transition.md -- explicit role ownership.
// Related: server.go -- subscriptions; server_handlers.go -- destination OPEN and refresh.
package rs

import "github.com/ze-software/ze/pkg/plugin/rpc"

const disseminationRole = "bgp-selected-output"

func disseminationDecls() []rpc.DisseminationDecl {
	return []rpc.DisseminationDecl{{
		Namespace: "bgp", Role: disseminationRole,
		SourceReceive: []string{"update-received", "state"},
		// OPEN supplies the destination ASN and families used by replay and
		// forwarding; refresh requests read that same authorized state.
		DestinationReceive: []string{"state", "open-received", "refresh"},
		DestinationSend:    []string{"update"},
	}}
}
