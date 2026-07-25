// Design: plan/spec-mpls-2-ldp.md -- discovery interface resolution tests
package ldp

import (
	"context"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: spec-ospf-ext-11 AC-11, A-4, R-4 -- a discovered adjacency (and hence
// the emitted SessionUp/SessionDown event) carries the local interface it was
// discovered on, so an LDP-IGP-sync consumer can key its per-interface state
// (RFC 5443 §2) instead of reverse-mapping a transport address.
func TestLDPSessionEventCarriesInterface(t *testing.T) {
	const ifName = "eth7"
	adjTable := NewAdjacencyTable()

	// Build a valid discovery Hello from a peer LSR (mirrors sendHello), addressed
	// from a different LSR-ID than the local one so it is not self-filtered.
	var buf [128]byte
	bodyLen := EncodeHello(buf[ldpHeaderLen:], HelloMessage{
		MessageID:     1,
		HoldTime:      15,
		TransportAddr: netip.MustParseAddr("10.0.0.2"),
	})
	pduLen := uint16(bodyLen + 6)
	EncodePDUHeader(buf[:], PDUHeader{
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
	adjTable := NewAdjacencyTable()
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

// test-relax: TestWaitForInterfaceFound moved to resolve_integration_linux_test.go
// (TestWaitForInterfaceFoundResolves). waitForInterface now resolves through the
// shared iface resolver, which needs the netlink backend (Linux-only), so the
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
