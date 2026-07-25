// Design: plan/learned/930-isis-4-component-config.md -- IS-IS event bus types
// Related: register.go -- registers the event namespace and wires the EventBus
// Related: server.go -- the engine emits session up/down through the eventSink here
//
// The IS-IS component publishes lifecycle events on the core event bus so the
// web UI, looking glass, and other consumers can react to adjacency and LSDB
// changes without polling. This spec declares the namespace and the typed event
// payloads; the runtime siblings (isis-5 adjacency, isis-6 LSDB) emit them. The
// namespace itself is registered by registerISIS (registration pattern), not in
// an init() here. The eventSink adapter (isis-5) bridges the circuit package's
// EventSink interface to the typed SessionUp/SessionDown handles.

package isis

import (
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/pkg/ze"
)

// Namespace is the IS-IS event-bus namespace.
const Namespace = "isis"

// IS-IS event types.
const (
	// EventSessionUp fires when an adjacency reaches the Up state (isis-5).
	EventSessionUp = "session-up"
	// EventSessionDown fires when an adjacency leaves the Up state (isis-5).
	EventSessionDown = "session-down"
	// EventLSPChange fires when the LSDB gains, refreshes, or purges an LSP
	// (isis-6).
	EventLSPChange = "lsp-change"
)

// SessionEvent carries IS-IS adjacency lifecycle information on the event bus.
type SessionEvent struct {
	Interface  string `json:"interface"`
	Level      string `json:"level"`       // "l1" | "l2"
	NeighborID string `json:"neighbor-id"` // neighbor System ID (dotted hex)
	State      string `json:"state"`       // adjacency FSM state
}

// LSPChangeEvent carries an LSDB change on the event bus.
type LSPChangeEvent struct {
	Level    string `json:"level"`    // "l1" | "l2"
	LSPID    string `json:"lsp-id"`   // LSPID (dotted hex)
	Sequence uint32 `json:"sequence"` // LSP sequence number
	Action   string `json:"action"`   // "add" | "refresh" | "purge"
}

// Typed event handles. registerISIS (register.go) calls
// events.RegisterNamespace with the same event-type strings so the namespace
// knows its valid events; events.Register binds each payload type to its
// event-type string.
var (
	SessionUp   = events.Register[*SessionEvent](Namespace, EventSessionUp)
	SessionDown = events.Register[*SessionEvent](Namespace, EventSessionDown)
	LSPChange   = events.Register[*LSPChangeEvent](Namespace, EventLSPChange)
)

// eventSink adapts the circuit package's EventSink interface to the typed
// SessionUp/SessionDown event handles, emitting on the supplied EventBus. The
// circuit calls SessionUp/SessionDown on a transition; the sink maps the
// neighbor snapshot to the SessionEvent payload and emits. A nil bus makes the
// sink a no-op (the engine ran without an event bus).
type eventSink struct {
	bus ze.EventBus
}

// newEventSink constructs an eventSink emitting on bus.
func newEventSink(bus ze.EventBus) *eventSink { return &eventSink{bus: bus} }

// SessionUp emits an IS-IS session-up event from an adjacency snapshot row.
func (s *eventSink) SessionUp(n adjacency.NeighborSnapshot) {
	if s == nil || s.bus == nil {
		return
	}
	if _, err := SessionUp.Emit(s.bus, sessionEventOf(n)); err != nil {
		logger().Debug("isis: session-up emit", "neighbor", n.SystemID, "err", err)
	}
}

// SessionDown emits an IS-IS session-down event from an adjacency snapshot row.
func (s *eventSink) SessionDown(n adjacency.NeighborSnapshot) {
	if s == nil || s.bus == nil {
		return
	}
	if _, err := SessionDown.Emit(s.bus, sessionEventOf(n)); err != nil {
		logger().Debug("isis: session-down emit", "neighbor", n.SystemID, "err", err)
	}
}

// sessionEventOf maps a neighbor snapshot row to the SessionEvent payload.
func sessionEventOf(n adjacency.NeighborSnapshot) *SessionEvent {
	return &SessionEvent{
		Level:      n.Level,
		NeighborID: n.SystemID,
		State:      n.State,
	}
}
