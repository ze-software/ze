// Design: docs/architecture/rib-transition.md -- explicit role ownership.
// Related: rr.go -- subscriptions and destination OPEN family capture.
package rr

import "github.com/ze-software/ze/pkg/plugin/rpc"

const disseminationRole = "bgp-selected-output"

func disseminationDecls() []rpc.DisseminationDecl {
	return []rpc.DisseminationDecl{{
		Namespace: "bgp", Role: disseminationRole,
		SourceReceive: []string{"update-received", "state"},
		// The reflector's replay requires destination families captured from
		// OPEN. It subscribes to neither refresh nor sent UPDATE events.
		DestinationReceive: []string{"state", "open-received"},
		DestinationSend:    []string{"update"},
	}}
}
