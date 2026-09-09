// Design: docs/research/l2tpv2-ze-integration.md -- receiver goroutines and a failing socket
//
// Goal: prove dhcpv6ServerLoop paces a read error it cannot classify and
// counts it, the same way TestDiscoveryReaderPacesAFailingSocket
// (internal/component/l2tp/pppoe/discovery_reader_test.go) proves both for
// discoveryReader.
// Method: force a real, persistent, non-context-done error by setting the
// socket's own read deadline into the past. dhcpv6ServerLoop gives the
// reader no external success signal to time against (no channel push, and
// a too-short packet stops at the length check before touching svc), so
// the discriminator here is ze_ppp_reader_errors_total{loop="dhcpv6"}
// itself: a busy loop would run it into the hundreds of thousands within
// the window below, a paced one stays under a few dozen.

//go:build linux

package ppp

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

// scrapeDHCPv6ReaderErrors renders reg through the same promhttp handler an
// operator's exporter serves, and returns the
// ze_ppp_reader_errors_total{loop="dhcpv6"} line, or the empty string when
// the series does not exist yet.
func scrapeDHCPv6ReaderErrors(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)

	const prefix = `ze_ppp_reader_errors_total{loop="dhcpv6"} `
	for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

// TestDHCPv6ServerLoopPacesAFailingSocket
// VALIDATES: AC-1 and AC-7 for dhcpv6ServerLoop: a socket that fails on
// every read does not spin the goroutine at full speed, and each swallowed
// error is counted on ze_ppp_reader_errors_total{loop="dhcpv6"}.
// PREVENTS: dhcpv6ServerLoop retrying an unclassified read error at once,
// forever -- this loop already logged the error at Debug before this
// phase, so it is proof the log line alone was never the fix.
func TestDHCPv6ServerLoopPacesAFailingSocket(t *testing.T) {
	previous := pppMetricsPtr.Swap(nil)
	t.Cleanup(func() { pppMetricsPtr.Store(previous) })
	reg := metrics.NewPrometheusRegistry()
	bindPPPMetrics(reg)

	udpConn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.ParseIP("::1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer udpConn.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Force every read to fail at once and persistently: a deadline
	// already in the past. Not context-done, and not recovered by the
	// next attempt.
	if err := udpConn.SetReadDeadline(time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	svc := newIPv6Service(iPv6ServiceConfig{})
	go dhcpv6ServerLoop(ctx, udpConn, svc, DHCPv6DUID{}, nil, "test0", slog.Default())

	// Long enough for several paced retries (growth 0, 10, 20, 40, 80,
	// 160ms then the 250ms ceiling), short of the ceiling itself.
	time.Sleep(200 * time.Millisecond)

	got := scrapeDHCPv6ReaderErrors(t, reg)
	if got == "" {
		t.Fatal(`ze_ppp_reader_errors_total{loop="dhcpv6"} never appeared on a persistently failing socket`)
	}
	const prefix = `ze_ppp_reader_errors_total{loop="dhcpv6"} `
	value, parseErr := strconv.ParseFloat(strings.TrimPrefix(got, prefix), 64)
	if parseErr != nil {
		t.Fatalf("scraped %q, could not parse the count: %v", got, parseErr)
	}
	// A generous bound: the growth curve above predicts about 5 or 6
	// attempts in 200ms. 50 is an order of magnitude above that and
	// still four orders of magnitude below what an unpaced spin would
	// produce in the same window.
	if value >= 50 {
		t.Fatalf("scraped %q (%v swallowed errors), want fewer than 50 in 200ms of pacing", got, value)
	}
}
