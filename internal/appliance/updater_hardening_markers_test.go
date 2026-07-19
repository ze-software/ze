package appliance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The vendored gokrazy/updater is a hard fork: after every `go mod vendor`,
// scripts/dev/reapply-updater-fixes.py re-applies local DoS-hardening fixes
// (the upstream PR in scripts/dev/gokrazy-updater-upstream.patch is unmerged).
// A re-vendor that forgets to re-run that script silently drops the caps and
// reopens the DoS. This test is the durable guard: it asserts every DoS-hardening
// marker survives in the vendored source, so the unit gate goes red the moment
// a re-vendor regresses the fork. When the upstream PR merges, delete the fork,
// the reapply script, and this test, then pin the fixed tag (spec AC-3 alt path).

// updaterHardeningMarkers maps each required marker literal to the minimum
// number of times it must appear in the vendored updater. Keep in lockstep
// with scripts/dev/reapply-updater-fixes.py.
var updaterHardeningMarkers = []struct {
	label   string
	literal string
	min     int
}{
	// Two io.LimitReader DoS caps: StreamTo's remoteHash read and
	// requestFeatures' body read (apply_limitreader in the reapply script).
	{"io.LimitReader DoS cap", "io.LimitReader(resp.Body, 1<<20)", 2},
	// Every request with a nil body uses http.NoBody (apply_http_nobody).
	{"http.NoBody request body", "http.NoBody", 1},
	// Response bodies are always closed (apply_resp_body_close).
	{"defer resp.Body.Close()", "defer resp.Body.Close()", 1},
	// NOTE: the reapply script also rewrites Supports() to slices.Contains,
	// but that is a cosmetic refactor, not DoS hardening: a re-vendor that
	// reverts it to the equivalent loop is harmless, so it is deliberately
	// NOT guarded here. Only the three DoS-relevant markers above are gated.
}

// vendoredUpdaterPath returns the absolute path to the vendored updater source,
// resolved relative to this test file so it is independent of the working dir.
func vendoredUpdaterPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed: cannot locate test file")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(repoRoot, "vendor", "github.com", "gokrazy", "updater", "updater.go")
}

// countMarkers returns how many times each marker literal appears in src.
func countMarkers(src string) map[string]int {
	counts := make(map[string]int, len(updaterHardeningMarkers))
	for _, m := range updaterHardeningMarkers {
		counts[m.literal] = strings.Count(src, m.literal)
	}
	return counts
}

func TestUpdaterHardeningMarkersPresent(t *testing.T) {
	path := vendoredUpdaterPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading vendored updater %s: %v (run go mod vendor)", path, err)
	}

	counts := countMarkers(string(data))
	for _, m := range updaterHardeningMarkers {
		got := counts[m.literal]
		if got < m.min {
			t.Errorf("hardening marker %q (%s) found %d time(s), want >= %d; "+
				"a re-vendor dropped it -- re-run scripts/dev/reapply-updater-fixes.py",
				m.literal, m.label, got, m.min)
		}
	}
}

// TestUpdaterHardeningMarkersDetectRegression is the boundary case: it proves
// the marker-counting logic actually fails when a marker is missing, so a
// passing TestUpdaterHardeningMarkersPresent is meaningful and not vacuous.
func TestUpdaterHardeningMarkersDetectRegression(t *testing.T) {
	// Source with Body.Close and NoBody but only ONE LimitReader cap: a
	// realistic partial regression a sloppy re-vendor could produce.
	regressed := "" +
		"defer resp.Body.Close()\n" +
		"req, _ := http.NewRequestWithContext(ctx, \"GET\", u, http.NoBody)\n" +
		"body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))\n"

	counts := countMarkers(regressed)

	var missed int
	for _, m := range updaterHardeningMarkers {
		if counts[m.literal] < m.min {
			missed++
		}
	}
	if missed == 0 {
		t.Fatal("marker check passed on regressed source that has only one LimitReader cap; " +
			"the guard is vacuous and would not catch a real re-vendor regression")
	}
}
