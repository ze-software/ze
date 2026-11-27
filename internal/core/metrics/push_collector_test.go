package metrics

import (
	"io"
	"log/slog"
	"slices"
	"testing"
)

func resetPushCollectorsForTest(t *testing.T) {
	t.Helper()
	restore := SnapshotPushCollectorsForTest()
	pushCollectorsMu.Lock()
	pushCollectors = map[string]PushCollectorStarter{}
	pushCollectorsMu.Unlock()
	t.Cleanup(restore)
}

// TestPushCollectorRegisterStartAndStop verifies push-collector registration
// validation, deterministic sorted start order, nil-stop handling, and
// idempotent reverse-order shutdown.
//
// VALIDATES: RegisterPushCollector rejects empty names, nil starters, and
// duplicates; StartPushCollectors runs starters in sorted name order and
// returns a stop closure that invokes non-nil stops once in reverse order.
// PREVENTS: silent duplicate overwrite, non-deterministic start/stop ordering,
// double-stop panics, and global registry contamination across tests.
func TestPushCollectorRegisterStartAndStop(t *testing.T) {
	resetPushCollectorsForTest(t)

	if err := RegisterPushCollector("", func(Registry, *slog.Logger) func() { return nil }); err == nil {
		t.Fatal("RegisterPushCollector accepted empty name")
	}
	if err := RegisterPushCollector("nil-starter", nil); err == nil {
		t.Fatal("RegisterPushCollector accepted nil starter")
	}

	var wantReg Registry = NopRegistry{}
	wantLog := slog.New(slog.NewTextHandler(io.Discard, nil))

	var started []string
	var stopped []string

	makeStarter := func(name string, withStop bool) PushCollectorStarter {
		return func(reg Registry, log *slog.Logger) func() {
			if reg != wantReg {
				t.Errorf("collector %q received unexpected registry", name)
			}
			if log != wantLog {
				t.Errorf("collector %q received unexpected logger", name)
			}
			started = append(started, name)
			if !withStop {
				return nil
			}
			return func() {
				stopped = append(stopped, name)
			}
		}
	}

	if err := RegisterPushCollector("beta", makeStarter("beta", true)); err != nil {
		t.Fatalf("RegisterPushCollector(beta): %v", err)
	}
	if err := RegisterPushCollector("alpha", makeStarter("alpha", true)); err != nil {
		t.Fatalf("RegisterPushCollector(alpha): %v", err)
	}
	if err := RegisterPushCollector("gamma-nil-stop", makeStarter("gamma-nil-stop", false)); err != nil {
		t.Fatalf("RegisterPushCollector(gamma-nil-stop): %v", err)
	}
	if err := RegisterPushCollector("alpha", makeStarter("alpha-dup", true)); err == nil {
		t.Fatal("RegisterPushCollector accepted duplicate name")
	}

	stop := StartPushCollectors(wantReg, wantLog)
	wantStarted := []string{"alpha", "beta", "gamma-nil-stop"}
	if !slices.Equal(started, wantStarted) {
		t.Fatalf("started order = %v, want %v", started, wantStarted)
	}
	if len(stopped) != 0 {
		t.Fatalf("stopped before stop() = %v, want empty", stopped)
	}

	stop()
	stop()
	wantStopped := []string{"beta", "alpha"}
	if !slices.Equal(stopped, wantStopped) {
		t.Fatalf("stopped order = %v, want %v", stopped, wantStopped)
	}
}
