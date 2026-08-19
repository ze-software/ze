// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- IKE SA lifecycle events
package engine

import "github.com/ze-software/ze/internal/core/events"

const Namespace = "vpn-ipsec"

// SAEvent carries IKE SA lifecycle information on the event bus.
type SAEvent struct {
	PeerName      string `json:"peer-name"`
	InitiatorSPI  string `json:"initiator-spi"`
	ResponderSPI  string `json:"responder-spi"`
	RemoteAddress string `json:"remote-address"`
	AuthMethod    string `json:"auth-method"`
}

// ChildSAEvent carries Child SA lifecycle information on the event bus.
type ChildSAEvent struct {
	PeerName    string `json:"peer-name"`
	InboundSPI  uint32 `json:"inbound-spi"`
	OutboundSPI uint32 `json:"outbound-spi"`
	IfID        uint32 `json:"if-id"`
	TSLocal     string `json:"ts-local,omitempty"`
	TSRemote    string `json:"ts-remote,omitempty"`
}

// SAUp and SADown are a PAIR, and every producer of one MUST produce the other.
// A path that emits SAUp for an IKE SA MUST emit exactly one SADown when that SA
// goes down, on every way out including the error ways, and MUST NOT emit a second
// one for the same SA. Subscribers count them against each other -- a `show` view,
// a metric, a fleet dashboard -- so an unpaired emit is drift that never settles,
// once per reconnect rather than once per process.
//
// The two owner loops are the model: runInitiator and runResponder each emit SAUp
// at establishment and call emitSADown when their runEstablished returns, and each
// clears ps.sa there so the operator teardown paths (TerminatePeerSA,
// TerminateAllSAs, reconcilePeers) do not emit a second down for the same SA.
var (
	SAUp       = events.Register[*SAEvent](Namespace, "sa-up")
	SADown     = events.Register[*SAEvent](Namespace, "sa-down")
	ChildUp    = events.Register[*ChildSAEvent](Namespace, "child-up")
	ChildDown  = events.Register[*ChildSAEvent](Namespace, "child-down")
	ChildRekey = events.Register[*ChildSAEvent](Namespace, "child-rekey")
)
