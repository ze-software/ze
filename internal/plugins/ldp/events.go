// Design: plan/learned/920-mpls-ldp.md -- LDP event bus types
// Related: wire.go -- LDP message types used in events
package ldp

import "github.com/ze-software/ze/internal/core/events"

const Namespace = "ldp"

const (
	EventSessionUp   = "session-up"
	EventSessionDown = "session-down"
	EventLabelBind   = "label-bind"
)

// SessionEvent carries LDP session lifecycle information on the event bus.
type SessionEvent struct {
	PeerAddress   string `json:"peer-address"`
	TransportAddr string `json:"transport-address"`
	LDPIdentifier string `json:"ldp-identifier"`
	SessionState  string `json:"session-state"`
	HoldTime      uint16 `json:"hold-time"`
	KeepaliveTime uint16 `json:"keepalive-time"`
	// Interface is the local interface the LDP adjacency was discovered on (the
	// discovering interface name). It lets an IGP LDP-IGP-sync consumer (RFC 5443 /
	// RFC 6138) map a session event to one of its interfaces without reverse-mapping
	// a transport address. Empty for any emitter that has no interface context (e.g.
	// a targeted session); such an event is a no-op for interface-scoped consumers.
	Interface string `json:"interface"`
}

// LabelBindEvent carries label binding information on the event bus.
type LabelBindEvent struct {
	FEC      string `json:"fec"`
	Label    uint32 `json:"label"`
	PeerAddr string `json:"peer-address"`
	Action   string `json:"action"`
}

var (
	SessionUp   = events.Register[*SessionEvent](Namespace, EventSessionUp)
	SessionDown = events.Register[*SessionEvent](Namespace, EventSessionDown)
	LabelBind   = events.Register[*LabelBindEvent](Namespace, EventLabelBind)
)
