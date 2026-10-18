// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_v4_loop_linux_test.go -- the lease loop tests this rig drives

//go:build linux

package ifacedhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/test/sim"
)

// loopBackendName is the test-only iface backend the loop rig loads.
const loopBackendName = "dhcp-v4-loop-test"

// loopKernelDevice is the kernel device every rig's logical name resolves to:
// it differs from the logical name, as on an os-name or mac/match entry.
const loopKernelDevice = "eth7"

var (
	loopServerA = net.IP{192, 0, 2, 1}
	loopServerB = net.IP{192, 0, 2, 2}
	loopLeased  = net.IP{192, 0, 2, 50}
	loopRouter1 = net.IP{192, 0, 2, 254}
	loopRouter2 = net.IP{192, 0, 2, 253}
)

// loopBackend is an iface.Backend recording the address and route effects the
// worker asks for. The embedded Backend is nil: a method the worker should not
// reach panics.
type loopBackend struct {
	iface.Backend
	mu      sync.Mutex
	logical string
	now     func() time.Duration // the rig's fake-clock offset
	ops     []loopOp
	// routeFailures fails that many AddRoute calls before one succeeds.
	routeFailures int
	replaceErr    error
}

// loopOp is one backend effect and the fake-clock instant it happened at.
type loopOp struct {
	at time.Duration
	op string
}

var (
	loopBackendCurrent atomic.Pointer[loopBackend]
	loopBackendOnce    sync.Once
	loopBackendErr     error
	loopNameSeq        atomic.Int32
)

func (b *loopBackend) Close() error { return nil }

func (b *loopBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	if name != b.logical {
		return nil, fmt.Errorf("loop backend: no interface %q", name)
	}
	return &iface.InterfaceInfo{Name: name, OsName: loopKernelDevice, Index: 7}, nil
}

func (b *loopBackend) record(op string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ops = append(b.ops, loopOp{at: b.now(), op: op})
}

func (b *loopBackend) ReplaceAddressWithLifetime(dev, cidr string, valid, preferred int) error {
	b.mu.Lock()
	err := b.replaceErr
	b.mu.Unlock()
	if err != nil {
		return err
	}
	b.record(fmt.Sprintf("replace %s %s %d %d", dev, cidr, valid, preferred))
	return nil
}

func (b *loopBackend) RemoveAddress(dev, cidr string) error {
	b.record("remove-addr " + dev + " " + cidr)
	return nil
}

func (b *loopBackend) AddRoute(dev, dest, gw string, _ int, _ rtproto.Proto) error {
	b.mu.Lock()
	fail := b.routeFailures > 0
	if fail {
		b.routeFailures--
	}
	b.mu.Unlock()
	if fail {
		b.record("add-route-failed " + dev + " " + dest + " " + gw)
		return fmt.Errorf("loop backend: route via %s refused", gw)
	}
	b.record("add-route " + dev + " " + dest + " " + gw)
	return nil
}

func (b *loopBackend) RemoveRoute(dev, dest, gw string, _ int, _ rtproto.Proto) error {
	b.record("remove-route " + dev + " " + dest + " " + gw)
	return nil
}

// exchangeCall is one call the worker made on the exchanger.
type exchangeCall struct {
	at      time.Duration // fake-clock offset from the rig start
	acquire bool
	device  string
	attempt v4Attempt
}

// scriptedExchanger answers each call from the test's script functions.
// acquireAnswer and extendAnswer get the 0-based index of the call of their
// kind. An unset function answers a failed exchange.
type scriptedExchanger struct {
	rig           *v4Rig
	mu            sync.Mutex
	calls         []exchangeCall
	acquires      int
	extends       int
	acquireAnswer func(n int) v4Reply
	extendAnswer  func(n int, attempt v4Attempt) v4Reply
	// acquireDelay advances the clock between the REQUEST send and the ACK.
	acquireDelay time.Duration
}

func (x *scriptedExchanger) acquire(_ context.Context, device string, _ []dhcpv4.Modifier) v4Reply {
	sent := x.rig.clk.Now()
	x.mu.Lock()
	x.calls = append(x.calls, exchangeCall{at: x.rig.offset(sent), acquire: true, device: device})
	n := x.acquires
	x.acquires++
	answer := x.acquireAnswer
	x.mu.Unlock()
	if answer == nil {
		return v4Reply{outcome: v4OutcomeFailed, sent: sent, err: fmt.Errorf("script: no acquire answer")}
	}
	reply := answer(n)
	reply.sent = sent
	if x.acquireDelay > 0 {
		x.rig.clk.Add(x.acquireDelay)
	}
	return reply
}

func (x *scriptedExchanger) extend(_ context.Context, attempt v4Attempt) v4Reply {
	sent := x.rig.clk.Now()
	x.mu.Lock()
	x.calls = append(x.calls, exchangeCall{at: x.rig.offset(sent), device: attempt.device, attempt: attempt})
	n := x.extends
	x.extends++
	answer := x.extendAnswer
	x.mu.Unlock()
	if answer == nil {
		return v4Reply{outcome: v4OutcomeNoAnswer, sent: sent}
	}
	reply := answer(n, attempt)
	reply.sent = sent
	return reply
}

func (x *scriptedExchanger) snapshot() []exchangeCall {
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([]exchangeCall(nil), x.calls...)
}

// logCapture records every log record the worker emits.
type logCapture struct {
	mu      sync.Mutex
	records []capturedLog
}

// capturedLog is the part of a record the tests judge.
type capturedLog struct {
	level slog.Level
	msg   string
}

func (h *logCapture) Enabled(context.Context, slog.Level) bool { return true }

func (h *logCapture) Handle(_ context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler fixes the value parameter
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, capturedLog{level: r.Level, msg: r.Message})
	return nil
}

func (h *logCapture) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *logCapture) WithGroup(string) slog.Handler      { return h }

// count is the number of records at level whose message contains text.
func (h *logCapture) count(level slog.Level, text string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, r := range h.records {
		if r.level == level && strings.Contains(r.msg, text) {
			n++
		}
	}
	return n
}

// v4Rig runs one DHCPClient's v4 worker against the loop backend, a scripted
// exchanger and a fake clock. Tests using it MUST NOT run in parallel: the
// iface backend is process-global.
type v4Rig struct {
	t       *testing.T
	clk     *sim.FakeClock
	start   time.Time
	backend *loopBackend
	exch    *scriptedExchanger
	bus     *recordingBus
	logs    *logCapture
	client  *DHCPClient
	stopped bool
}

func newV4Rig(t *testing.T, cfg dHCPConfig) *v4Rig {
	t.Helper()
	loopBackendOnce.Do(func() {
		loopBackendErr = iface.RegisterBackend(loopBackendName, func() (iface.Backend, error) {
			return loopBackendCurrent.Load(), nil
		})
	})
	if loopBackendErr != nil {
		t.Fatalf("register loop backend: %v", loopBackendErr)
	}

	// A fresh logical name per rig: the resolver caches bindings by name.
	logical := fmt.Sprintf("uplink%d", loopNameSeq.Add(1))
	backend := &loopBackend{logical: logical}
	loopBackendCurrent.Store(backend)
	if err := iface.LoadBackend(loopBackendName); err != nil {
		t.Fatalf("load loop backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })

	logs := &logCapture{}
	prev := loggerPtr.Load()
	loggerPtr.Store(slog.New(logs))
	t.Cleanup(func() { loggerPtr.Store(prev) })

	start := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	clk := sim.NewFakeClock(start)
	bus := &recordingBus{}
	client, err := newDHCPClient(logical, "0", bus, true, false, cfg, clk)
	if err != nil {
		t.Fatalf("newDHCPClient: %v", err)
	}
	r := &v4Rig{t: t, clk: clk, start: start, backend: backend, bus: bus, logs: logs, client: client}
	r.exch = &scriptedExchanger{rig: r}
	backend.now = func() time.Duration { return r.offset(clk.Now()) }
	client.v4Exchange = r.exch
	t.Cleanup(r.stop)
	return r
}

func (r *v4Rig) offset(at time.Time) time.Duration { return at.Sub(r.start) }

// startWorker starts the client.
func (r *v4Rig) startWorker() {
	r.t.Helper()
	if err := r.client.Start(); err != nil {
		r.t.Fatalf("Start: %v", err)
	}
}

// runUntil steps the fake clock from one armed deadline to the next, each
// time the worker has parked, until the next deadline lies beyond horizon or
// the worker parks with no deadline at all (an infinite lease).
func (r *v4Rig) runUntil(horizon time.Duration) {
	r.t.Helper()
	// The bound: a lease loop that arms a deadline but never advances it
	// would otherwise step forever.
	for range 10000 {
		if !r.clk.AwaitTimers(1, 2*time.Second) {
			return
		}
		next, ok := r.clk.NextTimer()
		if !ok || r.offset(next) > horizon {
			return
		}
		r.clk.Set(next)
	}
	r.t.Fatal("runUntil: the worker kept arming deadlines without reaching the horizon")
}

// stop stops the client; safe to call twice.
func (r *v4Rig) stop() {
	if r.stopped {
		return
	}
	r.stopped = true
	r.client.Stop()
}

// ops is the backend effects so far, as strings.
func (r *v4Rig) ops() []string {
	r.backend.mu.Lock()
	defer r.backend.mu.Unlock()
	out := make([]string, 0, len(r.backend.ops))
	for _, o := range r.backend.ops {
		out = append(out, o.op)
	}
	return out
}

// opTimes is the offsets of the backend effects starting with prefix.
func (r *v4Rig) opTimes(prefix string) []time.Duration {
	r.backend.mu.Lock()
	defer r.backend.mu.Unlock()
	var out []time.Duration
	for _, o := range r.backend.ops {
		if strings.HasPrefix(o.op, prefix+" ") {
			out = append(out, o.at)
		}
	}
	return out
}

// countOps counts backend effects starting with prefix.
func (r *v4Rig) countOps(prefix string) int {
	n := 0
	for _, op := range r.ops() {
		if strings.HasPrefix(op, prefix) {
			n++
		}
	}
	return n
}

// events is the published event types, in order.
func (r *v4Rig) events() []string {
	r.bus.mu.Lock()
	defer r.bus.mu.Unlock()
	out := make([]string, 0, len(r.bus.events))
	for _, ev := range r.bus.events {
		out = append(out, ev.eventType)
	}
	return out
}

func (r *v4Rig) countEvents(eventType string) int {
	n := 0
	for _, ev := range r.events() {
		if ev == eventType {
			n++
		}
	}
	return n
}

// extendTimes is the offsets of the extend calls, split by delivery.
func (r *v4Rig) extendTimes(delivery v4Delivery) []time.Duration {
	var out []time.Duration
	for _, c := range r.exch.snapshot() {
		if !c.acquire && c.attempt.delivery == delivery {
			out = append(out, c.at)
		}
	}
	return out
}

// acquireTimes is the offsets of the acquire calls.
func (r *v4Rig) acquireTimes() []time.Duration {
	var out []time.Duration
	for _, c := range r.exch.snapshot() {
		if c.acquire {
			out = append(out, c.at)
		}
	}
	return out
}

// testAck builds a DHCPACK for loopLeased from server. lease 0 leaves option
// 51 out; router nil leaves option 3 out.
func testAck(t *testing.T, server net.IP, lease time.Duration, router net.IP, mods ...dhcpv4.Modifier) *dhcpv4.DHCPv4 {
	t.Helper()
	all := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(loopLeased),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(server)),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32))),
	}
	if lease > 0 {
		all = append(all, dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(lease)))
	}
	if router != nil {
		all = append(all, dhcpv4.WithOption(dhcpv4.OptRouter(router)))
	}
	ack, err := dhcpv4.New(append(all, mods...)...)
	if err != nil {
		t.Fatalf("build ack: %v", err)
	}
	return ack
}

// testNak builds a DHCPNAK from server carrying message (option 56).
func testNak(t *testing.T, server net.IP, message string) *dhcpv4.DHCPv4 {
	t.Helper()
	nak, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeNak),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(server)),
		dhcpv4.WithOption(dhcpv4.OptMessage(message)),
	)
	if err != nil {
		t.Fatalf("build nak: %v", err)
	}
	return nak
}

func ackReply(msg *dhcpv4.DHCPv4) v4Reply { return v4Reply{outcome: v4OutcomeAck, msg: msg} }
func nakReply(msg *dhcpv4.DHCPv4) v4Reply { return v4Reply{outcome: v4OutcomeNak, msg: msg} }

// seconds converts a float number of seconds, the unit the ACs are written in.
func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// Event type shorthands.
var (
	evAcquired = ifaceevents.EventDHCPAcquired
	evRenewed  = ifaceevents.EventDHCPRenewed
	evExpired  = ifaceevents.EventDHCPExpired
)
