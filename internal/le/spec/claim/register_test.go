package specclaim

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/spec"
)

// captureStderr runs fn with os.Stderr replaced by a temporary file and
// answers what fn wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("create the capture file: %v", err)
	}
	saved := os.Stderr
	os.Stderr = file
	fn()
	os.Stderr = saved
	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("read the capture file: %v", err)
	}
	return string(data)
}

// TestClaimWithoutASpecRefusesWithClaimsUsageOnly checks AC-9 for `le spec
// claim` before the area reaches an action table: with no spec on a pipe, stderr
// carries claim's own usage line and names no other member of spec, and the
// code is the lifecycle's usage code, 2. Method: call the registered handler
// with no words and read stderr.
func TestClaimWithoutASpecRefusesWithClaimsUsageOnly(t *testing.T) {
	var code int
	stderr := captureStderr(t, func() { _, code = spec.Claim(nil) })
	if code != 2 {
		t.Errorf("a claim with no spec answered %d, want 2", code)
	}
	if !strings.Contains(stderr, "\nusage: le spec claim <spec> ['|' json|yaml|table]\n") {
		t.Errorf("the refusal does not carry claim's usage line:\n%s", stderr)
	}
	for _, sibling := range []string{"release", "wip", "state", "model", "review", "current"} {
		if strings.Contains(stderr, sibling) {
			t.Errorf("the refusal names the sibling %q:\n%s", sibling, stderr)
		}
	}
}
