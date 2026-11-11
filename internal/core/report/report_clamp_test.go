package report

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestCapClampRecordedForDoctor proves a cap the environment sets outside 1
// to its maximum is recorded where the doctor check doctor-env-invalid-value
// reads it, so the correction is more than a log line at start.
//
// VALIDATES: AC-42 env half: newStoreFromEnv clamps ze.report.warnings.max set
// above its maximum and answers ze.report.errors.max set to 0 with the
// default; env.InvalidValues lists each as out of range with the value set,
// the range and the value used.
// PREVENTS: an operator's export silently replaced with only a start-up log
// line, or with nothing at all for a zero (plan/journal/zero-value-as-valid-answer.md).
func TestCapClampRecordedForDoctor(t *testing.T) {
	t.Setenv("ze.report.warnings.max", "20000")
	t.Setenv("ze.report.errors.max", "0")
	env.ResetCache()
	t.Cleanup(func() {
		os.Unsetenv("ze.report.warnings.max") //nolint:errcheck // t.Setenv restores; the cache must forget too
		os.Unsetenv("ze.report.errors.max")   //nolint:errcheck // same
		env.ResetCache()
	})

	s := newStoreFromEnv()
	if s.warningCap != maxWarningCap {
		t.Fatalf("warning cap = %d, want clamped to %d", s.warningCap, maxWarningCap)
	}
	if s.errorCap != defaultErrorCap {
		t.Fatalf("error cap = %d, want the default %d", s.errorCap, defaultErrorCap)
	}

	type want struct{ value, used string }
	wants := map[string]want{
		envWarningCap: {value: "20000", used: "10000"},
		envErrorCap:   {value: "0", used: "256"},
	}
	for _, v := range env.InvalidValues() {
		w, named := wants[v.Key]
		if !named {
			continue
		}
		if v.Kind != env.InvalidOutOfRange || v.Value != w.value || v.Used != w.used || v.Range != "1 to 10000" {
			t.Errorf("%s recorded as %+v; want out of range 1 to 10000, value %s, used %s", v.Key, v, w.value, w.used)
		}
		delete(wants, v.Key)
	}
	for key := range wants {
		t.Errorf("%s corrected but not recorded for doctor", key)
	}
}
