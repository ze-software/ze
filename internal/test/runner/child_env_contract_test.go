// Design: docs/architecture/testing/runner-architecture.md -- child environment ownership.
// Related: runner_exec_util_test.go -- runtime child environment assertions.

package runner_test

import (
	"strings"
	"testing"

	lescratch "github.com/ze-software/ze/internal/le/scratch"
	"github.com/ze-software/ze/internal/test/runner"
)

// TestChildEnvStoreTrimKeyMatchesScratch compares the runner's actual child key
// with le's real setting. The external package avoids scratch's job dependency
// importing runner back into its own internal tests.
func TestChildEnvStoreTrimKeyMatchesScratch(t *testing.T) {
	if got := strings.ToLower(strings.ReplaceAll(lescratch.StoreTrimKey, ".", "_")); got != runner.StoreTrimVariableForTest {
		t.Fatalf("the runner switches %q, le reads %q", runner.StoreTrimVariableForTest, got)
	}
}
