package firewall

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

// countingRegistry is the smallest metrics.Registry that can answer "was it
// counted": it records every observation and increment by metric name. Only
// Counter and HistogramVec are used by the firewall component; the other four
// methods satisfy the interface and are never called.
type countingRegistry struct {
	counters      map[string]*countingCounter
	histogramVecs map[string]*countingHistogramVec
}

type countingCounter struct{ n int }

func (c *countingCounter) Inc()          { c.n++ }
func (c *countingCounter) Add(_ float64) { c.n++ }

type countingHistogram struct{ observed []float64 }

func (h *countingHistogram) Observe(v float64) { h.observed = append(h.observed, v) }

// countingHistogramVec records observations per label value, so a test can ask
// under WHICH result a reconcile was timed, not merely whether it was timed. It
// also keeps the buckets it was registered with: discarding them would leave
// the histogram's ceiling asserted nowhere.
type countingHistogramVec struct {
	byLabel map[string]*countingHistogram
	buckets []float64
}

func (v *countingHistogramVec) With(labelValues ...string) metrics.Histogram {
	key := strings.Join(labelValues, ",")
	h, ok := v.byLabel[key]
	if !ok {
		h = &countingHistogram{}
		v.byLabel[key] = h
	}
	return h
}

func (v *countingHistogramVec) Delete(_ ...string) bool { return false }

// observations returns how many reconciles were recorded under one result.
func (v *countingHistogramVec) observations(result string) int {
	h, ok := v.byLabel[result]
	if !ok {
		return 0
	}
	return len(h.observed)
}

func newCountingRegistry() *countingRegistry {
	return &countingRegistry{
		counters:      make(map[string]*countingCounter),
		histogramVecs: make(map[string]*countingHistogramVec),
	}
}

func (r *countingRegistry) Counter(name, _ string) metrics.Counter {
	c := &countingCounter{}
	r.counters[name] = c
	return c
}

func (r *countingRegistry) HistogramVec(name, _ string, buckets []float64, _ []string) metrics.HistogramVec {
	v := &countingHistogramVec{byLabel: make(map[string]*countingHistogram), buckets: buckets}
	r.histogramVecs[name] = v
	return v
}

func (r *countingRegistry) Gauge(_, _ string) metrics.Gauge { panic("unused") }
func (r *countingRegistry) CounterVec(_, _ string, _ []string) metrics.CounterVec {
	panic("unused")
}
func (r *countingRegistry) GaugeVec(_, _ string, _ []string) metrics.GaugeVec { panic("unused") }
func (r *countingRegistry) Histogram(_, _ string, _ []float64) metrics.Histogram {
	panic("unused")
}

// errBackend fails every Apply with a fixed error, so a test can drive the
// registry's reconcile path to each outcome the metrics distinguish.
type errBackend struct{ err error }

func (e *errBackend) Apply(_ []Table) error                       { return e.err }
func (e *errBackend) ListTables() ([]Table, error)                { return nil, nil }
func (e *errBackend) GetCounters(string) ([]ChainCounters, error) { return nil, nil }
func (e *errBackend) Close() error                                { return nil }

// bindCountingMetrics binds a fresh counting registry and restores the previous
// binding afterwards, so one test cannot read another's counts.
func bindCountingMetrics(t *testing.T) *countingRegistry {
	t.Helper()
	prev := applyMetricsPtr.Load()
	t.Cleanup(func() { applyMetricsPtr.Store(prev) })
	reg := newCountingRegistry()
	bindMetrics(reg)
	return reg
}

// TestApplyAllCountsKernelTimeout pins AC-10 of
// fixit-firewall-concurrency-deadlock: "a wedged kernel ... the failure is
// logged and counted".
//
// VALIDATES: a reconcile that fails with ErrKernelTimeout increments
// ze_firewall_apply_timeout_total; any other outcome leaves it alone. Every
// reconcile, failed or not, observes ze_firewall_apply_duration_seconds.
//
// PREVENTS: a wedged kernel being visible only as a returned error. ApplyAll is
// the sole caller of Backend.Apply, so an owner that logs nothing (or swallows
// the error) would otherwise leave the kernel silently behind the registry,
// with no signal an operator can alert on.
func TestApplyAllCountsKernelTimeout(t *testing.T) {
	for _, tt := range []struct {
		name         string
		applyErr     error
		wantTimeouts int
		wantResult   string
	}{
		{"kernel timeout is counted", ErrKernelTimeout, 1, applyResultTimeout},
		{"wrapped kernel timeout is counted", fmt.Errorf("firewallnft: flush: %w", ErrKernelTimeout), 1, applyResultTimeout},
		{"a rejected ruleset is not a timeout", errors.New("EINVAL: bad rule"), 0, applyResultError},
		{"a successful reconcile is not a timeout", nil, 0, applyResultOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reg := bindCountingMetrics(t)
			installBackend(t, "errbackend", &errBackend{err: tt.applyErr})
			RegisterTables("owner", []Table{{Name: "ze_m", Family: FamilyInet}})

			err := ApplyAll()
			if !errors.Is(err, tt.applyErr) {
				t.Fatalf("ApplyAll error = %v, want %v", err, tt.applyErr)
			}

			c, ok := reg.counters["ze_firewall_apply_timeout_total"]
			if !ok {
				t.Fatal("ze_firewall_apply_timeout_total was never registered")
			}
			if c.n != tt.wantTimeouts {
				t.Errorf("timeout counter = %d, want %d", c.n, tt.wantTimeouts)
			}

			h, ok := reg.histogramVecs["ze_firewall_apply_duration_seconds"]
			if !ok {
				t.Fatal("ze_firewall_apply_duration_seconds was never registered")
			}
			if got := h.observations(tt.wantResult); got != 1 {
				t.Errorf("apply-duration observations under result=%q = %d, want 1", tt.wantResult, got)
			}
			// The label must SEPARATE outcomes: a 10s timeout and a 10s slow
			// success are the same latency, and only the result tells them
			// apart. Any other label carrying this reconcile means it does not.
			for _, other := range []string{applyResultOK, applyResultTimeout, applyResultError, applyResultPanic} {
				if other == tt.wantResult {
					continue
				}
				if got := h.observations(other); got != 0 {
					t.Errorf("reconcile also recorded under result=%q (%d observations)", other, got)
				}
			}
		})
	}
}

// panicBackend never returns from Apply, which is the one outcome a written
// (rather than deferred) observation cannot record.
type panicBackend struct{}

func (p *panicBackend) Apply(_ []Table) error                       { panic("backend exploded") }
func (p *panicBackend) ListTables() ([]Table, error)                { return nil, nil }
func (p *panicBackend) GetCounters(string) ([]ChainCounters, error) { return nil, nil }
func (p *panicBackend) Close() error                                { return nil }

// TestApplyDurationBucketsReachTheMaxDeadline ties the histogram's ceiling to
// the deadline ceiling both backends clamp to.
//
// VALIDATES: the buckets REGISTERED with the metrics registry carry a finite
// bucket at least as large as firewall.MaxBackendDeadline.
//
// PREVENTS: a max-deadline timeout landing in +Inf. The three places that must
// agree (this bucket list, the nft clamp, the vpp clamp) were three independent
// literals; raising a backend clamp alone then made every timeout at the new
// ceiling indistinguishable from any other overrun, with no test to notice.
// Asserting on the REGISTERED buckets rather than on the package variable is
// deliberate: the fake used to discard that argument, so a registration that
// passed the wrong list would have gone unseen.
func TestApplyDurationBucketsReachTheMaxDeadline(t *testing.T) {
	reg := bindCountingMetrics(t)

	h, ok := reg.histogramVecs["ze_firewall_apply_duration_seconds"]
	if !ok {
		t.Fatal("ze_firewall_apply_duration_seconds was never registered")
	}
	if len(h.buckets) == 0 {
		t.Fatal("registered with no buckets")
	}
	last := h.buckets[len(h.buckets)-1]
	if want := MaxBackendDeadline.Seconds(); last < want {
		t.Errorf("last finite bucket = %vs, want >= %vs (a max-deadline timeout would land in +Inf)", last, want)
	}
	for i := 1; i < len(h.buckets); i++ {
		if h.buckets[i] <= h.buckets[i-1] {
			t.Fatalf("buckets are not strictly increasing at index %d: %v", i, h.buckets)
		}
	}
}

// TestApplyAllRecordsAPanickingBackend pins the deferred observation.
//
// VALIDATES: a Backend.Apply that panics is still timed, and is recorded under
// result="panic" rather than as a healthy reconcile.
//
// PREVENTS: the shape this replaced, where the observation was written after
// the call: a panicking backend unwound past it, so the reconcile vanished from
// the histogram entirely and a timeout on that path would never have been
// logged. It also prevents the opposite error, filing the unwind as result="ok"
// because the error variable was still nil.
func TestApplyAllRecordsAPanickingBackend(t *testing.T) {
	reg := bindCountingMetrics(t)
	installBackend(t, "panicbackend", &panicBackend{})
	RegisterTables("owner", []Table{{Name: "ze_m", Family: FamilyInet}})

	func() {
		defer func() {
			if recover() == nil {
				t.Error("ApplyAll swallowed the backend panic; it must propagate")
			}
		}()
		_ = ApplyAll()
	}()

	h, ok := reg.histogramVecs["ze_firewall_apply_duration_seconds"]
	if !ok {
		t.Fatal("ze_firewall_apply_duration_seconds was never registered")
	}
	if got := h.observations(applyResultPanic); got != 1 {
		t.Errorf("observations under result=%q = %d, want 1", applyResultPanic, got)
	}
	if got := h.observations(applyResultOK); got != 0 {
		t.Errorf("a panicking reconcile was recorded as healthy (%d observations under ok)", got)
	}
}

// lockProbeRegistry answers one question at observation time: was the
// process-wide reconcile lock still held? TryLock succeeds only when nobody
// holds it, and the probe releases immediately so it disturbs nothing.
type lockProbeRegistry struct {
	*countingRegistry
	heldDuringObserve bool
}

func (r *lockProbeRegistry) HistogramVec(name, help string, buckets []float64, labels []string) metrics.HistogramVec {
	inner := r.countingRegistry.HistogramVec(name, help, buckets, labels)
	return &lockProbeHistogramVec{inner: inner, owner: r}
}

type lockProbeHistogramVec struct {
	inner metrics.HistogramVec
	owner *lockProbeRegistry
}

func (v *lockProbeHistogramVec) With(labelValues ...string) metrics.Histogram {
	return &lockProbeHistogram{inner: v.inner.With(labelValues...), owner: v.owner}
}

func (v *lockProbeHistogramVec) Delete(labelValues ...string) bool {
	return v.inner.Delete(labelValues...)
}

type lockProbeHistogram struct {
	inner metrics.Histogram
	owner *lockProbeRegistry
}

func (h *lockProbeHistogram) Observe(f float64) {
	if reconcileMu.TryLock() {
		reconcileMu.Unlock()
	} else {
		h.owner.heldDuringObserve = true
	}
	h.inner.Observe(f)
}

// TestApplyAllObservesOutsideTheReconcileLock pins WHERE the observation runs.
//
// VALIDATES: observeApply is called after ApplyAll has released reconcileMu.
//
// PREVENTS: extending the hold on the process-wide reconcile lock, which is the
// one thing this spec exists to keep short. observeApply writes a slog Error on
// a timeout, so ordering it inside the lock puts a syscall on the path every
// other firewall owner is queued behind, at exactly the moment the dataplane is
// already misbehaving. Defer order is the whole mechanism and it is invisible:
// registering the observation after the unlock defer silently reverses it.
func TestApplyAllObservesOutsideTheReconcileLock(t *testing.T) {
	prev := applyMetricsPtr.Load()
	t.Cleanup(func() { applyMetricsPtr.Store(prev) })
	probe := &lockProbeRegistry{countingRegistry: newCountingRegistry()}
	bindMetrics(probe)

	installBackend(t, "lockprobe", &errBackend{err: ErrKernelTimeout})
	RegisterTables("owner", []Table{{Name: "ze_m", Family: FamilyInet}})

	if err := ApplyAll(); !errors.Is(err, ErrKernelTimeout) {
		t.Fatalf("ApplyAll error = %v, want ErrKernelTimeout", err)
	}

	h, ok := probe.histogramVecs["ze_firewall_apply_duration_seconds"]
	if !ok || h.observations(applyResultTimeout) != 1 {
		t.Fatal("the reconcile was never observed, so the probe proves nothing")
	}
	if probe.heldDuringObserve {
		t.Error("observeApply ran while reconcileMu was held: the log and metric write extend the reconcile hold")
	}
}

// TestApplyAllWithoutMetricsRegistry covers the window before
// Registration.ConfigureMetrics runs, and the case where telemetry is disabled
// altogether.
//
// VALIDATES: a reconcile still completes and still returns its error when no
// metrics registry has been bound.
//
// PREVENTS: a nil-pointer panic on the one path every firewall owner shares.
// The registry is injected after the plugin starts, so the daemon reconciles
// through this path at least once with nothing bound.
func TestApplyAllWithoutMetricsRegistry(t *testing.T) {
	prev := applyMetricsPtr.Load()
	t.Cleanup(func() { applyMetricsPtr.Store(prev) })
	applyMetricsPtr.Store(nil)

	installBackend(t, "errbackend-unbound", &errBackend{err: ErrKernelTimeout})
	RegisterTables("owner", []Table{{Name: "ze_m", Family: FamilyInet}})

	if err := ApplyAll(); !errors.Is(err, ErrKernelTimeout) {
		t.Fatalf("ApplyAll error = %v, want ErrKernelTimeout", err)
	}
}
