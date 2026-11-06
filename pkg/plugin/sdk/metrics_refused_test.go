// Design: docs/plugin-development/metrics.md -- what a refused name holds
// Related: metrics.go -- the publisher under test
package sdk

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// refusalLines counts the WARN lines the publisher writes while a test runs. The
// tests using it do not call t.Parallel, and Go runs a test that does not only
// while every test that did is paused, so swapping the default logger reaches no
// other running test.
type refusalLines struct {
	slog.Handler
	lines *atomic.Int64
	seen  *refusalRecords
}

// refusalRecord is what one WARN line said. A refusal reads back its message and
// the two attributes a plugin's own strings can reach.
type refusalRecord struct {
	message, metric, reason string
}

// refusalRecords holds the lines of one test, in order. Safe for concurrent use.
type refusalRecords struct {
	mu      sync.Mutex
	records []refusalRecord
}

func (r *refusalRecords) all() []refusalRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.records)
}

func (h refusalLines) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // hugeParam: slog.Handler fixes this signature.
func (h refusalLines) Handle(_ context.Context, record slog.Record) error {
	if record.Level != slog.LevelWarn {
		return nil
	}
	h.lines.Add(1)
	seen := refusalRecord{message: record.Message}
	record.Attrs(func(attr slog.Attr) bool {
		switch attr.Key {
		case "metric":
			seen.metric = attr.Value.String()
		case "reason":
			seen.reason = attr.Value.String()
		}
		return true
	})
	h.seen.mu.Lock()
	defer h.seen.mu.Unlock()
	h.seen.records = append(h.seen.records, seen)
	return nil
}

func captureRefusalLines(t *testing.T) (*atomic.Int64, *refusalRecords) {
	t.Helper()
	lines, seen := &atomic.Int64{}, &refusalRecords{}
	previous := slog.Default()
	slog.SetDefault(slog.New(refusalLines{Handler: previous.Handler(), lines: lines, seen: seen}))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return lines, seen
}

func countRefusalLines(t *testing.T) *atomic.Int64 {
	t.Helper()
	lines, _ := captureRefusalLines(t)
	return lines
}

// refuseAtTheEngine answers one publish with a declare refusal of each name the
// publish carries, the way the engine refuses a name another plugin holds.
func refuseAtTheEngine(t *testing.T, m *MetricsPublisher) {
	t.Helper()
	in, sent := m.collect(false)
	refusals := make([]rpc.MetricRefusal, 0, len(in.Declare))
	for i := range in.Declare {
		refusals = append(refusals, rpc.MetricRefusal{Item: "declare", Name: in.Declare[i].Name, Reason: "another plugin holds it"})
	}
	m.settle(sent, refusals)
}

// VALIDATES: a name over rpc.MetricNameMax reports itself as that: one WARN line
// that says so, a logged name and reason cut to size, and the "too many names" line
// left for the condition it describes.
// METHOD: one oversized name with nothing else refused, then a second one, then
// enough refused names to reach the bound.
// PREVENTS: one long name logging "too many refused metric names", writing itself
// to the log whole, and using up the one line the real overflow owes.
func TestMetricsOversizedNameIsItsOwnRefusal(t *testing.T) {
	lines, seen := captureRefusalLines(t)
	m := NewMetricsPublisher()
	oversized := strings.Repeat("n", 16*rpc.MetricNameMax)

	m.Counter(oversized, "Refused.").Inc()
	m.Gauge(oversized+"x", "Refused.").Set(1)

	require.Equal(t, int64(1), lines.Load(), "oversized names share one WARN line")
	first := seen.all()[0]
	assert.NotContains(t, first.message, "too many")
	assert.Contains(t, first.reason, "above the limit")
	assert.LessOrEqual(t, len(first.metric), rpc.MetricNameMax+len("..."), "the log holds a cut name")
	assert.Less(t, len(first.reason), rpc.MetricNameMax, "the reason does not repeat the name")
	held := m.family(rpc.MetricDeclaration{Name: oversized, Kind: rpc.MetricCounter})
	assert.LessOrEqual(t, len(held.decl.Name), rpc.MetricNameMax, "the handle keeps no more than the limit")
	assert.Less(t, len(held.refused), rpc.MetricNameMax, "the handle keeps no more than the limit")

	for i := range rpc.MetricsDeclaredMax + 1 {
		m.Counter("not_prefixed_"+strconv.Itoa(i), "Refused.")
	}
	all := seen.all()
	assert.Equal(t, int64(rpc.MetricsDeclaredMax+2), lines.Load(), "the overflow line is still owed")
	assert.Contains(t, all[len(all)-1].message, "too many")
}

// VALIDATES: a name the engine refuses keeps only its name and kind, below the bound
// and with histogram buckets among what it drops.
// PREVENTS: the type comment's "by name and kind" meaning the whole declaration, so
// each remembered name pinned its help, labels and buckets.
func TestMetricsEngineRefusalKeepsOnlyNameAndKind(t *testing.T) {
	countRefusalLines(t)
	m := NewMetricsPublisher()
	vec := m.HistogramVec("ze_refused_seconds", "Refused by the engine.", []float64{1, 2}, []string{"peer"})
	series := vec.With("a")
	scalar := m.Histogram("ze_refused_total_seconds", "Refused by the engine.", []float64{1, 2})
	require.NotEmpty(t, m.byName["ze_refused_seconds"].decl.Help, "the fixture passed local validation")
	require.Empty(t, m.byName["ze_refused_seconds"].refused, "the fixture passed local validation")

	refuseAtTheEngine(t, m)

	for _, name := range []string{"ze_refused_seconds", "ze_refused_total_seconds"} {
		require.Contains(t, m.byName, name)
		held := m.byName[name].decl
		assert.Equal(t, name, held.Name)
		assert.NotEmpty(t, held.Kind)
		assert.Empty(t, held.Help)
		assert.Empty(t, held.Labels)
		assert.Empty(t, held.Buckets)
	}
	series.Observe(1.5)
	scalar.Observe(1.5)
}

// VALIDATES: a name the engine refuses once the remembered refusals reach their
// bound is not evicted: a re-declaration finds it, sends nothing and warns nothing.
// It keeps its declared slot instead, so the set stays bounded, and only when those
// slots are gone does the one overflow line fire.
// METHOD: fill the remembered refusals, then refuse every declared slot at the
// engine, re-declaring each name after its refusal.
// PREVENTS: an evicted name being a new name again, which spends another RPC, another
// refusal and another WARN line on every cycle of a plugin that re-declares.
func TestMetricsEngineRefusalPastTheBoundStaysHeld(t *testing.T) {
	lines, seen := captureRefusalLines(t)
	m := NewMetricsPublisher()
	for i := range rpc.MetricsDeclaredMax {
		m.Counter("not_prefixed_"+strconv.Itoa(i), "Refused.")
	}
	require.Equal(t, int64(rpc.MetricsDeclaredMax), lines.Load())

	for i := range rpc.MetricsDeclaredMax {
		name := "ze_refused_" + strconv.Itoa(i)
		vec := m.CounterVec(name, "Refused.", []string{"peer"})
		vec.With("a").Inc()
		refuseAtTheEngine(t, m)
		require.Equal(t, int64(rpc.MetricsDeclaredMax+i+1), lines.Load(), "the engine's refusal is one WARN line")

		again := m.CounterVec(name, "Refused.", []string{"peer"})
		again.With("a").Inc()
		in, _ := m.collect(false)
		require.Empty(t, in.Declare, "a remembered refusal is not sent again")
		require.Equal(t, int64(rpc.MetricsDeclaredMax+i+1), lines.Load(), "a repeat says nothing new")
		require.Equal(t, 0, m.series, "the refused name gave its labeled slots back")
		require.Contains(t, m.byName, name, "the name stays held")
	}
	assert.Len(t, m.families, rpc.MetricsDeclaredMax, "past the bound a refused name keeps its declared slot")
	assert.Len(t, m.byName, 2*rpc.MetricsDeclaredMax, "the set stays bounded")

	m.CounterVec("ze_one_too_many", "Refused.", []string{"peer"})
	all := seen.all()
	assert.Contains(t, all[len(all)-1].message, "too many", "only now are names going unnamed")
	assert.NotContains(t, m.byName, "ze_one_too_many")
	assert.Len(t, m.byName, 2*rpc.MetricsDeclaredMax)
}

// VALIDATES: the MetricsPublisher type comment. A plugin deriving metric names
// from runtime data, every one refused, holds a bounded set however long it runs.
// METHOD: declare many times the bound, with each name unique and invalid.
// PREVENTS: byName gaining an entry before any check, so refused names accumulated
// for the life of the plugin while the comment said the set was bounded.
func TestMetricsRefusedNamesStayBounded(t *testing.T) {
	lines := countRefusalLines(t)
	m := NewMetricsPublisher()

	for i := range 4 * rpc.MetricsDeclaredMax {
		m.Counter("not_prefixed_"+strconv.Itoa(i), "Refused.").Inc()
	}

	assert.Len(t, m.byName, rpc.MetricsDeclaredMax, "refused names are remembered up to the bound")
	assert.Empty(t, m.families, "a refused name is never forwarded")
	assert.Equal(t, int64(rpc.MetricsDeclaredMax+1), lines.Load(),
		"each remembered name is reported once, and the rest by one further line")
}

// VALIDATES: refusal reporting survives the bound: a repeat of a remembered name
// says nothing new, and a handle from it detaches however it is used.
// PREVENTS: a remembered family that kept only its name and kind panicking a With
// over labels it no longer carries.
func TestMetricsRefusedNameWarnsOnceAndDetaches(t *testing.T) {
	lines := countRefusalLines(t)
	m := NewMetricsPublisher()

	first := m.CounterVec("not_prefixed", "Refused.", []string{"peer"})
	second := m.CounterVec("not_prefixed", "Refused.", []string{"peer"})

	assert.Equal(t, int64(1), lines.Load(), "one name, one WARN line")
	first.With("a").Inc()
	second.With("b").Inc()
	assert.False(t, second.Delete("b"), "a refused family holds no series to delete")
	assert.Equal(t, 0, m.series, "a refused family takes no labeled slot")
	assert.Panics(t, func() { m.Gauge("not_prefixed", "Refused.") },
		"a remembered name still catches a declaration under another kind")
}

// VALIDATES: a refused name's declaration is not retained: only the name and kind
// of a name within the limit, and nothing of one beyond it.
// PREVENTS: the bound on names counting entries while each one pinned an unbounded
// name, help text or label list.
func TestMetricsRefusedDeclarationHoldsNoPayload(t *testing.T) {
	countRefusalLines(t)
	m := NewMetricsPublisher()

	m.CounterVec("not_prefixed", strings.Repeat("h", 4096), []string{"peer"})
	require.Contains(t, m.byName, "not_prefixed")
	held := m.byName["not_prefixed"].decl
	assert.Empty(t, held.Help)
	assert.Empty(t, held.Labels)

	oversized := strings.Repeat("n", rpc.MetricNameMax+1)
	m.Counter(oversized, "Refused.").Inc()
	assert.NotContains(t, m.byName, oversized, "a name past the limit is not held")
}

// VALIDATES: a name the engine refuses after the plugin held it gives back its
// declared slot and every labeled-series slot, and its handles detach.
// METHOD: a recorder answering the publish with a declare refusal, then a second
// family taking the slots the first one held.
// PREVENTS: series of a refused family holding m.series until each was deleted,
// which starved every healthy vector of the plugin.
func TestMetricsEngineRefusalReleasesTheFamily(t *testing.T) {
	countRefusalLines(t)
	m := NewMetricsPublisher()
	refused, err := json.Marshal(&rpc.MetricsPublishOutput{Refused: []rpc.MetricRefusal{{
		Item: "declare", Name: "ze_refused_events", Reason: "another plugin holds it",
	}}})
	require.NoError(t, err)
	recorder := &publishRecorder{inputs: make(chan rpc.MetricsPublishInput, 8), answers: []string{string(refused)}}

	vec := m.CounterVec("ze_refused_events", "Refused by the engine.", []string{"peer"})
	held := vec.With("a")
	for i := range rpc.MetricsSeriesMax {
		vec.With(strconv.Itoa(i))
	}
	require.Equal(t, rpc.MetricsSeriesMax, m.series, "the fixture fills every labeled slot")

	require.NoError(t, m.publish(t.Context(), recorder.call, true))

	assert.Equal(t, 0, m.series, "a refused family gives its labeled slots back")
	assert.Empty(t, m.families, "a refused family gives its declared slot back")
	assert.Contains(t, m.byName, "ze_refused_events", "the name stays held, so a repeat warns no further")
	held.Inc()

	healthy := m.CounterVec("ze_healthy_events", "Healthy.", []string{"peer"})
	healthy.With("a").Inc()
	assert.Equal(t, 1, m.series, "a healthy vector takes the freed slots")
}
