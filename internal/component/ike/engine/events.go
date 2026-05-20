// Design: plan/spec-ipsec-7-ikev2-engine.md -- IKE SA lifecycle events
package engine

import "codeberg.org/thomas-mangin/ze/internal/core/events"

const Namespace = "vpn-ipsec"

// SAEvent carries IKE SA lifecycle information on the event bus.
type SAEvent struct {
	PeerName      string `json:"peer-name"`
	InitiatorSPI  string `json:"initiator-spi"`
	ResponderSPI  string `json:"responder-spi"`
	RemoteAddress string `json:"remote-address"`
	AuthMethod    string `json:"auth-method"`
}

var (
	SAUp   = events.Register[*SAEvent](Namespace, "sa-up")
	SADown = events.Register[*SAEvent](Namespace, "sa-down")
)
