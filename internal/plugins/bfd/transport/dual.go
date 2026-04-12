// Design: rfc/short/rfc5881.md -- single-hop UDP encapsulation
// Design: rfc/short/rfc5883.md -- multi-hop UDP encapsulation
// Related: udp.go -- IPv4 UDP transport wrapped by Dual
// Related: socket.go -- Transport interface contract
//
// Dual wraps two UDP transports -- one IPv4, one IPv6 -- behind a
// single Transport interface so engine.Loop can handle mixed-family
// deployments without caring about which socket saw the packet.
//
// Both inner transports push their Inbounds onto the same merged
// channel; Send dispatches to the v4 or v6 inner based on the
// destination address family. Closing is ordered: Stop() closes
// both inner transports, then closes the merged channel after both
// readers have drained. The channel close is deferred via a small
// drain-counter goroutine so no caller ever observes a half-closed
// state.
//
// Stage 2b uses Dual to pair an IPv4 UDP transport with an
// optional IPv6 companion; operators who want IPv6 BFD add
// `bfd { bind-v6 true }` (the YANG flag toggles Dual vs UDP). When
// the v6 side is disabled the plugin creates a bare UDP directly,
// avoiding the indirection.
package transport

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Dual is a Transport that fans out to an IPv4 and an IPv6 inner
// transport. Both inners MUST be constructed with matching Mode and
// VRF so the merged Inbound stream carries consistent metadata.
type Dual struct {
	V4 *UDP
	V6 *UDP

	merged  chan Inbound
	started atomic.Bool
	stopped atomic.Bool
	wg      sync.WaitGroup
	closeWG sync.WaitGroup
}

// errDualAlreadyStarted is returned if Start is called twice.
var errDualAlreadyStarted = errors.New("bfd: Dual transport already started")

// errDualMissingInners is returned if neither v4 nor v6 is set.
var errDualMissingInners = errors.New("bfd: Dual transport has no inner UDP transports")

// Start binds both inner transports (v4 and/or v6 as configured)
// and launches merge goroutines that drain their RX channels into
// the Dual's merged channel. Start is NOT idempotent.
func (d *Dual) Start() error {
	if !d.started.CompareAndSwap(false, true) {
		return errDualAlreadyStarted
	}
	if d.V4 == nil && d.V6 == nil {
		return errDualMissingInners
	}

	d.merged = make(chan Inbound, 256)

	if d.V4 != nil {
		if err := d.V4.Start(); err != nil {
			d.started.Store(false)
			return err
		}
		d.spawnMerge(d.V4)
	}
	if d.V6 != nil {
		if err := d.V6.Start(); err != nil {
			if d.V4 != nil {
				if stopErr := d.V4.Stop(); stopErr != nil {
					transportLog().Warn("bfd Dual: v4 stop after failed v6 start", "err", stopErr)
				}
			}
			d.started.Store(false)
			return err
		}
		d.spawnMerge(d.V6)
	}

	// A parent waiter closes the merged channel once every inner
	// reader exits so the engine sees a clean channel-close.
	d.closeWG.Go(func() {
		d.wg.Wait()
		close(d.merged)
	})
	return nil
}

// spawnMerge starts a goroutine that copies Inbounds from one
// inner transport onto the merged channel. Stops when the inner
// transport closes its RX channel.
func (d *Dual) spawnMerge(inner *UDP) {
	d.wg.Go(func() {
		for in := range inner.RX() {
			d.merged <- in
		}
	})
}

// Stop closes both inner transports. Idempotent.
func (d *Dual) Stop() error {
	if !d.stopped.CompareAndSwap(false, true) {
		return nil
	}
	var firstErr error
	if d.V4 != nil {
		if err := d.V4.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if d.V6 != nil {
		if err := d.V6.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RX returns the merged inbound channel.
func (d *Dual) RX() <-chan Inbound { return d.merged }

// Send routes the Outbound to the v4 or v6 inner transport based
// on the destination address family. If the matching inner is nil
// (e.g., v6 destination but only v4 configured) Send returns an
// error so the engine can log and move on.
func (d *Dual) Send(out Outbound) error {
	if out.To.Is6() && !out.To.Is4In6() {
		if d.V6 == nil {
			return errors.New("bfd: Dual has no IPv6 transport for v6 peer")
		}
		return d.V6.Send(out)
	}
	if d.V4 == nil {
		return errors.New("bfd: Dual has no IPv4 transport for v4 peer")
	}
	return d.V4.Send(out)
}

// Compile-time check that Dual satisfies the Transport interface.
var _ Transport = (*Dual)(nil)

// Wrap promotes an existing UDP transport to a Dual with no v6
// companion. Used by the BFD plugin when IPv6 is disabled -- the
// engine uniformly calls through the Transport interface and the
// Dual is effectively a zero-cost passthrough.
func Wrap(v4 *UDP) *Dual { return &Dual{V4: v4} }
