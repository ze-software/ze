// Design: docs/architecture/rib-transition.md -- ordinary selected output ownership.
// Related: rib.go -- received, sent, state and refresh subscriptions.
package rib

import "github.com/ze-software/ze/pkg/plugin/rpc"

// The role is opaque to infrastructure; participating plugins agree on it
// without importing each other's implementations.
const disseminationRole = "bgp-selected-output"

const stateEvent = "state"

func disseminationDecls() []rpc.DisseminationDecl {
	return []rpc.DisseminationDecl{{
		Namespace: "bgp", Role: disseminationRole,
		SourceReceive:      []string{"update-received", stateEvent},
		DestinationReceive: []string{stateEvent, "refresh", "update-sent"},
		DestinationSend:    []string{"update"},
	}}
}
