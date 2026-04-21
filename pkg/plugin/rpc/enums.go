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
)

// MessageDirection is the typed wire direction of a BGP message from the
// reactor's perspective. Wire form: "sent", "received".
type MessageDirection uint8

const (
	DirectionUnspecified MessageDirection = 0
	DirectionSent        MessageDirection = 1
	DirectionReceived    MessageDirection = 2
)

func (d MessageDirection) String() string {
	switch d {
	case DirectionSent:
		return wireSent
	case DirectionReceived:
		return wireReceived
	case DirectionUnspecified:
		return wireUnspecified
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
	}
	return wireUnspecified
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
	}
	return wireUnspecified
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
	}
	return wireUnspecified
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
	}
	return wireUnspecified
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
	wireUp   = "up"
	wireDown = "down"
)

// SessionState is the typed peer session lifecycle state carried by
// StructuredEvent.State. Wire form: "up", "down".
type SessionState uint8

const (
	SessionStateUnspecified SessionState = 0
	SessionStateUp          SessionState = 1
	SessionStateDown        SessionState = 2
	SessionStateCount       SessionState = 3
)

func (s SessionState) String() string {
	switch s {
	case SessionStateUp:
		return wireUp
	case SessionStateDown:
		return wireDown
	case SessionStateUnspecified, SessionStateCount:
		return wireUnspecified
	}
	return wireUnspecified
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
