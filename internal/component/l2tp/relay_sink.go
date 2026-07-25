// Design: plan/learned/1100-followup-l2tp-call.md -- AC-3 PPPoE->L2TP relay (LAC role)
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 10.1 (LAC incoming call)
// Related: reactor_dial.go -- PlaceIncomingCall originates the ICRQ

package l2tp

import (
	"errors"

	"github.com/ze-software/ze/internal/core/callsink"
)

var errRelayNoListener = errors.New("l2tp: relay configured but no L2TP listener is running")

// relaySink implements callsink.Sink. When a PPPoE subscriber completes
// discovery, the pppoe server consults the registered sink; this one matches
// the subscriber's Service-Name against the configured relay bindings and, on
// a match, originates a LAC-side incoming call (ICRQ) toward the bound remote
// instead of letting pppoe terminate PPP locally. The pppoe package never
// imports l2tp: it reaches this through the neutral callsink registry (R-1).
type relaySink struct {
	s *Subsystem
}

// Relay matches req.Service against the relay bindings. On a match it dials
// the bound remote and originates an incoming call (fire-and-forget: the
// ICRQ/ICRP/ICCN handshake and the kernel session are driven asynchronously
// by the reactor). Returns accepted=true so the caller skips local PPP
// termination. A match whose remote cannot be dialed returns an error so the
// caller can fall back to local termination rather than strand the subscriber.
func (rs *relaySink) Relay(req callsink.Request) (bool, error) {
	rs.s.mu.Lock()
	rem, matched := rs.s.relayTargetLocked(req.Service)
	var reactor *L2TPReactor
	if matched && len(rs.s.reactors) > 0 {
		reactor = rs.s.reactors[0]
	}
	rs.s.mu.Unlock()

	if !matched {
		// No relay binding for this service: terminate locally.
		return false, nil
	}
	if reactor == nil {
		return false, errRelayNoListener
	}

	target := DialTarget{Remote: rem.Address, SharedSecret: rem.SharedSecret}
	// The LAC forwards the subscriber's identity as the Calling Number; the
	// PPPoE channel fd travels with the call for the kernel data-plane bridge
	// (A-4), which the reactor performs when the session's kernel resources
	// are created.
	params := callParams{callingNumber: req.SubscriberMAC, pppoeChannelFD: req.ChannelFD}
	if _, err := reactor.PlaceIncomingCall(target, params); err != nil {
		return false, err
	}
	rs.s.logger.Info("l2tp: relaying PPPoE subscriber into L2TP incoming call",
		"service", req.Service, "remote", rem.Name, "subscriber", req.SubscriberMAC,
		"interface", req.Interface)
	return true, nil
}

// relayTargetLocked returns the configured remote a PPPoE Service-Name is
// relayed to, and whether a binding matched. Caller MUST hold s.mu.
func (s *Subsystem) relayTargetLocked(service string) (Remote, bool) {
	for i := range s.params.Relays {
		if s.params.Relays[i].Service == service {
			if rem, ok := s.params.LookupRemote(s.params.Relays[i].Remote); ok {
				return rem, true
			}
			return Remote{}, false
		}
	}
	return Remote{}, false
}
