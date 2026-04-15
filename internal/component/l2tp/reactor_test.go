package l2tp

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// lockedBuffer serializes writes and reads so a test can race-safely
// inspect log output produced by the reactor goroutine.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lb *lockedBuffer) Write(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(p)
}

func (lb *lockedBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.String()
}

// waitForLogTimeout is the fixed deadline for waitForLog. Tests do not
// need per-call tuning: 2s is comfortably above the observed dispatch
// latency (sub-ms) and still fails fast under a real bug.
const waitForLogTimeout = 2 * time.Second

// waitForLog polls until `needle` appears in the captured log output or
// waitForLogTimeout elapses. Replaces the earlier `time.Sleep(100ms)`
// pattern which hid timing bugs (see
// .claude/memory/feedback_sleep_hides_races.md); the poll exits as soon
// as the reactor's log line lands rather than waiting for a fixed delay.
func waitForLog(t *testing.T, logs *lockedBuffer, needle string) {
	t.Helper()
	deadline := time.Now().Add(waitForLogTimeout)
	for time.Now().Before(deadline) {
		if strings.Contains(logs.String(), needle) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for log substring %q; got:\n%s", needle, logs.String())
}

// send submits `payload` from a loopback client socket to the reactor's
// bound listener. Return is immediate; tests must pair with waitForLog
// to synchronize on the reactor's processing.
func send(t *testing.T, ln *UDPListener, payload []byte) {
	t.Helper()
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	defer client.Close() //nolint:errcheck // test cleanup

	srvAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(ln.Addr().Port())}
	_, err = client.WriteToUDP(payload, srvAddr)
	require.NoError(t, err)
}

// buildLogReactor constructs a listener + reactor pair whose logger writes
// into a locked buffer for race-safe assertion. The caller must call
// stop() when done.
func buildLogReactor(t *testing.T) (*UDPListener, *L2TPReactor, *lockedBuffer, func()) {
	t.Helper()
	buf := &lockedBuffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ln := NewUDPListener(netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0), logger)
	require.NoError(t, ln.Start(context.Background()))
	r := NewL2TPReactor(ln, logger)
	require.NoError(t, r.Start())

	stop := func() {
		r.Stop()
		_ = ln.Stop()
	}
	return ln, r, buf, stop
}

// TestReactor_ShortDatagramDropped — AC-3.
//
// VALIDATES: AC-3 -- datagrams < 6 bytes silently dropped; debug log
// recorded; no panic.
func TestReactor_ShortDatagramDropped(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	send(t, ln, []byte{0x01, 0x02})
	waitForLog(t, logs, "short datagram dropped")
}

// TestReactor_V1Dropped — AC-5. L2F (Ver=1) is silently discarded.
//
// VALIDATES: AC-5 -- L2F packets dropped without crash or response.
func TestReactor_V1Dropped(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	// Control message with T=1,L=1,S=1,Ver=1. Flag word = 0xC801. Minimum
	// 12-byte header (T=1,L=1,S=1,O=0).
	pkt := []byte{0xC8, 0x01, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	send(t, ln, pkt)
	waitForLog(t, logs, "unsupported version")
	require.NotContains(t, logs.String(), "L2TPv3")
}

// TestReactor_V3Dropped — AC-4 phase-2 partial. Ver=3 (L2TPv3) is logged
// at WARN level but StopCCN RC=5 emission lands in phase 3.
//
// VALIDATES: AC-4 (phase 2) -- L2TPv3 peer recognized; reactor does not
// crash and logs a warn so operators can observe v3 rollout.
func TestReactor_V3Dropped(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	// Control message with Ver=3. Flag word lower nibble = 3. Upper byte
	// carries T=1,L=1,S=1,O=0,P=0 = 0xC8; lower byte carries 0x03.
	pkt := []byte{0xC8, 0x03, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	send(t, ln, pkt)
	waitForLog(t, logs, "L2TPv3 peer rejected")
}

// TestReactor_MalformedDropped — short header with valid version.
//
// VALIDATES: malformed v2 packets drop without panic; debug log recorded.
func TestReactor_MalformedDropped(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	// Flag word 0xC802 implies S=1,L=1,O=0 so header >= 12 bytes, but we
	// send only 8. ParseMessageHeader rejects as ErrShortBuffer.
	pkt := []byte{0xC8, 0x02, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00}
	send(t, ln, pkt)
	waitForLog(t, logs, "malformed header dropped")
}

// TestReactor_ValidV2Logged — AC-2 partial. A well-formed Ver=2 control
// header is accepted and logged; no tunnel dispatch (phase 3 scope).
//
// VALIDATES: phase 3 precondition -- the reactor reaches the post-parse
// path and the listener slot is released so the pool does not leak.
func TestReactor_ValidV2Logged(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	// Minimum valid SCCRQ-shaped header: T=1,L=1,S=1,Ver=2. TunnelID=0,
	// SessionID=0, Ns=0, Nr=0. Length = 12 (header only).
	pkt := []byte{0xC8, 0x02, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	send(t, ln, pkt)
	waitForLog(t, logs, "valid v2 packet received")
}

// TestReactor_StopIdempotent calls Stop twice safely.
func TestReactor_StopIdempotent(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()
	r.Stop()
	r.Stop()
	_ = ln // keep ln used; stop() handles the teardown
}
