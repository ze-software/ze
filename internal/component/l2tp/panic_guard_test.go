package l2tp

import (
	"context"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// panicOnMessage is a slog.Handler that panics, once, on the first record whose
// message is message while armed is set. It is the injection seam of the
// contained-panic tests: the reactor and every tunnel log through the logger the
// test hands newL2TPReactor, so the panic fires inside the real dispatch, at
// the line that logs, with no hook in production code.
type panicOnMessage struct {
	slog.Handler
	message string
	armed   *atomic.Bool
}

func (h panicOnMessage) Handle(ctx context.Context, record slog.Record) error { //nolint:gocritic // slog.Handler's signature takes the record by value

	if record.Message == h.message && h.armed.CompareAndSwap(true, false) {
		panic("injected test panic at " + h.message)
	}
	return h.Handler.Handle(ctx, record)
}

func (h panicOnMessage) WithAttrs(attrs []slog.Attr) slog.Handler {
	return panicOnMessage{Handler: h.Handler.WithAttrs(attrs), message: h.message, armed: h.armed}
}

func (h panicOnMessage) WithGroup(name string) slog.Handler {
	return panicOnMessage{Handler: h.Handler.WithGroup(name), message: h.message, armed: h.armed}
}

// swapL2TPPanics replaces the L2TP panic boundary with one that logs to the
// returned buffer, and points the crash records at a temporary directory, until
// the test ends.
func swapL2TPPanics(t *testing.T) (*l2tpPanicTasks, *lockedBuffer) {
	t.Helper()
	guardLog := &lockedBuffer{}
	tasks := newL2TPPanicTasks(slog.New(slog.NewTextHandler(guardLog, nil)))
	saved := l2tpPanics
	l2tpPanics = func() *l2tpPanicTasks { return tasks }
	t.Cleanup(func() { l2tpPanics = saved })
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	return tasks, guardLog
}

// openTunnel sends an SCCRQ from c and answers the tunnel's local TID from the
// SCCRP, then confirms it with an SCCCN.
func openTunnel(t *testing.T, c *testClient, peerTID uint16, host string) uint16 {
	t.Helper()
	c.Send(t, buildSCCRQ(t, peerTID, host))
	sccrp := readDatagram(t, c)
	tid := extractAVP(t, sccrp, AVPAssignedTunnelID)
	require.Len(t, tid, 2)
	localTID := uint16(tid[0])<<8 | uint16(tid[1])
	c.Send(t, buildSCCCN(t, localTID, 1, 1, nil))
	return localTID
}

// readStopCCN reads datagrams from c until a StopCCN arrives, failing the test
// when none arrives within readDatagram's deadline.
func readStopCCN(t *testing.T, c *testClient) []byte {
	t.Helper()
	for range 8 {
		pkt := readDatagram(t, c)
		if len(pkt) > 12 && extractMsgType(pkt[12:]) == MsgStopCCN {
			return pkt
		}
	}
	t.Fatal("no StopCCN among the datagrams the reactor sent")
	return nil
}

// VALIDATES: AC-7 for L2TP. A panic inside the control dispatch of one tunnel,
// raised while handle holds tunnelsMu (the tunnel FSM's "tunnel now
// established" log line, reached from Process), is contained by the control
// task's boundary: the process stays up, that tunnel is cleared with a StopCCN
// Result Code 1 (RFC 2661 Section 4.4.2), another established tunnel keeps its
// state, the reactor still answers a new SCCRQ, and the panic is logged,
// counted and crash-recorded.
// PREVENTS: a peer's datagram that trips a Ze defect taking every subscriber
// down with the daemon, and a contained panic that leaves tunnelsMu locked.
func TestL2TPReadPanicContained(t *testing.T) {
	tasks, guardLog := swapL2TPPanics(t)

	logs := &lockedBuffer{}
	armed := &atomic.Bool{}
	logger := slog.New(panicOnMessage{
		Handler: slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}),
		message: "l2tp: tunnel now established",
		armed:   armed,
	})
	ln := newUDPListener(netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0), logger)
	require.NoError(t, ln.Start(context.Background()))
	r := newL2TPReactor(ln, logger, reactorParams{
		Defaults: TunnelDefaults{HostName: "ze-test", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	require.NoError(t, r.Start())
	defer func() {
		r.Stop()
		_ = ln.Stop()
	}()

	survivor := newClient(t, ln)
	defer survivor.Close()
	survivorTID := openTunnel(t, survivor, 201, "peer-survivor")
	waitForLog(t, logs, "tunnel now established")

	armed.Store(true)
	victim := newClient(t, ln)
	defer victim.Close()
	victimTID := openTunnel(t, victim, 202, "peer-victim")

	stop := readStopCCN(t, victim)
	result := extractAVP(t, stop, AVPResultCode)
	require.GreaterOrEqual(t, len(result), 2, "the StopCCN carries no Result Code")
	require.Equal(t, resultGeneralError, uint16(result[0])<<8|uint16(result[1]),
		"RFC 2661 Section 4.4.2: the StopCCN clears the control connection")
	require.False(t, armed.Load(), "the injected panic never fired")

	count, ok := tasks.guard.Panics("control")
	require.True(t, ok, "the control task is not declared")
	require.Equal(t, uint64(1), count, "the contained panic was not counted")
	line := guardLog.String()
	require.Contains(t, line, "an internal Ze defect ended this unit")
	require.Contains(t, line, "tunnel "+strconv.Itoa(int(victimTID)))
	require.Contains(t, line, "message-type=SCCCN")
	crashes := crashlog.ListCrashes()
	require.Len(t, crashes, 1, "the contained panic left no crash record")
	require.Equal(t, crashlog.KindRecovered, crashes[0].Kind)

	tunnel := r.tunnelByLocalID(survivorTID)
	require.NotNil(t, tunnel, "the other tunnel was removed")
	r.tunnelsMu.Lock()
	state := tunnel.state
	r.tunnelsMu.Unlock()
	require.Equal(t, L2TPTunnelEstablished, state, "the other tunnel left established")

	late := newClient(t, ln)
	defer late.Close()
	late.Send(t, buildSCCRQ(t, 203, "peer-late"))
	sccrp := readDatagram(t, late)
	require.NotNil(t, extractAVP(t, sccrp, AVPAssignedTunnelID), "the reactor stopped answering after the panic")
	require.False(t, strings.Contains(logs.String(), "panic:"), "the panic reached the reactor's own log")
}
