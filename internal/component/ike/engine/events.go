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

var (
	SAUp       = events.Register[*SAEvent](Namespace, "sa-up")
	SADown     = events.Register[*SAEvent](Namespace, "sa-down")
	ChildUp    = events.Register[*ChildSAEvent](Namespace, "child-up")
	ChildDown  = events.Register[*ChildSAEvent](Namespace, "child-down")
	ChildRekey = events.Register[*ChildSAEvent](Namespace, "child-rekey")
)
