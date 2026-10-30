package locrib

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// recordingRegistry records the last value of every gauge series and the count
// of every counter series it handed out, keyed by metric name and labels.
type recordingRegistry struct {
	metrics.NopRegistry
	mu     sync.Mutex
	values map[string]float64
}

func newRecordingRegistry() *recordingRegistry {
	return &recordingRegistry{values: make(map[string]float64)}
}

func (r *recordingRegistry) value(name string, labels ...string) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.values[seriesKey(name, labels)]
}

func seriesKey(name string, labels []string) string {
	key := name
	for _, l := range labels {
		key += "|" + l
	}
	return key
}

type recordingVec struct {
	reg  *recordingRegistry
	name string
}

func (v recordingVec) With(labels ...string) recordingSeries {
	return recordingSeries{reg: v.reg, key: seriesKey(v.name, labels)}
}

func (v recordingVec) Delete(...string) bool { return false }

type recordingSeries struct {
	reg *recordingRegistry
	key string
}

func (s recordingSeries) add(d float64) {
	s.reg.mu.Lock()
	s.reg.values[s.key] += d
	s.reg.mu.Unlock()
}

func (s recordingSeries) Set(v float64) {
	s.reg.mu.Lock()
	s.reg.values[s.key] = v
	s.reg.mu.Unlock()
}
func (s recordingSeries) Inc()          { s.add(1) }
func (s recordingSeries) Dec()          { s.add(-1) }
func (s recordingSeries) Add(d float64) { s.add(d) }

// counterVec and gaugeVec adapt recordingVec to the two interfaces, whose With
// methods return different types.
type counterVec struct{ recordingVec }

func (v counterVec) With(labels ...string) metrics.Counter { return v.recordingVec.With(labels...) }

type gaugeVec struct{ recordingVec }

func (v gaugeVec) With(labels ...string) metrics.Gauge { return v.recordingVec.With(labels...) }

// TestLocRIBPolicyMetrics proves ze_rib_policy_hidden_paths{source} follows the
// hidden paths a source holds through hide, re-judge, remove and re-admit, and
// that ze_rib_policy_rejected_total{source,chain} counts each rejection once,
// under the chain that rejected it, with no recount when a reload re-judges an
// unchanged verdict. It also checks the chain each hidden path records
// (HiddenBy), which `show rib candidates` reports.
func TestLocRIBPolicyMetrics(t *testing.T) {
	ospf := redistevents.RegisterProtocol("ospf")
	static := redistevents.RegisterProtocol("static")
	reg := newRecordingRegistry()
	SetPolicyMetricsRegistry(wrapRecording{reg})
	t.Cleanup(func() { SetPolicyMetricsRegistry(nil) })

	hidden := func(source string) float64 { return reg.value("ze_rib_policy_hidden_paths", source) }
	rejected := func(source, chain string) float64 {
		return reg.value("ze_rib_policy_rejected_total", source, chain)
	}
	ospfBase := float64(hiddenPaths[ospf].Load())
	staticBase := float64(hiddenPaths[static].Load())

	r := NewRIB()
	sourceReject := netip.MustParsePrefix("10.61.0.0/24")
	centralReject := netip.MustParsePrefix("192.0.2.0/24")
	usePolicy(t, r, map[redistevents.ProtocolID]ribpolicy.Chain{
		ospf: chainFunc(func(rt ribpolicy.Route) ribpolicy.Verdict {
			if rt.Prefix == sourceReject {
				return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
			}
			return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
		}),
	}, chainFunc(func(rt ribpolicy.Route) ribpolicy.Verdict {
		if rt.Prefix == centralReject {
			return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
		}
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}))

	r.Insert(family.IPv4Unicast, sourceReject, ospfPath("192.0.2.9", 1))
	r.Insert(family.IPv4Unicast, centralReject, staticRoute())
	if got := hidden("ospf"); got != ospfBase+1 {
		t.Fatalf("ospf hidden paths = %v, want %v", got, ospfBase+1)
	}
	if got := hidden("static"); got != staticBase+1 {
		t.Fatalf("static hidden paths = %v, want %v", got, staticBase+1)
	}
	if got := rejected("ospf", "source"); got != 1 {
		t.Fatalf("ospf rejections by its source chain = %v, want 1", got)
	}
	if got := rejected("static", "central"); got != 1 {
		t.Fatalf("static rejections by the central chain = %v, want 1", got)
	}
	if got := rejected("ospf", "central"); got != 0 {
		t.Fatalf("ospf rejections by the central chain = %v, want 0: the source chain rejected it", got)
	}
	assertHiddenBy(t, r, sourceReject, ribpolicy.ChainRoleSource)
	assertHiddenBy(t, r, centralReject, ribpolicy.ChainRoleCentral)

	// A reload whose chains still reject the same paths counts nothing again.
	r.Reselect()
	if got := rejected("ospf", "source"); got != 1 {
		t.Fatalf("after an unchanged re-judge, ospf rejections = %v, want 1", got)
	}
	if got := hidden("ospf"); got != ospfBase+1 {
		t.Fatalf("after an unchanged re-judge, ospf hidden paths = %v, want %v", got, ospfBase+1)
	}

	// A producer re-offering the rejected path is a new rejection.
	r.Insert(family.IPv4Unicast, sourceReject, ospfPath("192.0.2.9", 1))
	if got := rejected("ospf", "source"); got != 2 {
		t.Fatalf("a re-offered path rejected again counts %v, want 2", got)
	}
	if got := hidden("ospf"); got != ospfBase+1 {
		t.Fatalf("a re-offered hidden path moved the gauge to %v, want %v", got, ospfBase+1)
	}

	r.Remove(family.IPv4Unicast, sourceReject, ospf, 1)
	if got := hidden("ospf"); got != ospfBase {
		t.Fatalf("after the hidden path left, ospf hidden paths = %v, want %v", got, ospfBase)
	}
	clearPolicy(r)
	if got := hidden("static"); got != staticBase {
		t.Fatalf("after the central chain was cleared, static hidden paths = %v, want %v", got, staticBase)
	}
	assertHiddenBy(t, r, centralReject, ribpolicy.ChainRoleUnspecified)
}

// TestLocRIBHiddenInsertZeroAlloc proves the metrics cost no allocation: a
// rejected path re-offered with the metrics bound allocates nothing.
func TestLocRIBHiddenInsertZeroAlloc(t *testing.T) {
	SetPolicyMetricsRegistry(wrapRecording{newRecordingRegistry()})
	t.Cleanup(func() { SetPolicyMetricsRegistry(nil) })
	r := NewRIB()
	prefix := netip.MustParsePrefix("10.62.0.0/24")
	ospf := ospfPath("192.0.2.9", 1)
	usePolicy(t, r, map[redistevents.ProtocolID]ribpolicy.Chain{ospf.Source: rejectAll}, nil)
	r.Insert(family.IPv4Unicast, prefix, ospf)
	metric := uint32(0)
	allocs := testing.AllocsPerRun(1000, func() {
		metric++
		ospf.Metric = metric
		r.Insert(family.IPv4Unicast, prefix, ospf)
	})
	if allocs != 0 {
		t.Errorf("a rejected insert allocates %.1f times per call, want 0", allocs)
	}
}

// wrapRecording gives recordingRegistry the two Vec constructors with the
// interface types metrics.Registry declares.
type wrapRecording struct{ *recordingRegistry }

func (w wrapRecording) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return counterVec{recordingVec{reg: w.recordingRegistry, name: name}}
}

func (w wrapRecording) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	return gaugeVec{recordingVec{reg: w.recordingRegistry, name: name}}
}

func assertHiddenBy(t *testing.T, r *RIB, prefix netip.Prefix, want ribpolicy.ChainRole) {
	t.Helper()
	g, ok := r.Lookup(family.IPv4Unicast, prefix)
	if !ok {
		t.Fatalf("%s has no entry", prefix)
	}
	for i := range g.Paths {
		if g.Paths[i].HiddenBy != want {
			t.Fatalf("%s: path hidden by %v, want %v", prefix, g.Paths[i].HiddenBy, want)
		}
		if g.Paths[i].Hidden != (want != ribpolicy.ChainRoleUnspecified) {
			t.Fatalf("%s: Hidden %v disagrees with HiddenBy %v", prefix, g.Paths[i].Hidden, want)
		}
	}
}
