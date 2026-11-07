// Design: docs/architecture/l2tp/followup-l2tp-call.md -- AC-3 PPPoE->L2TP relay call-sink (R-1 boundary)

// Package callsink is the neutral registration seam between the PPPoE access
// concentrator and the L2TP tunnel engine, letting a PADS-completed PPPoE
// subscriber be relayed into an L2TP incoming call (the LAC role, RFC 2661)
// WITHOUT the pppoe package importing the l2tp package.
//
// The deliberate import boundary in pppoe/doc.go forbids pppoe -> l2tp. Both
// packages instead depend on this core leaf: l2tp registers a Sink at
// startup; pppoe looks the Sink up when a service is configured for relay and
// hands the subscriber over. Registration over hardcoding is the repo-wide
// pattern (spec-followup-l2tp-call R-1).
//
// This package holds only value types and an atomic registry; it imports
// neither l2tp nor pppoe, so it introduces no cycle and sits in the core tier
// (ai/rules/architecture.md: a library with no config-driven lifecycle).
package callsink

import "sync/atomic"

// Request describes a PPPoE subscriber whose discovery (PADI/PADO/PADR/PADS)
// completed and who is a candidate for relay into an L2TP tunnel. The Sink
// decides, from Service, whether a relay binding matches.
type Request struct {
	// Service is the PPPoE Service-Name the subscriber requested (empty for
	// the default service). The Sink matches it against its relay bindings.
	Service string
	// Interface is the access interface the subscriber arrived on.
	Interface string
	// SubscriberMAC is the subscriber's hardware address, "aa:bb:...".
	SubscriberMAC string
	// SessionID is the PPPoE session ID assigned at PADS.
	SessionID uint16
	// ChannelFD is the subscriber's PPPoE pppox socket file descriptor. The
	// L2TP LAC data-plane bridge derives its kernel PPP channel number
	// (PPPIOCGCHAN) from it and cross-connects that channel to the pppol2tp
	// channel (PPPIOCBRIDGECHAN). Zero or negative when the platform has no
	// such fd (non-Linux) or the caller does not relay the data plane.
	ChannelFD int
}

// AuthKind names the PPP authentication a realm-relayed subscriber ran with
// the LAC, as the Proxy Authen Type AVP needs it. The zero value is
// AuthKindUnspecified and is never a valid answer: a realm request always
// comes from an authentication exchange the LAC started.
type AuthKind uint8

const (
	// AuthKindUnspecified is the zero value; a sink refuses it.
	AuthKindUnspecified AuthKind = iota
	// AuthKindCHAP is PPP CHAP with MD5 (RFC 1994).
	AuthKindCHAP
	// AuthKindPAP is PPP PAP (RFC 1334).
	AuthKindPAP
)

// ProxyLCP is the LCP the LAC negotiated with the subscriber, as RFC 2661
// Section 4.4.5 carries it in AVPs 26 to 28. Each field is the option body of
// one Configure-Request, starting at the first option (no LCP header).
// The slices are the request's own copies; a sink MUST NOT retain them past
// the call it places.
type ProxyLCP struct {
	// InitialReceived is the first Configure-Request the subscriber sent.
	InitialReceived []byte
	// LastSent is the LAC's Configure-Request the subscriber Acked.
	LastSent []byte
	// LastReceived is the subscriber's Configure-Request the LAC Acked.
	LastReceived []byte
}

// ProxyAuth is the authentication exchange the LAC ran with the subscriber.
// Challenge is empty for PAP; Response is the CHAP response value, or the
// PAP password in clear text, which a sink MUST NOT log and MUST NOT send
// unhidden.
type ProxyAuth struct {
	Kind       AuthKind
	Identifier uint8
	Challenge  []byte
	Response   []byte
}

// RealmRequest asks the sink to relay a PPPoE subscriber whose realm (the
// text after the last '@' of Username) may name a realm relay binding. It is
// made after the LAC ran LCP and received the subscriber's authentication
// Response, and before any credential check.
type RealmRequest struct {
	// Realm is the realm already split from Username.
	Realm string
	// Username is the full name the subscriber authenticated with.
	Username string
	// Interface, SubscriberMAC and SessionID identify the PPPoE session.
	Interface     string
	SubscriberMAC string
	SessionID     uint16
	// ChannelFD is the subscriber's pppox socket, bridged once Handoff
	// reports the driver quiesced.
	ChannelFD int
	LCP       ProxyLCP
	Auth      ProxyAuth
	// Handoff receives the call's progress. MUST be non-nil.
	Handoff RealmHandoff
}

// RealmHandoff is the caller's side of a realm relay. The sink calls Quiesce
// once the L2TP session is ready to bridge, then Bridged once with the bridge
// result. It calls Ended once when the L2TP call ends: before Bridged(nil)
// that is a failed call and the caller refuses the subscriber; after it the
// caller ends the PPPoE session (PADT). After Bridged(nil) the L2TP session
// owns the subscriber's PPP.
//
// Quiesce MUST return only after the caller's PPP reader on the subscriber's
// channel has returned, and MUST return an error when the subscriber is gone
// (the sink then tears the call down instead of bridging). Bridged and Ended
// MUST NOT block: the sink calls them from its reactor and kernel worker.
type RealmHandoff interface {
	Quiesce() error
	Bridged(err error)
	Ended(reason string)
}

// RealmCall is a realm relay the sink accepted. The caller MUST call Cancel
// when the subscriber leaves (PADT, PPP session down) or the caller stops,
// in every phase. Before the bridge the sink sends CDN for any call it placed
// and never bridges. After Bridged(nil) the sink tears the L2TP session down
// with CDN, as for a subscriber that hung up. Cancel is idempotent, safe for
// concurrent use, and a no-op after the call ended. A teardown it causes
// reports Ended before Cancel returns, so the caller MUST NOT hold a lock
// its Ended takes.
type RealmCall interface {
	Cancel()
}

// Sink accepts relay Requests. Implemented by the L2TP subsystem. Relay
// reports whether the subscriber was taken over for relay: accepted=true
// means a relay binding matched and an L2TP incoming call was originated (the
// caller MUST NOT terminate PPP locally); accepted=false means no binding
// matched (terminate locally as normal). A non-nil error is a relay attempt
// that matched but failed; the caller logs it and, per policy, terminates
// locally so the subscriber is not stranded.
//
// Implementations MUST be safe for concurrent use.
//
// RelayRealm matches req.Realm. accepted=false with a nil error means no
// binding names the realm (authenticate locally). accepted=true means the call
// is being placed and its progress reaches req.Handoff. A non-nil error is a
// matched realm whose call could not start; the caller refuses the
// subscriber and MUST NOT authenticate it locally.
//
// RealmRelayConfigured reports whether any realm binding exists, so the
// caller records the LCP exchange a realm relay forwards for a new session.
type Sink interface {
	Relay(req Request) (accepted bool, err error)
	RelayRealm(req RealmRequest) (call RealmCall, accepted bool, err error)
	RealmRelayConfigured() bool
}

// global holds the registered Sink. nil when no L2TP subsystem is running
// (or it has stopped), in which case Lookup returns nil and callers terminate
// PPP locally.
var global atomic.Pointer[Sink]

// Register publishes s as the process-wide relay Sink. The L2TP subsystem
// calls this from Start and clears it from Stop via Unregister. A nil s is
// equivalent to Unregister.
//
// Safe for concurrent use.
func Register(s Sink) {
	if s == nil {
		global.Store(nil)
		return
	}
	global.Store(&s)
}

// Unregister clears the registered Sink so later Lookups return nil.
//
// Safe for concurrent use.
func Unregister() { global.Store(nil) }

// Lookup returns the registered relay Sink, or nil when none is registered.
// A nil return means "no relay is possible; terminate locally."
//
// Safe for concurrent use.
func Lookup() Sink {
	if p := global.Load(); p != nil {
		return *p
	}
	return nil
}
