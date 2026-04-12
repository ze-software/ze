// Design: rfc/short/rfc5880.md -- Echo Function (Section 6.4)
// Related: loop.go -- express-loop goroutine that drains the echo RX channel
// Related: ../packet/echo.go -- ZEEC envelope codec
// Related: ../session/timers.go -- EchoInterval / NextEchoTxDeadline / AdvanceEcho
//
// Echo-mode half of the express-loop. RFC 5880 Section 6.4 leaves the
// packet format "a local matter"; the sender transmits periodically,
// the receiver reflects the bytes back to the source, and the sender
// measures round-trip time from the returning copies. This file owns
// the per-session echo scheduler and the RX path that demultiplexes
// between "our echo returning" (match and record RTT) and "peer's
// echo to reflect" (send the bytes straight back unchanged).
//
// Scheduling runs on the engine tick via echoTickLocked. Only Up
// sessions that negotiated echo (both DesiredMinEchoTx and the
// peer's advertised RequiredMinEchoRx non-zero) are considered; the
// RX path drops packets whose ZEEC magic does not match, drops
// packets from unknown sources (amplification guard), and matches
// returning echoes by LocalDiscriminator against byDiscr.
package engine

import (
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/transport"
)

// echoTickLocked fires per-session echo TX deadlines and drives
// the echo-mode detection timer. Caller MUST hold l.mu. For every
// session with echo negotiated and its deadline passed the loop
// encodes one ZEEC envelope, hands it to the echo transport, and
// advances the per-session schedule. Before the TX pass the
// session's outstanding ring is checked for stale entries; any
// outstanding echo older than DetectMult * EchoInterval fires a
// transition to Down with DiagEchoFailed.
//
// Sessions that are not Up have their echo schedule and
// outstanding ring cleared so a session that flapped down does
// not carry stale detection state back into Up.
func (l *Loop) echoTickLocked(now time.Time) {
	if l.echoTransport == nil {
		return
	}
	for _, entry := range l.sessions {
		m := entry.machine
		if m.State() != packet.StateUp || !m.EchoEnabled() {
			m.ClearEchoSchedule()
			continue
		}
		if m.EchoDetectionExpired(now) {
			engineLog().Info("bfd echo detection expired",
				"peer", m.PeerAddr().String(),
				"detect", m.EchoDetectInterval())
			m.EchoFail()
			if hook := l.metricsHook.Load(); hook != nil {
				(*hook).OnStateChange(packet.StateUp, packet.StateDown,
					packet.DiagEchoFailed, m.Key().Mode.String(), m.Key().VRF)
			}
			continue
		}
		m.PrimeEcho(now)
		deadline := m.NextEchoTxDeadline()
		if deadline.IsZero() || now.Before(deadline) {
			continue
		}
		l.sendEchoLocked(entry, now)
		m.AdvanceEcho(now)
	}
}

// sendEchoLocked encodes a single ZEEC envelope for the session and
// hands it to the echo transport. Caller MUST hold l.mu.
//
// The sequence counter lets the RX path match returning echoes
// against the per-session outstanding ring (RegisterEchoTx /
// MatchEchoRx). The envelope's TimestampMs field is still written
// with a truncated millisecond slice of the engine clock so the
// RTT can fall back to the self-carried value when the matching
// entry has already been evicted from a small ring under load.
func (l *Loop) sendEchoLocked(entry *sessionEntry, now time.Time) {
	buf := [packet.EchoLen]byte{}
	ts := uint32(now.UnixMilli())
	seq := entry.machine.NextEchoSequence()
	e := packet.Echo{
		LocalDiscriminator: entry.machine.LocalDiscriminator(),
		Sequence:           seq,
		TimestampMs:        ts,
	}
	packet.WriteEcho(buf[:], 0, e)

	key := entry.machine.Key()
	out := transport.Outbound{
		To:        entry.machine.PeerAddr(),
		VRF:       key.VRF,
		Interface: key.Interface,
		Mode:      key.Mode,
		Bytes:     buf[:],
	}
	if err := l.echoTransport.Send(out); err != nil {
		engineLog().Debug("bfd echo send failed", "peer", out.To, "err", err)
		return
	}
	entry.machine.RegisterEchoTx(seq, now)
	if hook := l.metricsHook.Load(); hook != nil {
		(*hook).OnEchoTx(key.Mode.String())
	}
}

// handleEchoInbound processes a single packet off the echo RX channel.
// The demux rule is:
//
//  1. ZEEC magic mismatch or short buffer -> drop.
//  2. LocalDiscriminator matches a live local session whose peer
//     address equals the source -> our echo returning; compute RTT
//     from the self-carried TimestampMs and record it.
//  3. Source address matches some live session's peer address ->
//     peer's echo; reflect the exact bytes back to the source.
//  4. Neither -> drop (amplification guard).
//
// The engine holds l.mu for the whole call so state updates on the
// matched session happen under the same lock as Control-path reception.
func (l *Loop) handleEchoInbound(in transport.Inbound) {
	defer in.Release()

	e, err := packet.ParseEcho(in.Bytes)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if entry := l.byDiscr[e.LocalDiscriminator]; entry != nil {
		if entry.machine.PeerAddr() == in.From {
			l.recordEchoRTTLocked(entry, e, l.clk.Now(), in.Mode.String())
			return
		}
	}

	if entry := l.findSessionByPeerLocked(in.From); entry != nil {
		l.reflectEchoLocked(in, entry)
		return
	}

	engineLog().Debug("bfd echo drop unknown source",
		"from", in.From.String(),
		"discr", e.LocalDiscriminator)
}

// findSessionByPeerLocked walks the session table for a peer match.
// Returns nil when no session is pinned to the source address. The
// walk is O(N) in the live session count; at the expected scale
// (tens to low hundreds of sessions per loop) the scan is cheaper
// than maintaining a second index and keeping it consistent under
// reload.
//
// Caller MUST hold l.mu.
func (l *Loop) findSessionByPeerLocked(peer netip.Addr) *sessionEntry {
	for _, entry := range l.sessions {
		if entry.machine.PeerAddr() == peer {
			return entry
		}
	}
	return nil
}

// recordEchoRTTLocked stores the round-trip time on the session and
// fires the metrics hook. Caller MUST hold l.mu.
//
// The ring lookup is the authoritative RTT source: Machine tracks
// the monotonic sentAt for every outstanding echo, so the delta is
// immune to wall-clock jumps and system-suspend artifacts. When the
// ring has already evicted the matching entry (burst loss, ring
// overflow) the calculation falls back to the self-carried
// TimestampMs in the ZEEC envelope, which is truncated from the
// 64-bit UnixMilli and treated as a signed 32-bit delta so a clock
// crossing the 2^32 ms boundary (~49 days) stays correct.
func (l *Loop) recordEchoRTTLocked(entry *sessionEntry, e packet.Echo, now time.Time, mode string) {
	rtt, ok := entry.machine.MatchEchoRx(e.Sequence, now)
	if !ok {
		nowMs := uint32(now.UnixMilli())
		delta := int32(nowMs - e.TimestampMs)
		delta = max(delta, 0)
		rtt = time.Duration(delta) * time.Millisecond
	}
	entry.machine.RecordEchoRTT(rtt)
	if hook := l.metricsHook.Load(); hook != nil {
		(*hook).OnEchoRx(mode)
		(*hook).OnEchoRTT(mode, rtt)
	}
}

// reflectEchoLocked sends the exact received bytes back to the
// source address via the echo transport. Used on the remote-receiver
// side of an echo round trip. The original buffer is freed by the
// deferred Release in handleEchoInbound, so the reflect copies into
// a short-lived stack buffer before the send.
//
// Caller MUST hold l.mu.
func (l *Loop) reflectEchoLocked(in transport.Inbound, entry *sessionEntry) {
	var buf [packet.EchoLen]byte
	copy(buf[:], in.Bytes)
	key := entry.machine.Key()
	out := transport.Outbound{
		To:        in.From,
		VRF:       key.VRF,
		Interface: key.Interface,
		Mode:      api.SingleHop,
		Bytes:     buf[:],
	}
	if err := l.echoTransport.Send(out); err != nil {
		engineLog().Debug("bfd echo reflect failed",
			"peer", out.To.String(),
			"err", err)
		return
	}
	if hook := l.metricsHook.Load(); hook != nil {
		(*hook).OnEchoRx(api.SingleHop.String())
	}
}
