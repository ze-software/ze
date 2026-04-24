// Design: rfc/short/rfc5880.md -- packet reception and timer firing
// Overview: engine.go -- Loop struct, lifecycle, and session registry
//
// Express-loop goroutine logic. Drains the transport's RX channel,
// dispatches incoming packets to sessions by Your Discriminator (or by
// session key for first packets), fires per-session timers on a fixed
// poll interval, and writes outgoing packets back to the transport.
package engine

import (
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/transport"
)

// engineLog is the lazy logger for the BFD engine express-loop. Send
// failures and detection-time events log here so operators can correlate
// session flaps with the underlying transport state.
var engineLog = slogutil.LazyLogger("bfd.engine")

// run is the express-loop goroutine. It owns the session map for the
// duration of its execution; mutations from EnsureSession/ReleaseSession
// must be made under l.mu but are observed safely between iterations.
//
// The loop alternates between draining one batch of received packets and
// running one timer-tick pass. The poll interval is PollInterval; ticker
// fires drive both the detection-time check and the periodic-TX check.
func (l *Loop) run() {
	defer close(l.doneCh)
	rx := l.transport.RX()
	var echoRx <-chan transport.Inbound
	if l.echoTransport != nil {
		echoRx = l.echoTransport.RX()
	}
	ticker := l.clk.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case in, ok := <-rx:
			if !ok {
				return
			}
			l.handleInbound(in)
		case in, ok := <-echoRx:
			if !ok {
				// A nil channel blocks forever so this
				// branch only fires when the echo transport
				// has actively closed its RX. Treat it as a
				// clean shutdown of the echo half without
				// disturbing the Control path.
				echoRx = nil
				continue
			}
			l.handleEchoInbound(in)
		case <-ticker.C():
			l.tick()
		}
	}
}

// handleInbound parses one received packet, looks up the session, and
// drives the FSM. Inbound buffers are released back to the transport
// pool before this function returns.
func (l *Loop) handleInbound(in transport.Inbound) {
	defer in.Release()

	c, _, err := packet.ParseControl(in.Bytes)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var entry *sessionEntry
	if c.YourDiscriminator != 0 {
		entry = l.byDiscr[c.YourDiscriminator]
	} else {
		// First packet: deterministic O(1) lookup via the byKey
		// index. RFC 5880 §6.8.6 leaves the demultiplexing tuple to
		// the application; we use (peer, vrf, mode, interface).
		entry = l.byKey[firstPacketKey{
			peer:  in.From,
			local: in.Local,
			vrf:   in.VRF,
			iface: in.Interface,
			mode:  in.Mode,
		}]
	}
	if entry == nil {
		return
	}

	// RFC 5881 Section 5 / RFC 5883 Section 5: discard packets that
	// fail the per-mode TTL gate BEFORE feeding them to the FSM. The
	// check runs after the session lookup because multi-hop MinTTL is
	// per-session.
	if !passesTTLGate(in, entry.machine.MinTTL()) {
		engineLog().Debug("ttl gate drop",
			"mode", in.Mode.String(),
			"ttl", in.TTL,
			"peer", in.From,
			"min-ttl", entry.machine.MinTTL())
		return
	}

	entry.rxPackets++
	if hook := l.metricsHook.Load(); hook != nil {
		(*hook).OnRxPacket(in.Mode.String())
	}

	// RFC 5880 §6.8.6: authenticate before the reception procedure
	// runs. A mismatch or short auth section drops the packet
	// entirely and increments the auth-failures counter. Sessions
	// without an installed signer/verifier skip this branch and
	// fall through to Receive, which enforces the A-bit match.
	if c.Auth && entry.machine.HasAuth() {
		if err := entry.machine.Verify(in.Bytes, c); err != nil {
			if hook := l.metricsHook.Load(); hook != nil {
				(*hook).OnAuthFailure(in.Mode.String())
			}
			engineLog().Debug("auth verify failed",
				"peer", in.From,
				"err", err)
			return
		}
	}

	if err := entry.machine.Receive(c); err != nil {
		return
	}

	// RFC 5880 Section 6.8.6: respond to a Poll with a Final immediately.
	if c.Poll {
		l.sendLocked(entry, entry.machine.BuildFinal())
	}
}

// passesTTLGate enforces the RFC 5881 Section 5 single-hop GTSM rule and
// the RFC 5883 Section 5 multi-hop minimum-TTL rule.
//
// Single-hop BFD requires the received IP TTL / IPv6 Hop Limit to be
// exactly 255. Any other value means the packet traversed at least one
// router and therefore cannot be a legitimate single-hop Control packet.
// A transport that cannot extract the real TTL leaves in.TTL = 0, which
// fails this check -- fail-closed is the intended behavior when the
// kernel does not expose the cmsg.
//
// Multi-hop BFD (RFC 5883) has no GTSM equivalent. Operators express a
// weak approximation via the session's MinTTL (default 254) and packets
// with TTL below that floor are discarded.
func passesTTLGate(in transport.Inbound, minTTL uint8) bool {
	switch in.Mode {
	case api.SingleHop:
		// RFC 5881 Section 5: "The TTL or Hop Limit of the received
		// packet MUST be 255."
		return in.TTL == 255
	case api.MultiHop:
		// RFC 5883 Section 5: MinTTL is inclusive, so TTL == MinTTL
		// passes.
		return in.TTL >= minTTL
	}
	return false
}

// tick runs the timer-driven half of the express loop. For every active
// session it: (1) checks the detection-time deadline; (2) decides whether
// the periodic-TX timer is due and sends a packet if so.
//
// RFC 5880 Section 6.8.7: the next transmission deadline is set with a
// per-packet jitter reduction via applyJitter. The reduction is drawn
// once per TX and applied through Machine.AdvanceTxWithJitter so the
// session's nextTxAt moves forward by TransmitInterval - jitter.
func (l *Loop) tick() {
	now := l.clk.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.sessions {
		entry.machine.CheckDetection(now)

		if entry.machine.State() == packet.StateAdminDown {
			continue
		}
		next := entry.machine.NextTxDeadline()
		if next.IsZero() {
			continue
		}
		if !now.Before(next) {
			l.sendLocked(entry, entry.machine.Build())
			base := entry.machine.TransmitInterval()
			reduction := l.applyJitter(base, entry.machine.DetectMult())
			entry.machine.AdvanceTxWithJitter(now, reduction)
		}
	}

	l.echoTickLocked(now)
}

// sendLocked encodes the Control packet via the packet pool and pushes it
// to the transport. Caller MUST hold l.mu.
//
// The VRF and Interface fields come from the session key so the transport
// layer can bind to the correct interface (SO_BINDTODEVICE on Linux) and
// tag inbound replies with the matching tuple. A peer's reply then matches
// the byKey index on (peer, vrf, mode, interface) exactly.
func (l *Loop) sendLocked(entry *sessionEntry, c packet.Control) {
	pb := packet.Acquire()
	defer packet.Release(pb)
	buf := pb.Data()

	n := c.WriteTo(buf, 0)
	// RFC 5880 §6.7: append the authentication section immediately
	// after the mandatory bytes when the session is authenticated.
	// Machine.Sign writes Type/Len/KeyID/Seq/Digest using the
	// current bfd.XmitAuthSeq; AdvanceAuthSeq bumps and persists
	// the counter for the next TX.
	if entry.machine.HasAuth() && c.Auth {
		n += entry.machine.Sign(buf, n)
		entry.machine.AdvanceAuthSeq()
	}

	key := entry.machine.Key()
	out := transport.Outbound{
		To:        entry.machine.PeerAddr(),
		VRF:       key.VRF,
		Interface: key.Interface,
		Mode:      key.Mode,
		Bytes:     buf[:n],
	}
	if err := l.transport.Send(out); err != nil {
		engineLog().Debug("transport send failed", "peer", out.To, "err", err)
		return
	}
	entry.txPackets++
	if hook := l.metricsHook.Load(); hook != nil {
		(*hook).OnTxPacket(key.Mode.String())
	}
}

// handle is the engine's implementation of api.SessionHandle.
type handle struct {
	loop *Loop
	key  api.Key
}

// Key returns the session identity.
func (h *handle) Key() api.Key { return h.key }

// Subscribe registers a buffered channel for state-change notifications.
func (h *handle) Subscribe() <-chan api.StateChange {
	ch := make(chan api.StateChange, SubscribeBuffer)
	h.loop.subsMu.Lock()
	h.loop.subscribers[h.key] = append(h.loop.subscribers[h.key], ch)
	h.loop.subsMu.Unlock()
	return ch
}

// Unsubscribe removes ch from the subscriber list so the express loop
// no longer delivers state changes to it. The channel is NOT closed
// here: the express loop may still hold a snapshot of the list (taken
// under subsMu in makeNotify) and attempt a send after Unsubscribe
// returns. Closing here would race with that send. Loop.Stop is
// responsible for closing every subscriber channel after the express
// loop has exited.
func (h *handle) Unsubscribe(ch <-chan api.StateChange) {
	h.loop.subsMu.Lock()
	defer h.loop.subsMu.Unlock()
	subs := h.loop.subscribers[h.key]
	for i, c := range subs {
		if c == ch {
			h.loop.subscribers[h.key] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// Shutdown forces the session into AdminDown via session.Machine.AdminDown
// (RFC 5880 §6.8.16). Returns an error only when the session has been torn
// down between handle creation and the call. Safe for concurrent use.
func (h *handle) Shutdown() error {
	h.loop.mu.Lock()
	defer h.loop.mu.Unlock()
	entry, ok := h.loop.sessions[h.key]
	if !ok {
		return ErrUnknownSession
	}
	entry.machine.AdminDown(packet.DiagAdminDown)
	return nil
}

// Enable transitions the session out of AdminDown back to Down so the
// handshake can resume. No-op if the session is not currently AdminDown.
func (h *handle) Enable() error {
	h.loop.mu.Lock()
	defer h.loop.mu.Unlock()
	entry, ok := h.loop.sessions[h.key]
	if !ok {
		return ErrUnknownSession
	}
	entry.machine.AdminEnable()
	return nil
}
