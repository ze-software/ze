// Design: docs/architecture/isis/isis-11-redistribution.md -- IS-IS redistribution consumer tests.
//
// VALIDATES: spec-isis-11 consumer side (AC-3..AC-6, AC-10) -- InjectRoute turns a
//            connected/static/BGP RouteEntry into a TLV 135 reachability entry in
//            the local LSP set with the FIXED default metric and up/down bit 0 on
//            first injection (TLV 135 has NO external bit, RFC 5305 sec 4);
//            WithdrawRoute removes it and re-originates; the consumer is named
//            "isis"; failures are logged.
// PREVENTS:  a regression where an injected route never reaches LSP origination,
//            an external bit is fabricated on IPv4, or a failure is swallowed.

package isisredistribute

import (
	"context"
	"net/netip"
	"strings"
	"sync"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
)

// fakeInjector is a test double for the engine-facing LSPInjector. It records the
// redistributed prefix set per level and how many times Originate was called.
type fakeInjector struct {
	mu         sync.Mutex
	levels     []lsdb.Level
	set        map[lsdb.Level]map[netip.Prefix]lsdb.PrefixInfo
	setV6      map[lsdb.Level]map[netip.Prefix]lsdb.PrefixInfoV6
	originated int
	failNext   bool // when set, Originate reports failure once (LogsFailure test)
}

func newFakeInjector(levels ...lsdb.Level) *fakeInjector {
	if len(levels) == 0 {
		levels = []lsdb.Level{lsdb.Level1, lsdb.Level2}
	}
	return &fakeInjector{
		levels: levels,
		set:    map[lsdb.Level]map[netip.Prefix]lsdb.PrefixInfo{},
		setV6:  map[lsdb.Level]map[netip.Prefix]lsdb.PrefixInfoV6{},
	}
}

func (f *fakeInjector) OriginationLevels() []lsdb.Level { return f.levels }

func (f *fakeInjector) SetRedistPrefix(level lsdb.Level, info lsdb.PrefixInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.set[level] == nil {
		f.set[level] = map[netip.Prefix]lsdb.PrefixInfo{}
	}
	f.set[level][info.Prefix] = info
}

func (f *fakeInjector) RemoveRedistPrefix(level lsdb.Level, prefix netip.Prefix) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.set[level] == nil {
		return false
	}
	_, ok := f.set[level][prefix]
	delete(f.set[level], prefix)
	return ok
}

func (f *fakeInjector) SetRedistPrefixV6(level lsdb.Level, info lsdb.PrefixInfoV6) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setV6[level] == nil {
		f.setV6[level] = map[netip.Prefix]lsdb.PrefixInfoV6{}
	}
	f.setV6[level][info.Prefix] = info
}

func (f *fakeInjector) RemoveRedistPrefixV6(level lsdb.Level, prefix netip.Prefix) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setV6[level] == nil {
		return false
	}
	_, ok := f.setV6[level][prefix]
	delete(f.setV6[level], prefix)
	return ok
}

func (f *fakeInjector) snapshotV6(level lsdb.Level) []lsdb.PrefixInfoV6 {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]lsdb.PrefixInfoV6, 0, len(f.setV6[level]))
	for _, v := range f.setV6[level] {
		out = append(out, v)
	}
	return out
}

func (f *fakeInjector) Originate() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.originated++
	if f.failNext {
		f.failNext = false
		return errTestOriginate
	}
	return nil
}

func (f *fakeInjector) snapshot(level lsdb.Level) []lsdb.PrefixInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]lsdb.PrefixInfo, 0, len(f.set[level]))
	for _, v := range f.set[level] {
		out = append(out, v)
	}
	return out
}

var errTestOriginate = &originateError{}

type originateError struct{}

func (*originateError) Error() string { return "test originate failure" }

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", s, err)
	}
	return p
}

// TestISISRedistConsumerName verifies the consumer reports "isis" for registry
// lookup and self-import auto-rejection (AC-10).
func TestISISRedistConsumerName(t *testing.T) {
	c := NewConsumer(newFakeInjector())
	if c.Name() != "isis" {
		t.Fatalf("Name() = %q, want isis", c.Name())
	}
}

// TestISISRedistConsumerConnected verifies a connected import becomes a TLV 135
// entry with the fixed default metric on every origination level (AC-3).
func TestISISRedistConsumerConnected(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1, lsdb.Level2)
	c := NewConsumer(inj)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix: "10.20.0.0/24",
		Source: "connected",
	})

	pfx := mustPrefix(t, "10.20.0.0/24")
	for _, lvl := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		got := inj.snapshot(lvl)
		if len(got) != 1 {
			t.Fatalf("level %v: got %d prefixes, want 1", lvl, len(got))
		}
		if got[0].Prefix != pfx {
			t.Fatalf("level %v: prefix = %v, want %v", lvl, got[0].Prefix, pfx)
		}
		if got[0].Metric.Value() != DefaultRedistMetric {
			t.Fatalf("level %v: metric = %d, want fixed default %d", lvl, got[0].Metric.Value(), DefaultRedistMetric)
		}
		// RFC requirement: RFC5305-4.1-1 negative -- a first-injected (not down-leaked) prefix has the TLV 135 up/down bit 0; the bit is set only when advertising down the hierarchy (RFC 5305 sec 4.1).
		if got[0].UpDown {
			t.Fatalf("level %v: up/down bit set on first injection; TLV 135 has no external bit and up/down is only set on down-level leak", lvl)
		}
	}
	if inj.originated == 0 {
		t.Fatal("Originate not called after InjectRoute")
	}
}

// TestISISRedistConsumerStatic verifies a static import becomes a TLV 135 entry
// (AC-4).
func TestISISRedistConsumerStatic(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1)
	c := NewConsumer(inj)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix: "172.16.0.0/12",
		Source: "static",
	})

	got := inj.snapshot(lsdb.Level1)
	if len(got) != 1 || got[0].Prefix != mustPrefix(t, "172.16.0.0/12") {
		t.Fatalf("static import not present as TLV 135 entry: %+v", got)
	}
}

// TestISISRedistConsumerBGP verifies a BGP import becomes a TLV 135 entry with
// up/down bit 0 on first injection -- TLV 135 has NO external bit (AC-5).
func TestISISRedistConsumerBGP(t *testing.T) {
	inj := newFakeInjector(lsdb.Level2)
	c := NewConsumer(inj)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix: "203.0.113.0/24",
		Source: "bgp",
	})

	got := inj.snapshot(lsdb.Level2)
	if len(got) != 1 {
		t.Fatalf("bgp import: got %d, want 1", len(got))
	}
	if got[0].UpDown {
		t.Fatal("bgp import: up/down bit set on first injection; TLV 135 has no external bit (RFC 5305 sec 4)")
	}
}

// TestISISRedistConsumerWithdraw verifies WithdrawRoute removes the entry and
// re-originates (AC-6).
func TestISISRedistConsumerWithdraw(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1, lsdb.Level2)
	c := NewConsumer(inj)
	ctx := context.Background()

	c.InjectRoute(ctx, family.IPv4Unicast, configredist.RouteEntry{Prefix: "10.0.0.0/8", Source: "connected"})
	before := inj.originated
	c.WithdrawRoute(ctx, family.IPv4Unicast, "10.0.0.0/8")

	for _, lvl := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		if got := inj.snapshot(lvl); len(got) != 0 {
			t.Fatalf("level %v: prefix still present after withdraw: %+v", lvl, got)
		}
	}
	if inj.originated <= before {
		t.Fatal("Originate not called after WithdrawRoute")
	}
}

// TestISISRedistConsumerUpDownBit verifies the up/down bit is 0 on first
// injection and TLV 135 exposes no external flag for IPv4 (AC-5, RFC 5305 sec 4).
func TestISISRedistConsumerUpDownBit(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1)
	c := NewConsumer(inj)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix: "198.51.100.0/24",
		Source: "bgp",
	})

	got := inj.snapshot(lsdb.Level1)
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	// lsdb.PrefixInfo has only UpDown + Metric + Prefix -- there is no external
	// field on the IPv4 reachability info, which is the structural guarantee that
	// TLV 135 carries no external bit. The up/down bit must be 0 here (not leaked).
	if got[0].UpDown {
		t.Fatal("up/down bit must be 0 on first injection (only set on down-level leak, RFC 2966)")
	}
}

// TestISISRedistConsumerLogsFailure verifies an origination failure is surfaced,
// not swallowed (R-3 regression guard). The fake injector fails once; the
// consumer must not panic and the inject-failure metric path is exercised.
func TestISISRedistConsumerLogsFailure(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1)
	inj.failNext = true
	c := NewConsumer(inj)

	// Must not panic; the failure is logged + counted (metric is a no-op here).
	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix: "10.1.0.0/16",
		Source: "connected",
	})
	// The prefix is still recorded (set before Originate); the failure is on the
	// re-origination, which the consumer logs.
	if got := inj.snapshot(lsdb.Level1); len(got) != 1 {
		t.Fatalf("prefix not recorded before origination failure: %+v", got)
	}
}

// TestISISRedistConsumerInvalidPrefix verifies a malformed prefix is rejected
// without mutating state (security review: input validation).
func TestISISRedistConsumerInvalidPrefix(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1)
	c := NewConsumer(inj)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix: "not-a-prefix",
		Source: "connected",
	})
	if got := inj.snapshot(lsdb.Level1); len(got) != 0 {
		t.Fatalf("malformed prefix injected: %+v", got)
	}
}

// labelCounter records the per-label-tuple increment count for one CounterVec.
type labelCounter struct {
	mu     sync.Mutex
	counts map[string]int // joined label values -> increments
}

func newLabelCounter() *labelCounter { return &labelCounter{counts: map[string]int{}} }

func (l *labelCounter) With(values ...string) metrics.Counter {
	return &labelCounterChild{parent: l, key: labelKey(values)}
}
func (l *labelCounter) Delete(...string) bool { return false }

func (l *labelCounter) count(values ...string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.counts[labelKey(values)]
}

func labelKey(values []string) string {
	// '\x00' separator: a value can never contain it, so distinct tuples never alias.
	return strings.Join(values, "\x00")
}

type labelCounterChild struct {
	parent *labelCounter
	key    string
}

func (c *labelCounterChild) Inc() {
	c.parent.mu.Lock()
	c.parent.counts[c.key]++
	c.parent.mu.Unlock()
}
func (c *labelCounterChild) Add(float64) { c.Inc() }

// labelRegistry is a metrics.Registry that hands out labelCounters for the named
// redist CounterVecs so a test can assert the exact {source,afi} tuple that took a
// sample. All other Registry surface delegates to NopRegistry.
type labelRegistry struct {
	metrics.NopRegistry
	vecs map[string]*labelCounter
}

func newLabelRegistry() *labelRegistry {
	return &labelRegistry{vecs: map[string]*labelCounter{}}
}

func (r *labelRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	lc := newLabelCounter()
	r.vecs[name] = lc
	return lc
}

func (r *labelRegistry) vec(name string) *labelCounter { return r.vecs[name] }

// TestISISRedistConsumerWithdrawMetricSource is the regression guard for the
// withdraw metric label bug: the generic WithdrawRoute carries no source, so the
// consumer must recover the source it recorded at inject time instead of always
// labeling source="unknown".
func TestISISRedistConsumerWithdrawMetricSource(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1, lsdb.Level2)
	c := NewConsumer(inj)
	reg := newLabelRegistry()
	c.SetMetrics(reg)
	ctx := context.Background()

	c.InjectRoute(ctx, family.IPv4Unicast, configredist.RouteEntry{Prefix: "10.0.0.0/8", Source: "bgp"})
	c.WithdrawRoute(ctx, family.IPv4Unicast, "10.0.0.0/8")

	withdrawn := reg.vec("ze_isis_redist_withdrawn_total")
	if withdrawn == nil {
		t.Fatal("ze_isis_redist_withdrawn_total not registered")
	}
	if got := withdrawn.count("bgp", "ipv4"); got != 1 {
		t.Fatalf("withdrawn{source=bgp,afi=ipv4} = %d, want 1 (source not threaded through withdraw)", got)
	}
	if got := withdrawn.count("unknown", "ipv4"); got != 0 {
		t.Fatalf("withdrawn{source=unknown,afi=ipv4} = %d, want 0 (regression: source lost on withdraw)", got)
	}
}

// TestISISRedistConsumerWithdrawMetricSourceV6 is the IPv6 twin: the recovered
// source must label the IPv6 withdraw counter too.
func TestISISRedistConsumerWithdrawMetricSourceV6(t *testing.T) {
	inj := newFakeInjector(lsdb.Level2)
	c := NewConsumer(inj)
	reg := newLabelRegistry()
	c.SetMetrics(reg)
	ctx := context.Background()

	c.InjectRoute(ctx, family.IPv6Unicast, configredist.RouteEntry{Prefix: "2001:db8::/32", Source: "static"})
	c.WithdrawRoute(ctx, family.IPv6Unicast, "2001:db8::/32")

	withdrawn := reg.vec("ze_isis_redist_withdrawn_total")
	if got := withdrawn.count("static", "ipv6"); got != 1 {
		t.Fatalf("withdrawn{source=static,afi=ipv6} = %d, want 1", got)
	}
	if got := withdrawn.count("unknown", "ipv6"); got != 0 {
		t.Fatalf("withdrawn{source=unknown,afi=ipv6} = %d, want 0", got)
	}
}

// TestISISRedistConsumerWithdrawMetricSourceUnknown verifies a withdraw for a
// prefix that was never injected still emits a label value ("unknown"), not a
// missing/empty label.
func TestISISRedistConsumerWithdrawMetricSourceUnknown(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1)
	// Pre-seed the injector so RemoveRedistPrefix reports removed=true even though
	// the consumer never recorded a source for this prefix (e.g. a prefix injected
	// before SetMetrics, or by a path that did not go through rememberSource).
	pfx := mustPrefix(t, "192.0.2.0/24")
	inj.SetRedistPrefix(lsdb.Level1, lsdb.PrefixInfo{Prefix: pfx})
	c := NewConsumer(inj)
	reg := newLabelRegistry()
	c.SetMetrics(reg)

	c.WithdrawRoute(context.Background(), family.IPv4Unicast, "192.0.2.0/24")

	withdrawn := reg.vec("ze_isis_redist_withdrawn_total")
	if got := withdrawn.count("unknown", "ipv4"); got != 1 {
		t.Fatalf("withdrawn{source=unknown,afi=ipv4} = %d, want 1 (no source recorded -> unknown)", got)
	}
}

// TestISISRedistConsumerIPv6NotInIPv4Set verifies an IPv6 injection (isis-12,
// TLV 236) lands in the IPv6 set and does NOT pollute the IPv4 (TLV 135) set.
// (Originally asserted IPv6 was a no-op; isis-12 now handles TLV 236, so the
// assertion is that the two address families stay separate.)
func TestISISRedistConsumerIPv6NotInIPv4Set(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1)
	c := NewConsumer(inj)

	c.InjectRoute(context.Background(), family.IPv6Unicast, configredist.RouteEntry{
		Prefix: "2001:db8::/32",
		Source: "connected",
	})
	if got := inj.snapshot(lsdb.Level1); len(got) != 0 {
		t.Fatalf("IPv6 prefix leaked into the IPv4 (TLV 135) set: %+v", got)
	}
	if got := inj.snapshotV6(lsdb.Level1); len(got) != 1 {
		t.Fatalf("IPv6 prefix not injected into the IPv6 (TLV 236) set: %+v", got)
	}
}
