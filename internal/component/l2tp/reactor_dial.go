// Design: docs/research/l2tpv2-ze-integration.md -- L2TP reactor dial path
// RFC: rfc/short/rfc2661.md -- RFC 2661 Sections 4.4.3 (Tie Breaker AVP), 5.1.1 (tunnel authentication secret), 5.3 (concurrent tunnels to one peer)
// Related: tunnel_initiator.go -- L2TPTunnel.initiate builds the SCCRQ
// Related: reactor.go -- the single reactor goroutine that owns the tunnel map

package l2tp

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/netip"
	"time"
)

// ErrReactorStopped is returned by Dial when the reactor goroutine is not
// running (Stop already called, or Start never called).
var ErrReactorStopped = errors.New("l2tp: reactor stopped")

// ErrMaxTunnels is returned by Dial when the configured max-tunnels limit
// has been reached.
var ErrMaxTunnels = errors.New("l2tp: max-tunnels limit reached")

// ErrDialFromLoopback is returned by Dial when the reactor's listener is bound
// to a loopback address and the remote is on another host. Every tunnel of a
// reactor sends from its listener socket, and the kernel refuses a datagram
// from a loopback source to an off-host destination (Linux answers sendto
// with EINVAL), so the SCCRQ and each retransmit would fail and the dial
// could only time out. The dial is refused before any tunnel exists.
var ErrDialFromLoopback = errors.New("l2tp: a listener on a loopback address cannot dial a remote on another host")

// ErrCallTimeout is returned by placeOutgoingCallSync when neither a
// success nor a failure outcome arrives within the caller's deadline. The
// underlying dial/tunnel keeps running; the operator can observe it via
// `show l2tp tunnels/sessions`.
var ErrCallTimeout = errors.New("l2tp: outgoing call timed out")

// Failure-outcome causes surfaced to a blocking placeOutgoingCallSync when a
// call cannot complete. These distinguish the reasons an operator sees.
var (
	errCallPlacementRefused  = errors.New("l2tp: call refused (max sessions, tunnel not established, or a capability the peer did not advertise)")
	errCallTunnelAuthFailed  = errors.New("l2tp: outgoing call failed: tunnel authentication rejected")
	errCallTunnelSetupFailed = errors.New("l2tp: outgoing call failed: tunnel did not establish")
	errCallTornDown          = errors.New("l2tp: outgoing call torn down before it established")
)

// callOutcome is the terminal result of an operator-initiated call
// (spec-followup-l2tp-call AC-4). Exactly one is delivered to the blocking
// RPC: on success err is nil and localSID/remoteSID identify the session;
// on failure err describes the cause (tunnel auth reject, tie-breaker loss,
// setup timeout, or a peer CDN) and resultCode carries the RFC 2661 CDN/
// StopCCN Result Code when one is known (0 otherwise).
type callOutcome struct {
	localSID   uint16
	remoteSID  uint16
	resultCode uint16
	err        error
}

// DialTarget describes a remote to initiate a tunnel toward. Remote is the
// peer's control address:port (typically UDP 1701). SharedSecret is the
// per-remote CHAP-MD5 tunnel-authentication secret; empty disables our end
// of authentication for this dial (RFC 2661 Section 5.1.1).
type DialTarget struct {
	Remote       netip.AddrPort
	SharedSecret string
}

// pendingCall is a call the reactor originates the moment a dialed tunnel
// reaches established. outgoing selects the direction: true => LNS OCRQ
// (placeOutgoingCall), false => LAC ICRQ (placeIncomingCall).
type pendingCall struct {
	outgoing bool
	params   callParams
	// result, when non-nil, receives the call's terminal outcome (AC-4).
	// placeOutgoingCallSync installs it and blocks on it; the fire-and-forget
	// Dial/PlaceOutgoingCall/placeIncomingCall paths leave it nil. Buffered
	// (cap 1) at the call site so the reactor never blocks delivering.
	result chan callOutcome
}

// dialRequest is one Dial marshaled onto the reactor goroutine. result
// carries the outcome back to the caller. Buffered with capacity 1 by Dial so
// handleDial never blocks delivering the result even if the caller has gone.
// call, when non-nil, is originated once the tunnel establishes.
type dialRequest struct {
	target DialTarget
	call   *pendingCall
	result chan dialResult
}

type dialResult struct {
	localTID uint16
	err      error
}

// Dial initiates an L2TP tunnel toward target and returns the local tunnel ID
// once the SCCRQ has been sent (the tunnel is then in wait-ctl-reply; the
// SCCRP that establishes it arrives asynchronously on the reactor's receive
// path). Safe to call from any goroutine: the request is handed to the single
// reactor goroutine, which is the sole writer of the tunnel map (R-2). Returns
// ErrReactorStopped if the reactor is not running, ErrMaxTunnels if the limit
// is reached, ErrDialFromLoopback if the listener cannot reach the remote, or
// the tunnel-ID allocation error.
func (r *l2tpReactor) Dial(target DialTarget) (uint16, error) {
	return r.dialWithCall(target, nil)
}

// PlaceOutgoingCall dials target and, once the tunnel establishes, originates
// an LNS-side outgoing call (OCRQ) with the given call parameters. Returns the
// local tunnel ID (the call's local session ID is assigned asynchronously when
// the tunnel establishes; observe it via the tunnel's session snapshot). Safe
// to call from any goroutine (marshaled onto the reactor goroutine, R-2).
func (r *l2tpReactor) PlaceOutgoingCall(target DialTarget, p callParams) (uint16, error) {
	return r.dialWithCall(target, &pendingCall{outgoing: true, params: p})
}

// placeIncomingCall dials target and, once the tunnel establishes, originates
// a LAC-side incoming call (ICRQ) with the given call parameters. Returns the
// local tunnel ID. Safe to call from any goroutine (R-2).
func (r *l2tpReactor) placeIncomingCall(target DialTarget, p callParams) (uint16, error) {
	return r.dialWithCall(target, &pendingCall{outgoing: false, params: p})
}

// placeOutgoingCallSync dials target, originates an LNS-side outgoing call
// (OCRQ) once the tunnel establishes, and BLOCKS until the call reaches a
// terminal outcome (session established, or a failure: tunnel auth reject,
// tie-breaker loss, setup failure, or peer CDN) or timeout elapses. This is
// the surface the `request l2tp outgoing-call` RPC needs: unlike the
// fire-and-forget PlaceOutgoingCall, it reports whether the call actually
// came up and why it did not. Safe to call from any goroutine (the dial is
// marshaled onto the reactor goroutine, R-2; the result channel is buffered
// so the reactor never blocks on delivery even after a timeout).
func (r *l2tpReactor) placeOutgoingCallSync(target DialTarget, p callParams, timeout time.Duration) (callOutcome, error) {
	pc := &pendingCall{outgoing: true, params: p, result: make(chan callOutcome, 1)}
	if _, err := r.dialWithCall(target, pc); err != nil {
		return callOutcome{}, err
	}
	r.mu.Lock()
	stop := r.stop
	r.mu.Unlock()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case o := <-pc.result:
		return o, nil
	case <-timer.C:
		return callOutcome{}, ErrCallTimeout
	case <-stop:
		return callOutcome{}, ErrReactorStopped
	}
}

// dialWithCall is the shared marshaling path for Dial / PlaceOutgoingCall /
// placeIncomingCall / placeOutgoingCallSync.
func (r *l2tpReactor) dialWithCall(target DialTarget, call *pendingCall) (uint16, error) {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return 0, ErrReactorStopped
	}
	stop := r.stop
	r.mu.Unlock()

	req := dialRequest{target: target, call: call, result: make(chan dialResult, 1)}
	select {
	case r.dialCh <- req:
	case <-stop:
		return 0, ErrReactorStopped
	}
	select {
	case res := <-req.result:
		return res.localTID, res.err
	case <-stop:
		return 0, ErrReactorStopped
	}
}

// handleDial runs on the reactor goroutine. It allocates a local tunnel ID,
// creates an initiator tunnel, drives the idle -> wait-ctl-reply transition
// (sending the SCCRQ), transmits the datagram after releasing tunnelsMu, and
// arms the retransmit timer. The result (local TID or error) is returned to
// the Dial caller.
func (r *l2tpReactor) handleDial(req dialRequest) {
	now := r.params.Clock()

	// RFC 2661 Section 8.1: "The initiator of an L2TP tunnel picks an
	// available source UDP port (which may or may not be 1701), and sends to
	// the desired destination address at port 1701." and "Once the source and
	// destination ports and addresses are established, they MUST remain
	// static for the life of the tunnel." The source is the listener socket,
	// fixed for the tunnel's life, so a loopback-bound listener has no source
	// address that can ever reach an off-host destination.
	local := r.listener.Addr()
	if dialSourceUnreachable(local.Addr(), req.target.Remote.Addr()) {
		r.tunnelsMu.Lock()
		configured := r.params.ConfiguredListen
		r.tunnelsMu.Unlock()
		label := "remote " + req.target.Remote.String()
		err := dialFromLoopbackError(label, local, r.params.ServerName, req.target.Remote.Addr())
		// A reload already moved the listener off the loopback, but a
		// listener is bound only at start: repeating the applied fix would
		// send the operator in a circle.
		if listenerAwaitsRestart(configured, local, req.target.Remote.Addr()) {
			err = dialFromStaleListenerError(label, local, r.params.ServerName, configured)
		}
		r.logger.Warn("l2tp: dial refused; listener cannot reach the remote",
			"remote", req.target.Remote.String(), "listener", local.String(), "error", err.Error())
		req.result <- dialResult{err: err}
		return
	}

	r.tunnelsMu.Lock()
	// AC-10 (spec-home-cpe-14): an incoming call to a remote this LAC already
	// holds an established tunnel to rides that tunnel, so the sessions of
	// one remote share one tunnel.
	// RFC 2661 Section 5.3: "Multiple tunnels may
	// exist between a given LNS-LAC pair, and multiple sessions may exist
	// within a tunnel."
	if req.call != nil && !req.call.outgoing {
		// A tunnel to the same remote still connecting takes the call into
		// its queue; it is placed when that tunnel establishes.
		if t := r.connectingTunnelToLocked(req.target); t != nil {
			t.pendingCalls = append(t.pendingCalls, req.call)
			localTID := t.localTID
			r.tunnelsMu.Unlock()
			req.result <- dialResult{localTID: localTID}
			return
		}
		if t := r.establishedTunnelToLocked(req.target); t != nil {
			t.pendingCalls = append(t.pendingCalls, req.call)
			sends := r.placePendingCallLocked(t, now)
			localTID := t.localTID
			newDeadline := t.engine.NextDeadline()
			r.tunnelsMu.Unlock()
			r.transmit(localTID, sends)
			select {
			case r.updateCh <- heapUpdate{tunnelID: localTID, deadline: newDeadline}:
			case <-r.stop:
			}
			req.result <- dialResult{localTID: localTID}
			return
		}
	}
	if r.params.MaxTunnels != 0 && uint16(len(r.tunnelsByLocalID)) >= r.params.MaxTunnels {
		r.tunnelsMu.Unlock()
		r.logger.Warn("l2tp: dial rejected; max-tunnels limit reached",
			"remote", req.target.Remote.String(), "limit", r.params.MaxTunnels)
		req.result <- dialResult{err: ErrMaxTunnels}
		return
	}
	localTID, err := r.allocateLocalTID()
	if err != nil {
		r.tunnelsMu.Unlock()
		r.logger.Warn("l2tp: dial failed; local tunnel ID allocation",
			"remote", req.target.Remote.String(), "error", err.Error())
		req.result <- dialResult{err: err}
		return
	}

	// Effective defaults: a per-remote secret overrides the global default.
	defaults := r.params.Defaults
	if req.target.SharedSecret != "" {
		defaults.SharedSecret = req.target.SharedSecret
	}

	t := newTunnel(localTID, 0, req.target.Remote,
		r.reliableConfig(), r.logger, now)
	t.maxSessions = r.params.MaxSessions
	t.initiatorSecret = req.target.SharedSecret
	if req.call != nil {
		t.pendingCalls = []*pendingCall{req.call}
	}
	r.tunnelsByLocalID[localTID] = t

	// A random 8-byte Tie Breaker lets a simultaneous open (the peer sends
	// its own SCCRQ before our SCCRP) resolve deterministically (RFC 2661
	// S4.4.3). rand.Read on crypto/rand does not fail in practice; on the
	// impossible error we dial without a tie breaker (both tunnels survive,
	// which is legal per S5.3).
	tb := make([]byte, 8)
	if _, rerr := rand.Read(tb); rerr != nil {
		tb = nil
	}
	outbound := t.initiate(now, defaults, tb)
	newDeadline := t.engine.NextDeadline()
	r.tunnelsMu.Unlock()

	r.transmit(localTID, outbound)

	select {
	case r.updateCh <- heapUpdate{tunnelID: localTID, deadline: newDeadline}:
	case <-r.stop:
	}

	req.result <- dialResult{localTID: localTID}
}

// dialSourceUnreachable reports whether a socket bound to local can never send
// to remote: a loopback source reaches only a loopback destination. A wildcard
// or a host address leaves the choice to the kernel's route, so it is never
// refused here.
func dialSourceUnreachable(local, remote netip.Addr) bool {
	if !local.IsLoopback() {
		return false
	}
	return !remote.IsLoopback()
}

// dialFromLoopbackError builds the operator error for a loopback listener
// asked to reach an off-host remote: the problem, why, then the exact command
// on its own line. remote describes the remote in the caller's terms; server
// is the `environment l2tp server` key that declared the listener;
// remoteAddr picks the wildcard the command proposes. Shared by the config
// check (validateDialSources), the reload check (Reload) and the runtime
// refusal (handleDial).
func dialFromLoopbackError(remote string, listener netip.AddrPort, server string, remoteAddr netip.Addr) error {
	return fmt.Errorf("%w\n"+
		"The L2TP listener %s (environment l2tp server %s) is on the loopback, and %s is not.\n"+
		"Ze sends every tunnel from its listener, and the kernel does not send from a loopback address to another host.\n"+
		"Bind the listener to every address, or to an address that reaches the remote:\n"+
		"  set environment l2tp server %s ip %s",
		ErrDialFromLoopback, listener, server, remote, server, wildcardFor(remoteAddr))
}

// dialFromStaleListenerError builds the operator error for a loopback listener
// whose configuration a reload already moved to configured, an address that
// reaches the remote. A listener is bound only when ze starts, so the fix is a
// restart, not the command the operator has already applied.
func dialFromStaleListenerError(remote string, listener netip.AddrPort, server string, configured netip.AddrPort) error {
	return fmt.Errorf("%w\n"+
		"The L2TP listener %s (environment l2tp server %s) is on the loopback, and %s is not.\n"+
		"Ze sends every tunnel from its listener, and the kernel does not send from a loopback address to another host.\n"+
		"The configuration already sets this listener to %s, but ze binds a listener only when it starts.\n"+
		"Restart ze to rebind the listener",
		ErrDialFromLoopback, listener, server, remote, configured)
}

// listenerAwaitsRestart reports whether a reload moved a loopback listener to
// configured, an address that reaches remote, while the socket stays bound to
// local until ze restarts. An unset configured (a reactor no reload has
// touched) or an unchanged one is not a pending rebind.
func listenerAwaitsRestart(configured, local netip.AddrPort, remote netip.Addr) bool {
	if !configured.IsValid() {
		return false
	}
	if configured == local {
		return false
	}
	return !dialSourceUnreachable(configured.Addr(), remote)
}

// wildcardFor returns the unspecified address of remote's family, the bind
// that lets the route choose the source. The listener binds udp6 with
// IPV6_V6ONLY (listener.go), so an IPv6 remote needs `::`; an IPv4 remote,
// written plainly or IPv4-mapped, needs `0.0.0.0`.
func wildcardFor(remote netip.Addr) string {
	if remote.Is6() {
		if !remote.Is4In6() {
			return "::"
		}
	}
	return "0.0.0.0"
}

// dialChanDepth sizes the reactor's dial request channel. Dial callers block
// on delivery, so a small buffer only smooths brief bursts.
const dialChanDepth = 16

// placePendingCallLocked originates every call queued on a tunnel (the dial's
// own, then those that joined it while it connected), in arrival order, the
// moment the tunnel reaches established, and clears the queue so each fires
// once. Returns the ICRQ/OCRQ sends. Caller MUST hold tunnelsMu and MUST have
// verified the tunnel just transitioned to established, or that it is
// established. No-op when the queue is empty.
func (r *l2tpReactor) placePendingCallLocked(t *L2TPTunnel, now time.Time) []sendRequest {
	calls := t.pendingCalls
	t.pendingCalls = nil
	sends := make([]sendRequest, 0, len(calls)) // one ICRQ/OCRQ per queued call
	for _, pc := range calls {
		sends = append(sends, r.placeOnePendingCallLocked(t, pc, now)...)
	}
	return sends
}

// placeOnePendingCallLocked originates one queued call on the established
// tunnel t and returns its ICRQ/OCRQ send. Caller MUST hold tunnelsMu.
func (r *l2tpReactor) placeOnePendingCallLocked(t *L2TPTunnel, pc *pendingCall, now time.Time) []sendRequest {
	var localSID uint16
	var sends []sendRequest
	if pc.outgoing {
		localSID, sends = t.placeOutgoingCall(now, pc.params, r.logger)
	} else {
		localSID, sends = t.placeIncomingCall(now, pc.params, r.logger)
	}
	// Attach the caller's result channel to the newly-created session so its
	// establish/teardown signals reach the blocking RPC. A zero localSID
	// means the call was refused (max-sessions, allocation) -- surface that
	// synchronously instead of leaving the caller to time out.
	if localSID == 0 && pc.params.realm != nil {
		pc.params.realm.end("L2TP call to realm " + pc.params.realm.realm + " refused by this LAC (session limit)")
	}
	if pc.result == nil {
		return sends
	}
	if localSID == 0 {
		pc.result <- callOutcome{err: errCallPlacementRefused}
		return sends
	}
	if sess := t.lookupSession(localSID); sess != nil {
		sess.callResult = pc.result
	}
	return sends
}

// resolvePendingCall delivers a failure outcome to every call still waiting
// for its tunnel to establish (none was placed on a session) and clears the
// queue. Used by the tunnel-teardown paths (auth reject, tie-breaker loss,
// retransmit exhaustion) so a blocking placeOutgoingCallSync learns the
// tunnel died instead of timing out, and each queued realm call refuses its
// subscriber. A call with no result channel and no realm needs no report.
// Caller MUST hold tunnelsMu.
func (t *L2TPTunnel) resolvePendingCall(o callOutcome) {
	calls := t.pendingCalls
	t.pendingCalls = nil
	for _, pc := range calls {
		// A realm call whose tunnel died before the call was placed refuses
		// its subscriber; it carries no result channel.
		if pc.params.realm != nil {
			pc.params.realm.end("L2TP tunnel to realm " + pc.params.realm.realm + " failed before the call was placed")
			continue
		}
		if pc.result == nil {
			continue
		}
		pc.result <- o
	}
}

// transmit sends every outbound datagram for a tunnel, mirroring the
// capture-aware send loop in handle(). Called AFTER releasing tunnelsMu so a
// slow UDP write does not serialize inbound dispatch.
func (r *l2tpReactor) transmit(localTID uint16, outbound []sendRequest) {
	for _, req := range outbound {
		if r.capture != nil && len(req.bytes) > 12 {
			outSID := uint16(req.bytes[8])<<8 | uint16(req.bytes[9])
			r.capture.appendOutbound(localTID, outSID, extractMsgType(req.bytes[12:]), req.to, len(req.bytes))
		}
		if rc := r.rawCapture.Load(); rc != nil {
			rc.Append(1, req.bytes)
		}
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed",
				"to", req.to.String(), "len", len(req.bytes), "error", err.Error())
		}
	}
}

// pendingCallsMax bounds the calls one connecting tunnel queues. A call past
// it opens a tunnel of its own, which RFC 2661 Section 5.3 allows ("Multiple
// tunnels may exist between a given LNS-LAC pair"). One PPPoE subscriber
// queues at most one call, so the bound is subscribers authenticating while
// one SCCRQ is answered.
const pendingCallsMax = 256

// connectingTunnelToLocked returns a tunnel this LAC initiated to target's
// address with target's shared secret that is still waiting for SCCRP, with
// room in its call queue, or nil. Caller MUST hold tunnelsMu.
func (r *l2tpReactor) connectingTunnelToLocked(target DialTarget) *L2TPTunnel {
	for _, t := range r.tunnelsByLocalID {
		if t.state != L2TPTunnelWaitCtlReply {
			continue
		}
		if len(t.pendingCalls) >= pendingCallsMax {
			continue
		}
		if t.peerAddr.Addr() != target.Remote.Addr() {
			continue
		}
		if t.initiatorSecret != target.SharedSecret {
			continue
		}
		return t
	}
	return nil
}

// establishedTunnelToLocked returns an established tunnel to target's
// address that authenticated with target's shared secret, or nil. An
// established tunnel has already placed its queued calls. The peer's UDP port is not compared: RFC 2661 Section 8.1
// lets the peer answer from another port than the one dialed. Caller MUST
// hold tunnelsMu.
func (r *l2tpReactor) establishedTunnelToLocked(target DialTarget) *L2TPTunnel {
	for _, t := range r.tunnelsByLocalID {
		if t.state != L2TPTunnelEstablished {
			continue
		}
		if t.peerAddr.Addr() != target.Remote.Addr() {
			continue
		}
		if t.initiatorSecret != target.SharedSecret {
			continue
		}
		return t
	}
	return nil
}
