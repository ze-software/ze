// Design: docs/research/l2tpv2-ze-integration.md -- L2TP reactor pattern
// Related: listener.go -- source of incoming packets and outbound sender
// Related: tunnel.go -- dispatch target and FSM state holder
// Related: tunnel_fsm.go -- message handling inside a tunnel
// Related: subsystem.go -- owns the reactor's lifecycle

package l2tp

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"time"
)

// peerKey uniquely identifies a tunnel during SCCRQ retransmit dedup.
// RFC 2661 S24.17 allows multiple tunnels between the same IP pair, so
// peer address alone is insufficient; the peer's Assigned Tunnel ID AVP
// disambiguates.
type peerKey struct {
	addr netip.AddrPort
	tid  uint16
}

// ReactorParams carries the per-reactor configuration. Constructed by
// the subsystem from parsed Parameters + hardcoded defaults (phase 3
// hardcodes host name and capabilities; phase 7 wires them through
// YANG).
type ReactorParams struct {
	MaxTunnels uint16 // 0 = unbounded (by this knob; uint16 still caps at 65535)
	Defaults   TunnelDefaults
	Clock      func() time.Time // injected for tests; time.Now if nil
}

// L2TPReactor is the single goroutine that owns the tunnel map and
// dispatches incoming datagrams to per-tunnel FSMs. All per-tunnel state
// (ReliableEngine, FSM state, peer addr:port) is mutated exclusively
// from this goroutine, which matches the phase-2 contract that
// ReliableEngine is not safe for concurrent use.
//
// tunnelsMu protects the two tunnel maps and the per-tunnel fields
// that tests introspect (state, peerAddr, peerHostName). Runtime hot
// paths (the reactor goroutine itself) still hold the lock for the
// brief moments of map mutation and tunnel state transition; tests
// grab it through TunnelCount/TunnelByLocalID/State accessors.
//
// Caller MUST call Stop after Start. Start is not idempotent; the
// underlying UDPListener must already be Start()ed before the reactor
// runs.
type L2TPReactor struct {
	listener *UDPListener
	logger   *slog.Logger
	params   ReactorParams

	tunnelsMu        sync.Mutex
	tunnelsByLocalID map[uint16]*L2TPTunnel
	tunnelsByPeer    map[peerKey]*L2TPTunnel
	nextLocalTID     uint16

	mu      sync.Mutex
	stop    chan struct{}
	wg      sync.WaitGroup
	started bool
}

// NewL2TPReactor constructs a reactor bound to the given listener. The
// listener must be started before the reactor is started; the reactor
// does not manage the listener's lifecycle.
func NewL2TPReactor(listener *UDPListener, logger *slog.Logger, params ReactorParams) *L2TPReactor {
	if logger == nil {
		logger = slog.Default()
	}
	if params.Clock == nil {
		params.Clock = time.Now
	}
	if params.Defaults.RecvWindow == 0 {
		params.Defaults.RecvWindow = 16
	}
	if params.Defaults.HostName == "" {
		params.Defaults.HostName = "ze"
	}
	return &L2TPReactor{
		listener:         listener,
		logger:           logger,
		params:           params,
		tunnelsByLocalID: make(map[uint16]*L2TPTunnel),
		tunnelsByPeer:    make(map[peerKey]*L2TPTunnel),
	}
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

// handle processes one received datagram:
//   - Drop too-short and unsupported-version datagrams (phase 2 behavior).
//   - TunnelID == 0: parse the AVP body first; if the body is a
//     well-formed SCCRQ the reactor looks up or creates a tunnel keyed
//     by (peer addr:port, peer's Assigned Tunnel ID). A malformed body
//     is dropped BEFORE any state is allocated, so a peer cannot fill
//     our tunnel map with half-formed SCCRQs.
//   - TunnelID != 0: look up by local TID; update peerAddr per S24.19.
//   - Hand the packet to the tunnel's Process method which runs the
//     reliable engine and FSM, then send every resulting outbound
//     datagram AFTER releasing tunnelsMu so a slow UDP write does not
//     serialize inbound dispatch.
//
// The pool slot is released before return. `bytes` MUST NOT be retained
// past this call.
func (r *L2TPReactor) handle(pkt rxPacket) {
	defer pkt.release()

	if len(pkt.bytes) < 6 {
		r.logger.Debug("l2tp: short datagram dropped", "from", pkt.from.String(), "len", len(pkt.bytes))
		return
	}

	hdr, err := ParseMessageHeader(pkt.bytes)
	if err != nil {
		if errors.Is(err, ErrUnsupportedVersion) {
			ver := pkt.bytes[1] & 0x0F
			if ver == 3 {
				r.logger.Warn("l2tp: L2TPv3 peer rejected (StopCCN emission arrives in later phase)", "from", pkt.from.String())
				return
			}
			r.logger.Debug("l2tp: unsupported version dropped", "from", pkt.from.String(), "version", ver)
			return
		}
		r.logger.Debug("l2tp: malformed header dropped", "from", pkt.from.String(), "error", err.Error())
		return
	}
	if !hdr.IsControl {
		// Phase 3 does not touch data-plane packets; Linux's l2tp_ppp
		// kernel module handles those. Drop silently.
		return
	}
	payload := pkt.bytes[hdr.PayloadOff:int(hdr.Length)]

	// For TunnelID=0 (expected to be SCCRQ) parse the full AVP body
	// BEFORE grabbing tunnelsMu. A malformed body is rejected here,
	// without allocating a tunnel entry or consuming a local TID.
	var sccrq *sccrqInfo
	if hdr.TunnelID == 0 {
		info, perr := parseSCCRQ(payload)
		if perr != nil {
			r.logger.Debug("l2tp: TunnelID=0 packet with malformed body dropped",
				"from", pkt.from.String(), "error", perr.Error())
			return
		}
		if info.MessageType != MsgSCCRQ {
			r.logger.Debug("l2tp: TunnelID=0 packet that is not SCCRQ dropped",
				"from", pkt.from.String(), "message-type", uint16(info.MessageType))
			return
		}
		sccrq = &info
	}

	// Hold tunnelsMu across dispatch so every per-tunnel mutation (map
	// insert, FSM state change, peerAddr update) is race-free with
	// test introspection. We release the lock BEFORE sending outbound
	// bytes because listener.Send may block on a full kernel TX queue.
	r.tunnelsMu.Lock()
	tunnel := r.locateTunnelLocked(pkt.from, hdr, sccrq)
	if tunnel == nil {
		r.tunnelsMu.Unlock()
		return
	}
	tunnel.peerAddr = pkt.from
	outbound := tunnel.Process(hdr, payload, r.params.Clock(), r.params.Defaults, sccrq)
	r.tunnelsMu.Unlock()

	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed",
				"to", req.to.String(), "len", len(req.bytes), "error", err.Error())
		}
	}
}

// locateTunnelLocked resolves the target tunnel for an inbound control
// datagram. Returns nil if the packet should be dropped (unknown TID or
// max-tunnels limit reached). The caller MUST hold tunnelsMu; mutations
// to tunnelsByLocalID / tunnelsByPeer happen inline. For TunnelID=0
// the caller MUST pass a pre-validated sccrqInfo (parseSCCRQ has
// already run); no tunnel is created for malformed input.
func (r *L2TPReactor) locateTunnelLocked(from netip.AddrPort, hdr MessageHeader, sccrq *sccrqInfo) *L2TPTunnel {
	if hdr.TunnelID != 0 {
		t, ok := r.tunnelsByLocalID[hdr.TunnelID]
		if !ok {
			r.logger.Debug("l2tp: packet for unknown tunnel dropped",
				"from", from.String(), "tunnel-id", hdr.TunnelID)
			return nil
		}
		return t
	}
	// TunnelID=0: caller has already parsed and validated the SCCRQ.
	// sccrq.AssignedTunnelID is guaranteed non-zero by parseSCCRQ.
	if sccrq == nil {
		panic("BUG: locateTunnelLocked called with TunnelID=0 and nil sccrqInfo")
	}
	key := peerKey{addr: from, tid: sccrq.AssignedTunnelID}
	if existing, ok := r.tunnelsByPeer[key]; ok {
		// Retransmitted SCCRQ. Let the existing tunnel's reliable engine
		// handle dedup + ACK. INFO level so operators can see retransmit
		// pressure in a log -- legitimate first-contact retransmits are
		// rare enough that this does not flood.
		r.logger.Info("l2tp: SCCRQ retransmit matched existing tunnel",
			"from", from.String(), "peer-tid", sccrq.AssignedTunnelID, "local-tid", existing.localTID)
		return existing
	}
	// Max-tunnels enforcement. MaxTunnels == 0 means unbounded by this
	// knob; the map can still grow to 65535 (the uint16 TID ceiling).
	if r.params.MaxTunnels != 0 && uint16(len(r.tunnelsByLocalID)) >= r.params.MaxTunnels {
		r.logger.Warn("l2tp: max-tunnels limit reached; SCCRQ rejected",
			"from", from.String(), "limit", r.params.MaxTunnels)
		// Phase 3 drops; phase 4 will emit StopCCN Result Code 2.
		return nil
	}
	localTID, err := r.allocateLocalTID()
	if err != nil {
		r.logger.Warn("l2tp: local tunnel ID allocation failed; SCCRQ dropped",
			"from", from.String(), "error", err.Error())
		return nil
	}
	t := newTunnel(localTID, sccrq.AssignedTunnelID, from, ReliableConfig{RecvWindow: r.params.Defaults.RecvWindow}, r.logger)
	r.tunnelsByLocalID[localTID] = t
	r.tunnelsByPeer[key] = t
	r.logger.Info("l2tp: new tunnel created from SCCRQ",
		"from", from.String(), "local-tid", localTID, "peer-tid", sccrq.AssignedTunnelID)
	return t
}

// allocateLocalTID picks a non-zero uint16 not already present in
// tunnelsByLocalID. It uses a monotonic counter with wrap-around that
// skips zero; on collision it scans forward up to 8 slots. Returns an
// error only if the 65535 address space is fully occupied (which
// coincides with max-tunnels at its ceiling).
func (r *L2TPReactor) allocateLocalTID() (uint16, error) {
	const maxProbe = 8
	for range maxProbe {
		r.nextLocalTID++
		if r.nextLocalTID == 0 {
			r.nextLocalTID = 1
		}
		if _, taken := r.tunnelsByLocalID[r.nextLocalTID]; !taken {
			return r.nextLocalTID, nil
		}
	}
	return 0, fmt.Errorf("l2tp: no free tunnel IDs after %d probes", maxProbe)
}

// TunnelCount returns the number of tunnels currently tracked by the
// reactor. Acquires tunnelsMu so tests may call it concurrently with
// reactor-goroutine map mutations.
func (r *L2TPReactor) TunnelCount() int {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	return len(r.tunnelsByLocalID)
}

// TunnelByLocalID returns the tunnel with the given local TID, or nil
// if none. Intended for tests; thread-safe.
func (r *L2TPReactor) TunnelByLocalID(tid uint16) *L2TPTunnel {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	return r.tunnelsByLocalID[tid]
}
