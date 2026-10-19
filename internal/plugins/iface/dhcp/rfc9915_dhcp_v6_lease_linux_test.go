// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: rfc9915_dhcp_v6_exchange_linux_test.go -- the exchange engine tests
// Related: dhcp_v4_rig_linux_test.go -- the loop backend and log capture this rig reuses
// RFC: rfc/short/rfc9915.md

//go:build linux

package ifacedhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/nclient6"
	"github.com/insomniacslk/dhcp/iana"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

var (
	v6TestMAC     = net.HardwareAddr{0x52, 0x54, 0x00, 0x12, 0x34, 0x56}
	v6ServerA     = &dhcpv6.DUIDLL{HWType: iana.HWTypeEthernet, LinkLayerAddr: net.HardwareAddr{2, 0, 0, 0, 0, 0xa}}
	v6ServerB     = &dhcpv6.DUIDLL{HWType: iana.HWTypeEthernet, LinkLayerAddr: net.HardwareAddr{2, 0, 0, 0, 0, 0xb}}
	v6TestAddr    = net.ParseIP("2001:db8::50")
	v6TestPrefix  = &net.IPNet{IP: net.ParseIP("2001:db8:100::"), Mask: net.CIDRMask(56, 128)}
	v6TestCIDR    = "2001:db8::50/128"
	v6ClientIAID  = [4]byte{0x00, 0x12, 0x34, 0x56}
	v6ClientPDIAI = [4]byte{0xff, 0x12, 0x34, 0x56}
)

// v6Sent is one transmission the worker made.
type v6Sent struct {
	at     time.Duration
	device string
	msg    *dhcpv6.Message // decoded from the bytes the worker would send
}

// v6Server answers the worker's transmissions from a script. serve gets the
// decoded message and returns the answers the link offers to the matcher, in
// order; none means silence until the listen window closes.
type v6Server struct {
	mu    sync.Mutex
	rig   *v6Rig
	sent  []v6Sent
	serve func(req *dhcpv6.Message) []*dhcpv6.Message
}

type scriptedV6Link struct {
	srv    *v6Server
	device string
}

func (l scriptedV6Link) exchange(ctx context.Context, clk clock.Clock, window time.Duration, msg *dhcpv6.Message, accept nclient6.Matcher) (*dhcpv6.Message, error) {
	decoded, err := dhcpv6.MessageFromBytes(msg.ToBytes())
	if err != nil {
		return nil, err
	}
	l.srv.mu.Lock()
	l.srv.sent = append(l.srv.sent, v6Sent{at: l.srv.rig.offset(l.srv.rig.clk.Now()), device: l.device, msg: decoded})
	serve := l.srv.serve
	l.srv.mu.Unlock()
	if serve != nil {
		for _, a := range serve(decoded) {
			a.TransactionID = msg.TransactionID
			if accept(a) {
				return a, nil
			}
		}
	}
	closed := make(chan struct{})
	timer := clk.AfterFunc(window, func() { close(closed) })
	defer timer.Stop()
	select {
	case <-closed:
	case <-ctx.Done():
	}
	return nil, errV6NoAnswer
}

func (l scriptedV6Link) close() error { return nil }

// v6Rig runs one DHCPClient's v6 worker against the loop backend, a scripted
// server and a fake clock. Tests using it MUST NOT run in parallel: the iface
// backend is process-global.
type v6Rig struct {
	t       *testing.T
	clk     *sim.FakeClock
	start   time.Time
	backend *loopBackend
	srv     *v6Server
	bus     *recordingBus
	logs    *logCapture
	client  *DHCPClient
	stopped bool
}

func newV6Rig(t *testing.T, cfg dHCPConfig) *v6Rig {
	t.Helper()
	loopBackendOnce.Do(func() {
		loopBackendErr = iface.RegisterBackend(loopBackendName, func() (iface.Backend, error) {
			return loopBackendCurrent.Load(), nil
		})
	})
	if loopBackendErr != nil {
		t.Fatalf("register loop backend: %v", loopBackendErr)
	}
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
	client, err := newDHCPClient(logical, "0", bus, false, true, cfg, clk)
	if err != nil {
		t.Fatalf("newDHCPClient: %v", err)
	}
	r := &v6Rig{t: t, clk: clk, start: start, backend: backend, bus: bus, logs: logs, client: client}
	r.srv = &v6Server{rig: r}
	backend.now = func() time.Duration { return r.offset(clk.Now()) }
	client.v6Dial = func(device string) (v6Link, net.HardwareAddr, error) {
		return scriptedV6Link{srv: r.srv, device: device}, v6TestMAC, nil
	}
	// RAND pinned: 0 gives no Solicit delay, a first Solicit RAND of +0.1 and
	// a RAND of -0.1 everywhere else.
	client.v6Random = func() float64 { return 0 }
	t.Cleanup(func() {
		if !r.stop() {
			t.Error("Stop did not return")
		}
	})
	return r
}

func (r *v6Rig) offset(at time.Time) time.Duration { return at.Sub(r.start) }

func (r *v6Rig) startWorker() {
	r.t.Helper()
	if err := r.client.Start(); err != nil {
		r.t.Fatalf("Start: %v", err)
	}
}

// runUntil steps the fake clock from one armed deadline to the next, each
// time the worker has parked, until the next deadline lies beyond horizon or
// the worker parks with no deadline.
func (r *v6Rig) runUntil(horizon time.Duration) {
	r.t.Helper()
	for range 100000 {
		if !r.clk.AwaitTimers(1, 2*time.Second) {
			return
		}
		next, ok := r.clk.NextTimer()
		if !ok {
			continue // the timer AwaitTimers saw was stopped since
		}
		if r.offset(next) > horizon {
			return
		}
		r.clk.Set(next)
	}
	r.t.Fatal("runUntil: the worker kept arming deadlines without reaching the horizon")
}

// stop stops the client and steps the fake clock while Stop runs, so the
// bounded Release exchange can end; safe to call twice. It answers whether
// Stop returned within 5 s of real time.
func (r *v6Rig) stop() bool {
	if r.stopped {
		return true
	}
	r.stopped = true
	done := make(chan struct{})
	go func() {
		r.client.Stop()
		close(done)
	}()
	// Let the worker see Stop and drop the waits it armed before: stepping
	// the clock to one of those would charge it to the Release.
	select {
	case <-done:
		return true
	case <-time.After(50 * time.Millisecond):
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-done:
			return true
		case <-deadline:
			return false
		default:
		}
		if r.clk.AwaitTimers(1, 10*time.Millisecond) {
			if next, ok := r.clk.NextTimer(); ok {
				r.clk.Set(next)
			}
		}
	}
}

func (r *v6Rig) sent() []v6Sent {
	r.srv.mu.Lock()
	defer r.srv.mu.Unlock()
	return slices.Clone(r.srv.sent)
}

// sentOf is the transmissions of one message type.
func (r *v6Rig) sentOf(kind dhcpv6.MessageType) []v6Sent {
	var out []v6Sent
	for _, s := range r.sent() {
		if s.msg.MessageType == kind {
			out = append(out, s)
		}
	}
	return out
}

// describeSent is every transmission as "type@offset", for failure output.
func (r *v6Rig) describeSent() string {
	var b strings.Builder
	for _, s := range r.sent() {
		b.WriteString(s.msg.MessageType.String())
		b.WriteString("@")
		b.WriteString(s.at.String())
		b.WriteString(" ")
	}
	return b.String()
}

func sentTimes(sent []v6Sent) []time.Duration {
	out := make([]time.Duration, 0, len(sent))
	for _, s := range sent {
		out = append(out, s.at)
	}
	return out
}

func (r *v6Rig) countOps(prefix string) int {
	r.backend.mu.Lock()
	defer r.backend.mu.Unlock()
	n := 0
	for _, op := range r.backend.ops {
		if strings.HasPrefix(op.op, prefix) {
			n++
		}
	}
	return n
}

func (r *v6Rig) countEvents(eventType string) int {
	r.bus.mu.Lock()
	defer r.bus.mu.Unlock()
	n := 0
	for _, ev := range r.bus.events {
		if ev.eventType == eventType {
			n++
		}
	}
	return n
}

// --- answer builders ------------------------------------------------------

func testIAAddr(pref, valid time.Duration) *dhcpv6.OptIAAddress {
	return &dhcpv6.OptIAAddress{IPv6Addr: v6TestAddr, PreferredLifetime: pref, ValidLifetime: valid}
}

func testIANA(t1, t2 time.Duration, opts ...dhcpv6.Option) *dhcpv6.OptIANA {
	na := &dhcpv6.OptIANA{IaId: v6ClientIAID, T1: t1, T2: t2}
	for _, o := range opts {
		na.Options.Add(o)
	}
	return na
}

func testIAPD(t1, t2, pref, valid time.Duration) *dhcpv6.OptIAPD {
	pd := &dhcpv6.OptIAPD{IaId: v6ClientPDIAI, T1: t1, T2: t2}
	pd.Options.Add(&dhcpv6.OptIAPrefix{PreferredLifetime: pref, ValidLifetime: valid, Prefix: v6TestPrefix})
	return pd
}

// testV6Answer answers req from server with kind and opts, carrying the
// client's own Client Identifier, and Rapid Commit when req asked for it.
func testV6Answer(req *dhcpv6.Message, kind dhcpv6.MessageType, server dhcpv6.DUID, opts ...dhcpv6.Option) *dhcpv6.Message {
	m := &dhcpv6.Message{MessageType: kind, TransactionID: req.TransactionID}
	m.AddOption(dhcpv6.OptServerID(server))
	m.AddOption(dhcpv6.OptClientID(req.Options.ClientID()))
	if kind == dhcpv6.MessageTypeReply && req.MessageType == dhcpv6.MessageTypeSolicit {
		m.AddOption(&dhcpv6.OptionGeneric{OptionCode: dhcpv6.OptionRapidCommit})
	}
	for _, o := range opts {
		m.AddOption(o)
	}
	return m
}

// leaseServer answers Solicit with a Rapid Commit Reply and every Renew with
// a Reply, each with lease; it ignores Rebind unless rebind is set.
func leaseServer(lease func() []dhcpv6.Option, renew, rebind bool) func(*dhcpv6.Message) []*dhcpv6.Message {
	return func(req *dhcpv6.Message) []*dhcpv6.Message {
		//exhaustive:ignore // the scripted server answers only the types it serves
		switch req.MessageType {
		case dhcpv6.MessageTypeSolicit:
			return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeReply, v6ServerA, lease()...)}
		case dhcpv6.MessageTypeRenew:
			if renew {
				return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeReply, v6ServerA, lease()...)}
			}
		case dhcpv6.MessageTypeRebind:
			if rebind {
				return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeReply, v6ServerB, lease()...)}
			}
		}
		return nil
	}
}

func lease10x16x30() []dhcpv6.Option {
	return []dhcpv6.Option{testIANA(10*time.Second, 16*time.Second, testIAAddr(20*time.Second, 30*time.Second))}
}

// --- state machine tests --------------------------------------------------

// TestRFC9915Section1824RenewAtT1CarriesLease drives a lease with T1 10 s,
// T2 16 s and valid 30 s to its first T1 and decodes the message sent.
//
// VALIDATES: AC-1 -- at T1 a RENEW goes out carrying the client's DUID, the
// leasing server's DUID, Elapsed Time 0, the IA_NA IAID and the leased
// address, and no second SOLICIT. AC-9 -- every exchange binds the kernel
// device (eth7), never the logical name.
// PREVENTS: D-1 (renewal by a fresh Solicit), D-2 (logical-name socket).
func TestRFC9915Section1824RenewAtT1CarriesLease(t *testing.T) {
	// RFC requirement: RFC9915-18.2.4-4 positive -- the Renew sent at T1 carries the leasing server's Server ID.
	// RFC requirement: RFC9915-18.2.4-5 positive -- the Renew sent at T1 carries the Client ID of the Solicit.
	// RFC requirement: RFC9915-18.2.4-6 positive -- the Renew sent at T1 carries an Elapsed Time of 0.
	// RFC requirement: RFC9915-21.4-2 positive -- with server T1 10 s the Renew goes out at 10 s.
	// RFC requirement: RFC9915-11.4-1 positive -- the default Client ID is a DUID-LL of hardware type Ethernet and the interface MAC.
	r := newV6Rig(t, dHCPConfig{})
	r.srv.serve = leaseServer(lease10x16x30, true, false)
	r.startWorker()
	r.runUntil(seconds(11))

	if got := len(r.sentOf(dhcpv6.MessageTypeSolicit)); got != 1 {
		t.Fatalf("%d SOLICIT sent, want 1", got)
	}
	renews := r.sentOf(dhcpv6.MessageTypeRenew)
	if len(renews) != 1 {
		t.Fatalf("RENEW sent at %v, want one at 10s; sent %s; ops %v; logs %v", sentTimes(renews), r.describeSent(), r.backend.ops, r.logs.records)
	}
	renew := renews[0]
	if renew.at != seconds(10) {
		t.Errorf("RENEW at %v, want 10s", renew.at)
	}
	solicit := r.sentOf(dhcpv6.MessageTypeSolicit)[0].msg
	if !renew.msg.Options.ClientID().Equal(solicit.Options.ClientID()) {
		t.Errorf("RENEW Client ID %v differs from the Solicit's %v", renew.msg.Options.ClientID(), solicit.Options.ClientID())
	}
	if want := (&dhcpv6.DUIDLL{HWType: iana.HWTypeEthernet, LinkLayerAddr: v6TestMAC}); !renew.msg.Options.ClientID().Equal(want) {
		t.Errorf("Client ID %v, want the DUID-LL of the MAC", renew.msg.Options.ClientID())
	}
	if sid := renew.msg.Options.ServerID(); sid == nil || !sid.Equal(v6ServerA) {
		t.Errorf("RENEW Server ID %v, want the leasing server's", sid)
	}
	if et := renew.msg.Options.ElapsedTime(); et != 0 {
		t.Errorf("RENEW Elapsed Time %v, want 0", et)
	}
	na := renew.msg.Options.OneIANA()
	if na == nil || na.IaId != v6ClientIAID {
		t.Fatalf("RENEW IA_NA %v, want IAID %v", na, v6ClientIAID)
	}
	addrs := na.Options.Addresses()
	if len(addrs) != 1 || !addrs[0].IPv6Addr.Equal(v6TestAddr) {
		t.Errorf("RENEW IA Addresses %v, want %v", addrs, v6TestAddr)
	}
	for _, s := range r.sent() {
		if s.device != loopKernelDevice {
			t.Errorf("%s bound to %q, want the kernel device %q", s.msg.MessageType, s.device, loopKernelDevice)
		}
	}
}

// TestDHCPv6LeaseHeldAcrossFiveRenewals runs five lease periods with a server
// answering every Renew.
//
// VALIDATES: AC-2 -- no removal and no dhcp-expired; one dhcp-renewed per
// Reply; the address reinstalled with the lease's lifetimes each time.
// PREVENTS: D-3, the address removed after two renewals.
func TestDHCPv6LeaseHeldAcrossFiveRenewals(t *testing.T) {
	// RFC requirement: RFC9915-18.2.10.1-3 positive -- the leased address is installed as a /128 on every Reply.
	// RFC requirement: RFC9915-21.6-3 positive -- the leased address is installed with valid 30 s and preferred 20 s, the IA Address lifetimes, on every Reply.
	r := newV6Rig(t, dHCPConfig{})
	r.srv.serve = leaseServer(lease10x16x30, true, false)
	r.startWorker()
	r.runUntil(seconds(150))

	renews := sentTimes(r.sentOf(dhcpv6.MessageTypeRenew))
	want := make([]time.Duration, 0, 15)
	for i := 1; i <= 15; i++ {
		want = append(want, seconds(float64(10*i)))
	}
	assertTimes(t, "RENEW", renews, want)
	if got := len(r.sentOf(dhcpv6.MessageTypeRebind)); got != 0 {
		t.Errorf("%d REBIND sent, want 0", got)
	}
	if got := r.countOps("replace " + loopKernelDevice + " " + v6TestCIDR + " 30 20"); got != 16 {
		t.Errorf("address installed with 30/20 %d times, want 16", got)
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("%d removals while the server answered", got)
	}
	if got := r.countEvents(evExpired); got != 0 {
		t.Errorf("%d dhcp-expired, want 0", got)
	}
	if got := r.countEvents(evRenewed); got != 15 {
		t.Errorf("%d dhcp-renewed, want 15", got)
	}
}

// TestRFC9915Section1825RebindAtT2WithoutServerID ignores every Renew.
//
// VALIDATES: AC-3 -- the Renew exchange ends at T2 and a REBIND without
// Server ID, carrying the same IA, goes out; the rebinding server's Reply
// keeps the lease.
func TestRFC9915Section1825RebindAtT2WithoutServerID(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{})
	r.srv.serve = leaseServer(lease10x16x30, false, true)
	r.startWorker()
	r.runUntil(seconds(17))

	renews := sentTimes(r.sentOf(dhcpv6.MessageTypeRenew))
	if len(renews) == 0 || renews[0] != seconds(10) {
		t.Fatalf("RENEW at %v, want the first at 10s", renews)
	}
	for _, at := range renews {
		if at >= seconds(16) {
			t.Errorf("RENEW at %v, at or after T2", at)
		}
	}
	rebinds := r.sentOf(dhcpv6.MessageTypeRebind)
	if len(rebinds) != 1 || rebinds[0].at != seconds(16) {
		t.Fatalf("REBIND at %v, want one at 16s", sentTimes(rebinds))
	}
	if sid := rebinds[0].msg.Options.ServerID(); sid != nil {
		t.Errorf("REBIND carries Server ID %v", sid)
	}
	if na := rebinds[0].msg.Options.OneIANA(); na == nil || len(na.Options.Addresses()) != 1 {
		t.Errorf("REBIND IA_NA %v, want the leased address", na)
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("%d removals although the Rebind was answered", got)
	}
}

// TestRFC9915Section1825ExpiryRemovesAndSolicits answers nothing after the
// acquisition, with an IA_NA and an IA_PD leased.
//
// VALIDATES: AC-4 -- when all valid lifetimes have run out the address is
// removed, one dhcp-expired is published per lease (the /128 and the /56),
// and a SOLICIT exchange starts. D-12: the prefix expiry is reported.
func TestRFC9915Section1825ExpiryRemovesAndSolicits(t *testing.T) {
	// RFC requirement: RFC9915-18-1 positive -- when every valid lifetime has run out the address is removed once and the new Solicit starts at 30 s, after it.
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	acquired := false
	r.srv.serve = func(req *dhcpv6.Message) []*dhcpv6.Message {
		if req.MessageType != dhcpv6.MessageTypeSolicit || acquired {
			return nil
		}
		acquired = true
		return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeReply, v6ServerA,
			testIANA(10*time.Second, 16*time.Second, testIAAddr(20*time.Second, 30*time.Second)),
			testIAPD(10*time.Second, 16*time.Second, 20*time.Second, 30*time.Second))}
	}
	r.startWorker()
	r.runUntil(seconds(32))

	if got := r.countOps("remove-addr " + loopKernelDevice + " " + v6TestCIDR); got != 1 {
		t.Errorf("address removed %d times, want 1", got)
	}
	if got := r.countEvents(evExpired); got != 2 {
		t.Errorf("%d dhcp-expired, want 2 (address and prefix)", got)
	}
	solicits := sentTimes(r.sentOf(dhcpv6.MessageTypeSolicit))
	if len(solicits) < 2 || solicits[1] != seconds(30) {
		t.Errorf("SOLICIT at %v, want a new exchange at 30s", solicits)
	}
	for _, s := range r.sentOf(dhcpv6.MessageTypeRebind) {
		if s.at >= seconds(30) {
			t.Errorf("REBIND at %v, after every lease expired", s.at)
		}
	}
}

// TestDHCPv6InstalledLifetimeEqualsLease installs a finite and an infinite
// lease.
//
// VALIDATES: AC-10 -- the kernel gets the lease's valid and preferred
// lifetimes; 0xffffffff installs as forever (4294967295) and is never
// renewed. A-6 -- the infinite address is registered as leased.
func TestDHCPv6InstalledLifetimeEqualsLease(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{})
	r.srv.serve = leaseServer(func() []dhcpv6.Option {
		return []dhcpv6.Option{testIANA(v6Infinity, v6Infinity, testIAAddr(v6Infinity, v6Infinity))}
	}, true, true)
	r.startWorker()
	r.runUntil(seconds(10000))
	if got := r.countOps("replace " + loopKernelDevice + " " + v6TestCIDR + " 4294967295 4294967295"); got != 1 {
		t.Errorf("infinite lease installed %d times as forever, want 1", got)
	}
	if got := len(r.sentOf(dhcpv6.MessageTypeRenew)) + len(r.sentOf(dhcpv6.MessageTypeRebind)); got != 0 {
		t.Errorf("%d Renew/Rebind for an infinite T1/T2 (AC-15)", got)
	}
	// The registry refuses a second owner of a registered address, and
	// leaves itself unchanged when it does.
	if err := iface.RegisterLeasedAddresses(loopKernelDevice, "probe", []string{v6TestCIDR}); err == nil {
		iface.UnregisterLeasedAddresses("probe")
		t.Errorf("infinite lease %s not registered as leased", v6TestCIDR)
	}
}

// TestRFC9915Section182101NoBindingSendsRequest answers the first Renew with
// NoBinding in the IA_NA.
//
// VALIDATES: AC-16 -- a REQUEST goes to the responding server with the IAs
// the client holds; its Reply keeps the lease.
func TestRFC9915Section182101NoBindingSendsRequest(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{})
	renewed := false
	r.srv.serve = func(req *dhcpv6.Message) []*dhcpv6.Message {
		//exhaustive:ignore // the scripted server answers only the types it serves
		switch req.MessageType {
		case dhcpv6.MessageTypeSolicit, dhcpv6.MessageTypeRequest:
			return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeReply, v6ServerA, lease10x16x30()...)}
		case dhcpv6.MessageTypeRenew:
			if renewed {
				return nil
			}
			renewed = true
			na := testIANA(0, 0)
			na.Options.Add(&dhcpv6.OptStatusCode{StatusCode: iana.StatusNoBinding})
			return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeReply, v6ServerA, na)}
		}
		return nil
	}
	r.startWorker()
	r.runUntil(seconds(11))
	requests := r.sentOf(dhcpv6.MessageTypeRequest)
	if len(requests) != 1 || requests[0].at != seconds(10) {
		t.Fatalf("REQUEST at %v, want one at 10s", sentTimes(requests))
	}
	if sid := requests[0].msg.Options.ServerID(); sid == nil || !sid.Equal(v6ServerA) {
		t.Errorf("REQUEST Server ID %v, want the responding server", sid)
	}
	if na := requests[0].msg.Options.OneIANA(); na == nil || len(na.Options.Addresses()) != 1 {
		t.Errorf("REQUEST IA_NA %v, want the held address", na)
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("%d removals after NoBinding recovery", got)
	}
}

// TestDHCPv6ConfiguredDUIDReachesEveryMessage configures the duid leaf.
//
// VALIDATES: AC-20 -- the configured DUID is the Client Identifier of every
// message across exchanges and a worker restart.
// PREVENTS: D-6, the duid leaf read by nothing and a new DUID per Solicit.
func TestDHCPv6ConfiguredDUIDReachesEveryMessage(t *testing.T) {
	const configured = "00:04:01:02:03:04:05:06:07:08:09:0a:0b:0c:0d:0e:0f:10"
	want, err := parseV6DUID(configured)
	if err != nil {
		t.Fatalf("parseV6DUID: %v", err)
	}
	for run := range 2 {
		r := newV6Rig(t, dHCPConfig{DUID: configured})
		r.srv.serve = leaseServer(lease10x16x30, false, false)
		r.startWorker()
		r.runUntil(seconds(20))
		sent := r.sent()
		if len(sent) < 3 {
			t.Fatalf("run %d: %d messages sent, want a Solicit and Renews", run, len(sent))
		}
		for _, s := range sent {
			if !s.msg.Options.ClientID().Equal(want) {
				t.Errorf("run %d: %s Client ID %v, want %v", run, s.msg.MessageType, s.msg.Options.ClientID(), want)
			}
		}
		r.stop()
	}
}

// TestDHCPv6DefaultDUIDStable restarts the worker twice with no duid.
//
// VALIDATES: AC-20 -- the default DUID-LL is the same in every message of
// every run.
func TestDHCPv6DefaultDUIDStable(t *testing.T) {
	var first dhcpv6.DUID
	for run := range 2 {
		r := newV6Rig(t, dHCPConfig{})
		r.startWorker() // no server: Solicit retransmissions
		r.runUntil(seconds(10))
		for _, s := range r.sent() {
			if first == nil {
				first = s.msg.Options.ClientID()
			}
			if !s.msg.Options.ClientID().Equal(first) {
				t.Errorf("run %d: Client ID %v, want %v", run, s.msg.Options.ClientID(), first)
			}
		}
		r.stop()
	}
	if first == nil {
		t.Fatal("no message sent")
	}
}

// TestDHCPv6ReleaseOnStop stops a client holding a lease.
//
// VALIDATES: AC-21 -- the address is removed, then a RELEASE with Server ID,
// Client ID and the lease goes out; with no answer Stop returns within the
// 3 s bound after at most REL_MAX_RC transmissions.
func TestDHCPv6ReleaseOnStop(t *testing.T) {
	// RFC requirement: RFC9915-18.2.7-6 positive -- on Stop the leased address is removed once and a Release carrying the lease goes out.
	// RFC requirement: RFC9915-18.2.7-7 positive -- on Stop the leased address is removed from the interface once and a Release goes out.
	r := newV6Rig(t, dHCPConfig{})
	r.srv.serve = leaseServer(lease10x16x30, true, false)
	r.startWorker()
	r.runUntil(seconds(5))

	stopAt := r.offset(r.clk.Now())
	if !r.stop() {
		t.Fatal("Stop did not return")
	}
	if spent := r.offset(r.clk.Now()) - stopAt; spent > v6ReleaseBound {
		t.Errorf("Stop took %v of client time, past the %v Release bound", spent, v6ReleaseBound)
	}
	releases := r.sentOf(dhcpv6.MessageTypeRelease)
	if len(releases) == 0 || len(releases) > v6RelMaxRC {
		t.Fatalf("%d RELEASE sent, want 1..%d", len(releases), v6RelMaxRC)
	}
	last := releases[len(releases)-1].at
	if last-releases[0].at >= v6ReleaseBound {
		t.Errorf("RELEASE retransmitted until %v after the first, past the %v bound", last-releases[0].at, v6ReleaseBound)
	}
	rel := releases[0].msg
	if sid := rel.Options.ServerID(); sid == nil || !sid.Equal(v6ServerA) {
		t.Errorf("RELEASE Server ID %v", sid)
	}
	if na := rel.Options.OneIANA(); na == nil || len(na.Options.Addresses()) != 1 {
		t.Errorf("RELEASE IA_NA %v, want the lease", na)
	}
	if got := r.countOps("remove-addr " + loopKernelDevice + " " + v6TestCIDR); got != 1 {
		t.Errorf("address removed %d times on Stop, want 1", got)
	}
}

// TestDHCPv6StopDuringEveryWait stops the worker in each wait.
//
// VALIDATES: AC-24 -- Stop during Solicit, Bound, Renew and Rebind returns
// promptly and leaves no leased address.
func TestDHCPv6StopDuringEveryWait(t *testing.T) {
	cases := []struct {
		name  string
		serve func(*dhcpv6.Message) []*dhcpv6.Message
		at    time.Duration
	}{
		{"solicit", nil, seconds(30)},
		{"bound", leaseServer(lease10x16x30, true, false), seconds(5)},
		{"renew", leaseServer(lease10x16x30, false, false), seconds(12)},
		{"rebind", leaseServer(lease10x16x30, false, false), seconds(20)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newV6Rig(t, dHCPConfig{})
			r.srv.serve = tc.serve
			r.startWorker()
			r.runUntil(tc.at)
			stopAt := r.offset(r.clk.Now())
			if !r.stop() {
				t.Fatal("Stop did not return")
			}
			if spent := r.offset(r.clk.Now()) - stopAt; spent > v6ReleaseBound {
				t.Errorf("Stop took %v of client time, past the %v Release bound", spent, v6ReleaseBound)
			}
			if tc.serve != nil && r.countOps("remove-addr") != 1 {
				t.Errorf("address removed %d times, want 1", r.countOps("remove-addr"))
			}
		})
	}
}

// --- lease record tests ---------------------------------------------------

func applyReply(l *v6Lease, at time.Time, opts ...dhcpv6.Option) v6Applied {
	m := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeReply}
	for _, o := range opts {
		m.AddOption(o)
	}
	return l.apply(m, at, v6ClientIAID, v6ClientPDIAI)
}

// TestRFC9915Section182101ValidZeroDiscarded applies a valid lifetime of 0.
//
// VALIDATES: AC-11 -- a new lease with valid 0 is never recorded; a held
// lease answered with valid 0 is removed.
// PREVENTS: D-7, a valid-0 lease installed permanent.
func TestRFC9915Section182101ValidZeroDiscarded(t *testing.T) {
	// RFC requirement: RFC9915-18.3.5-1 negative -- a new lease with valid lifetime 0 is not recorded and a held lease answered with valid 0 is removed.
	now := time.Unix(1000, 0)
	l := &v6Lease{}
	a := applyReply(l, now, testIANA(10*time.Second, 16*time.Second, testIAAddr(0, 0)))
	if len(a.installed) != 0 || !l.empty() {
		t.Errorf("valid 0 recorded: %+v", a)
	}
	applyReply(l, now, lease10x16x30()...)
	a = applyReply(l, now, testIANA(10*time.Second, 16*time.Second, testIAAddr(0, 0)))
	if len(a.removed) != 1 || !l.empty() {
		t.Errorf("held lease answered with valid 0 not removed: %+v", a)
	}
}

// TestRFC9915Section216PreferredAboveValidDiscarded applies preferred > valid.
//
// VALIDATES: AC-12 -- the address is not recorded.
func TestRFC9915Section216PreferredAboveValidDiscarded(t *testing.T) {
	// RFC requirement: RFC9915-21.6-4 negative -- an IA Address with preferred lifetime above valid lifetime is not recorded.
	l := &v6Lease{}
	a := applyReply(l, time.Unix(0, 0), testIANA(10*time.Second, 16*time.Second, testIAAddr(31*time.Second, 30*time.Second)))
	if !l.empty() || a.discarded != 1 {
		t.Errorf("preferred above valid recorded: %+v", a)
	}
}

// TestRFC9915Section214T1AboveT2DiscardsIA applies an IA_NA with T1 > T2
// beside a valid IA_PD.
//
// VALIDATES: AC-12 -- the IA_NA is discarded and the IA_PD processed.
func TestRFC9915Section214T1AboveT2DiscardsIA(t *testing.T) {
	l := &v6Lease{}
	applyReply(l, time.Unix(0, 0),
		testIANA(20*time.Second, 10*time.Second, testIAAddr(20*time.Second, 30*time.Second)),
		testIAPD(10*time.Second, 16*time.Second, 20*time.Second, 30*time.Second))
	if l.holds(v6KindAddress) {
		t.Error("IA_NA with T1 > T2 recorded")
	}
	if !l.holds(v6KindPrefix) {
		t.Error("the valid IA_PD beside it was not processed")
	}
}

// TestRFC9915Section142ZeroTimersChosenByClient applies T1 = T2 = 0.
//
// VALIDATES: AC-13 -- T1 is 0.5 and T2 0.8 of the shortest preferred
// lifetime; an infinite preferred lifetime means no renewal.
func TestRFC9915Section142ZeroTimersChosenByClient(t *testing.T) {
	// RFC requirement: RFC9915-14.2-1 positive -- with T1 and T2 of 0 the client sets T1 to 0.5 and T2 to 0.8 of the shortest preferred lifetime.
	// RFC requirement: RFC9915-14.2-2 positive -- with T1 and T2 of 0 the chosen T1 is 50 s, not an immediate transmission.
	at := time.Unix(0, 0)
	l := &v6Lease{}
	applyReply(l, at, testIANA(0, 0, testIAAddr(100*time.Second, 200*time.Second)))
	t1, t2 := l.renewTimes()
	if t1.never || t1.at != at.Add(50*time.Second) {
		t.Errorf("T1 %+v, want 50s", t1)
	}
	if t2.never || t2.at != at.Add(80*time.Second) {
		t.Errorf("T2 %+v, want 80s", t2)
	}
	l = &v6Lease{}
	applyReply(l, at, testIANA(0, 0, testIAAddr(v6Infinity, v6Infinity)))
	if t1, _ := l.renewTimes(); !t1.never {
		t.Errorf("infinite preferred lifetime renews at %+v", t1)
	}
}

// TestRFC9915Section1824EarliestT1AcrossIAs holds an IA_NA with T1 100 s and
// an IA_PD with T1 40 s.
//
// VALIDATES: AC-14 -- the renewal time is 40 s.
// PREVENTS: D-9, timers read from the first IA_NA only.
func TestRFC9915Section1824EarliestT1AcrossIAs(t *testing.T) {
	// RFC requirement: RFC9915-18.2.4-1 positive -- with IAs carrying different T1/T2 the client renews at the earliest, 40 s, and rebinds at 64 s.
	at := time.Unix(0, 0)
	l := &v6Lease{}
	applyReply(l, at,
		testIANA(100*time.Second, 160*time.Second, testIAAddr(200*time.Second, 300*time.Second)),
		testIAPD(40*time.Second, 64*time.Second, 200*time.Second, 300*time.Second))
	t1, t2 := l.renewTimes()
	if t1.at != at.Add(40*time.Second) || t2.at != at.Add(64*time.Second) {
		t.Errorf("T1/T2 %v/%v, want 40s/64s", t1.at.Sub(at), t2.at.Sub(at))
	}
}

// TestRFC9915Section182101OmittedIAKept answers a Renew with the IA_NA only
// while the client holds an IA_PD.
//
// VALIDATES: AC-17 -- the prefix stays recorded unchanged and its renewal is
// delayed, not repeated at once.
func TestRFC9915Section182101OmittedIAKept(t *testing.T) {
	at := time.Unix(0, 0)
	l := &v6Lease{}
	applyReply(l, at, lease10x16x30()[0], testIAPD(10*time.Second, 16*time.Second, 20*time.Second, 30*time.Second))
	later := at.Add(10 * time.Second)
	applyReply(l, later, lease10x16x30()...)
	if !l.holds(v6KindPrefix) {
		t.Fatal("omitted IA_PD forgotten")
	}
	if p := l.live(v6KindPrefix, later); len(p) != 1 || p[0].received != at {
		t.Errorf("omitted IA_PD changed: %+v", p)
	}
	if t1, _ := l.iaTimes(v6KindPrefix); !t1.at.After(later) {
		t.Errorf("omitted IA_PD renews again at %v, want after %v", t1.at, later)
	}
}

// TestDHCPv6LeaseBindingsCapped offers 20 addresses in one IA_NA.
//
// VALIDATES: the resource bound on untrusted input: the lease records at most
// v6BindingsMax (16) addresses, the rest are discarded and never installed.
func TestDHCPv6LeaseBindingsCapped(t *testing.T) {
	na := testIANA(10*time.Second, 16*time.Second)
	for i := range 20 {
		ip := net.ParseIP("2001:db8::")
		ip[15] = byte(i + 1)
		na.Options.Add(&dhcpv6.OptIAAddress{IPv6Addr: ip, PreferredLifetime: 20 * time.Second, ValidLifetime: 30 * time.Second})
	}
	l := &v6Lease{}
	a := applyReply(l, time.Unix(0, 0), na)
	if len(a.installed) != v6BindingsMax || a.discarded != 4 {
		t.Errorf("installed %d, discarded %d; want %d and 4", len(a.installed), a.discarded, v6BindingsMax)
	}
	if got := len(l.live(v6KindAddress, time.Unix(0, 0))); got != v6BindingsMax {
		t.Errorf("recorded %d addresses, want %d", got, v6BindingsMax)
	}
}
