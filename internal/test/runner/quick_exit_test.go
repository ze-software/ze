// Design: docs/architecture/testing/ci-format.md -- quick-ze exit assertions.

package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestQuickZeCommandExit runs native children through the quick-ze arm and keeps
// successful zero, matching nonzero, and mismatching statuses distinct.
func TestQuickZeCommandExit(t *testing.T) {
	for _, tc := range []struct {
		name             string
		actual, expected int
		pass             bool
	}{
		{"zero", 0, 0, true},
		{"nonzero", 7, 7, true},
		{"mismatch", 7, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, rec := helperExitRecord(t, "cmd=foreground:seq=1:exec=ze version:exit="+strconv.Itoa(tc.expected)+"\n")
			r.zePath = filepath.Join(t.TempDir(), "ze")
			if err := os.WriteFile(r.zePath, []byte("#!/bin/sh\necho quick-output\nexit "+strconv.Itoa(tc.actual)+"\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			if passed := r.runTest(t.Context(), rec, &RunOptions{}); passed != tc.pass {
				t.Fatalf("passed=%v, error=%v", passed, rec.Error)
			}
			if !strings.Contains(rec.ClientOutput, "quick-output") {
				t.Fatalf("quick output was lost: %q", rec.ClientOutput)
			}
			if !tc.pass && rec.FailureType != failTypeExitCodeMismatch {
				t.Fatalf("failure=%q, error=%v", rec.FailureType, rec.Error)
			}
		})
	}
}
