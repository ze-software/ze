// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- panic containment at the signaling and receive tasks
// Related: panic_guard.go -- the guard, its tasks and the signaling task's end
// Related: pathmtu_floor_test.go -- floorPath, the egress PATH this file varies per LSP
// Related: rfc3209_engine_test.go -- testEngine, fakeTransport, fakeFIB
//
// A Ze defect a peer's input reaches ends the LSP that input named, or drops
// that datagram, and leaves the plugin and every other LSP running. The count
// is read on the real guard against a baseline: the package guard is never
// swapped, because transports other tests leave behind still read it.
package rsvpte

import (
	"context"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// panickingPopFIB is a fakeFIB whose programPop panics on one call, the
// injected Ze defect. Only the engine's signaling goroutine calls it.
type panickingPopFIB struct {
	*fakeFIB
	// panicAt is the 1-based programPop call that panics; calls counts them.
	panicAt int32
	calls   atomic.Int32
}

func (f *panickingPopFIB) programPop(inLabel uint32, nextHop netip.Addr, pathMTU uint32) error {
	if f.calls.Add(1) == f.panicAt {
		panic("injected test panic in programPop")
	}
	return f.fakeFIB.programPop(inLabel, nextHop, pathMTU)
}

// guardEvidence is what a contained panic leaves: the task's count and the
// guard's ERROR lines in the log ring.
type guardEvidence struct {
	panics uint64
	lines  int
}

func snapshotGuard(t *testing.T, task string) guardEvidence {
	t.Helper()
	count, declared := rsvpPanics().guard.Panics(task)
	if !declared {
		t.Fatalf("task %q is not declared on the rsvp-te guard", task)
	}
	entries, err := slogutil.GlobalLogRing().Snapshot(0, "ERROR", "rsvp-te")
	if err != nil {
		t.Fatalf("log ring snapshot: %v", err)
	}
	lines := 0
	for _, entry := range entries {
		if strings.Contains(entry.Message, "an internal Ze defect ended this unit") {
			lines++
		}
	}
	return guardEvidence{panics: count, lines: lines}
}

// requireContained asserts one more panic was counted, logged and recorded.
func requireContained(t *testing.T, task string, before guardEvidence) {
	t.Helper()
	after := snapshotGuard(t, task)
	if after.panics != before.panics+1 {
		t.Errorf("rsvp-te %s panics = %d, want %d: the panic was not counted", task, after.panics, before.panics+1)
	}
	if after.lines != before.lines+1 {
		t.Errorf("guard ERROR lines in the log ring = %d, want %d: the panic was not logged", after.lines, before.lines+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
}

// guardedEgressPath is floorPath's egress PATH at MTU 1500 for tunnel id, so
// each LSP sits in its own session and its admission charge is its own.
func guardedEgressPath(t *testing.T, tunnelID uint16) (Packet, lspKey) {
	t.Helper()
	psb := samplePSB()
	psb.ERO = nil
	psb.Session.TunnelID = tunnelID
	var adspec [adspecSize]byte
	if n := encodeAdspec(adspec[:], 1500, serviceControlledLoad); n != adspecSize {
		t.Fatalf("encodeAdspec wrote %d octets, want %d", n, adspecSize)
	}
	psb.Adspec = adspec[:]
	sender := netip.MustParseAddr("10.0.0.1")
	key := lspKey{TunnelEndpoint: psb.Session.TunnelEndpoint, TunnelID: tunnelID, ExtTunnelID: psb.Session.ExtTunnelID,
		SenderAddr: psb.SenderTemplate.SenderAddr, LSPID: psb.SenderTemplate.LSPID}
	return Packet{Src: sender, Payload: buildPath(psb, sender, defaultIPTTL)}, key
}

// awaitResvCount waits until the engine has sent want RESV messages.
func awaitResvCount(t *testing.T, ft *fakeTransport, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for ft.countByType(MsgTypeResv) < want {
		if time.Now().After(deadline) {
			t.Fatalf("RESV count = %d, want %d: the engine stopped serving", ft.countByType(MsgTypeResv), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestRSVPSignalingPanicContained: the egress handles three PATHs on its real
// run loop. The second one's pop installation panics, a Ze defect a peer's
// PATH reaches. VALIDATES: the process survives; the victim LSP is torn down
// locally, its admission charge included; the LSP set up before keeps its
// state; the PATH after is still answered; and the panic is counted, logged
// and recorded. PREVENTS: one bad message ending RSVP-TE on the node.
func TestRSVPSignalingPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	before := snapshotGuard(t, taskSignaling)

	e, ft, fib := testEngine(t, "10.0.0.9", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 1e12, MaxReservableBW: 1e12}}
	})
	e.admission.setInterface("eth0", 1e12, 1e12)
	e.fib = &panickingPopFIB{fakeFIB: fib, panicAt: 2}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		e.run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-stopped
	})

	survivorPkt, survivor := guardedEgressPath(t, 1)
	victimPkt, victim := guardedEgressPath(t, 2)
	afterPkt, after := guardedEgressPath(t, 3)

	ft.recvCh <- survivorPkt
	awaitResvCount(t, ft, 1)
	ft.recvCh <- victimPkt
	ft.recvCh <- afterPkt
	awaitResvCount(t, ft, 2)

	if _, found := e.table.Get(victim); found {
		t.Error("the victim LSP is still in the table: the panic's end did not tear it down")
	}
	for name, key := range map[string]lspKey{"survivor": survivor, "after": after} {
		lsp, found := e.table.Get(key)
		if !found {
			t.Errorf("the %s LSP is missing from the table", name)
			continue
		}
		lsp.mu.Lock()
		role, hasRSB := lsp.Role, lsp.RSB != nil
		lsp.mu.Unlock()
		if role != RoleEgress || !hasRSB {
			t.Errorf("the %s LSP role = %v, reservation held = %v: want an egress holding its reservation", name, role, hasRSB)
		}
	}

	// One LSP charges TokenRate * 8 bits per second: the survivor and the LSP
	// after it remain charged, the victim's charge is gone.
	e.admission.mu.Lock()
	reserved := e.admission.interfaces["eth0"].ReservedBandwidth
	e.admission.mu.Unlock()
	if want := 2 * float64(samplePSB().SenderTSpec.TokenRate) * 8; reserved != want {
		t.Errorf("eth0 reserved bandwidth = %v, want %v: the victim's admission charge leaked", reserved, want)
	}
	requireContained(t, taskSignaling, before)
}

// guardRestart returns the restart field of the newest guard ERROR line the
// task logged.
func guardRestart(t *testing.T, task string) string {
	t.Helper()
	entries, err := slogutil.GlobalLogRing().Snapshot(0, "ERROR", "rsvp-te")
	if err != nil {
		t.Fatalf("log ring snapshot: %v", err)
	}
	for _, entry := range entries {
		if !strings.Contains(entry.Message, "an internal Ze defect ended this unit") {
			continue
		}
		attrs, err := entry.Attributes()
		if err != nil {
			t.Fatalf("guard line attributes: %v", err)
		}
		if attrs["task"] != task {
			continue
		}
		restart, ok := attrs["restart"].(string)
		if !ok {
			t.Fatalf("guard line restart = %v, want a string", attrs["restart"])
		}
		return restart
	}
	t.Fatalf("no guard ERROR line for task %q in the log ring", task)
	return ""
}

// TestRSVPSignalingPanicTearsOnlyInstalledState: the egress's first PATH
// panics in programPop, after the handler created the LSP and before it wrote
// the LSP's role. VALIDATES: the contained panic's teardown withdraws no
// forwarding entry, neither the tunnel endpoint's push (which the zero role
// once read as a head-end's) nor a swap or pop the LSP never installed; the
// next PATH is still answered; and the logged restart names what brings back
// each role. PREVENTS: a contained panic removing another LSP's host route to
// the same endpoint, and a restart line that tells the operator to wait for a
// refresh a head-end never receives.
func TestRSVPSignalingPanicTearsOnlyInstalledState(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	before := snapshotGuard(t, taskSignaling)

	e, ft, fib := testEngine(t, "10.0.0.9", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 1e12, MaxReservableBW: 1e12}}
	})
	e.admission.setInterface("eth0", 1e12, 1e12)
	e.fib = &panickingPopFIB{fakeFIB: fib, panicAt: 1}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		e.run(ctx)
	}()

	victimPkt, victim := guardedEgressPath(t, 2)
	afterPkt, after := guardedEgressPath(t, 3)
	ft.recvCh <- victimPkt
	ft.recvCh <- afterPkt
	awaitResvCount(t, ft, 1)
	cancel()
	<-stopped

	if _, found := e.table.Get(victim); found {
		t.Error("the victim LSP is still in the table: the panic's end did not tear it down")
	}
	if _, found := e.table.Get(after); !found {
		t.Error("the LSP after the panic is missing: the engine stopped serving")
	}
	if len(fib.removed) != 0 {
		t.Errorf("push entries withdrawn = %v, want none: the victim never installed a push", fib.removed)
	}
	if len(fib.removedSwap) != 0 {
		t.Errorf("swap or pop entries withdrawn = %v, want none: the victim never installed one", fib.removedSwap)
	}
	requireContained(t, taskSignaling, before)

	restart := guardRestart(t, taskSignaling)
	for _, want := range []string{
		"a transit or egress LSP is set up again by the upstream's next PATH refresh",
		"a tunnel or bypass this router heads is signaled again only when the configuration is next committed",
	} {
		if !strings.Contains(restart, want) {
			t.Errorf("restart = %q, want it to name %q", restart, want)
		}
	}
}

// TestTearLSPLocalWithdrawsOnlyInstalledPush: a head-end LSP still waiting for
// its RESV (path-sent, no reservation recorded) and an LSP no handler gave a
// role are torn down locally. VALIDATES: neither withdraws the tunnel
// endpoint's push, which only a recorded reservation installed, while a
// reserved head-end still withdraws its own. PREVENTS: a local teardown
// removing the host route another LSP to the same endpoint installed.
func TestTearLSPLocalWithdrawsOnlyInstalledPush(t *testing.T) {
	e, _, fib := testEngine(t, "10.0.0.9", nil)
	endpoint := netip.MustParseAddr("10.0.0.2")
	keys := map[string]lspKey{
		"path-sent head-end": {TunnelEndpoint: endpoint, TunnelID: 1, LSPID: 1},
		"unspecified role":   {TunnelEndpoint: endpoint, TunnelID: 2, LSPID: 1},
		"reserved head-end":  {TunnelEndpoint: endpoint, TunnelID: 3, LSPID: 1},
	}
	for name, key := range keys {
		lsp, _ := e.table.GetOrCreate(key)
		lsp.mu.Lock()
		if got := lsp.Role; got != RoleUnspecified {
			t.Errorf("%s: a new LSP's role = %v, want %v", name, got, RoleUnspecified)
		}
		switch name {
		case "path-sent head-end":
			lsp.Role, lsp.State = RoleIngress, LSPStatePathSent
		case "reserved head-end":
			lsp.Role, lsp.State, lsp.Reserved = RoleIngress, LSPStateUp, true
		}
		lsp.mu.Unlock()
	}

	e.tearLSPLocal(keys["path-sent head-end"])
	e.tearLSPLocal(keys["unspecified role"])
	if len(fib.removed) != 0 {
		t.Fatalf("push entries withdrawn = %v, want none: neither LSP installed a push", fib.removed)
	}
	e.tearLSPLocal(keys["reserved head-end"])
	if want := netip.PrefixFrom(endpoint, 32); len(fib.removed) != 1 || fib.removed[0] != want {
		t.Errorf("push entries withdrawn = %v, want [%v]: the reserved head-end's push stays installed", fib.removed, want)
	}
	if len(fib.removedSwap) != 0 {
		t.Errorf("swap or pop entries withdrawn = %v, want none", fib.removedSwap)
	}
	if n := e.table.Len(); n != 0 {
		t.Errorf("LSP table holds %d entries after the teardowns, want 0", n)
	}
}
