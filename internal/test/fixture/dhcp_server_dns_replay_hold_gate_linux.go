//go:build linux

// Design: docs/architecture/provisioning/dhcp-server.md -- Functional tests: the probe and its WAN lease
// Related: dhcp_server_dns_replay_hold_linux.go -- dhcpServerDNSReplayHold, the scenario that closes and opens this gate

package fixture

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The gate gives the window the DHCP server's replay hold exists for a fixed
// length. Nothing else does: the server binds its listener when it is
// configured (Stage 2) and sends its snapshot-request once started, and the
// engine answers that request within milliseconds, so without the gate
// whether a DHCPDISCOVER lands between the two is a race, and a break that
// removes the hold could pass.
//
// It works through the engine's event delivery, with no product hook. The
// engine queues a fixed number of deliveries per plugin process, and an
// emitter whose subscriber's queue is full blocks in process.Process.Deliver
// until that queue drains. An emitted event reaches every subscribed plugin
// process before the engine's own handlers run (Server.deliverEvent defers
// dispatchEngineEvent), and the registry answers a snapshot-request from such
// a handler (Resolvers.PublishOn). So the fixture subscribes to the snapshot
// and to the snapshot-request, holds its event handler on the first delivery,
// and emits snapshot-requests until its queue is full, because each answer is
// a snapshot delivered to it. The DHCP server's own snapshot-request then
// blocks in its delivery to the fixture, before the registry sees it, and no
// snapshot reaches the server until the gate opens while its listener
// answers.
//
// A second fixture plugin that stalls its startup cannot do this. A reload
// starts only the plugins a config root selects (selectAutoLoad), never a new
// `plugin external` block, and an internal plugin's engine calls take its
// DirectBridge once its Stage 5 is answered (wireBridgeDispatch), so they wait
// for no other plugin's startup.
const (
	// replayHoldGateHeld is how long the gate stays closed once the server's
	// listener is bound: a hundred DHCPDISCOVERs at replayHoldInterval, and
	// well under dnssource.ReplayWaitMax, past which the fixed server answers
	// from its empty list.
	replayHoldGateHeld = 500 * time.Millisecond
	// replayHoldGateBudget bounds the whole stall, from the delivery the
	// handler holds to the gate opening. The engine waits 5 s for a delivery
	// answer (process.defaultDeliveryTimeout) and then moves on to the queued
	// ones, which would open the gate in silence, so the scenario fails first.
	replayHoldGateBudget = 4 * time.Second
	// replayHoldGateQueue is the engine's per-process delivery queue depth
	// (process.eventDeliveryCapacity). The queue is full once more answers
	// than that were delivered past the batch the handler holds.
	replayHoldGateQueue = 64
	// replayHoldGateEmitsMax bounds the filler: an engine that answered this
	// many snapshot-requests without blocking does not gate as described.
	replayHoldGateEmitsMax = 4 * replayHoldGateQueue
	// replayHoldGateQuiet is how long the filler's emit must stay unanswered,
	// with more than the queue's depth answered, to read as blocked.
	replayHoldGateQuiet = 100 * time.Millisecond
	// replayHoldGatePoll is the cadence of the fullness and listener checks.
	replayHoldGatePoll = 2 * time.Millisecond
	// replayHoldServerPort is the DHCP server's UDP port (RFC 2131 Section 4.1).
	replayHoldServerPort = 67
)

// replayHoldSpan is what the gate's watcher measured: when the server's
// listener was first seen bound and when the gate opened.
type replayHoldSpan struct {
	boundAt  time.Time
	openedAt time.Time
	err      error
}

// replayHoldGate holds the DHCP server's snapshot-request unanswered. Close
// it with closeReplayHoldGate before the reload, start its watcher with
// watchListener once the reload is signalled, and call finish once, which
// opens it and waits for both workers. open is safe for concurrent use and
// idempotent; the other methods are for the scenario's goroutine.
type replayHoldGate struct {
	opened   chan struct{}
	openOnce sync.Once
	openedAt time.Time // written inside openOnce, before opened closes

	held     chan struct{}
	heldOnce sync.Once
	heldAt   time.Time // written inside heldOnce, before held closes

	answered   atomic.Int64
	fillerDone chan struct{}
	fillErr    error // written by fill before fillerDone closes

	watcherDone    chan struct{}
	watcherStarted bool
	span           replayHoldSpan // written by watch before watcherDone closes
}

// closeReplayHoldGate subscribes p to the system-dns snapshot and
// snapshot-request, starts the filler and returns once the fixture's delivery
// queue is full. No DHCP listener may be bound yet, or the window it measures
// would not be the reload's.
func closeReplayHoldGate(ctx context.Context, p *sdk.Plugin) (*replayHoldGate, error) {
	bound, err := udp4PortBound(replayHoldServerPort)
	if err != nil {
		return nil, err
	}
	if bound {
		return nil, errors.New("BUG: dhcp-server-dns-replay-hold: a UDP socket is bound to port 67 before the reload adds the DHCP server")
	}

	g := &replayHoldGate{
		opened:      make(chan struct{}),
		held:        make(chan struct{}),
		fillerDone:  make(chan struct{}),
		watcherDone: make(chan struct{}),
	}
	p.OnEvent(g.hold)
	for _, event := range []string{dnssource.SnapshotEventType, dnssource.SnapshotRequestEventType} {
		var raw json.RawMessage
		if _, err := Dispatch(ctx, p, "request subscribe "+dnssource.Namespace+" event "+event, &raw); err != nil {
			g.open()
			return nil, fmt.Errorf("subscribe to %s %s: %w", dnssource.Namespace, event, err)
		}
	}

	go g.fill(ctx, p)
	if err := g.awaitFull(ctx); err != nil {
		_, finishErr := g.finish()
		return nil, errors.Join(err, finishErr)
	}
	return g, nil
}

// hold is the fixture's event handler. The SDK answers a delivery batch only
// once the handler returns, so holding the first delivery keeps every later
// one in the engine's queue for this process.
func (g *replayHoldGate) hold(string) error {
	g.heldOnce.Do(func() {
		g.heldAt = time.Now()
		close(g.held)
	})
	<-g.opened
	return nil
}

// fill is the gate's one filler worker. It emits snapshot-requests until the
// gate opens or replayHoldGateEmitsMax were answered; once the fixture's
// queue is full its emit blocks in the engine until the gate opens. It
// records its result in fillErr and closes fillerDone.
func (g *replayHoldGate) fill(ctx context.Context, p *sdk.Plugin) {
	defer close(g.fillerDone)
	for range replayHoldGateEmitsMax {
		select {
		case <-g.opened:
			return
		default:
		}
		if _, err := p.EmitEvent(ctx, dnssource.Namespace, dnssource.SnapshotRequestEventType, "", "", ""); err != nil {
			g.fillErr = fmt.Errorf("emit %s %s: %w", dnssource.Namespace, dnssource.SnapshotRequestEventType, err)
			return
		}
		g.answered.Add(1)
	}
	g.fillErr = errors.New("the engine answered " + strconv.Itoa(replayHoldGateEmitsMax) +
		" snapshot-requests without blocking on the fixture's full delivery queue, so the gate cannot hold the DHCP server's request")
}

// awaitFull waits until the handler holds a delivery and the filler's emit
// has stayed unanswered for replayHoldGateQuiet with more than
// replayHoldGateQueue answered: the queue is then full.
func (g *replayHoldGate) awaitFull(ctx context.Context) error {
	start := time.Now()
	ticker := time.NewTicker(replayHoldGatePoll)
	defer ticker.Stop()

	last, lastAt := int64(-1), start
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-g.fillerDone:
			if g.fillErr != nil {
				return g.fillErr
			}
			return errors.New("BUG: dhcp-server-dns-replay-hold: the gate's filler stopped while the gate was closed")
		case now := <-ticker.C:
			if now.Sub(start) > replayHoldGateBudget {
				var tb textbuf.Buffer
				tb.Str("the fixture's delivery queue never filled: ").Int(g.answered.Load()).
					Str(" snapshot-requests answered in ").Str(replayHoldGateBudget.String()).
					Str(", want more than ").Int(replayHoldGateQueue).Str(" and then one left unanswered")
				return errors.New(tb.String())
			}
			answered := g.answered.Load()
			if answered != last {
				last, lastAt = answered, now
				continue
			}
			if answered <= replayHoldGateQueue {
				continue
			}
			if now.Sub(lastAt) < replayHoldGateQuiet {
				continue
			}
			if g.isHeld() {
				return nil
			}
		}
	}
}

// isHeld reports whether the handler holds a delivery.
func (g *replayHoldGate) isHeld() bool {
	select {
	case <-g.held:
		return true
	default:
		return false
	}
}

// watchListener starts the gate's watcher. It MUST be called once, after
// closeReplayHoldGate returned the gate and the reload is signalled.
func (g *replayHoldGate) watchListener(ctx context.Context) {
	g.watcherStarted = true
	go g.watch(ctx)
}

// watch is the gate's one watcher worker. It opens the gate
// replayHoldGateHeld after the DHCP server's listener is first seen bound, or
// with an error once the stall budget would not leave that long, records the
// span and closes watcherDone.
func (g *replayHoldGate) watch(ctx context.Context) {
	defer close(g.watcherDone)
	defer g.open()

	bindBy := g.heldAt.Add(replayHoldGateBudget - replayHoldGateHeld)
	ticker := time.NewTicker(replayHoldGatePoll)
	defer ticker.Stop()

	for g.span.boundAt.IsZero() {
		select {
		case <-ctx.Done():
			g.span.err = ctx.Err()
			return
		case <-g.opened:
			g.span.err = errors.New("the gate opened before its watcher saw the DHCP server's listener bound")
			return
		case now := <-ticker.C:
			if now.After(bindBy) {
				g.span.err = errors.New("the DHCP server's listener was not bound on port 67 within " +
					(replayHoldGateBudget - replayHoldGateHeld).String() +
					" of the gate closing, so the gate would outlast the engine's delivery timeout")
				return
			}
			bound, err := udp4PortBound(replayHoldServerPort)
			if err != nil {
				g.span.err = err
				return
			}
			if bound {
				g.span.boundAt = now
			}
		}
	}

	timer := time.NewTimer(replayHoldGateHeld)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		g.span.err = ctx.Err()
	case <-g.opened:
		g.span.err = errors.New("the gate opened before it held the DHCP server's request for " + replayHoldGateHeld.String())
	}
}

// open releases the handler, and with it the deliveries the engine queued.
// Safe for concurrent use; only the first call records openedAt.
func (g *replayHoldGate) open() {
	g.openOnce.Do(func() {
		g.openedAt = time.Now()
		close(g.opened)
	})
}

// finish opens the gate and waits for its workers. It MUST be called once,
// after closeReplayHoldGate returned the gate, and answers the watcher's span
// with the first worker error.
func (g *replayHoldGate) finish() (replayHoldSpan, error) {
	g.open()
	<-g.fillerDone
	if g.watcherStarted {
		<-g.watcherDone
	}
	span := g.span
	span.openedAt = g.openedAt
	return span, errors.Join(g.fillErr, span.err)
}

// udp4PortBound reports whether an IPv4 UDP socket in this network namespace
// is bound to port, from /proc/net/udp, whose second field on each socket's
// line is the local address as hex address:port.
func udp4PortBound(port uint16) (bool, error) {
	f, err := os.Open("/proc/net/udp")
	if err != nil {
		return false, err
	}
	defer f.Close() //nolint:errcheck // read-only; a close error changes no answer

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return false, fmt.Errorf("/proc/net/udp: %w", err)
		}
		return false, errors.New("/proc/net/udp has no header line")
	}
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			return false, errors.New("/proc/net/udp: a socket line with no local address: " + sc.Text())
		}
		_, portHex, found := strings.Cut(fields[1], ":")
		if !found {
			return false, errors.New("/proc/net/udp: a local address with no port: " + fields[1])
		}
		local, err := strconv.ParseUint(portHex, 16, 16)
		if err != nil {
			return false, fmt.Errorf("/proc/net/udp: local port %q: %w", portHex, err)
		}
		if uint16(local) == port {
			return true, nil
		}
	}
	return false, sc.Err()
}
