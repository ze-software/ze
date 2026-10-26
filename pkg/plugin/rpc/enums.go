// Design: docs/architecture/api/ipc_protocol.md -- typed RPC enums
// Related: types.go -- struct fields that reference these enums
//
// Typed enums for wire-level string constants carried by plugin RPC.
// Each enum has: zero-invalid sentinel, const values, String/AppendTo for
// zero-alloc diagnostics, MarshalText/UnmarshalText for JSON wire
// compatibility with external plugins. Wire format is unchanged; Go-to-Go
// consumers compare on typed identity instead of string roundtripping.

package rpc

import "fmt"

// Shared wire-level string constants. Reused across enum String() methods
// so a single token produces a single interned literal and a single source
// of truth defines the on-wire spellings.
const (
	wireAccept      = "accept"
	wireReject      = "reject"
	wireModify      = "modify"
	wireImport      = "import"
	wireExport      = "export"
	wireBoth        = "both"
	wireHex         = "hex"
	wireBase64      = "b64"
	wireText        = "text"
	wireSent        = "sent"
	wireReceived    = "received"
	wireUnspecified = "unspecified"
	wireRestart     = "restart"
	wireIgnore      = "ignore"
	wireFatal       = "fatal"
)

// FailurePolicy is what a plugin asks ze to do when the plugin fails. The
// plugin declares it in DeclareRegistrationInput, and the engine applies it
// when the plugin's process ends or its startup handshake fails.
// Wire form: "restart", "ignore", "fatal".
//
// One value answers two questions, because ze starts a plugin again for exactly
// one reason, which is that the plugin failed. FailureRestart says what happens
// on a failure AND states that the plugin may be started again. The other two
// say it MUST NOT be, so a configuration that asks for a respawn disagrees with
// them and ze refuses to start (Server.applyFailurePolicy,
// internal/component/plugin/server/failure_policy.go).
type FailurePolicy uint8

const (
	// FailureUnspecified is what a plugin that declared nothing carries. It is
	// never an answer: pluginFailurePolicy is the one function that turns it
	// into an outcome, and it reads it as FailureIgnore.
	FailureUnspecified FailurePolicy = 0
	FailureRestart     FailurePolicy = 1
	FailureIgnore      FailurePolicy = 2
	FailureFatal       FailurePolicy = 3
)

var failurePolicyStrings = [4]string{
	FailureUnspecified: wireUnspecified,
	FailureRestart:     wireRestart,
	FailureIgnore:      wireIgnore,
	FailureFatal:       wireFatal,
}

func (p FailurePolicy) String() string {
	if p < FailurePolicy(len(failurePolicyStrings)) {
		return failurePolicyStrings[p]
	}
	return wireUnspecified
}

func (p FailurePolicy) AppendTo(buf []byte) []byte { return append(buf, p.String()...) }

// AllowsRestart reports whether the plugin permits ze to start it again after a
// failure. Only a plugin that declared FailureRestart does. A plugin that
// declared nothing does not, because ze must not read silence as consent.
func (p FailurePolicy) AllowsRestart() bool { return p == FailureRestart }

func (p FailurePolicy) MarshalText() ([]byte, error) {
	if p == FailureUnspecified {
		return nil, fmt.Errorf("rpc: unspecified FailurePolicy is invalid on the wire")
	}
	return []byte(p.String()), nil
}

func (p *FailurePolicy) UnmarshalText(data []byte) error {
	switch string(data) {
	case wireRestart:
		*p = FailureRestart
	case wireIgnore:
		*p = FailureIgnore
	case wireFatal:
		*p = FailureFatal
	default:
		return fmt.Errorf("rpc: unknown failure policy %q: write %q, %q or %q",
			string(data), wireRestart, wireIgnore, wireFatal)
	}
	return nil
}

// MessageDirection is the typed wire direction of a BGP message from the
// reactor's perspective. Wire form: "sent", "received".
type MessageDirection uint8

const (
	DirectionUnspecified MessageDirection = 0
	DirectionSent        MessageDirection = 1
	DirectionReceived    MessageDirection = 2
)

var directionStrings = [3]string{
	DirectionUnspecified: wireUnspecified,
	DirectionSent:        wireSent,
	DirectionReceived:    wireReceived,
}

func (d MessageDirection) String() string {
	if d < MessageDirection(len(directionStrings)) {
		return directionStrings[d]
	}
	return wireUnspecified
}

func (d MessageDirection) AppendTo(buf []byte) []byte { return append(buf, d.String()...) }

func (d MessageDirection) MarshalText() ([]byte, error) {
	if d == DirectionUnspecified {
		return nil, fmt.Errorf("rpc: unspecified MessageDirection is invalid on the wire")
	}
	return []byte(d.String()), nil
}

func (d *MessageDirection) UnmarshalText(data []byte) error {
	switch string(data) {
	case wireSent:
		*d = DirectionSent
	case wireReceived:
		*d = DirectionReceived
	default:
		return fmt.Errorf("rpc: unknown message direction %q", string(data))
	}
	return nil
}

// FilterAction is the typed wire decision returned by a filter plugin
// via FilterUpdateOutput.Action. Wire form: "accept", "reject", "modify".
type FilterAction uint8

const (
	FilterUnspecified FilterAction = 0
	FilterAccept      FilterAction = 1
	FilterReject      FilterAction = 2
	FilterModify      FilterAction = 3
)

func (a FilterAction) String() string {
	switch a {
	case FilterAccept:
		return wireAccept
	case FilterReject:
		return wireReject
	case FilterModify:
		return wireModify
	case FilterUnspecified:
		return wireUnspecified
	default:
		// Plugin API values are open; unknown actions retain the unspecified spelling.
		return wireUnspecified
	}
}

func (a FilterAction) AppendTo(buf []byte) []byte { return append(buf, a.String()...) }

func (a FilterAction) MarshalText() ([]byte, error) {
	if a == FilterUnspecified {
		return nil, fmt.Errorf("rpc: unspecified FilterAction is invalid on the wire")
	}
	return []byte(a.String()), nil
}

func (a *FilterAction) UnmarshalText(data []byte) error {
	switch string(data) {
	case wireAccept:
		*a = FilterAccept
	case wireReject:
		*a = FilterReject
	case wireModify:
		*a = FilterModify
	default:
		return fmt.Errorf("rpc: unknown filter action %q", string(data))
	}
	return nil
}

// FilterDirection is the typed wire direction for a named filter declared
// in FilterDecl.Direction. Wire form: "import", "export", "both".
type FilterDirection uint8

const (
	FilterDirectionUnspecified FilterDirection = 0
	FilterImport               FilterDirection = 1
	FilterExport               FilterDirection = 2
	FilterBoth                 FilterDirection = 3
)

func (d FilterDirection) String() string {
	switch d {
	case FilterImport:
		return wireImport
	case FilterExport:
		return wireExport
	case FilterBoth:
		return wireBoth
	case FilterDirectionUnspecified:
		return wireUnspecified
	default:
		// Plugin API values are open; unknown directions retain the unspecified spelling.
		return wireUnspecified
	}
}

func (d FilterDirection) AppendTo(buf []byte) []byte { return append(buf, d.String()...) }

func (d FilterDirection) MarshalText() ([]byte, error) {
	if d == FilterDirectionUnspecified {
		return nil, fmt.Errorf("rpc: unspecified FilterDirection is invalid on the wire")
	}
	return []byte(d.String()), nil
}

func (d *FilterDirection) UnmarshalText(data []byte) error {
	switch string(data) {
	case wireImport:
		*d = FilterImport
	case wireExport:
		*d = FilterExport
	case wireBoth:
		*d = FilterBoth
	default:
		return fmt.Errorf("rpc: unknown filter direction %q", string(data))
	}
	return nil
}

// OnErrorPolicy is the typed wire failure policy for a named filter
// declared in FilterDecl.OnError. Wire form: "reject", "accept".
type OnErrorPolicy uint8

const (
	OnErrorUnspecified OnErrorPolicy = 0
	OnErrorReject      OnErrorPolicy = 1
	OnErrorAccept      OnErrorPolicy = 2
)

func (p OnErrorPolicy) String() string {
	switch p {
	case OnErrorReject:
		return wireReject
	case OnErrorAccept:
		return wireAccept
	case OnErrorUnspecified:
		return wireUnspecified
	default:
		// Plugin API values are open; unknown policies retain the unspecified spelling.
		return wireUnspecified
	}
}

func (p OnErrorPolicy) AppendTo(buf []byte) []byte { return append(buf, p.String()...) }

func (p OnErrorPolicy) MarshalText() ([]byte, error) {
	if p == OnErrorUnspecified {
		return nil, fmt.Errorf("rpc: unspecified OnErrorPolicy is invalid on the wire")
	}
	return []byte(p.String()), nil
}

func (p *OnErrorPolicy) UnmarshalText(data []byte) error {
	switch string(data) {
	case wireReject:
		*p = OnErrorReject
	case wireAccept:
		*p = OnErrorAccept
	default:
		return fmt.Errorf("rpc: unknown on-error policy %q", string(data))
	}
	return nil
}

// CapEncoding is the typed wire encoding for a capability's payload
// declared in CapabilityDecl.Encoding. Wire form: "hex", "b64", "text".
// Named CapEncoding (not EncodingFormat) to avoid confusion with
// internal/component/plugin WireEncoding, which is a different domain
// (BGP subscription output encoding).
type CapEncoding uint8

const (
	CapEncodingUnspecified CapEncoding = 0
	CapEncodingHex         CapEncoding = 1
	CapEncodingBase64      CapEncoding = 2
	CapEncodingText        CapEncoding = 3
)

func (e CapEncoding) String() string {
	switch e {
	case CapEncodingHex:
		return wireHex
	case CapEncodingBase64:
		return wireBase64
	case CapEncodingText:
		return wireText
	case CapEncodingUnspecified:
		return wireUnspecified
	default:
		// Plugin API values are open; unknown encodings retain the unspecified spelling.
		return wireUnspecified
	}
}

func (e CapEncoding) AppendTo(buf []byte) []byte { return append(buf, e.String()...) }

func (e CapEncoding) MarshalText() ([]byte, error) {
	if e == CapEncodingUnspecified {
		return nil, fmt.Errorf("rpc: unspecified CapEncoding is invalid on the wire")
	}
	return []byte(e.String()), nil
}

func (e *CapEncoding) UnmarshalText(data []byte) error {
	switch string(data) {
	case wireHex:
		*e = CapEncodingHex
	case wireBase64:
		*e = CapEncodingBase64
	case wireText:
		*e = CapEncodingText
	default:
		return fmt.Errorf("rpc: unknown capability encoding %q", string(data))
	}
	return nil
}

const (
	wireUp           = "up"
	wireDown         = "down"
	wireUpdate       = "update"
	wireOpen         = "open"
	wireNotification = "notification"
	wireKeepalive    = "keepalive"
	wireRefresh      = "refresh"
	wireState        = "state"
	wireEOR          = "eor"
	wireNegotiated   = "negotiated"
)

// EventKind is the typed BGP event kind carried by StructuredEvent.EventType.
// Wire form: "update", "open", "notification", "keepalive", "refresh", "state", "eor".
type EventKind uint8

const (
	EventKindUnspecified  EventKind = 0
	EventKindUpdate       EventKind = 1
	EventKindOpen         EventKind = 2
	EventKindNotification EventKind = 3
	EventKindKeepalive    EventKind = 4
	EventKindRefresh      EventKind = 5
	EventKindState        EventKind = 6
	EventKindEOR          EventKind = 7
	EventKindBoRR         EventKind = 8
	EventKindEoRR         EventKind = 9
	EventKindSent         EventKind = 10
	EventKindNegotiated   EventKind = 11
	EventKindCount        EventKind = 12
)

var eventKindStrings = [EventKindCount]string{
	EventKindUnspecified:  wireUnspecified,
	EventKindUpdate:       wireUpdate,
	EventKindOpen:         wireOpen,
	EventKindNotification: wireNotification,
	EventKindKeepalive:    wireKeepalive,
	EventKindRefresh:      wireRefresh,
	EventKindState:        wireState,
	EventKindEOR:          wireEOR,
	EventKindBoRR:         "borr",
	EventKindEoRR:         "eorr",
	EventKindSent:         wireSent,
	EventKindNegotiated:   wireNegotiated,
}

func (k EventKind) String() string {
	if k < EventKindCount {
		return eventKindStrings[k]
	}
	return wireUnspecified
}

func (k EventKind) AppendTo(buf []byte) []byte { return append(buf, k.String()...) }

func (k EventKind) MarshalText() ([]byte, error) {
	if k == EventKindUnspecified || k >= EventKindCount {
		return nil, fmt.Errorf("rpc: invalid EventKind %d on the wire", k)
	}
	return []byte(k.String()), nil
}

func (k *EventKind) UnmarshalText(data []byte) error {
	switch string(data) {
	case wireUpdate:
		*k = EventKindUpdate
	case wireOpen:
		*k = EventKindOpen
	case wireNotification:
		*k = EventKindNotification
	case wireKeepalive:
		*k = EventKindKeepalive
	case wireRefresh:
		*k = EventKindRefresh
	case wireState:
		*k = EventKindState
	case wireEOR:
		*k = EventKindEOR
	case "borr":
		*k = EventKindBoRR
	case "eorr":
		*k = EventKindEoRR
	case wireSent:
		*k = EventKindSent
	case wireNegotiated:
		*k = EventKindNegotiated
	default:
		*k = EventKindUnspecified
		return fmt.Errorf("rpc: unknown event kind %q", string(data))
	}
	return nil
}

// SessionState is the typed peer session lifecycle state carried by
// StructuredEvent.State. Wire form: "up", "down".
type SessionState uint8

const (
	SessionStateUnspecified SessionState = 0
	SessionStateUp          SessionState = 1
	SessionStateDown        SessionState = 2
	SessionStateCount       SessionState = 3
)

// ReasonPeerRemoved is the SessionStateDown reason emitted when a peer is
// removed from configuration (deconfigured), as opposed to a transient session
// drop. Plugins use it to release all per-peer state rather than treating the
// down as recoverable: e.g. Graceful Restart skips route retention and deletes
// its per-peer Prometheus series instead of starting a restart timer. Emitted
// unconditionally by the reactor's removePeer (not via the FSM teardown), so it
// fires even for peers that are not Established at removal time.
const ReasonPeerRemoved = "removed"

// ReasonAttached is the SessionStateUp reason emitted when a reload attaches a
// process to a session that was already up. The process is told the session is
// up before any message of it reaches the process, and the event is not a new
// session: it carries no initial-replay token, arms no End-of-RIB barrier, and
// a plugin that replays routes toward a peer on up skips the replay for it.
// Emitted by the reactor's ordered bind delivery (peer_derived_bindings.go)
// through the plugin server's targeted up, to the attached processes alone.
const ReasonAttached = "attached"

// The four SessionStateDown reasons below say why an Established session
// closed. The reactor derives one from what the session observed
// (internal/component/bgp/reactor/peer_close.go, sessionCloseFor), and every
// plugin that has to act on the cause reads it here rather than guessing from
// prose: Graceful Restart decides route retention on it (RFC 4724 Section 4)
// and BMP picks the Peer Down reason (RFC 7854 Section 4.9). Whether the
// NOTIFICATION was sent or received is not repeated here: the NOTIFICATION
// message event that preceded the down already carries its direction and PDU.

// ReasonNotification is the down reason for a session that ended because a
// NOTIFICATION was sent to, or received from, the peer.
const ReasonNotification = "notification"

// ReasonTCPFailure is the down reason for a session whose transport ended
// with no NOTIFICATION exchanged: RFC 4271 Section 8.1 Event 18.
const ReasonTCPFailure = "tcp-failure"

// ReasonLocalClose is the down reason for a session ze closed itself with no
// NOTIFICATION reaching the peer, for example when the Cease could not be
// written. The FSM event that closed it travels beside it (StructuredEvent
// FSMEvent).
const ReasonLocalClose = "local-close"

// ReasonAdjRIBInInvalid is the down reason for a session ze ended because it
// could not publish the UPDATEs it received from the peer, when no
// NOTIFICATION crossed the wire: the Cease ze sent was not written, or the
// session had already ended when the publication failed. The routes the
// session's Adj-RIB-In holds are known to be invalid, so no plugin may keep
// them. When the Cease was delivered the reason is ReasonNotification.
const ReasonAdjRIBInInvalid = "adj-rib-in-invalid"

// ReasonForbidsRetention reports whether a SessionStateDown reason forbids
// keeping the peer's routes past the down, which Graceful Restart otherwise
// does. A plugin that retains routes asks this rather than comparing reasons,
// so a reason added later is decided here once.
//
// RFC 4724 Section 4: "It is noted that the normal BGP procedures MUST be
// followed when the TCP session terminates due to the sending or receiving of
// a BGP NOTIFICATION message." A session whose received routes ze could not
// publish takes the same procedures: a route kept from it would be one no
// plugin ever saw, or one the peer had already withdrawn.
func ReasonForbidsRetention(reason string) bool {
	if reason == ReasonNotification {
		return true
	}
	return reason == ReasonAdjRIBInInvalid
}

func (s SessionState) String() string {
	switch s {
	case SessionStateUp:
		return wireUp
	case SessionStateDown:
		return wireDown
	case SessionStateUnspecified, SessionStateCount:
		return wireUnspecified
	default:
		// Plugin API values are open; unknown states retain the unspecified spelling.
		return wireUnspecified
	}
}

func (s SessionState) AppendTo(buf []byte) []byte { return append(buf, s.String()...) }

func (s SessionState) MarshalText() ([]byte, error) {
	if s == SessionStateUnspecified || s >= SessionStateCount {
		return nil, fmt.Errorf("rpc: invalid SessionState %d on the wire", s)
	}
	return []byte(s.String()), nil
}

func (s *SessionState) UnmarshalText(data []byte) error {
	switch string(data) {
	case wireUp:
		*s = SessionStateUp
	case wireDown:
		*s = SessionStateDown
	default:
		return fmt.Errorf("rpc: unknown session state %q", string(data))
	}
	return nil
}
