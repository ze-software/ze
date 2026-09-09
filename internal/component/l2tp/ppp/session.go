// Design: docs/research/l2tpv2-ze-integration.md -- per-session PPP state ownership
// RFC: rfc/short/rfc5072.md -- RFC 5072 Section 4.1 (Interface-Identifier comparison outcomes, referenced by peerInterfaceIDNegotiated)
// Related: manager.go -- Driver owns pppSession values in sessions map
// Related: session_run.go -- per-session goroutine main loop
// Related: auth_events.go -- authEventsOut + authRespCh fields on pppSession

package ppp

import (
	"io"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// pppSession is the per-session state owned by one goroutine in the
// Manager. Field writes are exclusive to the goroutine that owns the
// session; reads from outside (e.g. SessionByID for `show l2tp
// session`) MUST acquire mu.
//
// Mirrors the L2TP reactor's per-tunnel ownership model: writer is
// the goroutine, readers lock briefly. RFC 1661 FSM transitions are
// dispatched via LCPDoTransition (pure function, no I/O) and the
// resulting actions are performed inline by the goroutine.
type pppSession struct {
	// Identification (immutable after StartSession).
	tunnelID  uint16
	sessionID uint16

	// Underlying I/O. chanFile is the wrapped chan fd for WRITING
	// PPP frames to the wire. unitFile is the wrapped unit fd for
	// READING PPP frames (the kernel delivers received frames to the
	// unit after PPPIOCCONNECT). framesIn is fed by readFrames which
	// reads from unitFile.
	chanFile io.ReadWriteCloser
	unitFile io.ReadCloser
	framesIn <-chan []byte
	chanFD   int
	unitFD   int
	unitNum  int
	lnsMode  bool

	// Configuration captured from StartSession (immutable after
	// goroutine start).
	maxMRU            uint16
	echoInterval      time.Duration
	echoFailures      uint8
	authTimeout       time.Duration
	authRequired      bool
	authFallbackOrder []AuthMethod
	reauthInterval    time.Duration

	// configuredAuthMethod is the Auth-Protocol value ze will
	// advertise in its next LCP Configure-Request. Initialized from
	// StartSession.AuthMethod by manager.spawnSession BEFORE the
	// per-session goroutine starts (happens-before established by
	// the spawn); thereafter it is goroutine-owned. The session
	// goroutine mutates it on LCP Configure-Nak / Configure-Reject
	// of the Auth-Protocol option (RFC 1661 §5.3-5.4, spec Phase 8,
	// AC-13). No lock is held on read/write because the goroutine
	// is the sole accessor after spawn.
	configuredAuthMethod AuthMethod

	// Magic-Number for THIS session. Generated via crypto/rand by
	// the goroutine on entry; non-zero per RFC 1661 §6.4.
	magic uint32

	// Per-session CHAP Identifier counter. Shared between CHAP-MD5
	// (runCHAPAuthPhase) and MS-CHAPv2 (runMSCHAPv2AuthPhase) because
	// LCP negotiates exactly one Auth-Protocol method per session and
	// both methods use the same RFC 1994 Identifier-MUST-change-per-
	// Challenge rule (RFC 2759 Section 4 inherits the discipline).
	// Wraps at 256 which is far beyond any realistic re-auth cadence.
	chapIdentifier uint8

	// Iface backend for setting pppN MTU after LCP-Opened.
	backend IfaceBackend

	// pppOps for ioctls (PPPIOCSMRU). Injected for tests.
	ops pppOps

	// Driver's lifecycle events channel (write-only from this
	// goroutine). Carries Event (EventLCPUp / EventSessionUp / ...).
	eventsOut chan<- Event

	// Driver's auth events channel (write-only from this goroutine).
	// Carries EventAuthRequest / EventAuthSuccess / EventAuthFailure
	// consumed by the external auth handler (l2tp-auth plugin in
	// production, auto-accept responder in tests). Separate from
	// eventsOut so L2TP's reactor is not forced to pattern-match
	// auth types -- see auth_events.go.
	authEventsOut chan<- AuthEvent

	// Driver's IP events channel (write-only from this goroutine).
	// Carries EventIPRequest consumed by the external IP handler
	// (l2tp-pool plugin in production, in-test pool stub). Separate
	// channel for the same reason the auth channel is separate.
	ipEventsOut chan<- IPEvent

	// Auth decision delivered by Driver.AuthResponse. Buffered(1) so
	// a caller that beats the session to the receive position does
	// not block. The session reads exactly once per auth phase.
	authRespCh chan authResponseMsg

	// IP decisions delivered by Driver.IPResponse. Buffered(2), one
	// slot per family, so IPv4 and IPv6 responses may arrive in any
	// order and both be accepted without blocking the caller. The
	// session reads exactly once per family per NCP round.
	ipRespCh chan ipResponseMsg

	// NCP configuration captured from StartSession. Zero/false means
	// the NCP is enabled (spec default "enable-*cp=true").
	disableIPCP   bool
	disableIPv6CP bool
	ipTimeout     time.Duration

	// earlyNCPFrames buffers NCP frames that arrive on the chan fd
	// before the NCP phase starts (race: peer sends IPCP before Ze
	// processes the LCP ConfAck that triggers Opened).
	earlyNCPFrames [][]byte

	// Driver's shutdown signal (the goroutine selects on this and
	// the chan fd's blocking read).
	stopCh <-chan struct{}

	// Per-session cancellation, closed by Driver.StopSession via
	// sessStopOnce.Do. The auth phase selects on this in addition
	// to stopCh so a single-session teardown (as opposed to a
	// whole-driver shutdown) unblocks a goroutine parked on
	// authRespCh. Closing chanFile alone is insufficient because
	// the auth phase runs before or alongside readFrames rather
	// than consuming from it. sessStopOnce guarantees idempotent
	// close if a future cleanup path joins StopSession.
	sessStop     chan struct{}
	sessStopOnce sync.Once

	// done is closed by the goroutine on exit; the manager waits on
	// it during StopSession to confirm cleanup.
	done chan struct{}

	logger *slog.Logger

	// State below is mu-protected; written by the goroutine, read
	// by SessionByID.
	mu                   sync.Mutex
	state                LCPState
	negotiatedMRU        uint16
	negotiatedAuthMethod AuthMethod
	echoOutstanding      uint8     // count of unanswered Echo-Request
	lastEchoSentAt       time.Time // wall-clock of most recent Echo-Request; goroutine-owned

	// NCP state, goroutine-owned after session spawn; no lock needed
	// because every writer is the session goroutine. Snapshot under
	// mu if SessionByID grows NCP-aware in a later phase.
	ipcpState        LCPState
	ipv6cpState      LCPState
	ipcpIdentifier   uint8
	ipv6cpIdentifier uint8
	localIPv4        netip.Addr
	peerIPv4         netip.Addr
	dnsPrimary       netip.Addr
	dnsSecondary     netip.Addr
	// localInterfaceID is Ze's own IPv6CP Interface-Identifier, seeded
	// once by requestIPv6CPInterfaceID (ncp.go) from
	// generateIPv6CPInterfaceID (ipv6cp.go), which retries until the
	// draw passes isValidIPv6CPInterfaceID -- so this field is never
	// legitimately the zero value. buildNakOrReject's equal-zero
	// Configure-Reject branch (ncp.go) is unreachable in production
	// for that reason; it exists for a peer that could otherwise force
	// the comparison RFC 5072 Section 4.1 still defines.
	localInterfaceID [8]byte
	peerInterfaceID  [8]byte

	// peerInterfaceIDNegotiated is the companion fact peerInterfaceID
	// needs and does not carry on its own: an all-zero identifier is
	// both the Go zero value and the one value RFC 5072 Section 4.1
	// never lets stand as the negotiated result, so the field alone
	// cannot tell "never set" from "set to a value ze would refuse".
	// True once peerInterfaceID holds a
	// value ze can trust, on either of its two legitimate paths:
	// evalIPv6CPRequest (ncp.go) accepted a Configure-Request that
	// carried the option, or requestIPv6CPInterfaceID (ncp.go) took an
	// explicit override from the address handler's IPResponse
	// (HasPeerInterface). The second path is not IPv6CP negotiation --
	// it is the operator's own assignment -- but it is an equally
	// trustworthy origin for the value, so it sets this fact too
	// (ai/rules/principles.md: a zero must never be a valid-looking
	// answer). Consulted by onNCPOpened (ncp.go) and
	// afterLCPOpenIPv6Service (session_run.go) before either publishes
	// or acts on peerInterfaceID.
	peerInterfaceIDNegotiated bool

	// ipv6cpMissingIdentifierNaked is RFC 5072 Section 4.1's one-shot
	// guard: "If the next Configure-Request does not include this
	// option, the peer MUST NOT send another Configure-Nak with this
	// option included." evalIPv6CPRequest (ncp.go) sets this true the
	// first time a Configure-Request arrives with no
	// Interface-Identifier option (earning one Configure-Nak
	// suggesting it), and Acks every later request that still omits
	// it, so a peer that never sends the option converges instead of
	// being Naked forever. Per session, and resets with it because a
	// struct field carries no state across pppSession values.
	ipv6cpMissingIdentifierNaked bool

	ipv6Svc *IPv6Service
}

// sessionInfo is a snapshot of pppSession state suitable for `show
// l2tp session` and tests. Returned by Manager.SessionByID under
// the per-session lock; safe to use after the lock is released.
type sessionInfo struct {
	TunnelID        uint16
	SessionID       uint16
	UnitNum         int
	State           LCPState
	NegotiatedMRU   uint16
	EchoOutstanding uint8
}

// snapshot copies the locked state into a SessionInfo. Caller MUST
// hold s.mu.
func (s *pppSession) snapshot() sessionInfo {
	return sessionInfo{
		TunnelID:        s.tunnelID,
		SessionID:       s.sessionID,
		UnitNum:         s.unitNum,
		State:           s.state,
		NegotiatedMRU:   s.negotiatedMRU,
		EchoOutstanding: s.echoOutstanding,
	}
}

// IfaceBackend is the small subset of internal/component/iface.Backend
// that the PPP package needs, expressed locally so ppp does not import
// iface (which would create a cycle through transport packages later).
// Production callers wrap iface.GetBackend(); tests inject fakes.
type IfaceBackend interface {
	SetMTU(name string, mtu int) error
	SetAdminUp(name string) error
	// AddAddressP2P installs a point-to-point address on pppN after
	// IPCP negotiation completes (spec-l2tp-6c-ncp AC-5).
	AddAddressP2P(name, localCIDR, peerCIDR string) error
	// AddRoute programs the kernel to reach the peer via pppN
	// (spec-l2tp-6c-ncp AC-6). gateway == "" means "onlink via dev".
	AddRoute(name, destCIDR, gateway string, metric int, proto rtproto.Proto) error
	// RemoveAddress / RemoveRoute undo the above on session teardown
	// (spec-l2tp-6c-ncp AC-18).
	RemoveAddress(name, cidr string) error
	RemoveRoute(name, destCIDR, gateway string, metric int, proto rtproto.Proto) error
}
