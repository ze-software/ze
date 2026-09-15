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
	"errors"
	"net"
	"net/netip"
	"syscall"
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
	// DrainErrors hands every entry queued on the socket's error queue to
	// visit, bounded by probe.ErrQueueDrainMax, and never blocks.
	DrainErrors(visit func(probe.QueuedError)) error
	// Identifier is the echo identifier every probe on this socket carries,
	// and the value a reply or a queued error is matched on. The real
	// pingConn is *probe.Socket, which knows who chose it.
	Identifier() uint16
}

// Per-reply status values beside "ok" and "timeout". A probe the path refused
// for its size is "too-big", and the row carries the reported next-hop MTU
// when a router put one on the wire. A probe this host's own kernel refused
// against its cached path MTU, which honor-cache mode asks for, is
// "too-big-cached": it never reached the wire, and the value on the row is
// the cache's estimate rather than a router's answer on this run.
const (
	statusTooBig       = "too-big"
	statusTooBigCached = "too-big-cached"
)

// The payload keys a refused probe carries are the probe layer's, the same
// two on ping and on traceroute.
const (
	fieldNextHopMTU         = probe.FieldNextHopMTU
	fieldNextHopMTUReported = probe.FieldNextHopMTUReported
)

// readErrorsMax bounds the receiver's tolerance for consecutive read errors.
// Under IP_RECVERR a queued refusal makes one ordinary read fail with the
// refusal's errno, once, and the receiver then asks the main goroutine to
// drain the queue. A descriptor that fails on every read would otherwise
// keep the receiver spinning, so after this many failures in a row with no
// datagram between them it gives up, exactly as it gave up on the first
// error before the error queue existed.
const readErrorsMax = 32

// NewPingSession starts a continuous ping stream to the given target.
// Each reply is sent as a map on the returned channel. The channel is
// closed when the context is canceled. Cancel stops the session.
//
// count bounds the number of probes; 0 streams until the context is canceled,
// which is the `monitor ping <dest>` default. size is the ICMP echo payload in
// bytes; 0 sends the small default payload.
func NewPingSession(ctx context.Context, target string, interval, timeout time.Duration, count, size int) (<-chan map[string]any, context.CancelFunc, error) {
	addr, err := probe.ResolveTarget(target, probe.FamilyAny)
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan map[string]any, 64)
	pingCtx, cancel := context.WithCancel(ctx)

	go streamPing(pingCtx, addr, interval, timeout, count, size, ch)

	return ch, cancel, nil
}

func streamPing(ctx context.Context, dest netip.Addr, interval, timeout time.Duration, count, size int, out chan<- map[string]any) {
	icmpEcho := byte(8)
	icmpEchoReply := byte(0)
	if dest.Is6() {
		icmpEcho = 128
		icmpEchoReply = 129
	}

	// The stream carries no DF keyword yet, so the mode is named here as off
	// rather than left to the zero value the probe layer refuses.
	conn, err := openProbeConn(ctx, probe.FamilyOf(dest), netip.Addr{}, probe.DFOff)
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
	echoID := conn.Identifier()

	type reply struct {
		seq uint16
		at  time.Time
	}
	replies := make(chan reply)
	expire := make(chan uint16)
	// queued carries the receiver's "a refusal is on the error queue" wake to
	// the main goroutine, which is the only drainer. Capacity one and a
	// non-blocking send: a second wake before the first drain adds nothing,
	// because one drain reads everything queued.
	queued := make(chan struct{}, 1)
	recvDone := make(chan struct{})
	done := make(chan struct{})

	// Receiver: a pure reader. It blocks in ReadFrom until a packet arrives or
	// the conn is closed on teardown, applies the same length/type/id/source
	// checks the serial loop used, and forwards the reply's sequence and arrival
	// time. It never touches out or the in-flight map, and it never drains the
	// error queue: a read that fails with a refusal's errno is the wake, and
	// the main goroutine reads the queue, so the sender's own drain after a
	// refused send and this wake can never race for one entry.
	go func() {
		defer close(recvDone)
		rb := make([]byte, max(1500, size+8))
		readErrors := 0
		for {
			n, from, readErr := conn.ReadFrom(rb)
			if readErr != nil {
				if errors.Is(readErr, net.ErrClosed) {
					return
				}
				readErrors++
				if readErrors > readErrorsMax {
					return
				}
				select {
				case queued <- struct{}{}:
				default:
				}
				continue
			}
			readErrors = 0
			if n < 8 || rb[0] != icmpEchoReply {
				continue
			}
			// RFC 792: the Identifier and Sequence Number fields "may be used by
			// the echo sender to aid in matching the replies with the requests."
			// Matching by sequence is exactly the mechanism this fix relies on.
			replyID := binary.BigEndian.Uint16(rb[4:6])
			replySeq := binary.BigEndian.Uint16(rb[6:8])
			if replyID != echoID {
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
	// A send the kernel refused is reported by the caller of send, which is
	// on the select loop and can block on out; send itself never blocks.
	refusedNum := -1

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
		pkt := probe.BuildICMPEcho(icmpEcho, echoID, wire, payload)
		if _, writeErr := conn.WriteTo(pkt, &net.IPAddr{IP: dest.AsSlice()}); writeErr != nil {
			if errors.Is(writeErr, syscall.EMSGSIZE) {
				// The kernel refused the send against its cached path MTU,
				// which is what honor-cache asks for. sendAndReport drains
				// the queue right after this returns and reports the probe
				// with the cache's estimate, and sending goes on, because
				// the operator asked for count probes and each one gets its
				// own row.
				t.Stop()
				delete(inflight, wire)
				refusedNum = num
				nextSeq++
				return
			}
			// A write error ends sending; outstanding probes still drain.
			t.Stop()
			delete(inflight, wire)
			stopSending = true
			return
		}
		nextSeq++
	}

	// emit sends one already-resolved row. Returns false if the context was
	// canceled while emitting.
	emit := func(result map[string]any) bool {
		select {
		case out <- result:
			return true
		case <-ctx.Done():
			return false
		}
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

	// drainQueue reads the error queue once. Every entry a router queued for
	// a probe of this socket resolves that probe as too-big, and the first
	// entry this host's own kernel queued for a refused send is handed back
	// to the caller, which is the sender when a send was refused. Both kinds
	// are read in one drain because a drain that kept one kind would drop
	// the other: under honor-cache a router's answer for probe N and the
	// cache's refusal of probe N+1 sit on the queue together, since that
	// answer is what shrank the cache. An entry that quotes no probe of
	// ours, or is not a size refusal, leaves its probe to time out as it did
	// before the queue was read. ok is false if the context was canceled
	// while emitting.
	drainQueue := func() (local *probe.QueuedError, ok bool) {
		var entries [probe.ErrQueueDrainMax]probe.QueuedError
		count := 0
		drainErr := conn.DrainErrors(func(q probe.QueuedError) {
			if count < len(entries) {
				entries[count] = q
				count++
			}
		})
		if drainErr != nil {
			// The queue could not be read: the probes it holds time out, which
			// is the answer the loop gave before the queue existed.
			return nil, true
		}
		for i := range entries[:count] {
			q := &entries[i]
			if q.Local {
				if local == nil {
					local = q
				}
				continue
			}
			if !q.Echo.Present {
				continue
			}
			if q.Echo.ID != echoID {
				continue
			}
			if q.Errno != syscall.EMSGSIZE {
				continue
			}
			if !resolve(q.Echo.Seq, tooBigResult(statusTooBig, q)) {
				return nil, false
			}
		}
		return local, true
	}

	// sendAndReport is send plus the row for a refused send. The kernel
	// queued the refusal before WriteTo returned, quoting nothing, and this
	// goroutine is the only drainer, so the local entry the drain finds is
	// this probe's. Returns false if the context was canceled while
	// emitting.
	sendAndReport := func() bool {
		send()
		if refusedNum < 0 {
			return true
		}
		local, ok := drainQueue()
		if !ok {
			return false
		}
		result := tooBigResult(statusTooBigCached, local)
		result["seq"] = refusedNum
		refusedNum = -1
		return emit(result)
	}

	// First probe goes out immediately (preserving seq-0-at-once); the ticker
	// paces the rest at exactly interval, independent of reply latency.
	if !sendAndReport() {
		return
	}

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
			if !sendAndReport() {
				return
			}
		case <-queued:
			// A local entry found here was left behind by a drain that hit
			// ErrQueueDrainMax; its probe was already reported without a
			// value, so it is dropped.
			if _, ok := drainQueue(); !ok {
				return
			}
		case r := <-replies:
			p, ok := inflight[r.seq]
			if !ok {
				// Reply for a probe already resolved/expired: drop it.
				continue
			}
			rtt := r.at.Sub(p.sentAt)
			result := map[string]any{
				fieldStatus: "ok",
				"rtt-ms":    float64(rtt.Microseconds()) / 1000.0,
			}
			if !resolve(r.seq, result) {
				return
			}
		case wire := <-expire:
			result := map[string]any{
				fieldStatus: "timeout",
			}
			if !resolve(wire, result) {
				return
			}
		}
	}
}

// tooBigResult is the row for a probe refused for its size. q is the queued
// entry the refusal came from, or nil when none was readable; the row then
// says no value was reported rather than carrying a zero.
func tooBigResult(status string, q *probe.QueuedError) map[string]any {
	result := map[string]any{
		fieldStatus:             status,
		fieldNextHopMTUReported: false,
	}
	if q == nil {
		return result
	}
	if q.Outcome != probe.ErrQueueMTUReported {
		return result
	}
	result[fieldNextHopMTUReported] = true
	result[fieldNextHopMTU] = int(q.MTU)
	return result
}
