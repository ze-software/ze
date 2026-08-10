// Design: docs/architecture/l2tp/followup-l2tp-call.md -- AC-4 operator-initiated outgoing call
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 7.9 (OCRQ), Section 10.4 (LNS outgoing call)
// Related: reactor_dial.go -- placeOutgoingCallSync drives the dial + OCRQ
// Related: cmd/l2tp.go -- the request l2tp outgoing-call RPC handler

package l2tp

import (
	"errors"
	"fmt"
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
	var reactor *L2TPReactor
	if len(s.reactors) > 0 {
		reactor = s.reactors[0]
	}
	s.mu.Unlock()

	res := OutgoingCallResult{Remote: remoteName, Called: calledNumber}
	if !ok {
		return res, fmt.Errorf("%w: %q", errNoOutgoingRemote, remoteName)
	}
	if !rem.OutgoingCalls {
		return res, fmt.Errorf("%w: %q", errRemoteNoOutgoing, remoteName)
	}
	if reactor == nil {
		return res, errNoReactorForOutgoing
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
