// Design: docs/architecture/api/commands.md -- streaming ping session
// Overview: register.go -- RPC + offline-root registration for this module
//
// stream.go provides the continuous ping session used by `monitor ping`. The
// CLI/hub streaming factories call NewPingSession; ICMP echo packets and target
// resolution come from internal/core/probe.
//
// The session is a sender/receiver split (see runPingSession): probe sends are
// timer-driven at `interval`, and a reply is matched to the probe it answers by
// sequence number, so cadence holds under loss and a late reply is attributed
// rather than dropped. The old serial "send, block until this reply, repeat"
// loop coupled the send rate to the reply latency, degrading cadence to
// ~max(interval, timeout) exactly on the lossy paths the tool exists to observe.

package cmd

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/probe"
)

// pingConn is the subset of net.PacketConn that the streaming session needs.
// It exists so tests can drive loss, delay, and reordering through a fake
// connection: the real path opens a raw ICMP socket (CAP_NET_RAW), which no
// unit test can reach, which is why this bug survived unobserved.
type pingConn interface {
	WriteTo(p []byte, addr net.Addr) (int, error)
	ReadFrom(p []byte) (int, net.Addr, error)
	Close() error
}

// NewPingSession starts a continuous ping stream to the given target.
// Each reply is sent as a map on the returned channel. The channel is
// closed when the context is canceled. Cancel stops the session.
//
// count bounds the number of probes; 0 streams until the context is canceled,
// which is the `monitor ping <dest>` default. size is the ICMP echo payload in
// bytes; 0 sends the small default payload.
func NewPingSession(ctx context.Context, target string, interval, timeout time.Duration, count, size int) (<-chan map[string]any, context.CancelFunc, error) {
	addr, err := probe.ResolveTarget(target)
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan map[string]any, 64)
	pingCtx, cancel := context.WithCancel(ctx)

	go streamPing(pingCtx, addr, interval, timeout, count, size, ch)

	return ch, cancel, nil
}

func streamPing(ctx context.Context, dest netip.Addr, interval, timeout time.Duration, count, size int, out chan<- map[string]any) {
	network := probe.NetworkICMPv4
	icmpEcho := byte(8)
	icmpEchoReply := byte(0)
	if dest.Is6() {
		network = probe.NetworkICMPv6
		icmpEcho = 128
		icmpEchoReply = 129
	}

	var lc net.ListenConfig
	conn, err := lc.ListenPacket(ctx, network, "")
	if err != nil {
		// The socket never opened: the session ends immediately. Consumers
		// detect the end by the channel closing, so close it exactly once here.
		close(out)
		return
	}

	// runPingSession takes ownership of conn and closes out exactly once.
	runPingSession(ctx, conn, clock.RealClock{}, dest, interval, timeout, count, size, icmpEcho, icmpEchoReply, out)
}

// inflightProbe is a probe that has been sent and is awaiting a reply or its
// timeout. num is the operator-visible sequence number (unbounded, as emitted
// on the channel); sentAt is used to compute RTT; timer fires the timeout.
type inflightProbe struct {
	num    int
	sentAt time.Time
	timer  clock.Timer
}

// runPingSession drives the ping loop over an already-open conn and an injected
// clock. It is the seam the unit tests exercise with a fake conn + fake clock.
//
// Concurrency model: exactly two goroutines share conn. The receiver goroutine
// only ReadFroms and forwards parsed replies; it never touches out or the
// in-flight map. The main goroutine owns the in-flight map (single writer, no
// mutex needed), sends probes on a ticker, matches replies by sequence, and
// closes out exactly once on teardown. The per-probe reaper is a clock.AfterFunc
// that only sends the wire sequence on the expire channel, so it too never
// touches the map.
func runPingSession(
	ctx context.Context,
	conn pingConn,
	clk clock.Clock,
	dest netip.Addr,
	interval, timeout time.Duration,
	count, size int,
	icmpEcho, icmpEchoReply byte,
	out chan<- map[string]any,
) {
	pid := uint16(os.Getpid() & 0xffff)

	type reply struct {
		seq uint16
		at  time.Time
	}
	replies := make(chan reply)
	expire := make(chan uint16)
	recvDone := make(chan struct{})
	done := make(chan struct{})

	// Receiver: a pure reader. It blocks in ReadFrom until a packet arrives or
	// the conn is closed on teardown, applies the same length/type/id/source
	// checks the serial loop used, and forwards the reply's sequence and arrival
	// time. It never touches out or the in-flight map.
	go func() {
		defer close(recvDone)
		rb := make([]byte, max(1500, size+8))
		for {
			n, from, readErr := conn.ReadFrom(rb)
			if readErr != nil {
				return
			}
			if n < 8 || rb[0] != icmpEchoReply {
				continue
			}
			// RFC 792: the Identifier and Sequence Number fields "may be used by
			// the echo sender to aid in matching the replies with the requests."
			// Matching by sequence is exactly the mechanism this fix relies on.
			replyID := binary.BigEndian.Uint16(rb[4:6])
			replySeq := binary.BigEndian.Uint16(rb[6:8])
			if replyID != pid {
				continue
			}
			if from != nil {
				if ipAddr, ok := from.(*net.IPAddr); ok {
					fromAddr, _ := netip.AddrFromSlice(ipAddr.IP)
					if fromAddr.IsValid() && fromAddr != dest {
						continue
					}
				}
			}
			select {
			case replies <- reply{seq: replySeq, at: clk.Now()}:
			case <-done:
				return
			}
		}
	}()

	inflight := make(map[uint16]*inflightProbe)

	// Teardown (single owner of out): stop outstanding timers, unblock the
	// receiver (Close makes ReadFrom return; done unblocks a pending reply
	// send), join it so no goroutine leaks, then close out exactly once.
	defer func() {
		for _, p := range inflight {
			p.timer.Stop()
		}
		if closeErr := conn.Close(); closeErr != nil {
			// Teardown close: nothing actionable, but the call is load-bearing
			// because closing is what unblocks the receiver's ReadFrom.
			_ = closeErr
		}
		close(done)
		<-recvDone
		close(out)
	}()

	payload := pingPayload(size)
	nextSeq := 0
	stopSending := false

	// send emits the next probe (if any remain) and registers it in-flight with
	// a timeout reaper.
	send := func() {
		if stopSending || (count > 0 && nextSeq >= count) {
			return
		}
		// Do not put a probe on the wire once the caller has canceled: the old
		// serial loop checked ctx before its first write, and the first send
		// here runs before the select loop can observe cancellation.
		if ctx.Err() != nil {
			return
		}
		wire := uint16(nextSeq & 0xffff)
		num := nextSeq
		// Arm the timeout reaper and record the probe in-flight BEFORE the write,
		// so sentAt is captured at send time and the reaper is armed the instant
		// the probe exists. A write error then unwinds this bookkeeping.
		sentAt := clk.Now()
		t := clk.AfterFunc(timeout, func() {
			select {
			case expire <- wire:
			case <-done:
			}
		})
		inflight[wire] = &inflightProbe{num: num, sentAt: sentAt, timer: t}
		pkt := probe.BuildICMPEcho(icmpEcho, pid, wire, payload)
		if _, writeErr := conn.WriteTo(pkt, &net.IPAddr{IP: dest.AsSlice()}); writeErr != nil {
			// A write error ends sending; outstanding probes still drain.
			t.Stop()
			delete(inflight, wire)
			stopSending = true
			return
		}
		nextSeq++
	}

	// resolve reports a probe (ok or timeout) and removes it from flight. A
	// wire sequence not in flight was already resolved or expired: ignore it,
	// so a duplicate or forged-late reply cannot resurrect it. Returns false if
	// the context was canceled while emitting.
	resolve := func(wire uint16, result map[string]any) bool {
		p, ok := inflight[wire]
		if !ok {
			return true
		}
		p.timer.Stop()
		delete(inflight, wire)
		result["seq"] = p.num
		select {
		case out <- result:
			return true
		case <-ctx.Done():
			return false
		}
	}

	// First probe goes out immediately (preserving seq-0-at-once); the ticker
	// paces the rest at exactly interval, independent of reply latency.
	send()

	ticker := clk.NewTicker(interval)
	defer ticker.Stop()

	for {
		// The session ends when every probe has been sent AND every one has
		// resolved (reply or timeout). This replaces the old loop-condition and
		// last-probe early return, with no trailing idle after the final reply.
		allSent := stopSending || (count > 0 && nextSeq >= count)
		if allSent && len(inflight) == 0 {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			send()
		case r := <-replies:
			p, ok := inflight[r.seq]
			if !ok {
				// Reply for a probe already resolved/expired: drop it.
				continue
			}
			rtt := r.at.Sub(p.sentAt)
			result := map[string]any{
				"status": "ok",
				"rtt-ms": float64(rtt.Microseconds()) / 1000.0,
			}
			if !resolve(r.seq, result) {
				return
			}
		case wire := <-expire:
			result := map[string]any{
				"status": "timeout",
			}
			if !resolve(wire, result) {
				return
			}
		}
	}
}
