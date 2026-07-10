// Design: docs/research/l2tpv2-ze-integration.md -- L2TP reactor dial path
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 6.1 (LAC/initiator SCCRQ)
// Related: tunnel_initiator.go -- L2TPTunnel.initiate builds the SCCRQ
// Related: reactor.go -- the single reactor goroutine that owns the tunnel map

package l2tp

import (
	"crypto/rand"
	"errors"
	"net/netip"
)

// ErrReactorStopped is returned by Dial when the reactor goroutine is not
// running (Stop already called, or Start never called).
var ErrReactorStopped = errors.New("l2tp: reactor stopped")

// ErrMaxTunnels is returned by Dial when the configured max-tunnels limit
// has been reached.
var ErrMaxTunnels = errors.New("l2tp: max-tunnels limit reached")

// DialTarget describes a remote to initiate a tunnel toward. Remote is the
// peer's control address:port (typically UDP 1701). SharedSecret is the
// per-remote CHAP-MD5 tunnel-authentication secret; empty disables our end
// of authentication for this dial (RFC 2661 Section 4.2).
type DialTarget struct {
	Remote       netip.AddrPort
	SharedSecret string
}

// dialRequest is one Dial marshaled onto the reactor goroutine. result
// carries the outcome back to the caller. Buffered with capacity 1 by Dial so
// handleDial never blocks delivering the result even if the caller has gone.
type dialRequest struct {
	target DialTarget
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
// is reached, or the tunnel-ID allocation error.
func (r *L2TPReactor) Dial(target DialTarget) (uint16, error) {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return 0, ErrReactorStopped
	}
	stop := r.stop
	r.mu.Unlock()

	req := dialRequest{target: target, result: make(chan dialResult, 1)}
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
func (r *L2TPReactor) handleDial(req dialRequest) {
	now := r.params.Clock()

	r.tunnelsMu.Lock()
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
		ReliableConfig{RecvWindow: r.params.Defaults.RecvWindow}, r.logger, now)
	t.maxSessions = r.params.MaxSessions
	t.initiatorSecret = req.target.SharedSecret
	r.tunnelsByLocalID[localTID] = t

	// A random 8-byte Tie Breaker lets a simultaneous open (the peer sends
	// its own SCCRQ before our SCCRP) resolve deterministically (RFC 2661
	// S9.5). rand.Read on crypto/rand does not fail in practice; on the
	// impossible error we dial without a tie breaker (both tunnels survive,
	// which is legal per S24.17).
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

// dialChanDepth sizes the reactor's dial request channel. Dial callers block
// on delivery, so a small buffer only smooths brief bursts.
const dialChanDepth = 16

// transmit sends every outbound datagram for a tunnel, mirroring the
// capture-aware send loop in handle(). Called AFTER releasing tunnelsMu so a
// slow UDP write does not serialize inbound dispatch.
func (r *L2TPReactor) transmit(localTID uint16, outbound []sendRequest) {
	for _, req := range outbound {
		if r.capture != nil && len(req.bytes) > 12 {
			outSID := uint16(req.bytes[8])<<8 | uint16(req.bytes[9])
			r.capture.AppendOutbound(localTID, outSID, extractMsgType(req.bytes[12:]), req.to, len(req.bytes))
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
