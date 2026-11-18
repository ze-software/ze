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
	errRelayNoHandoff     = errors.New("l2tp: relayed subscriber has no call-end handoff")
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
func (rs *relaySink) Relay(req callsink.Request) (callsink.RelayCall, bool, error) {
	rs.s.mu.Lock()
	rem, matched := rs.s.relayTargetLocked(req.Service)
	var reactor *l2tpReactor
	if matched && len(rs.s.reactors) > 0 {
		reactor = rs.s.reactors[0]
	}
	rs.s.mu.Unlock()

	if !matched {
		// No relay binding for this service: terminate locally.
		countRelay(relayKindService, relayOutcomeNoMatch)
		return nil, false, nil
	}
	if reactor == nil {
		countRelay(relayKindService, relayOutcomeRefused)
		return nil, false, errRelayNoListener
	}

	if req.Handoff == nil {
		countRelay(relayKindService, relayOutcomeRefused)
		return nil, false, errRelayNoHandoff
	}
	rc := &relayCall{remote: rem.Name, handoff: req.Handoff, teardown: reactor.teardownRelaySession}
	target := DialTarget{Remote: rem.Address, SharedSecret: rem.SharedSecret}
	// The LAC forwards the subscriber's identity as the Calling Number; the
	// PPPoE transport lease travels with the call for the kernel data-plane
	// bridge (A-4). The worker borrows its descriptor only during that ioctl.
	params := callParams{
		callingNumber: req.SubscriberMAC, channel: req.Channel,
		beforeBridge: req.BeforeBridge, relay: rc,
	}
	if _, err := reactor.placeIncomingCall(target, params); err != nil {
		countRelay(relayKindService, relayOutcomeRefused)
		return nil, false, err
	}
	countRelay(relayKindService, relayOutcomePlaced)
	rs.s.logger.Info("l2tp: relaying PPPoE subscriber into L2TP incoming call",
		"service", req.Service, "remote", rem.Name, "subscriber", req.SubscriberMAC,
		"interface", req.Interface)
	return rc, true, nil
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
func (rs *relaySink) RelayRealm(req callsink.RealmRequest) (callsink.RelayCall, bool, error) {
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
		countRelay(relayKindRealm, relayOutcomeNoMatch)
		return nil, false, nil
	}
	if !remoteKnown {
		countRelay(relayKindRealm, relayOutcomeRefused)
		// validateRelayRealms refuses this at load; a reload that dropped
		// the remote between the two reads lands here.
		return nil, false, fmt.Errorf("%w: realm %q -> remote %q", errRelayRealmNoRemote, binding.Realm, binding.Remote)
	}
	if reactor == nil {
		countRelay(relayKindRealm, relayOutcomeRefused)
		return nil, false, errRelayNoListener
	}
	if req.Auth.Kind == callsink.AuthKindUnspecified {
		countRelay(relayKindRealm, relayOutcomeRefused)
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
	rc := &relayCall{
		remote: rem.Name, realm: &relayRealmDetails{name: binding.Realm, proxy: proxy, authReason: authReason},
		handoff: req.Handoff, teardown: reactor.teardownRelaySession,
	}
	target := DialTarget{Remote: rem.Address, SharedSecret: rem.SharedSecret}
	params := callParams{callingNumber: req.SubscriberMAC, channel: req.Channel, beforeBridge: req.BeforeBridge, relay: rc}
	if _, err := reactor.placeIncomingCall(target, params); err != nil {
		countRelay(relayKindRealm, relayOutcomeRefused)
		return nil, false, err
	}
	countRelay(relayKindRealm, relayOutcomePlaced)
	rs.s.logger.Info("l2tp: relaying PPPoE subscriber by realm into L2TP incoming call",
		"realm", binding.Realm, "remote", rem.Name, "subscriber", req.SubscriberMAC,
		"interface", req.Interface, "proxy-auth", authReason == "")
	return rc, true, nil
}

// Cancel ends the relay call for a subscriber that left (callsink.RelayCall).
// Before the bridge, the next event the reactor or the kernel worker sees
// for it (ICRP, the bridge) tears the call down with CDN instead of going
// on. After the bridge, Cancel tears the bridged session down now: nothing
// else would: a bridged subscriber has no local PPP worker to tell the
// LNS that its access transport ended.
// Idempotent; safe for concurrent use. Caller MUST NOT hold tunnelsMu.
func (rc *relayCall) Cancel() {
	rc.bridgeMu.Lock()
	// refuse-by-default: guards relay lifetime -- a completed call owns no bridge to cancel or republish
	if rc.ended {
		rc.bridgeMu.Unlock()
		return
	}
	rc.canceled = true
	bridged, tid, sid := rc.bridged, rc.bridgedTID, rc.bridgedSID
	rc.bridged = false
	rc.bridgeMu.Unlock()
	// refuse-by-default: guards Cancel -- a pre-bridge cancellation is observed by the pending call or worker
	if !bridged {
		return
	}
	if rc.teardown == nil {
		panic("BUG: l2tp relay call bridged with no teardown")
	}
	rc.teardown(rc, tid, sid)
}

// markBridged records that the kernel worker bridged session tid/sid, so a
// later Cancel tears it down. It refuses cancellation or completion: the
// caller then fails the setup, which sends CDN, instead of reporting a
// bridge nobody owns.
func (rc *relayCall) markBridged(tid, sid uint16) bool {
	rc.bridgeMu.Lock()
	defer rc.bridgeMu.Unlock()
	// refuse-by-default: guards markBridged -- an abandoned subscriber cannot publish a live bridge
	if rc.canceled {
		return false
	}
	// refuse-by-default: guards relay lifetime -- a completed call owns no bridge to cancel or republish
	if rc.ended {
		return false
	}
	rc.bridged = true
	rc.bridgedTID = tid
	rc.bridgedSID = sid
	return true
}

// stopped reports whether cancellation or completion forbids new bridging.
// Safe for concurrent use; callers MUST hold neither bridgeMu nor a callback lock.
func (rc *relayCall) stopped() bool {
	rc.bridgeMu.Lock()
	defer rc.bridgeMu.Unlock()
	// refuse-by-default: guards relay lifetime -- a completed call owns no bridge to cancel or republish
	if rc.ended {
		return true
	}
	return rc.canceled
}

// end reports the end of the call to the PPPoE side exactly once.
func (rc *relayCall) end(reason string) {
	rc.bridgeMu.Lock()
	// refuse-by-default: guards relay lifetime -- a completed call owns no bridge to cancel or republish
	if rc.ended {
		rc.bridgeMu.Unlock()
		return
	}
	rc.ended = true
	rc.bridged = false
	rc.bridgeMu.Unlock()
	rc.handoff.Ended(reason)
}

var _ callsink.RelayCall = (*relayCall)(nil)

// target describes a call without inventing a realm for Service-Name relay.
func (rc *relayCall) target() string {
	if rc.realm != nil {
		return "realm " + rc.realm.name
	}
	return "remote " + rc.remote
}
