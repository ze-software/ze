package registry

import (
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

// VALIDATES: a plugin started before any metrics registry exists still gets its
// ConfigureMetrics hook, driven by whichever of spawn and SetMetricsRegistry
// happens second.
// PREVENTS: the regression where GetInternalPluginRunner read
// GetMetricsRegistry() at spawn time, found nil (runPluginPhase spawns every
// process before the tier handshake, and the registry is created inside the bgp
// plugin's stage-2 OnConfigure) and dropped the hook, so no internal plugin's
// counters were ever registered -- `show metrics values` carried 38 ze_* series
// and not one came from a plugin.
func TestInjectPluginMetricsDeferredUntilRegistryExists(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	var got metrics.Registry
	calls := 0
	InjectPluginMetrics("bgp-role", func(r metrics.Registry) {
		got = r
		calls++
	})

	if calls != 0 {
		t.Fatalf("hook ran with no registry set: calls=%d", calls)
	}

	mr := &fakeMetricsRegistry{}
	SetMetricsRegistry(mr)

	if calls != 1 {
		t.Fatalf("hook ran %d times after SetMetricsRegistry, want 1", calls)
	}
	if got != metrics.Registry(mr) {
		t.Errorf("hook received %v, want %v", got, mr)
	}
}

// VALIDATES: a plugin spawned after the registry already exists is configured
// immediately, and is NOT configured a second time by a later
// SetMetricsRegistry draining the pending set.
// PREVENTS: double registration of the same collector, which the Prometheus
// registry rejects.
func TestInjectPluginMetricsRunsOnceWhenRegistryAlreadySet(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	mr := &fakeMetricsRegistry{}
	SetMetricsRegistry(mr)

	calls := 0
	InjectPluginMetrics("bgp-rib", func(_ metrics.Registry) { calls++ })
	if calls != 1 {
		t.Fatalf("hook ran %d times on inject with a registry set, want 1", calls)
	}

	// A later set (config reload builds a fresh registry) must not re-run a hook
	// that already ran against the previous one.
	SetMetricsRegistry(&fakeMetricsRegistry{})
	if calls != 1 {
		t.Errorf("hook ran %d times after a second SetMetricsRegistry, want 1", calls)
	}

	// Nor may a repeated inject for the same plugin.
	InjectPluginMetrics("bgp-rib", func(_ metrics.Registry) { calls++ })
	if calls != 1 {
		t.Errorf("hook ran %d times after a repeated inject, want 1", calls)
	}
}

// VALIDATES: every deferred plugin is drained, not just the first.
// PREVENTS: a partial drain leaving later-registered plugins permanently
// uncounted -- the failure would look exactly like the original bug for every
// plugin but one.
func TestSetMetricsRegistryDrainsEveryPendingPlugin(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	seen := map[string]int{}
	for _, name := range []string{"bgp-role", "bgp-rib", "bgp-gr"} {
		InjectPluginMetrics(name, func(_ metrics.Registry) { seen[name]++ })
	}
	if len(seen) != 0 {
		t.Fatalf("hooks ran before a registry existed: %v", seen)
	}

	SetMetricsRegistry(&fakeMetricsRegistry{})

	for _, name := range []string{"bgp-role", "bgp-rib", "bgp-gr"} {
		if seen[name] != 1 {
			t.Errorf("plugin %q hook ran %d times, want 1 (all pending hooks drained)", name, seen[name])
		}
	}
}

// VALIDATES: a nil registry leaves the deferred hooks deferred, so the real
// registry that arrives afterwards still configures them.
// PREVENTS: a nil set draining the pending map, marking every plugin configured
// and running nothing -- the original bug with an extra step, and invisible
// because both symptoms are "no plugin metrics".
func TestSetMetricsRegistryNilKeepsHooksDeferred(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	calls := 0
	InjectPluginMetrics("bgp-role", func(_ metrics.Registry) { calls++ })

	SetMetricsRegistry(nil)
	if calls != 0 {
		t.Fatalf("hook ran against a nil registry: calls=%d", calls)
	}

	SetMetricsRegistry(&fakeMetricsRegistry{})
	if calls != 1 {
		t.Errorf("hook ran %d times after a real registry arrived, want 1", calls)
	}
}

// VALIDATES: Reset clears the deferred hooks and the configured marks with the
// registry they belong to.
// PREVENTS: cross-test leakage where one test's pending hook fires inside
// another test's SetMetricsRegistry, or a stale "configured" mark suppresses a
// hook that genuinely needs to run.
func TestResetClearsMetricsInjectionState(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	leaked := 0
	InjectPluginMetrics("bgp-role", func(_ metrics.Registry) { leaked++ })

	Reset()
	SetMetricsRegistry(&fakeMetricsRegistry{})
	if leaked != 0 {
		t.Errorf("pending hook survived Reset and ran %d times", leaked)
	}

	// The configured mark is cleared too, so the plugin can be configured again
	// against the new registry.
	calls := 0
	InjectPluginMetrics("bgp-role", func(_ metrics.Registry) { calls++ })
	if calls != 1 {
		t.Errorf("hook ran %d times after Reset, want 1", calls)
	}
}
