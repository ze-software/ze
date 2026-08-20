// Guard for scenario 55's failure-path diagnosis.
//
// `_check` (scenarios/bgp-wire-edit-api-origin-bird/check.py) catches its own
// assertion and asks whether a process plugin was the real cause. Since that
// read was made strict, it has two failure answers rather than one, and they
// must not be treated alike: a plugin sentinel REPLACES the assertion, an
// unreadable log must not. Getting that backwards hands the operator a docker
// error in place of the interop failure that actually happened.
//
// Go rather than Python for the same reason as run_test.go: there is no Python
// test root under test/interop (`pythonTestRoots`, scripts/dev/python_tests_test.go),
// so a *_test.py here would be a test nothing runs. The probe it drives is
// committed under testdata/ and starts no container.

package interop_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runCheckProbe drives testdata/check_except_probe.py and returns its output.
func runCheckProbe(t *testing.T, mode string) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	probe := filepath.Join(root, "test", "interop", "testdata", "check_except_probe.py")
	if _, err := os.Stat(probe); err != nil {
		t.Fatalf("stat %s: %v", probe, err)
	}
	python := pythonOrSkip(t)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, probe, mode)
	cmd.Dir = root
	// `_check_session_budget` reads SESSION_TIMEOUT from the environment and
	// denies below this scenario's floor, ahead of the handler under test. An
	// operator with the knob exported would red these tests on a correct tree,
	// so the value is pinned rather than inherited.
	cmd.Env = probeEnv(t, "SESSION_TIMEOUT=90")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("check probe %s did not run: %v\noutput:\n%s", mode, err, out)
	}
	return string(out)
}

// TestScenario55KeepsItsFailureWhenTheZeLogIsUnreadable
//
// VALIDATES: an unreadable Ze log is reported as a fact, and the interop
// assertion that actually failed is what reaches the runner.
// PREVENTS: the strict read replacing a real failure with a docker error. The
// bare re-raise never runs if the diagnosis raises first, so the operator would
// see "docker logs failed" for a scenario that failed on a missing route.
//
// Both unreadable shapes are driven. `docker_logs` converts only
// `TimeoutExpired`, so a missing or unusable docker binary arrives as an
// OSError, and a wrap narrowed to `RuntimeError` alone would let that one
// through while every RuntimeError case stayed green.
func TestScenario55KeepsItsFailureWhenTheZeLogIsUnreadable(t *testing.T) {
	for _, mode := range []string{"unreadable", "unreadable-oserror"} {
		t.Run(mode, func(t *testing.T) {
			out := runCheckProbe(t, mode)
			if !strings.Contains(out, "RAISED=AssertionError: BIRD route") {
				t.Errorf("the original interop assertion must survive an unreadable log; got:\n%s", out)
			}
			if !strings.Contains(out, "could not be read") {
				t.Errorf("the unreadable log must still be reported as a fact; got:\n%s", out)
			}
		})
	}
}

// TestScenario55ReportsThePluginWhenItSignalled
//
// VALIDATES: a plugin sentinel PROPAGATES, replacing the assertion, so the
// runner names the cause rather than the symptom.
// PREVENTS: the guard being wrapped so tightly that the sentinel is swallowed
// too, which is the mirror risk of the fix above.
//
// The assertion is anchored on `RAISED=`, which the probe prints only for the
// exception that actually escaped. Matching the message anywhere in the output
// does NOT discriminate: a wrap widened to `except Exception` catches the
// sentinel and prints the same words inside "ze log could not be read", so the
// test stayed green against the exact defect it names.
func TestScenario55ReportsThePluginWhenItSignalled(t *testing.T) {
	out := runCheckProbe(t, "sentinel")
	if !strings.Contains(out, "RAISED=AssertionError: process plugin failed") {
		t.Errorf("a plugin sentinel must ESCAPE and replace the route assertion; got:\n%s", out)
	}
}
