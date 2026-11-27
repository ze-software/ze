package traffic

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/sim"
)

const (
	testIfaceEth0    = "eth0"
	testIfaceEth1    = "eth1"
	testQdiscHTB     = "htb"
	testQdiscFQCodel = "fq_codel"
	testHandleRoot   = "1:0"
	testHandleCls1   = "1:1"
	testHandleCls2   = "1:2"
)

func scrapeMetricsBody(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	return rec.Body.String()
}

func assertContainsAll(t *testing.T, body string, want []string) {
	t.Helper()
	for _, line := range want {
		if !strings.Contains(body, line) {
			t.Errorf("metrics output missing %q\nfull body:\n%s", line, body)
		}
	}
}

func awaitRead(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for poller tick collection")
	}
}

// TestQdiscMetricsEndToEndHTTPScrape verifies all 14 ze_traffic_qdisc_* and
// ze_traffic_class_* series appear on HTTP /metrics immediately after startup,
// update on fake-clock ticks, and are wired through metrics.StartPushCollectors.
//
// VALIDATES: AC-5 (all 7 qdisc + 7 class GaugeVec series exported over HTTP
// /metrics with exact labels and values, updated every tick, registered via
// metrics.RegisterPushCollector).
// PREVENTS: missing metric series, label mismatches, or unwired push collector.
func TestQdiscMetricsEndToEndHTTPScrape(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	clk := sim.NewFakeClock(time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC))

	var mu sync.Mutex
	current := []InterfaceQoS{
		{
			Interface: testIfaceEth0,
			Qdisc: Qdisc{
				Type:   QdiscHTB,
				Kind:   testQdiscHTB,
				Handle: testHandleRoot,
				Stats: &QdiscStats{
					Bytes:      1024,
					Packets:    10,
					Qlen:       2,
					Backlog:    512,
					Drops:      3,
					Overlimits: 4,
					Requeues:   1,
				},
				Classes: []TrafficClass{
					{
						Name:   "control",
						Handle: testHandleCls1,
						Stats: &QdiscStats{
							Bytes:      2048,
							Packets:    20,
							Qlen:       5,
							Backlog:    768,
							Drops:      6,
							Overlimits: 7,
							Requeues:   2,
						},
					},
				},
			},
		},
	}

	collected := make(chan struct{}, 4)
	readAll := func() ([]InterfaceQoS, error) {
		mu.Lock()
		snap := current
		mu.Unlock()
		select {
		case collected <- struct{}{}:
		default:
		}
		return snap, nil
	}

	stop := startQdiscMetricsPollerWith(reg, nil, clk, readAll)
	t.Cleanup(stop)
	awaitRead(t, collected)

	body := scrapeMetricsBody(t, reg)
	assertContainsAll(t, body, []string{
		`ze_traffic_qdisc_bytes_total{handle="1:0",interface="eth0",qdisc="htb"} 1024`,
		`ze_traffic_qdisc_packets_total{handle="1:0",interface="eth0",qdisc="htb"} 10`,
		`ze_traffic_qdisc_qlen{handle="1:0",interface="eth0",qdisc="htb"} 2`,
		`ze_traffic_qdisc_backlog_bytes{handle="1:0",interface="eth0",qdisc="htb"} 512`,
		`ze_traffic_qdisc_drops_total{handle="1:0",interface="eth0",qdisc="htb"} 3`,
		`ze_traffic_qdisc_overlimits_total{handle="1:0",interface="eth0",qdisc="htb"} 4`,
		`ze_traffic_qdisc_requeues_total{handle="1:0",interface="eth0",qdisc="htb"} 1`,
		`ze_traffic_class_bytes_total{class="control",handle="1:1",interface="eth0",qdisc="htb"} 2048`,
		`ze_traffic_class_packets_total{class="control",handle="1:1",interface="eth0",qdisc="htb"} 20`,
		`ze_traffic_class_qlen{class="control",handle="1:1",interface="eth0",qdisc="htb"} 5`,
		`ze_traffic_class_backlog_bytes{class="control",handle="1:1",interface="eth0",qdisc="htb"} 768`,
		`ze_traffic_class_drops_total{class="control",handle="1:1",interface="eth0",qdisc="htb"} 6`,
		`ze_traffic_class_overlimits_total{class="control",handle="1:1",interface="eth0",qdisc="htb"} 7`,
		`ze_traffic_class_requeues_total{class="control",handle="1:1",interface="eth0",qdisc="htb"} 2`,
	})

	// Advance sample and fire one fake-clock tick.
	mu.Lock()
	current = []InterfaceQoS{
		{
			Interface: testIfaceEth0,
			Qdisc: Qdisc{
				Type:   QdiscHTB,
				Kind:   testQdiscHTB,
				Handle: testHandleRoot,
				Stats: &QdiscStats{
					Bytes:      4096,
					Packets:    40,
					Qlen:       0,
					Backlog:    0,
					Drops:      9,
					Overlimits: 11,
					Requeues:   3,
				},
				Classes: []TrafficClass{
					{
						Name:   "control",
						Handle: testHandleCls1,
						Stats: &QdiscStats{
							Bytes:      8192,
							Packets:    80,
							Qlen:       1,
							Backlog:    128,
							Drops:      12,
							Overlimits: 14,
							Requeues:   4,
						},
					},
				},
			},
		},
	}
	mu.Unlock()

	clk.FireTickers()
	awaitRead(t, collected)
	stop()

	body = scrapeMetricsBody(t, reg)
	assertContainsAll(t, body, []string{
		`ze_traffic_qdisc_bytes_total{handle="1:0",interface="eth0",qdisc="htb"} 4096`,
		`ze_traffic_qdisc_drops_total{handle="1:0",interface="eth0",qdisc="htb"} 9`,
		`ze_traffic_class_bytes_total{class="control",handle="1:1",interface="eth0",qdisc="htb"} 8192`,
		`ze_traffic_class_overlimits_total{class="control",handle="1:1",interface="eth0",qdisc="htb"} 14`,
	})

	// Verify the package init() registration wires through metrics.StartPushCollectors.
	t.Cleanup(ResetBackendsForTest())
	fb := &fakeBulkBackend{
		all: []InterfaceQoS{
			{
				Interface: testIfaceEth1,
				Qdisc: Qdisc{
					Kind:   testQdiscFQCodel,
					Handle: "0:0",
					Stats:  &QdiscStats{Bytes: 555, Packets: 5, Drops: 1},
				},
			},
		},
	}
	if err := RegisterBackend("metrics-wired", func() (Backend, error) { return fb, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := LoadBackend("metrics-wired"); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}

	wiredReg := metrics.NewPrometheusRegistry()
	stopWired := metrics.StartPushCollectors(wiredReg, nil)
	t.Cleanup(stopWired)

	wiredBody := scrapeMetricsBody(t, wiredReg)
	assertContainsAll(t, wiredBody, []string{
		`ze_traffic_qdisc_bytes_total{handle="0:0",interface="eth1",qdisc="fq_codel"} 555`,
		`ze_traffic_qdisc_packets_total{handle="0:0",interface="eth1",qdisc="fq_codel"} 5`,
		`ze_traffic_qdisc_drops_total{handle="0:0",interface="eth1",qdisc="fq_codel"} 1`,
	})
}

// TestQdiscMetricsStaleLabelCleanupAndReset verifies that vanished interfaces
// and classes are deleted on the next tick and counter decreases (qdisc
// replacement or uint32 wrap) update cleanly to the new lower values.
//
// VALIDATES: AC-6 (stale label deletion and non-monotonic kernel counter reset).
// PREVENTS: unbounded Prometheus label cardinality on dynamic links or counter
// decrease panics on qdisc replace.
func TestQdiscMetricsStaleLabelCleanupAndReset(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	clk := sim.NewFakeClock(time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC))

	var mu sync.Mutex
	current := []InterfaceQoS{
		{
			Interface: testIfaceEth0,
			Qdisc: Qdisc{
				Kind:   testQdiscHTB,
				Handle: testHandleRoot,
				Stats: &QdiscStats{
					Bytes: 9000, Packets: 90, Qlen: 4, Backlog: 400, Drops: 50, Overlimits: 20, Requeues: 7,
				},
				Classes: []TrafficClass{
					{
						Name:   "control",
						Handle: testHandleCls1,
						Stats:  &QdiscStats{Bytes: 3000, Packets: 30, Drops: 10},
					},
					{
						Name:   "default",
						Handle: testHandleCls2,
						Stats:  &QdiscStats{Bytes: 6000, Packets: 60, Drops: 40},
					},
				},
			},
		},
		{
			Interface: testIfaceEth1,
			Qdisc: Qdisc{
				Kind:   testQdiscFQCodel,
				Handle: "0:0",
				Stats:  &QdiscStats{Bytes: 1200, Packets: 12, Drops: 2},
			},
		},
	}

	collected := make(chan struct{}, 4)
	stop := startQdiscMetricsPollerWith(reg, nil, clk, func() ([]InterfaceQoS, error) {
		mu.Lock()
		snap := current
		mu.Unlock()
		select {
		case collected <- struct{}{}:
		default:
		}
		return snap, nil
	})
	t.Cleanup(stop)
	awaitRead(t, collected)

	body := scrapeMetricsBody(t, reg)
	assertContainsAll(t, body, []string{
		`ze_traffic_qdisc_bytes_total{handle="0:0",interface="eth1",qdisc="fq_codel"} 1200`,
		`ze_traffic_class_bytes_total{class="control",handle="1:1",interface="eth0",qdisc="htb"} 3000`,
	})

	// Drop eth1, drop class control, and reset eth0 + default counters downward.
	mu.Lock()
	current = []InterfaceQoS{
		{
			Interface: testIfaceEth0,
			Qdisc: Qdisc{
				Kind:   testQdiscHTB,
				Handle: testHandleRoot,
				Stats: &QdiscStats{
					Bytes: 128, Packets: 2, Qlen: 0, Backlog: 0, Drops: 1, Overlimits: 0, Requeues: 0,
				},
				Classes: []TrafficClass{
					{
						Name:   "default",
						Handle: testHandleCls2,
						Stats:  &QdiscStats{Bytes: 128, Packets: 2, Drops: 1},
					},
				},
			},
		},
	}
	mu.Unlock()

	clk.FireTickers()
	awaitRead(t, collected)
	stop()

	body = scrapeMetricsBody(t, reg)
	if strings.Contains(body, `interface="eth1"`) {
		t.Fatalf("stale interface eth1 still present in /metrics:\n%s", body)
	}
	if strings.Contains(body, `class="control"`) {
		t.Fatalf("stale class control still present in /metrics:\n%s", body)
	}
	assertContainsAll(t, body, []string{
		`ze_traffic_qdisc_bytes_total{handle="1:0",interface="eth0",qdisc="htb"} 128`,
		`ze_traffic_qdisc_drops_total{handle="1:0",interface="eth0",qdisc="htb"} 1`,
		`ze_traffic_class_bytes_total{class="default",handle="1:2",interface="eth0",qdisc="htb"} 128`,
		`ze_traffic_class_drops_total{class="default",handle="1:2",interface="eth0",qdisc="htb"} 1`,
	})
}

// TestQdiscMetricsOmitsNilStatsAndLifecycleStop verifies that nil Stats (such
// as VPP) and ErrNoBackendAvailable emit zero ze_traffic_qdisc_* /
// ze_traffic_class_* series without WARN spam, and stop() is idempotent and
// cleanly joins the worker goroutine under -race.
//
// VALIDATES: AC-4/AC-5 (nil-stats omission) and goroutine lifecycle shutdown.
// PREVENTS: fabricated zero metrics on VPP, non-Linux WARN log spam, or leaked
// poller goroutines.
func TestQdiscMetricsOmitsNilStatsAndLifecycleStop(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	clk := sim.NewFakeClock(time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC))

	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	vppData := []InterfaceQoS{
		{
			Interface: testIfaceEth0,
			Qdisc: Qdisc{
				Type: QdiscHTB,
				Classes: []TrafficClass{
					{Name: "fast", Rate: 10_000_000, Ceil: 20_000_000},
				},
			},
		},
	}

	stop := startQdiscMetricsPollerWith(reg, log, clk, func() ([]InterfaceQoS, error) {
		return vppData, nil
	})

	body := scrapeMetricsBody(t, reg)
	if strings.Contains(body, "ze_traffic_qdisc_") || strings.Contains(body, "ze_traffic_class_") {
		t.Fatalf("expected zero traffic metrics for nil Stats, got:\n%s", body)
	}

	// Concurrent and repeated stop calls must succeed cleanly under -race.
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(stop)
	}
	wg.Wait()
	stop()

	// ErrNoBackendAvailable emits no series and logs no WARN.
	noBackendReg := metrics.NewPrometheusRegistry()
	stopNoBackend := startQdiscMetricsPollerWith(noBackendReg, log, clk, func() ([]InterfaceQoS, error) {
		return nil, ErrNoBackendAvailable
	})
	stopNoBackend()

	if body := scrapeMetricsBody(t, noBackendReg); strings.Contains(body, "ze_traffic_") {
		t.Fatalf("expected zero traffic metrics on ErrNoBackendAvailable, got:\n%s", body)
	}
	if logBuf.Len() != 0 {
		t.Fatalf("expected no WARN logs on nil stats / ErrNoBackendAvailable, got %q", logBuf.String())
	}
}

func hasQdiscReadWarning() bool {
	warnings := report.Warnings()
	for i := range warnings {
		w := &warnings[i]
		if w.Source == trafficReportSource && w.Code == codeDoctorTrafficQdiscMetricsReadFailed && w.Subject == qdiscMetricsCollectorName {
			return true
		}
	}
	return false
}

// TestQdiscMetricsReadErrorDoctorCodeAndRateLimit verifies that read errors
// carry the registered doctor code, raise and clear a report-bus warning, and
// rate-limit repeated WARN logs to once per qdiscMetricsWarnInterval while
// resetting on recovery.
//
// VALIDATES: doctor-coverage and operator error reporting on the 1s metrics
// poller loop.
// PREVENTS: uncovered WARN logs without a doctor code, 1 Hz log spam on
// persistent netlink errors, or stuck report-bus warnings after recovery.
func TestQdiscMetricsReadErrorDoctorCodeAndRateLimit(t *testing.T) {
	t.Cleanup(func() {
		report.ClearWarning(trafficReportSource, codeDoctorTrafficQdiscMetricsReadFailed, qdiscMetricsCollectorName)
	})
	report.ClearWarning(trafficReportSource, codeDoctorTrafficQdiscMetricsReadFailed, qdiscMetricsCollectorName)

	meta := diagnostic.Lookup(codeDoctorTrafficQdiscMetricsReadFailed)
	if meta == nil {
		t.Fatalf("diagnostic code %q not registered", codeDoctorTrafficQdiscMetricsReadFailed)
	}

	reg := metrics.NewPrometheusRegistry()
	clk := sim.NewFakeClock(time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC))

	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	readErr := errors.New("netlink receive failed")
	p := newQdiscMetricsPoller(reg, log, clk, func() ([]InterfaceQoS, error) {
		if readErr != nil {
			return nil, readErr
		}
		return nil, nil
	})

	// First failure logs WARN with doctor code and raises report warning.
	p.collect()
	wantCodeAttr := "code=" + codeDoctorTrafficQdiscMetricsReadFailed
	if got := strings.Count(logBuf.String(), wantCodeAttr); got != 1 {
		t.Fatalf("first failure WARN count = %d, want 1; log:\n%s", got, logBuf.String())
	}
	if !hasQdiscReadWarning() {
		t.Fatal("expected active report-bus warning after first read failure")
	}

	// Repeated failure within 60s is rate-limited.
	clk.Add(30 * time.Second)
	p.collect()
	if got := strings.Count(logBuf.String(), wantCodeAttr); got != 1 {
		t.Fatalf("rate-limited WARN count = %d, want 1; log:\n%s", got, logBuf.String())
	}

	// Failure at or after 60s emits a second WARN.
	clk.Add(30 * time.Second)
	p.collect()
	if got := strings.Count(logBuf.String(), wantCodeAttr); got != 2 {
		t.Fatalf("post-window WARN count = %d, want 2; log:\n%s", got, logBuf.String())
	}

	// Recovery clears the report warning and resets the rate limiter.
	readErr = nil
	p.collect()
	if hasQdiscReadWarning() {
		t.Fatal("report-bus warning not cleared after read recovery")
	}

	// A new failure immediately after recovery logs right away without waiting 60s.
	clk.Add(time.Second)
	readErr = errors.New("netlink busy")
	p.collect()
	if got := strings.Count(logBuf.String(), wantCodeAttr); got != 3 {
		t.Fatalf("post-recovery WARN count = %d, want 3; log:\n%s", got, logBuf.String())
	}
	if !hasQdiscReadWarning() {
		t.Fatal("expected report-bus warning after post-recovery read failure")
	}

	// ErrNoBackendAvailable clears the warning without emitting another WARN.
	readErr = ErrNoBackendAvailable
	p.collect()
	if hasQdiscReadWarning() {
		t.Fatal("report-bus warning not cleared on ErrNoBackendAvailable")
	}
	if got := strings.Count(logBuf.String(), wantCodeAttr); got != 3 {
		t.Fatalf("WARN count after ErrNoBackendAvailable = %d, want 3; log:\n%s", got, logBuf.String())
	}
}
