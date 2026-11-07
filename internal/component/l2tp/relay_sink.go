// Design: docs/architecture/l2tp/followup-l2tp-call.md -- AC-3 PPPoE->L2TP relay (LAC role)
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 10.1 (LAC incoming call)
// Related: reactor_dial.go -- placeIncomingCall originates the ICRQ

package l2tp

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/callsink"
)

var (
	errRelayNoListener    = errors.New("l2tp: relay configured but no L2TP listener is running")
	errRelayRealmNoRemote = errors.New("l2tp: relay-realm names a remote that is not declared")
	errRelayRealmNoAuth   = errors.New("l2tp: realm relay request carries no authentication exchange")
)

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
	var reactor *l2tpReactor
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
	if _, err := reactor.placeIncomingCall(target, params); err != nil {
		return false, err
	}
	rs.s.logger.Info("l2tp: relaying PPPoE subscriber into L2TP incoming call",
		"service", req.Service, "remote", rem.Name, "subscriber", req.SubscriberMAC,
		"interface", req.Interface)
	return true, nil
}

// RealmRelayConfigured reports whether any relay-realm binding is configured
// (callsink.Sink). The PPPoE server asks it for each new session.
func (rs *relaySink) RealmRelayConfigured() bool {
	rs.s.mu.Lock()
	defer rs.s.mu.Unlock()
	return len(rs.s.params.RelayRealms) > 0
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

// RelayRealm matches req.Realm against the relay-realm bindings. On a match it
// dials the bound remote and places an incoming call whose ICCN carries the
// proxy LCP and proxy authentication AVPs (RFC 2661 Section 4.4.5); progress
// reaches req.Handoff. No match returns accepted=false and a nil error, so the
// caller authenticates locally. A match whose call cannot start returns the
// error: the caller refuses the subscriber, never authenticating it locally.
func (rs *relaySink) RelayRealm(req callsink.RealmRequest) (callsink.RealmCall, bool, error) {
	if req.Handoff == nil {
		panic("BUG: l2tp RelayRealm called with a nil handoff")
	}
	rs.s.mu.Lock()
	binding, matched := rs.s.params.LookupRealmRelay(req.Realm)
	rem, remoteKnown := rs.s.params.LookupRemote(binding.Remote)
	var reactor *l2tpReactor
	if matched && len(rs.s.reactors) > 0 {
		reactor = rs.s.reactors[0]
	}
	rs.s.mu.Unlock()

	if !matched {
		return nil, false, nil
	}
	if !remoteKnown {
		// validateRelayRealms refuses this at load; a reload that dropped
		// the remote between the two reads lands here.
		return nil, false, fmt.Errorf("%w: realm %q -> remote %q", errRelayRealmNoRemote, binding.Realm, binding.Remote)
	}
	if reactor == nil {
		return nil, false, errRelayNoListener
	}
	if req.Auth.Kind == callsink.AuthKindUnspecified {
		return nil, false, errRelayRealmNoAuth
	}

	proxy, authReason, fits := newICCNProxy(req.Username, req.LCP, req.Auth, rem.SharedSecret)
	if !fits {
		// RFC 2661 Section 4.4.5 makes the proxy AVPs optional ("these AVPs
		// may be included"); the LNS then negotiates LCP afresh.
		rs.s.logger.Warn("l2tp: realm relay proxy data does not fit one ICCN; sending none, the LNS renegotiates",
			"realm", binding.Realm, "remote", rem.Name, "subscriber", req.SubscriberMAC)
	}
	if authReason == proxyAuthReasonNoSharedSecret {
		rs.s.logger.Warn("l2tp: realm relay sends no proxy authentication for a PAP subscriber: remote has no shared-secret to hide the password; the LNS restarts authentication",
			"realm", binding.Realm, "remote", rem.Name,
			"fix", "set `l2tp remote "+rem.Name+" shared-secret <secret>` on both ends")
	}
	rc := &realmCall{realm: binding.Realm, remote: rem.Name, proxy: proxy, authReason: authReason, handoff: req.Handoff}
	target := DialTarget{Remote: rem.Address, SharedSecret: rem.SharedSecret}
	params := callParams{callingNumber: req.SubscriberMAC, pppoeChannelFD: req.ChannelFD, realm: rc}
	if _, err := reactor.placeIncomingCall(target, params); err != nil {
		return nil, false, err
	}
	rs.s.logger.Info("l2tp: relaying PPPoE subscriber by realm into L2TP incoming call",
		"realm", binding.Realm, "remote", rem.Name, "subscriber", req.SubscriberMAC,
		"interface", req.Interface, "proxy-auth", authReason == "")
	return rc, true, nil
}

// Cancel abandons the realm call: the next event the reactor or the kernel
// worker sees for it (ICRP, the bridge) tears the call down with CDN instead
// of going on. Idempotent; safe for concurrent use.
func (rc *realmCall) Cancel() { rc.canceled.Store(true) }

// isCancelled reports whether the PPPoE side abandoned the call.
func (rc *realmCall) isCancelled() bool { return rc.canceled.Load() }

// end reports the end of the call to the PPPoE side exactly once.
func (rc *realmCall) end(reason string) {
	if rc.ended.Swap(true) {
		return
	}
	rc.handoff.Ended(reason)
}

var _ callsink.RealmCall = (*realmCall)(nil)
