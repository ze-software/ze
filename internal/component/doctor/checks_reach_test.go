// Design: docs/features/ai-first.md — reachability probe timeout override tests

package doctor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// TestReachProbeTimeout verifies the doctorProbeTimeoutEnv override only ever
// shortens a probe timeout (so functional tests fail fast against unreachable
// fixtures) and never lengthens it (so production keeps its per-check default).
func TestReachProbeTimeout(t *testing.T) {
	const def = 5 * time.Second

	// Unset: returns the per-check default.
	if got := reachProbeTimeout(def); got != def {
		t.Fatalf("unset: got %v, want %v", got, def)
	}

	t.Run("smaller override caps the timeout", func(t *testing.T) {
		if err := env.Set(doctorProbeTimeoutEnv, "250ms"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = env.Set(doctorProbeTimeoutEnv, "") })
		if got := reachProbeTimeout(def); got != 250*time.Millisecond {
			t.Fatalf("override 250ms with 5s default: got %v, want 250ms", got)
		}
		// A check whose own default is already smaller than the override keeps
		// its default — the override is a cap, not a floor.
		if got := reachProbeTimeout(100 * time.Millisecond); got != 100*time.Millisecond {
			t.Fatalf("override 250ms with 100ms default: got %v, want 100ms", got)
		}
	})

	t.Run("larger override does not lengthen", func(t *testing.T) {
		if err := env.Set(doctorProbeTimeoutEnv, "30s"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = env.Set(doctorProbeTimeoutEnv, "") })
		if got := reachProbeTimeout(def); got != def {
			t.Fatalf("override 30s with 5s default: got %v, want %v (cap only shortens)", got, def)
		}
	})

	t.Run("invalid override falls back to default", func(t *testing.T) {
		if err := env.Set(doctorProbeTimeoutEnv, "not-a-duration"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = env.Set(doctorProbeTimeoutEnv, "") })
		if got := reachProbeTimeout(def); got != def {
			t.Fatalf("invalid override: got %v, want %v", got, def)
		}
	})
}
