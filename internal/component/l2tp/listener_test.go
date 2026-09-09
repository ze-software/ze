package l2tp

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
)

// ephemeralBind returns a loopback AddrPort with an OS-chosen port.
func ephemeralBind(t *testing.T) netip.AddrPort {
	t.Helper()
	return netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0)
}

// TestListener_BindAndClose exercises the lifecycle without any I/O.
//
// VALIDATES: AC-2 (partial) -- listener binds and closes cleanly; port
// is an ephemeral value chosen by the kernel.
func TestListener_BindAndClose(t *testing.T) {
	ln := newUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	addr := ln.Addr()
	assert.NotEqual(t, uint16(0), addr.Port(), "bound port should be non-zero")
	require.NoError(t, ln.Stop())
}

// TestListener_DoubleStart rejects the second Start.
func TestListener_DoubleStart(t *testing.T) {
	ln := newUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	defer ln.Stop() //nolint:errcheck // test cleanup
	err := ln.Start(context.Background())
	require.Error(t, err)
}

// TestListener_StopIdempotent calls Stop twice.
func TestListener_StopIdempotent(t *testing.T) {
	ln := newUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	require.NoError(t, ln.Stop())
	require.NoError(t, ln.Stop())
}

// TestListener_SendReceive round-trips bytes through the listener's RX
// channel using an external UDP client. Proves the slot-pool release
// path and the Send helper.
//
// VALIDATES: AC-2 -- external client can reach the bound port.
func TestListener_SendReceive(t *testing.T) {
	ln := newUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	defer ln.Stop() //nolint:errcheck // test cleanup

	// External client socket.
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	defer client.Close() //nolint:errcheck // test cleanup

	// Client -> listener.
	srvAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(ln.Addr().Port())}
	payload := []byte{0xC8, 0x02, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err = client.WriteToUDP(payload, srvAddr)
	require.NoError(t, err)

	// Wait for packet on listener's RX.
	select {
	case pkt := <-ln.RX():
		assert.Equal(t, payload, pkt.bytes)
		pkt.release()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for packet on listener.RX()")
	}

	// Listener -> client via Send.
	clientLocal, ok := client.LocalAddr().(*net.UDPAddr)
	require.True(t, ok, "client.LocalAddr() should be *net.UDPAddr")
	peer := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(clientLocal.Port))
	require.NoError(t, ln.Send(peer, []byte{0x01, 0x02, 0x03}))

	buf := make([]byte, 16)
	require.NoError(t, client.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, _, err := client.ReadFromUDP(buf)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, buf[:n])
}

// TestListener_SendBeforeStart returns errListenerNotStarted.
func TestListener_SendBeforeStart(t *testing.T) {
	ln := newUDPListener(ephemeralBind(t), nil)
	peer := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 1)
	err := ln.Send(peer, []byte{0x00})
	require.Error(t, err)
}

// TestListenerReadLoopPacesAFailingSocket proves AC-1: a socket that
// fails on every read does not spin readLoop at full speed. It forces a
// real, persistent, non-"closed" read error by setting the listener's own
// socket deadline into the past, so every ReadFromUDPAddrPort returns an
// "i/o timeout" error at once, forever -- neither the closed branch nor a
// failure that clears on the next attempt.
//
// The discriminator is timing rather than an attempt counter, because
// readLoop's own allocation discipline (AC-8) rules out adding one just for
// this test. A pacer that never paces (a stub always returning zero, which
// is what the wiring in this phase starts from) delivers a packet sent
// once the deadline is lifted in well under a millisecond, indistinguishable
// from the busy loop this spec removes. A pacer that is actually pacing
// leaves readLoop asleep in a real wait when the packet arrives, so the
// packet is not read until that wait elapses.
func TestListenerReadLoopPacesAFailingSocket(t *testing.T) {
	ln := newUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	defer ln.Stop() //nolint:errcheck // test cleanup

	// Force every read to fail at once and persistently: a deadline already
	// in the past. Not "closed", and not recovered by the next attempt --
	// exactly the error class this spec paces.
	require.NoError(t, ln.conn.SetReadDeadline(time.Now().Add(-time.Hour)))

	// 400ms lands inside the pacer's SEVENTH wait (its growth curve --
	// 0, 10, 20, 40, 80, 160ms then the 250ms ceiling -- crosses 310ms
	// before that wait starts and 560ms after it ends), so readLoop is
	// reliably asleep in a real, ceiling-length wait when the deadline
	// below is lifted, with about 90ms of margin against scheduler jitter
	// on either side.
	time.Sleep(400 * time.Millisecond)

	// Lift the deadline and send a legitimate packet. A spinning
	// (unpaced) readLoop would already be blocked in ReadFromUDPAddrPort
	// waiting for it and would deliver it in well under a millisecond.
	require.NoError(t, ln.conn.SetReadDeadline(time.Time{}))

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	defer client.Close() //nolint:errcheck // test cleanup
	srvAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(ln.Addr().Port())}

	sent := time.Now()
	payload := []byte{0xC8, 0x02, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err = client.WriteToUDP(payload, srvAddr)
	require.NoError(t, err)

	select {
	case pkt := <-ln.RX():
		elapsed := time.Since(sent)
		assert.Equal(t, payload, pkt.bytes)
		pkt.release()
		assert.Greater(t, elapsed, 30*time.Millisecond,
			"packet delivered in %s: readLoop is not pacing its retries", elapsed)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for packet on listener.RX() after the failing socket recovered")
	}
}

// TestListenerReadLoopStopsPromptlyWhilePacing proves AC-4 on the wired
// listener: stopping it while its pacer is waiting at the ceiling must not
// sit out that wait. The pacer's own interruptible wait is proved directly
// in the pacer package (TestPacerWaitReturnsOnStop); this test proves the
// listener's wiring passes u.stop to it correctly.
func TestListenerReadLoopStopsPromptlyWhilePacing(t *testing.T) {
	ln := newUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))

	require.NoError(t, ln.conn.SetReadDeadline(time.Now().Add(-time.Hour)))
	// Long enough that the pacer has reached its ceiling wait (see the
	// timeline note in TestListenerReadLoopPacesAFailingSocket).
	time.Sleep(400 * time.Millisecond)

	start := time.Now()
	require.NoError(t, ln.Stop())
	elapsed := time.Since(start)

	// Generous on purpose, and still far below one ceiling-length wait:
	// a Stop that sat out the delay would take at least 100+ms longer.
	assert.Less(t, elapsed, 150*time.Millisecond,
		"Stop took %s while the pacer was waiting at the ceiling: the stop path waited out the delay", elapsed)
}

// readLoopAllocsPerPacket is the packet-delivery path's allocation cost
// PRE-EXISTING this phase, measured 2026-09-09. It is not zero: sending a
// value through a `select` over two channel cases -- the free-slot receive
// at the top of readLoop and the rx send at the bottom, listener.go -- costs
// the compiler's escape analysis more than an unconditional channel op
// would, independent of the pacer. Confirmed by measuring with
// u.pacer.Succeed() (the only line phase 3 added to this path) commented
// out: the count did not change, so none of it is new.
const readLoopAllocsPerPacket = 3

// TestReadLoopAllocationsPerPacketUnchanged proves AC-8: readLoop's
// packet-delivery path allocates no more per packet than it did before this
// phase wired the pacer in. pacer.Succeed, called on every successful read,
// only assigns a field (internal/core/pacer/pacer.go), so it adds nothing
// to measure here; this test guards the path as a whole against a future
// regression, not the pacer specifically.
//
// The client sends with WriteToUDPAddrPort and the assertion is over the
// whole round trip (send call included), because that is the zero-alloc
// counterpart of readLoop's own ReadFromUDPAddrPort (listener.go) -- using
// the plain net.Addr-based WriteToUDP here would measure the test client's
// own allocation, not readLoop's.
func TestReadLoopAllocationsPerPacketUnchanged(t *testing.T) {
	ln := newUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	defer ln.Stop() //nolint:errcheck // test cleanup

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	defer client.Close() //nolint:errcheck // test cleanup
	srvAddr := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), ln.Addr().Port())
	payload := []byte{0xC8, 0x02, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	sendAndDrain := func() {
		if _, sendErr := client.WriteToUDPAddrPort(payload, srvAddr); sendErr != nil {
			t.Fatal(sendErr)
		}
		select {
		case pkt := <-ln.RX():
			pkt.release()
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for packet on listener.RX()")
		}
	}

	// One untracked round trip first, so any one-time setup cost (the
	// client socket's first internal buffer, for example) does not count
	// against the measured runs.
	sendAndDrain()

	const runs = 200 // testing.AllocsPerRun truncates the average to an integer
	allocs := testing.AllocsPerRun(runs, sendAndDrain)
	assert.Equal(t, float64(readLoopAllocsPerPacket), allocs,
		"readLoop's packet-delivery path allocated %.2f per packet, want %d (unchanged from before this phase)",
		allocs, readLoopAllocsPerPacket)
}

// scrapeListenerReadErrors renders reg through the same promhttp handler an
// operator's exporter serves, and returns the
// ze_l2tp_listener_read_errors_total line, or the empty string when the
// series does not exist yet. Reading the rendered text rather than the
// collector proves an operator can see the count, the same discipline
// internal/component/l2tp/pppoe/metrics_test.go and
// internal/component/l2tp/ppp/metrics_test.go apply to their own counters.
func scrapeListenerReadErrors(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)

	const prefix = "ze_l2tp_listener_read_errors_total "
	for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

// TestReaderLoopsCountSwallowedErrors proves AC-7 for readLoop: a read
// error it swallows and retries increments
// ze_l2tp_listener_read_errors_total, the counter this phase adds because
// the pacing readLoop already had from phase 1 left it uncounted.
//
// Binds listenerMetricsPtr directly with bindListenerMetrics rather than
// through registerListenerMetrics/registry.InjectPluginMetrics: that
// process-global hook is exercised once per package by design, the same
// choice TestIdentifierRefusalCounterBindsWhenTheRegistryArrivesLast
// (ppp/metrics_test.go) and TestDiscoveryCounterBindsWhenTheRegistryArrivesLast
// (pppoe/metrics_test.go) explain for their own counters, because the
// bookkeeping is idempotent per hook name and has no reset.
func TestReaderLoopsCountSwallowedErrors(t *testing.T) {
	previous := listenerMetricsPtr.Swap(nil)
	t.Cleanup(func() { listenerMetricsPtr.Store(previous) })
	reg := metrics.NewPrometheusRegistry()
	bindListenerMetrics(reg)

	// initListenerMetrics registers the collector as soon as
	// bindListenerMetrics runs, so it scrapes at "... 0" before readLoop
	// ever sees an error: a bare non-empty check would pass whether or not
	// countReadError is ever called. The zero value is a legitimate reading
	// here (no error yet), so the assertion is the CHANGE away from it, not
	// merely the series' presence. Checked before Start, so no read has had
	// a chance to run yet.
	const zero = "ze_l2tp_listener_read_errors_total 0"
	require.Equal(t, zero, scrapeListenerReadErrors(t, reg), "counter must start at zero, before any read error")

	ln := newUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	defer ln.Stop() //nolint:errcheck // test cleanup

	// Not "closed", and not recovered by the next attempt -- exactly the
	// error class readLoop counts.
	require.NoError(t, ln.conn.SetReadDeadline(time.Now().Add(-time.Hour)))

	require.Eventually(t, func() bool {
		line := scrapeListenerReadErrors(t, reg)
		return line != "" && line != zero
	}, 500*time.Millisecond, 5*time.Millisecond, "ze_l2tp_listener_read_errors_total never rose above zero on a persistently failing socket")
}
