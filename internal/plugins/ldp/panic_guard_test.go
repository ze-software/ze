// Design: docs/architecture/ldp/mpls-ldp.md -- panic containment at the session and discovery tasks
// Related: panic_guard.go -- the guard, its tasks and the session's Internal Error end
// Related: rfc5443_sessionup_test.go -- exchangeInitialization, encodeKeepAlivePDU, sessionPipe
// Related: rfc5036_test.go -- rfcTestSession, readLDPPDU, encodeHelloPDU
//
// A Ze defect a peer's input reaches ends that peer's session, or drops that
// Hello, and leaves the plugin and every other session running. The panic is
// injected through the plugin's event bus (ze.EventBus, the seam
// recordSessionUps already swaps), which runSession calls from its ReadLoop
// goroutine, and through discovery's onAdjHello callback, which
// readDiscoveryLoop calls for every Hello it applies.
package ldp

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

// panickingBus panics on the SessionUp of one LDP identifier and records every
// other SessionUp. Safe for concurrent use: each runSession emits from its own
// goroutine.
type panickingBus struct {
	victim string

	mu  sync.Mutex
	ups []string
	// arrived carries one token per recorded SessionUp.
	arrived chan struct{}
}

func (b *panickingBus) Emit(namespace, eventType string, payload any) (int, error) {
	if namespace != Namespace || eventType != EventSessionUp {
		return 0, nil
	}
	evt, ok := payload.(*SessionEvent)
	if !ok {
		return 0, nil
	}
	if evt.LDPIdentifier == b.victim {
		panic("injected: SessionUp handler defect")
	}
	b.mu.Lock()
	b.ups = append(b.ups, evt.LDPIdentifier)
	b.mu.Unlock()
	b.arrived <- struct{}{}
	return 0, nil
}

func (b *panickingBus) Subscribe(string, string, func(any)) func() { return func() {} }

var _ ze.EventBus = (*panickingBus)(nil)

// startGuardedSession runs runSession, the function startSessionForAdj starts
// for a dialed adjacency, and returns a channel closed when it ends.
func startGuardedSession(t *testing.T, sess *Session, key string) <-chan struct{} {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go runSession(ctx, slogutil.DiscardLogger(), sess, sess.lib, key,
		newLDPFIB(nil, slogutil.DiscardLogger()), &SessionEvent{LDPIdentifier: key},
		func() { close(done) })
	t.Cleanup(func() {
		cancel()
		sess.Stop()
		<-done
	})
	return done
}

// guardEvidence snapshots what a contained panic leaves: the task's count, the
// guard's ERROR lines in the log ring, and the crash records.
type guardEvidence struct {
	panics uint64
	lines  int
}

func snapshotGuard(t *testing.T, task string) guardEvidence {
	t.Helper()
	count, declared := ldpPanics().guard.Panics(task)
	if !declared {
		t.Fatalf("task %q is not declared on the ldp guard", task)
	}
	entries, err := slogutil.GlobalLogRing().Snapshot(0, "ERROR", "ldp")
	if err != nil {
		t.Fatalf("log ring snapshot: %v", err)
	}
	lines := 0
	for _, e := range entries {
		if strings.Contains(e.Message, "an internal Ze defect ended this unit") {
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
		t.Errorf("ldp %s panics = %d, want %d: the panic was not counted", task, after.panics, before.panics+1)
	}
	if after.lines != before.lines+1 {
		t.Errorf("guard ERROR lines in the log ring = %d, want %d: the panic was not logged", after.lines, before.lines+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
}

// TestLDPReadPanicContained: a panic on session A's read path (its SessionUp
// handler, reached from the peer's KeepAlive) sends A's peer an Internal Error
// Notification and ends A's runSession, while session B, set up afterwards on
// the same plugin, reaches OPERATIONAL and stays up.
func TestLDPReadPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	bus := &panickingBus{victim: "peer-a", arrived: make(chan struct{}, 4)}
	prev := eventBusPtr.Load()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(prev) })
	before := snapshotGuard(t, taskSession)

	localA, remoteA := sessionPipe(t)
	doneA := startGuardedSession(t, rfcTestSession(localA), "peer-a")
	localB, remoteB := sessionPipe(t)
	sessB := rfcTestSession(localB)
	doneB := startGuardedSession(t, sessB, "peer-b")

	exchangeInitialization(t, remoteA)
	if _, err := remoteA.Write(encodeKeepAlivePDU()); err != nil {
		t.Fatalf("write peer A KeepAlive: %v", err)
	}
	_, hdr, body := readLDPPDU(t, remoteA)
	if hdr.Type != MsgTypeNotification {
		t.Fatalf("PDU after the panic = %#x, want Notification (%#x)", hdr.Type, MsgTypeNotification)
	}
	if status := binary.BigEndian.Uint32(body[4:8]); status != statusInternalError {
		t.Fatalf("Notification status = %#x, want Internal Error %#x", status, statusInternalError)
	}
	select {
	case <-doneA:
	case <-time.After(2 * time.Second):
		t.Fatal("session A's runSession did not end after its contained panic")
	}
	requireContained(t, taskSession, before)

	exchangeInitialization(t, remoteB)
	if _, err := remoteB.Write(encodeKeepAlivePDU()); err != nil {
		t.Fatalf("write peer B KeepAlive: %v", err)
	}
	select {
	case <-bus.arrived:
	case <-time.After(2 * time.Second):
		t.Fatal("session B published no SessionUp after session A's panic")
	}
	expectSilence(t, remoteB, 300*time.Millisecond, "session B stays up with nothing to send")
	select {
	case <-doneB:
		t.Fatal("session B ended: the panic of session A reached it")
	default:
	}
	if sessB.State() != StateOperational {
		t.Errorf("session B state = %s, want operational", sessB.State())
	}
}

// TestLDPDiscoveryPanicContained: a panic while applying one Hello drops that
// Hello and leaves discovery reading the same socket, so the next neighbor's
// Hello still creates its adjacency.
func TestLDPDiscoveryPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	before := snapshotGuard(t, taskDiscovery)

	recvConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = recvConn.Close() })
	recvAddr, ok := recvConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("receive address %v is not UDP", recvConn.LocalAddr())
	}
	sendConn, err := net.DialUDP("udp4", nil, recvAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = sendConn.Close() })

	victim := [4]byte{10, 0, 0, 7}
	survivor := [4]byte{10, 0, 0, 8}
	applied := make(chan [4]byte, 4)
	onAdjHello := func(adj *Adjacency) {
		if adj.PeerLSRID == victim {
			panic("injected: adjacency handler defect")
		}
		applied <- adj.PeerLSRID
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	adjTable := newAdjacencyTable()
	go readDiscoveryLoop(ctx, recvConn, [4]byte{10, 0, 0, 1}, "", adjTable, onAdjHello, slogutil.DiscardLogger(), done)
	t.Cleanup(func() {
		cancel()
		_ = recvConn.Close()
		<-done
	})

	if _, err := sendConn.Write(encodeHelloPDU(victim, 15, 0)); err != nil {
		t.Fatalf("send victim Hello: %v", err)
	}
	if _, err := sendConn.Write(encodeHelloPDU(survivor, 15, 0)); err != nil {
		t.Fatalf("send survivor Hello: %v", err)
	}
	select {
	case got := <-applied:
		if got != survivor {
			t.Fatalf("applied Hello from %v, want %v", got, survivor)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the Hello after the contained panic was never applied: discovery stopped reading")
	}
	select {
	case <-done:
		t.Fatal("readDiscoveryLoop ended after a contained panic")
	default:
	}
	requireContained(t, taskDiscovery, before)
}
