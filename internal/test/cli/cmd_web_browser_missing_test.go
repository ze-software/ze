// Design: docs/architecture/testing/ci-format.md -- web browser test CLI.
// Related: cmd_web_test.go -- webBrowserMissing judged on its own.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// webMissingBrowserPattern selects one committed .wb test the suite can run.
const webMissingBrowserPattern = "admin-nav"

// TestWebSuiteMissingBrowserFollowsVerifyMode proves the web suite entry
// consults verify mode when agent-browser is absent. Method: point PATH at an
// empty directory so agent-browser cannot be found, run cmdWebMain on one
// runnable committed .wb test, once with ZE_VERIFY_MODE=1 and once with it
// unset, and read the error and the stdout it leaves.
//
// VALIDATES: under verify mode the suite fails and names the explicit skip,
// ZE_SKIP_SUITES=web; outside it the suite reports the skip and succeeds.
// PREVENTS: cmdWebMain calling webBrowserMissing with a constant, or skipping
// before it asks, so a verify run records a pass that never ran the suite.
func TestWebSuiteMissingBrowserFollowsVerifyMode(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Cleanup(env.ResetCache)

	t.Run("verify mode fails", func(t *testing.T) {
		t.Setenv("ZE_VERIFY_MODE", "1")
		env.ResetCache()
		out, err := runWebMainCapturingStdout(t, []string{"-p", webMissingBrowserPattern})
		if err == nil {
			t.Fatalf("verify mode skipped the web suite without agent-browser; stdout %q", out)
		}
		for _, want := range []string{"agent-browser", "ZE_SKIP_SUITES=web"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not name %q", err.Error(), want)
			}
		}
	})

	t.Run("local run skips", func(t *testing.T) {
		t.Setenv("ZE_VERIFY_MODE", "")
		env.ResetCache()
		out, err := runWebMainCapturingStdout(t, []string{"-p", webMissingBrowserPattern})
		if err != nil {
			t.Fatalf("a local run without agent-browser must skip, got error: %v", err)
		}
		if !strings.Contains(out, "agent-browser not found in PATH, skipping web tests") {
			t.Fatalf("the run returned without taking the missing-browser skip; stdout %q", out)
		}
	})
}

// runWebMainCapturingStdout runs cmdWebMain with os.Stdout redirected to a
// file. Callers MUST NOT run concurrently with another stdout capture.
func runWebMainCapturingStdout(t *testing.T, args []string) (string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdout")
	stdout, err := os.Create(path) //nolint:gosec // path under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdout
	runErr := cmdWebMain(args)
	os.Stdout = old
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path) //nolint:gosec // path under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	return string(out), runErr
}
