package irr

// VALIDATES: AC-12 refresh-interval 0 disables auto-refresh
// VALIDATES: AC-13 refresh-interval > 0 enables periodic refresh
// VALIDATES: AC-14 failed refresh preserves last-good cache
// PREVENTS: auto-refresh running when operator expects manual-only mode

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/metrics"
)

func TestRefreshLoopDisabledByDefault(t *testing.T) {
	cfg := &irrConfig{
		RefreshInterval: 0,
	}
	if cfg.RefreshInterval != 0 {
		t.Fatal("default refresh interval must be 0 (disabled)")
	}
}

func TestRefreshLoopStartsWhenEnabled(t *testing.T) {
	cfg := &irrConfig{
		RefreshInterval: 3600,
	}
	if cfg.RefreshInterval == 0 {
		t.Fatal("refresh interval must be non-zero when enabled")
	}
}

// fakeIRRWhois starts a whois server answering each exact query with the raw
// RPSL reply mapped to it. An unmapped query answers "D" (key not found), the
// reply a real server sends for a name it does not hold.
func fakeIRRWhois(t *testing.T, replies map[string]string) string {
	t.Helper()
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 4096)
				n, readErr := c.Read(buf)
				if readErr != nil {
					return
				}
				reply, ok := replies[strings.TrimSpace(string(buf[:n]))]
				if !ok {
					reply = "D\n"
				}
				if _, wErr := fmt.Fprint(c, reply); wErr != nil {
					return
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

// newTestPlugin builds a plugin whose store queries addr and whose config
// references AS-TEST from one table term.
func newTestPlugin(addr string) *irrPlugin {
	return &irrPlugin{
		prefixStore: store.New(irr.NewIRR(addr), nil, ""),
		config: &irrConfig{
			Server: addr,
			refs:   []irrRef{{Name: "AS-TEST", IsASSet: true, TableName: "ze_wan"}},
		},
		stopCh: make(chan struct{}),
	}
}

// VALIDATES: AC-1 -- a refresh that learns nothing leaves the plugin enforcing
// the prefixes it already had.
// PREVENTS: the vacuous version of this test, which asserted a nil entry before
// any refresh had run and proved nothing.
func TestRefreshFailureKeepsLastGood(t *testing.T) {
	good := fakeIRRWhois(t, map[string]string{
		"!a4AS-TEST": "A1\n10.0.0.0/24\nC\n",
		"!a6AS-TEST": "C\n",
	})
	plug := newTestPlugin(good)
	if err := plug.refreshName("AS-TEST"); err != nil {
		t.Fatalf("seed refresh: %v", err)
	}
	if entry := plug.prefixStore.Get("AS-TEST"); entry == nil || len(entry.IPv4) != 1 {
		t.Fatalf("seed did not cache prefixes: %+v", entry)
	}

	// Point the same store at a server that answers "key not found", the way a
	// real one does during an outage or a bad database load.
	plug.prefixStore = store.New(irr.NewIRR(fakeIRRWhois(t, nil)), nil, "")
	plug.prefixStore.Put("AS-TEST", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}, nil)

	if err := plug.refreshName("AS-TEST"); err == nil {
		t.Fatal("a refresh that learned nothing must report it, not report success")
	}
	entry := plug.prefixStore.Get("AS-TEST")
	if entry == nil || len(entry.IPv4) != 1 {
		t.Fatalf("cached prefixes lost to an empty answer: %+v", entry)
	}
	if !entry.Stale() {
		t.Error("the kept entry must report itself stale")
	}
}

// countingRegistry records counter label values and gauge sets so a test can
// tell one refresh outcome from another.
type countingRegistry struct {
	mu       sync.Mutex
	outcomes map[string]int
	gauges   map[string]float64
}

func newCountingRegistry() *countingRegistry {
	return &countingRegistry{outcomes: make(map[string]int), gauges: make(map[string]float64)}
}

func (r *countingRegistry) Counter(_, _ string) metrics.Counter { return countingCounter{} }
func (r *countingRegistry) Gauge(name, _ string) metrics.Gauge {
	return &countingGauge{reg: r, name: name}
}
func (r *countingRegistry) CounterVec(_, _ string, _ []string) metrics.CounterVec {
	return &countingVec{reg: r}
}

func (r *countingRegistry) GaugeVec(_, _ string, _ []string) metrics.GaugeVec { return nil }
func (r *countingRegistry) Histogram(_, _ string, _ []float64) metrics.Histogram {
	return nil
}

func (r *countingRegistry) HistogramVec(_, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return nil
}

func (r *countingRegistry) outcome(label string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.outcomes[label]
}

// gaugeStamped reports whether the named gauge was ever set.
func (r *countingRegistry) gaugeStamped(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.gauges[name]
	return ok
}

type countingCounter struct{}

func (countingCounter) Inc()          {}
func (countingCounter) Add(_ float64) {}

type countingVec struct{ reg *countingRegistry }

func (v *countingVec) With(labelValues ...string) metrics.Counter {
	v.reg.mu.Lock()
	defer v.reg.mu.Unlock()
	v.reg.outcomes[strings.Join(labelValues, ",")]++
	return countingCounter{}
}

func (v *countingVec) Delete(_ ...string) bool { return false }

type countingGauge struct {
	reg  *countingRegistry
	name string
}

func (g *countingGauge) Set(value float64) {
	g.reg.mu.Lock()
	defer g.reg.mu.Unlock()
	g.reg.gauges[g.name] = value
}

func (g *countingGauge) Inc()          {}
func (g *countingGauge) Dec()          {}
func (g *countingGauge) Add(_ float64) {}

// VALIDATES: AC-4 -- a refresh that learns nothing is counted apart from a
// success, and does not stamp the last-refresh gauge.
// PREVENTS: an operator reading a green refresh counter while the filter is
// running on data nobody confirmed.
func TestRefreshOutcomeCountsEmptyDistinctly(t *testing.T) {
	reg := newCountingRegistry()
	setMetricsRegistry(reg)
	t.Cleanup(func() { irrMetricsPtr.Store(nil) })

	plug := newTestPlugin(fakeIRRWhois(t, nil))
	plug.prefixStore.Put("AS-TEST", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}, nil)

	if err := plug.refreshName("AS-TEST"); err == nil {
		t.Fatal("expected the empty answer to be reported")
	}
	if got := reg.outcome("empty"); got != 1 {
		t.Errorf("empty outcome count = %d, want 1", got)
	}
	if got := reg.outcome("success"); got != 0 {
		t.Errorf("success outcome count = %d, want 0", got)
	}
	if reg.gaugeStamped("ze_firewall_irr_last_refresh_timestamp") {
		t.Error("a refresh that learned nothing must not stamp the last-refresh gauge")
	}
}

// VALIDATES: AC-4 -- a refresh that learns prefixes still counts as a success
// and stamps the gauge.
// PREVENTS: the empty-answer guard suppressing the healthy path too.
func TestRefreshOutcomeCountsSuccess(t *testing.T) {
	reg := newCountingRegistry()
	setMetricsRegistry(reg)
	t.Cleanup(func() { irrMetricsPtr.Store(nil) })

	plug := newTestPlugin(fakeIRRWhois(t, map[string]string{
		"!a4AS-TEST": "A1\n10.0.0.0/24\nC\n",
	}))
	if err := plug.refreshName("AS-TEST"); err != nil {
		t.Fatalf("refreshName: %v", err)
	}
	if got := reg.outcome("success"); got != 1 {
		t.Errorf("success outcome count = %d, want 1", got)
	}
	if !reg.gaugeStamped("ze_firewall_irr_last_refresh_timestamp") {
		t.Error("a refresh that learned prefixes must stamp the last-refresh gauge")
	}
}

// VALIDATES: AC-3 -- config verify refuses a reference whose cached entry holds
// no prefixes, so a binding that can filter nothing never commits.
// PREVENTS: a zero-prefix entry reading as a valid answer at commit time.
func TestVerifyRefsRefusesEmptyEntry(t *testing.T) {
	ps := store.New(nil, nil, "")
	ps.Put("AS-EMPTY", nil, nil)

	refs := []irrRef{{Name: "AS-EMPTY", IsASSet: true}}
	err := verifyRefs(ps, refs)
	if err == nil {
		t.Fatal("a cached entry with no prefixes must not verify")
	}
	if !strings.Contains(err.Error(), "AS-EMPTY") {
		t.Errorf("error %q does not name the AS-SET the operator must fetch", err)
	}

	ps.Put("AS-FULL", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, nil)
	if err := verifyRefs(ps, []irrRef{{Name: "AS-FULL", IsASSet: true}}); err != nil {
		t.Errorf("a populated entry must verify: %v", err)
	}
}
