// Design: docs/architecture/l2tp/followup-l2tp-call.md -- AC-4 operator-initiated outgoing call
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 7.9 (OCRQ), Section 10.4 (LNS outgoing call)
// Related: reactor_dial.go -- placeOutgoingCallSync drives the dial + OCRQ
// Related: cmd/l2tp.go -- the request l2tp outgoing-call RPC handler

package l2tp

import (
	"errors"
	"fmt"
	"net/netip"
	"time"
)

// outgoingCallTimeout bounds how long the RPC blocks waiting for a call to
// establish or fail. It sits below the reliable engine's ~31s retransmit
// exhaustion so a fast failure (auth reject, tie-breaker loss, peer CDN)
// returns its cause while a truly dead peer returns a timeout rather than
// hanging the CLI. The tunnel keeps trying in the background regardless.
const outgoingCallTimeout = 20 * time.Second

var (
	errNoOutgoingRemote     = errors.New("l2tp: no remote configured with that name")
	errRemoteNoOutgoing     = errors.New("l2tp: remote is not permitted for outgoing calls (set outgoing-calls true)")
	errNoReactorForOutgoing = errors.New("l2tp: no L2TP listener available to place the call")
)

// OutgoingCallResult reports the outcome of a request l2tp outgoing-call to
// the CLI. Established distinguishes a call that came up (LocalSID/RemoteSID
// valid) from one that failed; ResultCode carries the RFC 2661 CDN/StopCCN
// Result Code when the peer or ze supplied one (0 otherwise).
type OutgoingCallResult struct {
	Remote      string
	Called      string
	Established bool
	LocalSID    uint16
	RemoteSID   uint16
	ResultCode  uint16
}

// PlaceOutgoingCall resolves remoteName against the configured dial targets,
// verifies the remote permits outgoing calls, dials it, originates an OCRQ
// once the tunnel establishes, and blocks until the call reaches a terminal
// outcome (established or failed) or times out. It is the engine side of the
// `request l2tp outgoing-call remote <name> called <number>` RPC.
//
// The subsystem lock is held only to snapshot the remote and pick a reactor;
// the blocking wait runs unlocked so a concurrent Reload/Stop is never
// serialized behind a slow call setup.
//
// Safe for concurrent use.
func (s *Subsystem) PlaceOutgoingCall(remoteName, calledNumber string) (OutgoingCallResult, error) {
	s.mu.Lock()
	rem, ok := s.params.LookupRemote(remoteName)
	var reactor *l2tpReactor
	var reactorErr error
	if ok {
		if rem.OutgoingCalls {
			reactor, reactorErr = s.reactorForRemoteLocked(rem.Address, errNoReactorForOutgoing)
		}
	}
	s.mu.Unlock()

	res := OutgoingCallResult{Remote: remoteName, Called: calledNumber}
	if !ok {
		return res, fmt.Errorf("%w: %q", errNoOutgoingRemote, remoteName)
	}
	if !rem.OutgoingCalls {
		return res, fmt.Errorf("%w: %q", errRemoteNoOutgoing, remoteName)
	}
	if reactorErr != nil {
		return res, reactorErr
	}

	target := DialTarget{Remote: rem.Address, SharedSecret: rem.SharedSecret}
	params := callParams{calledNumber: calledNumber}
	outcome, err := reactor.placeOutgoingCallSync(target, params, outgoingCallTimeout)
	if err != nil {
		// Transport-level failure (timeout, reactor stopped, dial rejected):
		// no call outcome was produced.
		return res, err
	}
	res.LocalSID = outcome.localSID
	res.RemoteSID = outcome.remoteSID
	res.ResultCode = outcome.resultCode
	if outcome.err != nil {
		// The call reached a terminal FAILURE (auth reject, tie-breaker loss,
		// peer CDN, setup failure). Surface the cause and any result code.
		return res, outcome.err
	}
	res.Established = true
	return res, nil
}

// reactorForRemoteLocked picks the reactor whose bound UDP listener can reach
// remote. Caller MUST hold s.mu.
//
// Selection matches IP address family (udp4 vs IPV6_V6ONLY udp6) and loopback
// compatibility:
//   - For a loopback remote, a loopback listener of the same family is
//     preferred; if none matches, the first non-loopback listener of the same
//     family is used.
//   - For an off-host remote, the first non-loopback listener of the same
//     family is used.
//
// When no reactors are running, returns noListenerErr. When reactors exist but
// none can reach remote, returns an operator error naming the remote and the
// available listeners (ErrDialFromLoopback when a same-family loopback
// listener is the blocker, or ErrNoReachingListener when no listener matches
// the remote's IP address family).
func (s *Subsystem) reactorForRemoteLocked(remote netip.AddrPort, noListenerErr error) (*l2tpReactor, error) {
	if len(s.reactors) == 0 {
		return nil, noListenerErr
	}
	remoteAddr := remote.Addr()
	if remoteAddr.IsLoopback() {
		for _, r := range s.reactors {
			if r.listener == nil {
				continue
			}
			local := r.listener.Addr()
			if !local.IsValid() {
				continue
			}
			if !local.Addr().IsLoopback() {
				continue
			}
			if sameIPFamily(local.Addr(), remoteAddr) {
				return r, nil
			}
		}
	}
	for _, r := range s.reactors {
		if r.listener == nil {
			continue
		}
		local := r.listener.Addr()
		if !local.IsValid() {
			continue
		}
		if listenerCanReach(local.Addr(), remoteAddr) {
			return r, nil
		}
	}
	return nil, s.unreachableRemoteErrorLocked(remote)
}

// unreachableRemoteErrorLocked builds the operator error when s.reactors is
// non-empty but no bound listener can reach remote. Caller MUST hold s.mu.
func (s *Subsystem) unreachableRemoteErrorLocked(remote netip.AddrPort) error {
	remoteAddr := remote.Addr()
	label := "remote " + remote.String()
	var loopbackReactor *l2tpReactor
	var loopbackLocal netip.AddrPort
	bound := make([]netip.AddrPort, 0, len(s.reactors))
	for _, r := range s.reactors {
		if r.listener == nil {
			continue
		}
		local := r.listener.Addr()
		if !local.IsValid() {
			continue
		}
		bound = append(bound, local)
		if !sameIPFamily(local.Addr(), remoteAddr) {
			continue
		}
		r.tunnelsMu.Lock()
		configured := r.params.ConfiguredListen
		r.tunnelsMu.Unlock()
		if listenerAwaitsRestart(configured, local, remoteAddr) {
			return dialFromStaleListenerError(label, local, r.params.ServerName, configured)
		}
		if loopbackReactor == nil {
			loopbackReactor = r
			loopbackLocal = local
		}
	}
	if loopbackReactor != nil {
		return dialFromLoopbackError(label, loopbackLocal, loopbackReactor.params.ServerName, remoteAddr)
	}
	return noReachingListenerError(label, bound, remoteAddr)
}
