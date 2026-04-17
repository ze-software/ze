// Design: docs/research/l2tpv2-ze-integration.md -- PPP transport-agnostic boundary

package ppp

import (
	"net/netip"
	"time"
)

// StartSession is the payload sent on Manager.SessionsIn to launch a new
// PPP session. It is the contract between the transport (L2TP today,
// PPPoE later) and the PPP package.
//
// All file descriptors MUST be valid and owned by the PPP manager once
// StartSession is sent. The manager closes them on session teardown.
type StartSession struct {
	// Identification. Opaque to ppp; used only as a key for routing
	// events back to the caller and for log fields.
	TunnelID  uint16
	SessionID uint16

	// File descriptors from the kernel PPP/PPPoX setup. The chan fd
	// carries LCP, authentication, and NCP control packets. The unit
	// fd carries IP packets (data plane, kernel-handled) and is the
	// target of PPPIOCSMRU after LCP completes.
	ChanFD  int
	UnitFD  int
	UnitNum int // pppN unit number; iface name is "ppp" + strconv.Itoa(UnitNum)

	// LNSMode indicates whether ze acts as the LNS (true) or LAC
	// (false) for this session. Affects which side initiates LCP and
	// which authenticator role is used.
	LNSMode bool

	// MaxMRU is the largest MRU ze will accept from the peer in LCP
	// Configure-Request. Peer requests above this are NAKd with
	// MaxMRU as the suggested value. Zero means use the package
	// default (1500).
	MaxMRU uint16

	// Echo configuration. EchoInterval == 0 disables LCP Echo
	// keepalive. EchoFailures is the consecutive no-reply count that
	// triggers session teardown.
	EchoInterval time.Duration
	EchoFailures uint8

	// AuthTimeout bounds how long the per-session goroutine waits
	// for Driver.AuthResponse after emitting EventAuthRequest. Zero
	// means use the package default (30s). On timeout the session
	// emits EventAuthFailure{Reason: "timeout"} + EventSessionDown.
	AuthTimeout time.Duration

	// AuthMethod selects the PPP Authentication Protocol that ze
	// advertises in its LCP CONFREQ and that runAuthPhase dispatches
	// to after LCP-Opened. Zero (AuthMethodNone) omits the Auth-
	// Protocol option and runs the no-wire-auth phase (existing
	// handler still fires one EventAuthRequest for accounting).
	//
	// When proxy LCP AVPs are present the Auth-Protocol value from
	// the LAC's Last-Sent CONFREQ overrides this field, because the
	// LAC's negotiation with the peer is what the peer already
	// accepted; ze inherits that choice rather than forcing a new
	// one mid-session.
	AuthMethod AuthMethod

	// AuthFallbackOrder lists the AuthMethod values ze will accept
	// when the peer replies to ze's CONFREQ with a Configure-Nak
	// naming a different Auth-Protocol (RFC 1661 §5.3). The peer's
	// suggestion is looked up in this list; a match becomes ze's new
	// configured method for the next CONFREQ, a non-match falls back
	// to AuthMethodNone. An empty slice means "use the package
	// default": [CHAPMD5, MSCHAPv2, PAP].
	//
	// Configure-Reject of the Auth-Protocol option always clears the
	// method regardless of this list, because Reject means "I do not
	// recognize this option at all" (RFC 1661 §5.4).
	AuthFallbackOrder []AuthMethod

	// ReauthInterval enables periodic re-authentication after the
	// initial auth succeeds (RFC 1994 Section 2.3: CHAP "MAY be used
	// to re-authenticate at any time during the connection"; spec AC-14).
	// Zero disables re-auth. Only honored for CHAP-MD5 and MS-CHAPv2;
	// PAP re-auth is peer-initiated per RFC 1334 and is not in scope.
	// On tick, ze sends a fresh Challenge with a new Identifier and
	// runs the exchange inline in the session main loop. A Failure
	// decision from the auth handler tears the session down.
	ReauthInterval time.Duration

	// Proxy LCP bytes from L2TP ICCN AVPs (RFC 2661 Section 18).
	// When all three are present, the LCP FSM short-circuits to the
	// Opened state with the proxied options. Empty slices mean no
	// proxy data; LCP runs the full negotiation.
	ProxyLCPInitialRecv []byte // AVP 26: Initial-Received-LCP-CONFREQ
	ProxyLCPLastSent    []byte // AVP 27: Last-Sent-LCP-CONFREQ
	ProxyLCPLastRecv    []byte // AVP 28: Last-Received-LCP-CONFREQ

	// PeerAddr is informational, used for log fields. Not used for
	// any I/O decision (the chan fd already routes to the peer).
	PeerAddr netip.AddrPort
}
