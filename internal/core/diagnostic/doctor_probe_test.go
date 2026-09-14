// Design: docs/features/ai-first.md -- reachability probe timeout override tests

package diagnostic

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// TestDoctorProbeTimeout verifies the DoctorProbeTimeoutEnv override only ever
// shortens a probe timeout (so functional tests fail fast against unreachable
// fixtures) and never lengthens it (so production keeps its per-check default).
func TestDoctorProbeTimeout(t *testing.T) {
	const def = 5 * time.Second

	// Unset: returns the per-check default.
	if got := DoctorProbeTimeout(def); got != def {
		t.Fatalf("unset: got %v, want %v", got, def)
	}

	t.Run("smaller override caps the timeout", func(t *testing.T) {
		if err := env.Set(DoctorProbeTimeoutEnv, "250ms"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = env.Set(DoctorProbeTimeoutEnv, "") })
		if got := DoctorProbeTimeout(def); got != 250*time.Millisecond {
			t.Fatalf("override 250ms with 5s default: got %v, want 250ms", got)
		}
		// A check whose own default is already smaller than the override keeps
		// its default -- the override is a cap, not a floor.
		if got := DoctorProbeTimeout(100 * time.Millisecond); got != 100*time.Millisecond {
			t.Fatalf("override 250ms with 100ms default: got %v, want 100ms", got)
		}
	})

	t.Run("larger override does not lengthen", func(t *testing.T) {
		if err := env.Set(DoctorProbeTimeoutEnv, "30s"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = env.Set(DoctorProbeTimeoutEnv, "") })
		if got := DoctorProbeTimeout(def); got != def {
			t.Fatalf("override 30s with 5s default: got %v, want %v (cap only shortens)", got, def)
		}
	})

	t.Run("invalid override falls back to default", func(t *testing.T) {
		if err := env.Set(DoctorProbeTimeoutEnv, "not-a-duration"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = env.Set(DoctorProbeTimeoutEnv, "") })
		if got := DoctorProbeTimeout(def); got != def {
			t.Fatalf("invalid override: got %v, want %v", got, def)
		}
	})
}

// TestDoctorProbeWritableDirRefusesAnEmptyPath pins the one input that must not
// be read as "writable": an unset config leaf arrives here as "".
//
// VALIDATES: an empty directory is an error rather than a silent success.
// PREVENTS: a check reporting a destination as usable because nothing was
// configured for it (docs/contributing/ze-go-style.md, "A zero value is never
// an answer").
func TestDoctorProbeWritableDirRefusesAnEmptyPath(t *testing.T) {
	if err := DoctorProbeWritableDir(""); err == nil {
		t.Fatal("an empty directory must be an error, not a writable destination")
	}
	if err := DoctorProbeWritableDir(t.TempDir()); err != nil {
		t.Fatalf("a fresh temp dir must be writable: %v", err)
	}
}
