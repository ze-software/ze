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

// Sink accepts relay Requests. Implemented by the L2TP subsystem. Relay
// reports whether the subscriber was taken over for relay: accepted=true
// means a relay binding matched and an L2TP incoming call was originated (the
// caller MUST NOT terminate PPP locally); accepted=false means no binding
// matched (terminate locally as normal). A non-nil error is a relay attempt
// that matched but failed; the caller logs it and, per policy, terminates
// locally so the subscriber is not stranded.
//
// Implementations MUST be safe for concurrent use.
type Sink interface {
	Relay(req Request) (accepted bool, err error)
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
