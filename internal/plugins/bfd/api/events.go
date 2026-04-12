// Design: rfc/short/rfc5882.md -- BFD client contract
// Related: service.go -- consumer-facing Service interface
//
// Public types exposed to BFD clients (BGP, OSPF, static routes, ...).
//
// This package is intentionally minimal. It carries no implementation; the
// engine package provides the runtime that satisfies Service. Keeping the
// surface in its own package lets future external plugins depend on it
// without pulling in the express-loop machinery.
package api

import (
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// HopMode is the path category for a BFD session.
type HopMode uint8

const (
	// SingleHop is RFC 5881 single-hop BFD on UDP 3784. The session is
	// bound to an outgoing interface and enforces TTL=255 (GTSM).
	SingleHop HopMode = iota
	// MultiHop is RFC 5883 multi-hop BFD on UDP 4784. The session is keyed
	// on the address pair alone and enforces a configurable minimum TTL.
	MultiHop
)

// String returns the canonical name for the hop mode.
func (h HopMode) String() string {
	if h == SingleHop {
		return "single-hop"
	}
	if h == MultiHop {
		return "multi-hop"
	}
	return "invalid"
}

// SessionRequest describes a session a client wants the engine to maintain.
//
// Two requests with the same Key tuple coalesce into one session via
// refcounting (see RFC 5882 Section 2). When two clients disagree on
// timer parameters, the engine picks the more aggressive (smaller) value
// for each.
type SessionRequest struct {
	// Peer is the remote address. For single-hop sessions this is the
	// peer's address on the directly-connected link; for multi-hop it
	// is any routable address that reaches the peer.
	Peer netip.Addr

	// Local is the local source address. Single-hop sessions usually
	// derive this from the interface; multi-hop sessions may leave it
	// zero to let the routing table choose.
	Local netip.Addr

	// Interface is the egress interface name (single-hop only). Empty
	// for multi-hop sessions.
	Interface string

	// VRF identifies the routing/VRF instance. Empty means default VRF.
	VRF string

	// Mode selects single-hop or multi-hop semantics.
	Mode HopMode

	// DesiredMinTxInterval is the rate the local end wants to transmit
	// at, in microseconds. Zero means "use the engine default."
	DesiredMinTxInterval uint32

	// RequiredMinRxInterval is the rate the local end can receive at,
	// in microseconds. Zero means "use the engine default."
	RequiredMinRxInterval uint32

	// DetectMult is the local detection multiplier. Zero means default 3.
	DetectMult uint8

	// MinTTL is the minimum acceptable TTL on receive (multi-hop only).
	// Zero defaults to 254 (one hop allowed beyond the peer's first hop).
	MinTTL uint8

	// Passive flips the role from Active to Passive (RFC 5883 Section 4.3
	// unidirectional links). Most clients leave this false.
	Passive bool

	// Profile is the optional name of the YANG profile this request was
	// derived from. The engine does not consult it but exposes it through
	// Snapshot so operators see which profile a session inherits.
	Profile string

	// Auth is the RFC 5880 Section 6.7 authentication configuration
	// inherited from the profile. Nil means the session runs
	// unauthenticated. The engine passes this through to
	// session.Machine.SetAuth before any packets flow.
	Auth *AuthSettings

	// PersistDir is the absolute path where Meticulous Keyed
	// sequence numbers are persisted. Empty means Meticulous
	// variants still work at runtime but a process restart loses
	// the sequence floor until the peer's replay window slides.
	PersistDir string

	// DesiredMinEchoTxInterval requests RFC 5880 Section 6.4 Echo
	// mode at this rate (microseconds). Zero means echo is
	// disabled. Only valid on single-hop sessions; the config
	// parser rejects echo on multi-hop sessions because RFC 5883
	// Section 4 prohibits multi-hop echo.
	DesiredMinEchoTxInterval uint32
}

// AuthSettings is the subset of auth.Settings exported through the
// public api package. Duplicated here so the api package stays a
// leaf (no import of internal/plugins/bfd/auth from clients).
type AuthSettings struct {
	Type       uint8
	KeyID      uint8
	Secret     []byte //nolint:gosec // BFD auth key; AuthSettings is never serialized
	Meticulous bool
}

// Key is the tuple that uniquely identifies a session within an engine.
//
// The key intentionally excludes timer parameters: two clients with
// different timers but the same path share one session per RFC 5882.
type Key struct {
	Peer      netip.Addr
	Local     netip.Addr
	Interface string
	VRF       string
	Mode      HopMode
}

// Key derives the session key from a request.
func (r SessionRequest) Key() Key {
	return Key{
		Peer:      r.Peer,
		Local:     r.Local,
		Interface: r.Interface,
		VRF:       r.VRF,
		Mode:      r.Mode,
	}
}

// StateChange is the event a subscriber receives when a session crosses a
// state boundary. Subscribers receive the change after the engine has
// updated its own bookkeeping; the State field is the new state.
type StateChange struct {
	Key   Key
	State packet.State
	Diag  packet.Diag
	When  time.Time
}
