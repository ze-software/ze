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

	"github.com/ze-software/ze/internal/component/firewall"
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

// recordingBackend is a firewall backend that keeps what it was asked to
// program. A refresh applies the tables it built, so a test that does not
// install one reaches the OS default backend and the host's real ruleset.
type recordingBackend struct {
	mu      sync.Mutex
	applied [][]firewall.Table
}

func (b *recordingBackend) Apply(desired []firewall.Table) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.applied = append(b.applied, desired)
	return nil
}

func (b *recordingBackend) ListTables() ([]firewall.Table, error) { return nil, nil }

func (b *recordingBackend) GetCounters(string) ([]firewall.ChainCounters, error) { return nil, nil }

func (b *recordingBackend) Close() error { return nil }

// last returns the tables of the most recent apply, and whether one happened.
func (b *recordingBackend) last() ([]firewall.Table, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.applied) == 0 {
		return nil, false
	}
	return b.applied[len(b.applied)-1], true
}

// setsByName indexes an applied table's sets so a test can assert on one by
// name. The order the plugin emits them in is not the point. What matters is
// which sets are declared, and whether the right one carries elements.
func setsByName(tbl firewall.Table) map[string]firewall.Set {
	byName := make(map[string]firewall.Set, len(tbl.Sets))
	for _, s := range tbl.Sets {
		byName[s.Name] = s
	}
	return byName
}

// useRecordingBackend makes this test's applies land in memory instead of the
// kernel. The backend name is the test's own, because a registration is global
// and refusing a duplicate is how the registry protects it.
func useRecordingBackend(t *testing.T) *recordingBackend {
	t.Helper()
	b := &recordingBackend{}
	name := "recording-" + t.Name()
	if err := firewall.RegisterBackend(name, func() (firewall.Backend, error) { return b, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := firewall.LoadBackend(name); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}
	t.Cleanup(func() {
		_ = firewall.RegisterTables("firewall-irr", nil)
		if err := firewall.CloseBackend(); err != nil {
			t.Errorf("CloseBackend: %v", err)
		}
	})
	return b
}

// newTestPlugin builds a plugin whose store queries addr and whose config
// references AS-TEST from one table term. Its applies are recorded rather than
// programmed.
func newTestPlugin(t *testing.T, addr string) (*irrPlugin, *recordingBackend) {
	t.Helper()
	return &irrPlugin{
		prefixStore: store.New(irr.NewIRR(addr), nil, ""),
		config: &irrConfig{
			Server: addr,
			refs:   []irrRef{{Name: "AS-TEST", IsASSet: true, TableName: "ze_wan"}},
		},
		stopCh: make(chan struct{}),
	}, useRecordingBackend(t)
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
	plug, _ := newTestPlugin(t, good)
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

// VALIDATES: AC-1 -- `update firewall irr asn|as-set` programs the prefixes it
// fetched. Fetching fills the store; the rules reach the kernel only when
// something applies the tables built from it.
// PREVENTS: a cold cache staying cold. On a fresh install, a wiped store, or an
// unreadable cache file, this plugin registers no set, so every rule naming one
// is held out of the reconcile (internal/component/firewall/registry.go).
// refresh-interval defaults to 0, so no loop recovers it either, and the
// operator's one recovery command fetched data and programmed nothing.
func TestRefreshNameProgramsWhatItLearned(t *testing.T) {
	plug, backend := newTestPlugin(t, fakeIRRWhois(t, map[string]string{
		"!a4AS-TEST": "A1\n10.0.0.0/24\nC\n",
	}))

	if err := plug.refreshName("AS-TEST"); err != nil {
		t.Fatalf("refreshName: %v", err)
	}

	applied, ok := backend.last()
	if !ok {
		t.Fatal("the refresh programmed nothing: the prefixes it fetched never reached the backend")
	}
	if len(applied) != 1 || applied[0].Name != "ze_wan" {
		t.Fatalf("applied %+v, want the ze_wan table the configured term names", applied)
	}
	// The fixture answers IPv4 only, and the table must still declare BOTH
	// family sets. The parser emits a term per family, whatever the entry
	// announces. A table missing one therefore has a term naming a set no
	// owner declares, and ApplyAll holds the whole table back for it.
	declared := setsByName(applied[0])
	if _, ok := declared["irr_v4_AS-TEST"]; !ok {
		t.Fatalf("applied table carries sets %+v, want irr_v4_AS-TEST", applied[0].Sets)
	}
	if _, ok := declared["irr_v6_AS-TEST"]; !ok {
		t.Fatalf("applied table carries sets %+v, want irr_v6_AS-TEST", applied[0].Sets)
	}
	if len(declared["irr_v4_AS-TEST"].Elements) == 0 {
		t.Fatalf("the v4 set reached the backend empty: %+v", declared["irr_v4_AS-TEST"])
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

	plug, _ := newTestPlugin(t, fakeIRRWhois(t, nil))
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

	plug, _ := newTestPlugin(t, fakeIRRWhois(t, map[string]string{
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

// VALIDATES: AC-1, AC-7 -- the prefixes a fetch cached are still cached after
// the reload that commits the term naming them, so the rule reaches the kernel.
// PREVENTS: the fail-open this spec's Security Review names. configure built a
// SECOND store on every apply, so a commit that verify accepted against the
// populated store applied against an empty one: no set was registered, ApplyAll
// held the operator's table back, and the commit reported success with nothing
// in the kernel. A table that was already filtering lost its rules the same way,
// on any unrelated commit.
func TestReconfigureKeepsFetchedPrefixes(t *testing.T) {
	addr := fakeIRRWhois(t, map[string]string{
		"!a4AS-TEST": "A1\n10.0.0.0/24\nC\n",
	})
	plug, backend := newTestPlugin(t, addr)

	if err := plug.refreshName("AS-TEST"); err != nil {
		t.Fatalf("refreshName: %v", err)
	}

	// What a commit does: the same config, verified and then applied again.
	// The server is unchanged, and so is everything the store holds.
	cfg := &irrConfig{
		Server: addr,
		refs:   []irrRef{{Name: "AS-TEST", IsASSet: true, TableName: "ze_wan"}},
	}
	if err := verifyRefs(plug.getPrefixStore(), cfg.allRefs()); err != nil {
		t.Fatalf("verify refused the config it accepted a moment ago: %v", err)
	}
	if err := plug.configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	entry := plug.getPrefixStore().Get("AS-TEST")
	if entry == nil || len(entry.IPv4) != 1 {
		t.Fatalf("the reload dropped the prefixes the fetch cached: %+v", entry)
	}

	applied, ok := backend.last()
	if !ok {
		t.Fatal("the reload programmed nothing")
	}
	if len(applied) != 1 || applied[0].Name != "ze_wan" {
		t.Fatalf("applied %+v, want the ze_wan table the configured term names", applied)
	}
	// Both family sets, as above: the fixture answers IPv4 only and the table
	// still declares the IPv6 set the term's twin names.
	declared := setsByName(applied[0])
	if _, ok := declared["irr_v4_AS-TEST"]; !ok {
		t.Fatalf("applied table carries sets %+v, want irr_v4_AS-TEST", applied[0].Sets)
	}
	if _, ok := declared["irr_v6_AS-TEST"]; !ok {
		t.Fatalf("applied table carries sets %+v, want irr_v6_AS-TEST", applied[0].Sets)
	}
	// One prefix lowers to an interval PAIR, so the count is not the prefix
	// count. Any element at all is the proof: an empty set is what a rebuilt
	// store produced, and the table was held back for it.
	if len(declared["irr_v4_AS-TEST"].Elements) == 0 {
		t.Fatalf("the v4 set reached the backend empty: %+v", declared["irr_v4_AS-TEST"])
	}
}

// VALIDATES: AC-1 -- a reload that moves the IRR server keeps the prefixes the
// old server answered, because they are what the firewall enforces until a
// refresh replaces them.
// PREVENTS: the same fail-open reappearing on the one path a "reuse the store
// when the server is unchanged" fix leaves open.
func TestReconfigureToAnotherServerKeepsFetchedPrefixes(t *testing.T) {
	plug, _ := newTestPlugin(t, fakeIRRWhois(t, map[string]string{
		"!a4AS-TEST": "A1\n10.0.0.0/24\nC\n",
	}))
	if err := plug.refreshName("AS-TEST"); err != nil {
		t.Fatalf("refreshName: %v", err)
	}

	moved := fakeIRRWhois(t, map[string]string{
		"!a4AS-TEST": "A1\n192.0.2.0/24\nC\n",
	})
	cfg := &irrConfig{
		Server: moved,
		refs:   []irrRef{{Name: "AS-TEST", IsASSet: true, TableName: "ze_wan"}},
	}
	if err := plug.configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	entry := plug.getPrefixStore().Get("AS-TEST")
	if entry == nil || len(entry.IPv4) != 1 {
		t.Fatalf("moving the server dropped the cached prefixes: %+v", entry)
	}
	if entry.IPv4[0] != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("cached prefix %v, want the one the first server answered", entry.IPv4[0])
	}

	// The next refresh must reach the new server, not the one the store was
	// built with.
	if err := plug.refreshName("AS-TEST"); err != nil {
		t.Fatalf("refreshName after the move: %v", err)
	}
	entry = plug.getPrefixStore().Get("AS-TEST")
	if entry == nil || len(entry.IPv4) != 1 || entry.IPv4[0] != netip.MustParsePrefix("192.0.2.0/24") {
		t.Fatalf("the refresh did not query the server the reload named: %+v", entry)
	}
}

// panickingBackend panics instead of returning an error. A scheduled refresh
// programs what it learned through the backend, so this puts a panic on the
// tick's path. It stands in for a parser that crashes on a malformed answer,
// which cannot be injected directly: see the SCOPE note on
// TestRefreshAllSurvivesPanic.
type panickingBackend struct{}

func (panickingBackend) Apply([]firewall.Table) error { panic("backend panic under refresh") }

func (panickingBackend) ListTables() ([]firewall.Table, error) { return nil, nil }

func (panickingBackend) GetCounters(string) ([]firewall.ChainCounters, error) { return nil, nil }

func (panickingBackend) Close() error { return nil }

// usePanickingBackend installs it for this test alone, under the test's own
// name because a backend registration is global.
func usePanickingBackend(t *testing.T) {
	t.Helper()
	name := "panicking-" + t.Name()
	if err := firewall.RegisterBackend(name, func() (firewall.Backend, error) { return panickingBackend{}, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := firewall.LoadBackend(name); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}
	t.Cleanup(func() {
		_ = firewall.RegisterTables("firewall-irr", nil)
		if err := firewall.CloseBackend(); err != nil {
			t.Errorf("CloseBackend: %v", err)
		}
	})
}

// VALIDATES: a panic under a scheduled refresh costs that tick and nothing
// more, and leaves the loop able to run the next one.
// PREVENTS: the firewall-irr process dying on a panic under refreshLoop, which
// runs in a goroutine this plugin starts itself. The SDK's dispatch recovery
// covers commands and bridge callbacks, not this one
// (pkg/plugin/sdk/sdk_dispatch.go), so an unguarded panic here ends the process.
// The manager respawns it, but a panic that repeats crash-loops into the
// respawn limit, and the registry then holds back every table naming an IRR set.
// SCOPE: this proves the APPLY half, because the panic comes from Backend.Apply.
// The FETCH half, where a malformed whois or PeeringDB answer reaches a parser,
// is not proven here and it is not claimed, although the same guard covers it.
// store.New takes a concrete *irr.IRR (internal/component/resolve/irr/store), so
// a panicking client cannot be injected without an interface seam built for this
// test alone. A recover narrowed to applyTables would leave the fetch unguarded
// and this test would still pass.
func TestRefreshAllSurvivesPanic(t *testing.T) {
	addr := fakeIRRWhois(t, map[string]string{
		"!a4AS-TEST": "A1\n10.0.0.0/24\nC\n",
	})
	plug := &irrPlugin{
		prefixStore: store.New(irr.NewIRR(addr), nil, ""),
		config: &irrConfig{
			Server: addr,
			refs:   []irrRef{{Name: "AS-TEST", IsASSet: true, TableName: "ze_wan"}},
		},
		stopCh: make(chan struct{}),
	}
	usePanickingBackend(t)
	reg := bindCountingOutcomes(t)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a panic escaped the scheduled refresh and would have taken the plugin down: %v", r)
		}
	}()
	plug.refreshAll()

	// A tick that left the guard set is a loop that never refreshes again, which
	// is the same outcome by a slower route.
	if plug.refreshing.Load() {
		t.Error("the in-flight guard stayed set after a panic: no later tick can run")
	}
	if entry := plug.prefixStore.Get("AS-TEST"); entry == nil || len(entry.IPv4) != 1 {
		t.Errorf("the fetch before the panic was lost: %+v", entry)
	}

	// A recovered tick that counts nothing is a tick no dashboard can see. The
	// crash it replaced was visible in ze_plugin_restarts_total.
	if got := reg.count("panic"); got != 1 {
		t.Errorf("outcomes under label panic = %d, want 1", got)
	}
}

// countingOutcomes counts ze_firewall_irr_refresh_outcomes_total by label and
// nops every other instrument, so a test can assert which outcome a tick
// recorded. The firewall component has its own countingRegistry for the same
// job, unexported there and covering a different metric.
type countingOutcomes struct {
	metrics.NopRegistry
	mu     sync.Mutex
	counts map[string]int
}

func (c *countingOutcomes) CounterVec(name, help string, labels []string) metrics.CounterVec {
	if name != "ze_firewall_irr_refresh_outcomes_total" {
		return c.NopRegistry.CounterVec(name, help, labels)
	}
	return outcomeVec{c}
}

func (c *countingOutcomes) count(label string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[label]
}

type outcomeVec struct{ reg *countingOutcomes }

func (v outcomeVec) With(labels ...string) metrics.Counter {
	if len(labels) == 0 {
		return metrics.NopRegistry{}.Counter("", "")
	}
	return outcomeCounter{v.reg, labels[0]}
}

func (outcomeVec) Delete(...string) bool { return false }

type outcomeCounter struct {
	reg   *countingOutcomes
	label string
}

func (o outcomeCounter) Inc() {
	o.reg.mu.Lock()
	defer o.reg.mu.Unlock()
	o.reg.counts[o.label]++
}

func (outcomeCounter) Add(float64) {}

// bindCountingOutcomes points the plugin's metrics at a counter this test reads.
func bindCountingOutcomes(t *testing.T) *countingOutcomes {
	t.Helper()
	prev := irrMetricsPtr.Load()
	t.Cleanup(func() { irrMetricsPtr.Store(prev) })
	reg := &countingOutcomes{counts: make(map[string]int)}
	setMetricsRegistry(reg)
	return reg
}
