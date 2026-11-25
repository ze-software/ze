// Design: docs/architecture/ldp/mpls-ldp.md -- discovery interface resolution tests
package ldp

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: AC-3 -- a Hello that fails to send is counted on every failure
// and is one condition: the first failure WARNs once, naming the interface,
// why (the adjacencies there expire) and the fix, under
// doctor-ldp-hello-send-failed, and raises one warning; the next Hello that
// sends clears it, and a later failure WARNs and raises again. Method: drive
// helloSendHealth.observe with what sendHello answers on a closed socket (a
// real write error) and on an open one, as discoverOnInterface does each tick.
// PREVENTS: the failure logged at debug only, invisible at the default level;
// a WARN on every Hello interval while the cause lasts; and a latch that never
// resets, which would hide the next outage.
func TestLDPHelloSendFailureIsWarned(t *testing.T) {
	publisher, sendErrors, _ := useProblemFakes(t)
	handler := &recordingHandler{}
	log := slog.New(handler)
	cfg := ldpConfig{HelloHoldTime: 15 * time.Second}
	lsrID := [4]byte{10, 0, 0, 1}

	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen receiver: %v", err)
	}
	t.Cleanup(func() { _ = receiver.Close() })
	dest, isUDP := receiver.LocalAddr().(*net.UDPAddr)
	if !isUDP {
		t.Fatalf("receiver.LocalAddr() = %T, want *net.UDPAddr", receiver.LocalAddr())
	}
	open, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen sender: %v", err)
	}
	t.Cleanup(func() { _ = open.Close() })
	closed, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen closed sender: %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("close sender: %v", err)
	}

	health := newHelloSendHealth("eth0")
	for range 3 {
		health.observe(log, sendHello(closed, dest, lsrID, cfg))
	}

	if got := sendErrors.value("eth0"); got != 3 {
		t.Errorf("ze_ldp_hello_send_errors_total{interface=eth0} = %v, want 3", got)
	}
	warns := handler.levelRecords(slog.LevelWarn)
	if len(warns) != 1 {
		t.Fatalf("WARN lines after three failed Hellos = %d, want 1", len(warns))
	}
	warn := warns[0]
	if !strings.Contains(warn.Message, "interface eth0") {
		t.Errorf("WARN message %q does not name interface eth0", warn.Message)
	}
	if got := recordAttr(warn, "code"); got != codeLDPHelloSendFailed {
		t.Errorf("code = %q, want %q", got, codeLDPHelloSendFailed)
	}
	if got := recordAttr(warn, "why"); !strings.Contains(got, "adjacency") {
		t.Errorf("why = %q, want it to say the adjacencies expire", got)
	}
	if got := recordAttr(warn, "fix"); got != "ze show interface name eth0 detail" {
		t.Errorf("fix = %q, want the interface's show command", got)
	}
	if recordAttr(warn, "error") == "" {
		t.Error("WARN carries no error attribute: the write error is lost")
	}
	if raised, cleared := publisher.counts(); raised != 1 || cleared != 0 {
		t.Fatalf("raises, clears = %d, %d, want 1, 0", raised, cleared)
	}
	if issue := publisher.raised[0]; issue.Severity != rpc.ReportWarning || issue.Subject != "eth0" {
		t.Errorf("raised %+v, want a warning for subject eth0", issue)
	}

	health.observe(log, sendHello(open, dest, lsrID, cfg))
	if raised, cleared := publisher.counts(); raised != 1 || cleared != 1 {
		t.Fatalf("raises, clears after a Hello sent = %d, %d, want 1, 1", raised, cleared)
	}

	health.observe(log, sendHello(closed, dest, lsrID, cfg))
	if got := len(handler.levelRecords(slog.LevelWarn)); got != 2 {
		t.Errorf("WARN lines after the failure returned = %d, want 2", got)
	}
	if raised, _ := publisher.counts(); raised != 2 {
		t.Errorf("raises after the failure returned = %d, want 2", raised)
	}
	if got := sendErrors.value("eth0"); got != 4 {
		t.Errorf("ze_ldp_hello_send_errors_total{interface=eth0} = %v, want 4", got)
	}
}

// TestLDPRegistrationNeedsDataPlane proves LDP reaches forwarding through a
// declaration rather than a writer's name: it declares NeedsDataPlane, so the
// engine loads the writer of the configured data plane, and its doctor check
// depends on LDP itself.
//
// VALIDATES: AC-16 -- LDP declares NeedsDataPlane; no Dependencies entry is a
// registration that declares ProgramsFIB. This package links no FIB writer,
// so the cross-registry half is TestNoRegistrationDependsOnAFIBWriter
// (internal/component/plugin/all); here every dependency LDP names must be
// one whose registration, where linked, does not program a FIB.
func TestLDPRegistrationNeedsDataPlane(t *testing.T) {
	reg := registry.Lookup(pluginName)
	if reg == nil {
		t.Fatal("ldp is not registered")
	}
	if !reg.NeedsDataPlane {
		t.Fatal("ldp does not declare NeedsDataPlane, so no writer is loaded for its label entries")
	}
	for _, dep := range reg.Dependencies {
		if other := registry.Lookup(dep); other != nil && other.ProgramsFIB {
			t.Errorf("ldp Dependencies names %q, which programs a FIB", dep)
		}
	}
	for _, check := range reg.DoctorChecks {
		for _, dep := range check.Dependencies {
			if other := registry.Lookup(dep); other != nil && other.ProgramsFIB {
				t.Errorf("ldp doctor check %q depends on %q, which programs a FIB", check.Name, dep)
			}
		}
		if len(check.Dependencies) != 1 || check.Dependencies[0] != pluginName {
			t.Errorf("ldp doctor check %q Dependencies = %v, want [%s]", check.Name, check.Dependencies, pluginName)
		}
	}
}

// VALIDATES: spec-ospf-ext-11 AC-11, A-4, R-4 -- a discovered adjacency (and hence
// the emitted SessionUp/SessionDown event) carries the local interface it was
// discovered on, so an LDP-IGP-sync consumer can key its per-interface state
// (RFC 5443 §2) instead of reverse-mapping a transport address.
func TestLDPSessionEventCarriesInterface(t *testing.T) {
	const ifName = "eth7"
	adjTable := newAdjacencyTable()

	// Build a valid discovery Hello from a peer LSR (mirrors sendHello), addressed
	// from a different LSR-ID than the local one so it is not self-filtered.
	var buf [128]byte
	bodyLen := EncodeHello(buf[ldpHeaderLen:], HelloMessage{
		MessageID:     1,
		HoldTime:      15,
		TransportAddr: netip.MustParseAddr("10.0.0.2"),
	})
	pduLen := uint16(bodyLen + 6)
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  pduLen,
		LSRID:      [4]byte{10, 0, 0, 2},
		LabelSpace: 0,
	})
	data := buf[:ldpHeaderLen+bodyLen]

	var got *Adjacency
	processDiscoveryPacket(data, [4]byte{10, 0, 0, 1}, ifName, adjTable, func(a *Adjacency) { got = a }, slogutil.DiscardLogger())

	if got == nil {
		t.Fatal("onNewAdj was not called for a new peer Hello")
	}
	if got.Interface != ifName {
		t.Errorf("adjacency Interface = %q, want %q (the discovering interface tags the SessionEvent)", got.Interface, ifName)
	}
}

// TestLDPTransportAddressBinding verifies the configured transport-address is
// bound as the dialer's local source, and that an unset transport-address leaves
// the source OS-selected.
//
// VALIDATES: AC-9/AC-10 -- LDP binds transport-address as TCP source when
// configured (RFC 5036), unchanged when not.
// PREVENTS: transport-address being advertised in Hellos but not bound to the
// session TCP, so the session could originate from a different source.
func TestLDPTransportAddressBinding(t *testing.T) {
	// Configured: valid transport-address -> bound as LocalAddr.
	d := ldpSessionDialer(netip.MustParseAddr("10.0.0.1"))
	if d.LocalAddr == nil {
		t.Fatal("expected LocalAddr bound for configured transport-address")
	}
	if got := d.LocalAddr.IP.String(); got != "10.0.0.1" {
		t.Errorf("LocalAddr IP = %q, want %q", got, "10.0.0.1")
	}

	// Unconfigured: zero-value netip.Addr (IsValid()==false) -> no binding.
	d2 := ldpSessionDialer(netip.Addr{})
	if d2.LocalAddr != nil {
		t.Errorf("expected no LocalAddr for unset transport-address, got %v", d2.LocalAddr)
	}
}

// VALIDATES: spec-ospf-ext-11 review FIX 1 (DATA RACE) -- the discovering interface
// name must be written inside AdjacencyTable.Update under the table lock, never as an
// unlocked field write on the returned *Adjacency after Update returns. This test
// hammers Update (received-Hello path) concurrently with All() (the read-side
// snapshot) on the SAME adjacency; run under -race it must report no data race on
// Adjacency.Interface. It fails (races) if Interface is set outside the lock.
func TestAdjacencyInterfaceNoRace(t *testing.T) {
	adjTable := newAdjacencyTable()
	pdu := PDUHeader{Version: ldpVersion, LSRID: [4]byte{10, 0, 0, 2}, LabelSpace: 0}
	hello := HelloMessage{HoldTime: 15, TransportAddr: netip.MustParseAddr("10.0.0.2")}

	const iterations = 2000
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: refresh the same adjacency (as a repeated received Hello would),
	// each time setting the discovering interface under the lock.
	go func() {
		defer wg.Done()
		for range iterations {
			adjTable.Update(pdu, hello, "eth0")
		}
	}()

	// Reader: snapshot all adjacencies (reads every field, incl. Interface).
	go func() {
		defer wg.Done()
		for range iterations {
			for _, adj := range adjTable.All() {
				_ = adj.Interface
			}
		}
	}()

	wg.Wait()
}

// TestWaitForInterfaceFound moved to resolve_integration_linux_test.go
// (TestWaitForInterfaceFoundResolves). waitForInterface now resolves through the
// shared iface resolver, which needs the kernel backend (Linux-only), so the
// "interface found" path is no longer host-testable; the integration test
// replaces that coverage against a real device. The cancellation and warn-once
// paths below stay host tests: an absent interface fails to resolve whether or
// not a backend is loaded, so their behavior is unchanged.

// VALIDATES: waitForInterface returns nil (does not block forever) when the
// context is canceled before a missing interface appears -- the retry loop is
// cancellation-safe, so a goroutine for an absent interface unwinds cleanly.
func TestWaitForInterfaceCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	ifi := waitForInterface(ctx, slogutil.DiscardLogger(), "ze-nonexistent-iface-xyz", time.Second)
	if ifi != nil {
		t.Errorf("waitForInterface returned %v, want nil on canceled context", ifi)
	}
}

// warnCounter is a slog.Handler that counts WARN+ records.
type warnCounter struct{ warns atomic.Int64 }

func (w *warnCounter) Enabled(context.Context, slog.Level) bool { return true }
func (w *warnCounter) Handle(_ context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface mandates Record by value
	if r.Level >= slog.LevelWarn {
		w.warns.Add(1)
	}
	return nil
}
func (w *warnCounter) WithAttrs([]slog.Attr) slog.Handler { return w }
func (w *warnCounter) WithGroup(string) slog.Handler      { return w }

// VALIDATES: a permanently-missing interface is warned about once, not on every
// retry -- so a misconfigured interface name does not spam the log.
func TestWaitForInterfaceWarnsOnce(t *testing.T) {
	wc := &warnCounter{}
	log := slog.New(wc)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		// 1ms retry so several iterations elapse before we cancel.
		waitForInterface(ctx, log, "ze-nonexistent-iface-xyz", time.Millisecond)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // allow many retry cycles
	cancel()
	<-done

	if got := wc.warns.Load(); got != 1 {
		t.Errorf("warn count = %d, want exactly 1 (log-once across retries)", got)
	}
}
