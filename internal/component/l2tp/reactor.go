// Design: docs/research/l2tpv2-ze-integration.md -- L2TP reactor pattern
// Related: listener.go -- source of incoming packets and outbound sender
// Related: subsystem.go -- owns the reactor's lifecycle

package l2tp

import (
	"errors"
	"log/slog"
	"sync"
)

// L2TPReactor is the single goroutine that dispatches incoming L2TP
// datagrams to tunnel state machines. Phase 2 implements only the
// pre-FSM path: header parse, version reject / drop, malformed drop.
// Tunnel lookup, FSM transitions, and timer integration arrive in
// subsequent phases.
//
// Caller MUST call Stop after Start. Start is not idempotent; the
// underlying UDPListener must already be Start()ed before the reactor
// runs.
type L2TPReactor struct {
	listener *UDPListener
	logger   *slog.Logger

	mu      sync.Mutex
	stop    chan struct{}
	wg      sync.WaitGroup
	started bool
}

// NewL2TPReactor constructs a reactor bound to the given listener. The
// listener must be started before the reactor is started; the reactor
// does not manage the listener's lifecycle.
func NewL2TPReactor(listener *UDPListener, logger *slog.Logger) *L2TPReactor {
	if logger == nil {
		logger = slog.Default()
	}
	return &L2TPReactor{listener: listener, logger: logger}
}

// Start launches the reactor goroutine. Returns an error if already started.
func (r *L2TPReactor) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return errors.New("l2tp: reactor already started")
	}
	r.stop = make(chan struct{})
	r.started = true
	r.wg.Add(1)
	go r.run()
	return nil
}

// Stop signals the reactor to exit and waits for it. Idempotent.
func (r *L2TPReactor) Stop() {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return
	}
	r.started = false
	stop := r.stop
	r.mu.Unlock()

	close(stop)
	r.wg.Wait()
}

// run is the reactor's main loop. It consumes packets from the listener
// and dispatches each to handle. The loop exits when the listener's RX
// channel closes (Stop was called on the listener) or when r.stop fires.
// On stop, any packets already buffered in the RX channel are drained
// with release() only -- not dispatched -- so the listener's slot pool
// frees promptly instead of waiting for GC.
func (r *L2TPReactor) run() {
	defer r.wg.Done()
	rx := r.listener.RX()
	for {
		select {
		case pkt, ok := <-rx:
			if !ok {
				return
			}
			r.handle(pkt)
		case <-r.stop:
			r.drainOnStop(rx)
			return
		}
	}
}

// drainOnStop releases every packet currently buffered in rx without
// dispatching it. Called from run() exclusively after r.stop has fired.
// Keeps the listener's slot pool available for readLoop's own shutdown
// path, which otherwise would wait on GC to reclaim abandoned closures.
// Bounded by rxPoolSize (the listener cannot produce more than that in
// flight at any moment).
func (r *L2TPReactor) drainOnStop(rx <-chan rxPacket) {
	for len(rx) > 0 {
		pkt, ok := <-rx
		if !ok {
			return
		}
		pkt.release()
	}
}

// handle processes one received datagram. Phase 2 scope:
//   - Too-short datagrams are silently dropped.
//   - Unsupported version (Ver=1 L2F, Ver=3 L2TPv3) dropped with debug
//     log; StopCCN RC=5 synthesis for Ver=3 arrives in phase 3 together
//     with the encode helpers for control messages.
//   - Malformed valid-Ver=2 headers dropped with debug log.
//   - Valid Ver=2 datagrams are accepted for future tunnel dispatch
//     (phase 3). Phase 2 logs and drops.
//
// The pool slot is released before return. `bytes` must NOT be retained.
func (r *L2TPReactor) handle(pkt rxPacket) {
	defer pkt.release()

	if len(pkt.bytes) < 6 {
		r.logger.Debug("l2tp: short datagram dropped", "from", pkt.from.String(), "len", len(pkt.bytes))
		return
	}

	hdr, err := ParseMessageHeader(pkt.bytes)
	if err != nil {
		if errors.Is(err, ErrUnsupportedVersion) {
			// Lower nibble of byte 1 carries the Version field. Phase 1
			// returns ErrUnsupportedVersion for any Ver != 2.
			ver := pkt.bytes[1] & 0x0F
			// RFC 2661 S24.1: L2F (Ver=1) is silently discarded. L2TPv3
			// (Ver=3) should trigger a StopCCN Result Code 5 response
			// (implemented in phase 3 once the encode machinery is in
			// place). Phase 2 drops both with distinct log levels so
			// operators can spot v3 peers during rollout.
			if ver == 3 {
				r.logger.Warn("l2tp: L2TPv3 peer rejected (StopCCN emission arrives in phase 3)", "from", pkt.from.String())
				return
			}
			r.logger.Debug("l2tp: unsupported version dropped", "from", pkt.from.String(), "version", ver)
			return
		}
		r.logger.Debug("l2tp: malformed header dropped", "from", pkt.from.String(), "error", err.Error())
		return
	}

	// Phase 3 will look up the tunnel by hdr.TunnelID and dispatch to
	// the FSM. Phase 2 acknowledges receipt and drops.
	r.logger.Debug("l2tp: valid v2 packet received (tunnel dispatch arrives in phase 3)",
		"from", pkt.from.String(),
		"tunnel-id", hdr.TunnelID,
		"session-id", hdr.SessionID,
		"is-control", hdr.IsControl,
		"length", hdr.Length)
}
