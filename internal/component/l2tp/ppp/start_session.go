// Design: docs/research/l2tpv2-ze-integration.md -- PPP transport-agnostic boundary

package ppp

import (
	"net"
	"net/netip"
	"time"
)

// StartSession is the payload sent on Driver.SessionsIn to launch a PPP
// session. It is the ownership boundary between L2TP or PPPoE and PPP.
//
// Channel delivery transfers exclusive ownership of both descriptors to the
// PPP manager. It closes them on refusal, stopped-queue drain, or session
// teardown. Producers MUST stop sending before stopping the manager.
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

	// PPPoE applies RFC 2516's transport-specific LCP option restrictions.
	PPPoE bool

	// MaxMRU is the largest MRU ze will accept from the peer in LCP
	// Configure-Request. Peer requests above this are NAKd with
	// MaxMRU as the suggested value. Zero means use the package
	// default (1500).
	MaxMRU uint16

	// Echo configuration. EchoInterval <= 0 uses the default (10s).
	// EchoFailures is the consecutive no-reply count that triggers
	// session teardown.
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
	// Proxy LCP may override this choice only when RenegotiateProxyLCP is
	// false, its actual method is in AuthFallbackOrder and its Last-Sent MRU
	// meets MaxMRU; otherwise Ze negotiates LCP itself.
	AuthMethod AuthMethod

	// AuthRequired closes LCP before Opened if the peer refuses every
	// permitted method or rejects the Auth-Protocol option. False explicitly
	// permits the no-wire-auth path.
	AuthRequired bool

	// AuthFallbackOrder is the immutable, explicit set of permitted methods.
	// The caller MUST resolve transport defaults before enqueueing and MUST
	// NOT mutate this slice afterwards. Empty means no permitted fallback,
	// never a package-wide default. Configured CHAP is offered first; remaining
	// untried CHAP methods follow written order, and PAP is offered last.
	// A valid Configure-Reject removes Auth-Protocol for this negotiation,
	// irrespective of unused methods in this list (RFC 1661 Section 5.4).
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

	// DisableIPCP suppresses the IPv4 NCP. Inverted sense so the
	// zero value enables IPCP, matching the spec default
	// "enable-ipcp=true". When both DisableIPCP and DisableIPv6CP are
	// true the session emits a warning log and reaches EventSessionUp
	// immediately after auth with no IP configured.
	DisableIPCP bool

	// DisableIPv6CP suppresses the IPv6 NCP (RFC 5072 Interface-
	// Identifier negotiation). Zero value enables it.
	DisableIPv6CP bool

	// IPTimeout bounds how long the per-session goroutine waits for
	// Driver.IPResponse after emitting EventIPRequest. Zero means use
	// the package default (30s). On timeout the session emits
	// EventSessionDown and an LCP Terminate-Request.
	IPTimeout time.Duration

	// Proxy LCP bytes from L2TP ICCN AVPs (RFC 2661 Section 4.4.5).
	// When all three are present and RenegotiateProxyLCP is false, the LCP
	// FSM short-circuits to the Opened state with the proxied options.
	// Empty slices mean no proxy data; LCP runs the full negotiation.
	ProxyLCPInitialRecv []byte // AVP 26: Initial-Received-LCP-CONFREQ
	ProxyLCPLastSent    []byte // AVP 27: Last-Sent-LCP-CONFREQ
	ProxyLCPLastRecv    []byte // AVP 28: Last-Received-LCP-CONFREQ

	// RenegotiateProxyLCP is the LNS policy `l2tp proxy-lcp renegotiate`:
	// the session ignores the three proxy LCP slices and negotiates LCP
	// itself from the synthetic Open, with its own Magic-Number, MRU and
	// Auth-Protocol. False is `proxy-lcp accept`, the default, which uses
	// valid proxy LCP. The slices stay set either way so the outcome
	// (EventLCPUp.ProxyLCP) can tell a renegotiated session from one that
	// received no proxy LCP.
	RenegotiateProxyLCP bool

	// CaptureProxyLCP asks the session to record its LCP exchange and hand
	// it out on EventAuthRequest.ProxyLCP, for a LAC that may relay the
	// subscriber by realm (RFC 2661 Section 4.4.5). False everywhere else.
	CaptureProxyLCP bool

	// PeerAddr is informational, used for log fields. Not used for
	// any I/O decision (the chan fd already routes to the peer).
	PeerAddr netip.AddrPort

	// PPPoE-specific fields for RADIUS attribute passthrough.
	// Set by the PPPoE access concentrator; left empty for L2TP sessions.
	AccessInterface string           // Access interface name (e.g. "eth0")
	SubscriberMAC   net.HardwareAddr // Subscriber CPE MAC address
	ServiceName     string           // PPPoE Service-Name from PADR
	VendorTags      []byte           // Raw vendor-specific tags from PADI/PADR (TR-101 etc.)
}

// closeFiles releases an undelivered or refused ownership transfer.
func (s *StartSession) closeFiles() {
	if s.ChanFD > 0 {
		if channel := newChanFileFn(s.ChanFD, "ppp.unstarted.chan"); channel != nil {
			_ = channel.Close() //nolint:errcheck // rejected or canceled setup
		}
	}
	if s.UnitFD > 0 && s.UnitFD != s.ChanFD {
		if unit := newUnitFileFn(s.UnitFD); unit != nil {
			_ = unit.Close() //nolint:errcheck // rejected or canceled setup
		}
	}
	s.ChanFD, s.UnitFD = -1, -1
}
