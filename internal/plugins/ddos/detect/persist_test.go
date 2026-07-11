package detect

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func makeSamples(n int, v float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = v
	}
	return s
}

// VALIDATES: a persisted snapshot round-trips through save + load unchanged.
func TestSaveLoadBaselinesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "ddos-detect-baseline.json")
	pps := baselineState{Samples: makeSamples(300, 1000), Count: 300, P99Cache: 1000}
	bps := baselineState{Samples: makeSamples(300, 20000), Count: 300, P99Cache: 20000}
	if err := saveBaselines(path, pps, bps); err != nil {
		t.Fatalf("save: %v", err)
	}
	blob, ok := loadBaselines(path)
	if !ok {
		t.Fatal("load returned ok=false for a freshly saved file")
	}
	if blob.Version != baselineStateVersion {
		t.Errorf("version = %d, want %d", blob.Version, baselineStateVersion)
	}
	if len(blob.Pps.Samples) != 300 || blob.Pps.P99Cache != 1000 {
		t.Errorf("pps state not preserved: %+v", blob.Pps)
	}
	if len(blob.Bps.Samples) != 300 || blob.Bps.P99Cache != 20000 {
		t.Errorf("bps state not preserved: %+v", blob.Bps)
	}
}

// VALIDATES: AC-6 -- a version mismatch is rejected (warm fresh, no crash).
func TestLoadBaselines_RejectsVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, []byte(`{"version":999,"pps":{"samples":[1,2,3]},"bps":{"samples":[]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadBaselines(path); ok {
		t.Error("expected ok=false for a version mismatch")
	}
}

// VALIDATES: AC-6 -- a missing/corrupt file is rejected without error.
func TestLoadBaselines_MissingAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	if _, ok := loadBaselines(filepath.Join(dir, "absent.json")); ok {
		t.Error("expected ok=false for a missing file")
	}
	corrupt := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte(`{not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadBaselines(corrupt); ok {
		t.Error("expected ok=false for corrupt JSON")
	}
}

// VALIDATES: AC-5 -- a warmed baseline snapshot restores into a fresh baseline,
// preserving readiness and p99 (the restart skips the window re-warm).
func TestBaselineSnapshotRestoreRoundTrip(t *testing.T) {
	src := newBaseline(300, 3.0, 5000)
	for range 300 {
		src.Add(1000, false)
	}
	st := src.snapshot()

	dst := newBaseline(300, 3.0, 5000)
	if !dst.restore(st) {
		t.Fatal("restore rejected a valid full snapshot")
	}
	if !dst.Ready() {
		t.Error("restored baseline should be Ready (no warm-up needed)")
	}
	if dst.P99() != src.P99() {
		t.Errorf("restored p99 = %v, want %v", dst.P99(), src.P99())
	}
}

// VALIDATES: AC-6 -- restore rejects too-few-samples, NaN, Inf, and negative values,
// leaving the baseline empty so it warms fresh.
func TestBaselineRestore_RejectsBadState(t *testing.T) {
	cases := []struct {
		name string
		st   baselineState
	}{
		{"too-few", baselineState{Samples: makeSamples(10, 1000), Count: 10, P99Cache: 1000}},
		{"nan-sample", baselineState{Samples: append(makeSamples(60, 1000), math.NaN()), Count: 61, P99Cache: 1000}},
		{"inf-sample", baselineState{Samples: append(makeSamples(60, 1000), math.Inf(1)), Count: 61, P99Cache: 1000}},
		{"negative-sample", baselineState{Samples: append(makeSamples(60, 1000), -5), Count: 61, P99Cache: 1000}},
		{"nan-p99", baselineState{Samples: makeSamples(60, 1000), Count: 60, P99Cache: math.NaN()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBaseline(300, 3.0, 5000)
			if b.restore(tc.st) {
				t.Error("restore accepted invalid state")
			}
			if b.Ready() || len(b.samples) != 0 {
				t.Error("rejected restore should leave the baseline empty")
			}
		})
	}
}
