package engine

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/crashlog"
)

// panicOnRecord is a slog.Handler that panics, once, on the first record whose
// message is message while armed is set. It is the injection seam of the
// contained-panic tests: the engine logs through the logger the test hands it,
// so the panic fires inside the real dispatch or owner loop, at the line that
// logs, with no hook in production code.
type panicOnRecord struct {
	message string
	armed   *atomic.Bool
}

func (h panicOnRecord) Enabled(context.Context, slog.Level) bool { return true }

func (h panicOnRecord) Handle(_ context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler fixes the Record parameter by value
	if r.Message == h.message && h.armed.CompareAndSwap(true, false) {
		panic("injected test panic at " + r.Message)
	}
	return nil
}

func (h panicOnRecord) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h panicOnRecord) WithGroup(string) slog.Handler      { return h }

// panicLogger answers a logger that panics once on message, and the flag that
// says whether it has fired yet (false once it has).
func panicLogger(message string) (*slog.Logger, *atomic.Bool) {
	armed := &atomic.Bool{}
	armed.Store(true)
	return slog.New(panicOnRecord{message: message, armed: armed}), armed
}

// ikeGuardLog is the guard's log sink, written by the engine's goroutines and
// read by the test, so every access holds the lock.
type ikeGuardLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *ikeGuardLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *ikeGuardLog) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// swapIKEPanics replaces the IKE panic boundary with one that logs to the
// returned buffer, and points the crash records at a temporary directory, until
// the test ends.
func swapIKEPanics(t *testing.T) (*ikePanicTasks, *ikeGuardLog) {
	t.Helper()
	guardLog := &ikeGuardLog{}
	tasks := newIKEPanicTasks(slog.New(slog.NewTextHandler(guardLog, nil)))
	saved := ikePanics
	ikePanics = func() *ikePanicTasks { return tasks }
	t.Cleanup(func() { ikePanics = saved })
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	return tasks, guardLog
}

// requireContained checks the guard counted one panic on task, logged it with
// each of wants, and wrote one recovered crash record.
func requireContained(t *testing.T, tasks *ikePanicTasks, guardLog *ikeGuardLog, task string, wants ...string) {
	t.Helper()
	count, ok := tasks.guard.Panics(task)
	if !ok || count != 1 {
		t.Fatalf("guard count for ike/%s = %d (declared %v), want 1", task, count, ok)
	}
	line := guardLog.String()
	for _, want := range append([]string{"component=ike", "task=" + task}, wants...) {
		if !strings.Contains(line, want) {
			t.Fatalf("guard log line lacks %q:\n%s", want, line)
		}
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one of kind %q", crashes, crashlog.KindRecovered)
	}
}

// TestIKEInboundPanicContained
// VALIDATES: a panic while the shared dispatch goroutine handles one datagram is
// contained at deliverDatagram: that datagram is dropped, the next datagram is
// still delivered to its SA, and the panic is logged, counted and crash-recorded.
// METHOD: the real dispatchInbound over loopback UDP, as
// TestRFC7296ZeroInitiatorSPIIsDroppedOnReceipt drives it. The first datagram
// names no SA, so dispatch logs "ike: no SA for packet", where the injected
// logger panics; the second names a live SA, and its arrival in that session's
// inbound queue proves the goroutine survived (loopback keeps the order).
// PREVENTS: one malformed datagram ending IKE on the socket for every peer.
func TestIKEInboundPanicContained(t *testing.T) {
	tasks, guardLog := swapIKEPanics(t)
	log, armed := panicLogger("ike: no SA for packet")
	tr, err := transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	go tr.Run()

	table := NewSATable()
	liveSA := testSA()
	liveSA.InitiatorSPI = [8]byte{9, 9, 9, 9, 9, 9, 9, 9}
	liveSA.ResponderSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	liveSA.PeerName = "survivor"
	table.Insert(liveSA)
	livePS := &PeerSession{peerName: liveSA.PeerName, inbound: make(chan transport.Packet, 4)}
	livePS.ownedSA.Store(liveSA)
	SetActivePeersForTest(map[string]*PeerSession{liveSA.PeerName: livePS})
	t.Cleanup(func() { SetActivePeersForTest(nil) })

	go dispatchInbound(tr, table, log)

	local, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("transport LocalAddr is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

	datagram := func(initiatorSPI, responderSPI [8]byte) []byte {
		msg := wire.Message{Header: wire.Header{
			InitiatorSPI: initiatorSPI,
			ResponderSPI: responderSPI,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeInformational,
			Flags:        wire.FlagInitiator,
			MessageID:    7,
		}}
		buf := make([]byte, 512)
		n := msg.WriteTo(buf, 0)
		return buf[:n]
	}
	if _, err := sender.Write(datagram([8]byte{7, 7, 7, 7, 7, 7, 7, 7}, [8]byte{6, 6, 6, 6, 6, 6, 6, 6})); err != nil {
		t.Fatalf("Write the victim datagram: %v", err)
	}
	if _, err := sender.Write(datagram(liveSA.InitiatorSPI, liveSA.ResponderSPI)); err != nil {
		t.Fatalf("Write the survivor datagram: %v", err)
	}

	select {
	case <-livePS.inbound:
	case <-time.After(5 * time.Second):
		t.Fatal("the datagram after the panic was never delivered: the dispatch goroutine did not survive")
	}
	if armed.Load() {
		t.Fatal("the injected panic never fired, so containment was not exercised")
	}
	requireContained(t, tasks, guardLog, "inbound", "ispi 0707070707070707", `message-type="exchange type 37"`)
}

// guardedLoop runs maintainSA for sa with log, and answers the channel its
// error arrives on. No DPD and no lifetimes run, so the loop sends nothing on
// its own and ends only through its inbound queue or ps.stopCh.
func guardedLoop(t *testing.T, sa *SA, ps *PeerSession, myTr *transport.UDPTransport, log *slog.Logger) chan error {
	t.Helper()
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.inbound = make(chan transport.Packet, inboundQueueDepth)
	done := make(chan error, 1)
	exited := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-ps.stopCh:
		default:
			close(ps.stopCh)
		}
		<-exited
	})
	go func() {
		defer close(exited)
		done <- ps.maintainSA(sa, nil, nil, nil, testIKEGroup(), NewSATable(), nil, myTr, &sepBus{}, log)
	}()
	return done
}

// TestIKEEstablishedPanicSendsDelete
// VALIDATES: a panic while an established SA's owner loop handles one message is
// contained at handleOwnedInboundGuarded: the loop takes the re-establish exit,
// so the peer is sent an INFORMATIONAL Delete of the IKE SA (RFC 7296 Section
// 2.4) and the loop answers errTimeout, the error that sets the SA up again; a
// second session's owner loop keeps running; the panic is logged, counted and
// crash-recorded.
// METHOD: two established PSK rigs. The victim's loop runs with a logger that
// panics on "ike: owned inbound parse error", which a datagram too short to
// parse draws inside handleOwnedInbound. The other rig is built first, because
// each rig points the engine's test port at its own peer socket.
// PREVENTS: one defect in an SA's handling ending every IKE SA, or an SA
// abandoned without the Delete its peer is owed.
func TestIKEEstablishedPanicSendsDelete(t *testing.T) {
	tasks, guardLog := swapIKEPanics(t)
	other := newRefusedRig(t)
	victim := newRefusedRig(t)

	otherDone := guardedLoop(t, other.sa, other.ps, other.myTr, slog.New(slog.DiscardHandler))
	log, armed := panicLogger("ike: owned inbound parse error")
	victimDone := guardedLoop(t, victim.sa, victim.ps, victim.myTr, log)

	victim.ps.inbound <- transport.Packet{Data: []byte{1, 2, 3}, RemoteAddr: victim.sa.remoteUDPAddr()}
	if err := clfLoopEnd(t, victimDone, "a contained panic"); !errors.Is(err, errTimeout) {
		t.Fatalf("a contained panic ended the owner loop with %v, want errTimeout (re-establish)", err)
	}
	if armed.Load() {
		t.Fatal("the injected panic never fired, so containment was not exercised")
	}

	sawDelete := false
	for raw := rtxRecv(t, victim.peerTr); raw != nil && !sawDelete; raw = rtxRecv(t, victim.peerTr) {
		hdr := parseMsg(t, raw).Header
		if hdr.ExchangeType != wire.ExchangeInformational || hdr.Flags&wire.FlagResponse != 0 {
			continue
		}
		for _, del := range lcyDeletes(lcyDecrypt(t, victim.peer, raw)) {
			if del.ProtocolID == wire.ProtocolIKE {
				sawDelete = true
			}
		}
	}
	if !sawDelete {
		t.Fatal("the owner loop ended the IKE SA after a contained panic and sent the peer no Delete for it")
	}

	select {
	case err := <-otherDone:
		t.Fatalf("the other session's owner loop ended with %v after the victim's panic", err)
	default:
	}
	close(other.ps.stopCh)
	if err := clfLoopEnd(t, otherDone, "an operator stop"); err != nil {
		t.Fatalf("an operator stop ended the other session's loop with %v, want nil", err)
	}

	requireContained(t, tasks, guardLog, "established", "IKE SA of peer "+victim.ps.peerName)
}
