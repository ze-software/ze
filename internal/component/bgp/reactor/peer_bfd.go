// Design: rfc/short/rfc5882.md -- BFD client contract
// Overview: peer.go -- Peer struct and lifecycle
// Related: peer_run.go -- FSM callback that starts/stops the BFD client
//
// BFD client glue for a BGP peer. When the operator opts into BFD via
// `bgp peer connection bfd { ... }`, the FSM callback calls
// startBFDClient() on StateEstablished and stopBFDClient() on exit
// from Established. The client:
//
//  1. Calls api.GetService() to reach the in-process BFD engine. If
//     nil (BFD plugin not loaded), the peer runs without BFD and logs
//     a warning -- the BGP session is not blocked.
//  2. Builds a SessionRequest from PeerSettings and calls
//     Service.EnsureSession.
//  3. Subscribes to state changes on the returned handle.
//  4. Runs a per-session subscriber goroutine that translates a BFD
//     Down / AdminDown event into a Peer.Teardown with RFC 9384 Cease
//     subcode 10 ("BFD Down"). The BGP session drops without waiting
//     for the hold timer.
//
// Lifecycle: the subscriber goroutine is a per-session worker (not
// per-event) per rules/goroutine-lifecycle.md. It exits when either
// stopBFDClient closes the stop channel or the subscription channel
// closes (handle released or loop torn down). stopBFDClient waits on
// the done channel so the goroutine has exited by the time it
// returns.
package reactor

import (
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// bfdClient holds the per-peer BFD session state. Zero value is safe
// (unused until the FSM transitions to Established and BFD is opted
// in). All fields are protected by mu because stopBFDClient may be
// called concurrently with the subscriber goroutine draining the
// subscription channel.
type bfdClient struct {
	mu     sync.Mutex
	svc    api.Service
	handle api.SessionHandle
	sub    <-chan api.StateChange
	stop   chan struct{}
	done   chan struct{}
}

// startBFDClient opens a BFD session for this peer when the peer has
// opted in via config and the BFD plugin is running in the same
// process. Called from the FSM callback on StateEstablished. No-op if:
//
//   - PeerSettings.BFD is nil (operator did not opt in)
//   - PeerSettings.BFD.Enabled is false (opt-in suspended)
//   - api.GetService returns nil (BFD plugin not loaded)
//   - EnsureSession returns an error (logged, peer runs without BFD)
//
// In all of these cases the BGP session continues normally; BFD is
// strictly additive to the BGP hold-timer detection.
func (p *Peer) startBFDClient() {
	cfg := p.settings.BFD
	if cfg == nil || !cfg.Enabled {
		return
	}
	svc := api.GetService()
	if svc == nil {
		peerLogger().Warn("bfd configured on peer but BFD plugin not loaded; peer runs without BFD",
			"peer", p.settings.Address)
		return
	}
	req := bfdRequestFor(p.settings)
	handle, err := svc.EnsureSession(req)
	if err != nil {
		peerLogger().Warn("bfd EnsureSession failed; peer runs without BFD",
			"peer", p.settings.Address, "err", err)
		return
	}
	sub := handle.Subscribe()
	stop := make(chan struct{})
	done := make(chan struct{})

	p.bfd.mu.Lock()
	p.bfd.svc = svc
	p.bfd.handle = handle
	p.bfd.sub = sub
	p.bfd.stop = stop
	p.bfd.done = done
	p.bfd.mu.Unlock()

	peerLogger().Info("bfd session opened for peer",
		"peer", p.settings.Address,
		"multi-hop", cfg.MultiHop,
		"profile", cfg.Profile)

	go p.runBFDSubscriber(handle, sub, stop, done)
}

// runBFDSubscriber is the per-session subscriber worker. It drains the
// subscription channel until either stop is signaled or the channel
// closes (handle released). A Down / AdminDown transition triggers
// Peer.Teardown with RFC 9384 Cease subcode 10 ("BFD Down").
//
// Up and Init transitions are logged at debug but not acted on: the
// BGP session is independent of the BFD Up state. BFD is a failure
// detector, not a session driver.
func (p *Peer) runBFDSubscriber(
	handle api.SessionHandle,
	sub <-chan api.StateChange,
	stop <-chan struct{},
	done chan<- struct{},
) {
	_ = handle // retained for future Shutdown/Enable integration
	defer close(done)
	for {
		select {
		case <-stop:
			return
		case change, ok := <-sub:
			if !ok {
				return
			}
			if change.State == packet.StateDown || change.State == packet.StateAdminDown {
				peerLogger().Warn("bfd reported peer down; tearing BGP session",
					"peer", p.settings.Address,
					"bfd-state", change.State.String(),
					"bfd-diag", change.Diag.String())
				// RFC 9384: Cease subcode 10 is reserved for
				// "BFD Down". Hold timer is bypassed.
				if err := p.Teardown(message.NotifyCeaseBFDDown, "BFD detected forwarding path down"); err != nil {
					peerLogger().Debug("bfd-driven teardown failed",
						"peer", p.settings.Address, "err", err)
				}
				continue
			}
			peerLogger().Debug("bfd state change",
				"peer", p.settings.Address,
				"bfd-state", change.State.String())
		}
	}
}

// stopBFDClient releases the BFD handle, waits for the subscriber
// goroutine to exit, and clears the per-peer state so a subsequent
// startBFDClient is race-free. Idempotent: a no-op when no BFD session
// is currently open. Called from the FSM callback on exit from
// StateEstablished, and from the peer's deferred cleanup in runOnce.
func (p *Peer) stopBFDClient() {
	p.bfd.mu.Lock()
	svc := p.bfd.svc
	handle := p.bfd.handle
	sub := p.bfd.sub
	stop := p.bfd.stop
	done := p.bfd.done
	p.bfd.svc = nil
	p.bfd.handle = nil
	p.bfd.sub = nil
	p.bfd.stop = nil
	p.bfd.done = nil
	p.bfd.mu.Unlock()

	if handle == nil {
		return
	}
	if stop != nil {
		close(stop)
	}
	handle.Unsubscribe(sub)
	if svc != nil {
		if err := svc.ReleaseSession(handle); err != nil {
			peerLogger().Debug("bfd ReleaseSession failed",
				"peer", p.settings.Address, "err", err)
		}
	}
	if done != nil {
		<-done
	}
	peerLogger().Info("bfd session closed for peer", "peer", p.settings.Address)
}

// bfdRequestFor builds an api.SessionRequest from PeerSettings. The
// peer's Address and LocalAddress supply the session tuple; the BFD
// block supplies mode, min-TTL, and the optional egress interface.
// Timer fields are left zero so the BFD plugin uses its profile-driven
// defaults (the profile name is not carried in the SessionRequest
// because api.SessionRequest is timer-valued; profile resolution
// happens on the plugin side in a future pass).
func bfdRequestFor(s *PeerSettings) api.SessionRequest {
	mode := api.SingleHop
	if s.BFD != nil && s.BFD.MultiHop {
		mode = api.MultiHop
	}
	req := api.SessionRequest{
		Peer:  s.Address,
		Local: s.LocalAddress,
		Mode:  mode,
	}
	if s.BFD != nil {
		req.Interface = s.BFD.Interface
		req.MinTTL = s.BFD.MinTTL
	}
	return req
}
