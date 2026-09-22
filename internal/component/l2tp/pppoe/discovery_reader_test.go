// Design: docs/architecture/l2tp/bng-5-pppoe.md -- discovery reader pacing
//
// Goal: prove discoveryReader paces and counts a read error it cannot
// classify, and that it still exits at once when Stop closes s.stop while
// it is waiting.
// Method: read an unconnected stream socket, which fails with ENOTCONN.
// Unlike EAGAIN from an idle receive timeout, this is a persistent error.
// The non-Linux socket stub returns errNotLinux and exercises the same path.

package pppoe

import (
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/metrics"
)

// openUnconnectedStreamSocket supplies a persistent receive error without
// confusing an idle datagram timeout with a socket failure.
func openUnconnectedStreamSocket(t *testing.T) int {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socket: %v", err)
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		unix.Close(fd) //nolint:errcheck // test cleanup on an already-failing path
		t.Fatalf("set nonblock: %v", err)
	}
	sa := &unix.SockaddrInet4{Port: 0}
	sa.Addr = [4]byte{127, 0, 0, 1}
	if err := unix.Bind(fd, sa); err != nil {
		unix.Close(fd) //nolint:errcheck // test cleanup on an already-failing path
		t.Fatalf("bind: %v", err)
	}
	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck // test cleanup
	return fd
}

// newDiscoveryReaderTestSubsystem builds the minimum Subsystem state
// discoveryReader needs to run as its own goroutine: a socket, and the two
// channels Start would otherwise create.
func newDiscoveryReaderTestSubsystem(discFD int) *Subsystem {
	return &Subsystem{
		logger:   slog.Default(),
		discFD:   discFD,
		readDone: make(chan struct{}),
		stop:     make(chan struct{}),
	}
}

// TestDiscoveryReaderPacesAFailingSocket
// VALIDATES: AC-1 (bounded retry rate on an unclassifiable, persistent
// read error) and AC-7 (a counter names the loop and rises once per
// swallowed error) for discoveryReader.
// PREVENTS: discoveryReader spinning at full speed on a stalled discovery
// socket, and a swallowed error going uncounted. The discriminator is the
// counter itself rather than timing: an unpaced busy loop would run this
// into the hundreds of thousands of iterations within the window below; a
// paced one stays under a few dozen.
func TestDiscoveryReaderPacesAFailingSocket(t *testing.T) {
	previousMetrics := pppoeMetricsPtr.Swap(nil)
	t.Cleanup(func() { pppoeMetricsPtr.Store(previousMetrics) })
	reg := metrics.NewPrometheusRegistry()
	bindPPPoEMetrics(reg)

	sub := newDiscoveryReaderTestSubsystem(openUnconnectedStreamSocket(t))
	go sub.discoveryReader()
	t.Cleanup(func() {
		close(sub.stop)
		<-sub.readDone
	})

	// Long enough for several paced retries (growth 0, 10, 20, 40, 80,
	// 160ms then the 250ms ceiling), short of the ceiling itself.
	time.Sleep(200 * time.Millisecond)

	got := scrapeDiscoveryReadErrors(t, reg)
	if got == "" {
		t.Fatal("ze_pppoe_discovery_read_errors_total never appeared on a persistently failing socket")
	}
	const prefix = "ze_pppoe_discovery_read_errors_total "
	value, err := strconv.ParseFloat(strings.TrimPrefix(got, prefix), 64)
	if err != nil {
		t.Fatalf("scraped %q, could not parse the count: %v", got, err)
	}
	// A generous bound: the growth curve above predicts about 5 or 6
	// attempts in 200ms. 50 is an order of magnitude above that and
	// still four orders of magnitude below what an unpaced EAGAIN loop
	// would produce in the same window.
	if value >= 50 {
		t.Fatalf("scraped %q (%v swallowed errors), want fewer than 50 in 200ms of pacing", got, value)
	}
}

// TestDiscoveryReaderStopsPromptlyWhilePacing
// VALIDATES: AC-4's analog for discoveryReader -- stopping it while its
// pacer is waiting at the ceiling must not sit out that wait.
// PREVENTS: waiting for the error pacer's delay after cancellation.
func TestDiscoveryReaderStopsPromptlyWhilePacing(t *testing.T) {
	sub := newDiscoveryReaderTestSubsystem(openUnconnectedStreamSocket(t))
	go sub.discoveryReader()

	// Long enough that the pacer has reached its ceiling wait (see the
	// growth curve note in TestDiscoveryReaderPacesAFailingSocket).
	time.Sleep(400 * time.Millisecond)

	start := time.Now()
	close(sub.stop)
	<-sub.readDone
	elapsed := time.Since(start)

	// Generous on purpose, and still far below one ceiling-length wait:
	// a stop that sat out the delay would take at least 100+ms longer.
	if elapsed > 150*time.Millisecond {
		t.Fatalf("discoveryReader took %s to stop while pacing at the ceiling: it sat out the delay", elapsed)
	}
}
