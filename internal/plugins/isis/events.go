// Design: docs/architecture/isis/isis-4-component-config.md -- IS-IS event bus types
// Related: register.go -- registers the event namespace and wires the EventBus
// Related: server.go -- the engine emits session up/down through the eventSink here
// Related: issues.go -- a refused emit is raised through the report queue there
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
	"context"
	"log/slog"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/pkg/ze"
)

// labelLevel is the Prometheus label that carries the IS-IS level (1 or 2) a
// sample belongs to.
const labelLevel = "level"

// Namespace is the IS-IS event-bus namespace.
const Namespace = "isis"

// componentName is the name the component registry, the plugin SDK and the
// doctor check know this component by.
const componentName = "isis"

// configRoot is the YANG container this component reads its config from. It
// holds the same text as componentName and means something else: one names the
// component, the other names a subtree of the config, so they are two constants.
const configRoot = "isis"

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
		warnSessionEventRefused(EventSessionUp, n, err)
	}
}

// SessionDown emits an IS-IS session-down event from an adjacency snapshot row.
func (s *eventSink) SessionDown(n adjacency.NeighborSnapshot) {
	if s == nil || s.bus == nil {
		return
	}
	if _, err := SessionDown.Emit(s.bus, sessionEventOf(n)); err != nil {
		warnSessionEventRefused(EventSessionDown, n, err)
	}
}

// showNeighborsCommand shows the IS-IS adjacencies as the engine holds them, the
// state a consumer that missed a session event can read instead.
const showNeighborsCommand = "ze show isis neighbor"

// showDatabaseCommand shows the IS-IS LSDB as the engine holds it, the state a
// consumer that missed an lsp-change event can read instead.
const showDatabaseCommand = "ze show isis database"

// warnSessionEventRefused reports a session event the bus refused to publish.
// The bus refuses an emit only for an event pair its registry does not hold
// (ze.EventBus.Emit), and registerISIS registers both session events, so only a
// defect in Ze gets here: the line carries doctor-internal-defect and the error
// event is raised under it through the SDK (issues.go), as every IS-IS problem
// is. The adjacency itself is unaffected, which is why the circuit goes on: only
// the plugins that follow IS-IS adjacencies miss the change. An adjacency changes
// state at most once per Hello exchange, so a line for each refusal does not
// flood the log.
func warnSessionEventRefused(eventType string, n adjacency.NeighborSnapshot, err error) {
	issue := opererr.Wrap(err,
		"IS-IS did not publish the "+eventType+" event of neighbor "+n.SystemID+" at level "+n.Level,
		"Plugins that follow IS-IS adjacencies missed this change, and only a defect in Ze refuses the event; "+
			"the adjacency itself is unaffected",
		showNeighborsCommand)
	subject := n.SystemID + " " + n.Level
	reportErr := issueQueue.Load().RaiseError(diagnostic.CodeDoctorInternalDefect, subject,
		issue.Problem(), issue.Fix(), internalDefectRaiseFailed(logger(), subject, issue.Fix()))
	logger().LogAttrs(context.Background(), slog.LevelWarn, issue.Problem(),
		issue.LogAttrs(slog.String("code", diagnostic.CodeDoctorInternalDefect),
			slog.String("event", eventType),
			slog.String("neighbor", n.SystemID),
			slog.String("level", n.Level),
			issuequeue.Outcome(reportErr))...)
}

// noteLSPChangeRefused counts an lsp-change event the bus refused to publish
// and logs the first one. Like a session event, the emit is refused only for an
// event pair the registry does not hold, so only a defect in Ze gets here and
// the database itself is unaffected. Unlike a session event it is per LSP: every
// LSP received, flooded, refreshed or purged publishes one, and a refused pair
// stays refused for the life of the process. So the first refusal WARNs and
// raises the error event, and ze_isis_lsp_change_events_refused_total counts
// every one. Safe for concurrent use.
func (e *engine) noteLSPChangeRefused(level, lspID string, err error) {
	e.mu.Lock()
	refused := e.lspChangesRefused
	e.mu.Unlock()
	refused.Inc()
	if !e.lspChangeRefusalLogged.CompareAndSwap(false, true) {
		return
	}
	issue := opererr.Wrap(err,
		"IS-IS did not publish the lsp-change event of LSP "+lspID+" at level "+level,
		"Plugins that follow the IS-IS database miss its changes, and only a defect in Ze refuses the event; "+
			"the database itself is unaffected, and ze_isis_lsp_change_events_refused_total counts the later refusals, "+
			"which are not logged",
		showDatabaseCommand)
	reportErr := issueQueue.Load().RaiseError(diagnostic.CodeDoctorInternalDefect, lspID,
		issue.Problem(), issue.Fix(), internalDefectRaiseFailed(e.log, lspID, issue.Fix()))
	e.log.LogAttrs(context.Background(), slog.LevelWarn, issue.Problem(),
		issue.LogAttrs(slog.String("code", diagnostic.CodeDoctorInternalDefect),
			slog.String("event", EventLSPChange),
			slog.String("lsp-id", lspID),
			slog.String("level", level),
			issuequeue.Outcome(reportErr))...)
}

// sessionEventOf maps a neighbor snapshot row to the SessionEvent payload.
func sessionEventOf(n adjacency.NeighborSnapshot) *SessionEvent {
	return &SessionEvent{
		Level:      n.Level,
		NeighborID: n.SystemID,
		State:      n.State,
	}
}
