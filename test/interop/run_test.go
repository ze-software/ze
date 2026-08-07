// Guard for the BGP interop runner's missing-lab polarity
// (plan/spec-rfcgate-2-evidence.md AC-1).
//
// test/interop/run.py used to print "Docker unavailable, skipping interop tests"
// and exit 0, while its sibling test/ipsec-interop/run.py exits 1 on the same
// condition. Two runners, opposite failure polarity, in one repo. The fail-open
// one is the dangerous half: once an interop scenario may carry an
// `RFC requirement:` tag (this spec's Phase 2), a green-but-unrun suite would
// satisfy `make ze-rfc-check` with evidence nothing executed
// (ai/rules/evidence.md).
//
// Go rather than Python because there is no Python test root under test/interop
// (scripts/dev/python_tests_test.go pins the three that exist), and this package
// follows test/hub's test-only-package precedent.

package interop_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestInteropRunnerFailsClosedWithoutDocker
//
// VALIDATES: run.py exits non-zero and names Docker when the lab is unreachable.
// PREVENTS: the runner reporting success over an absence -- the same class of
// fail-open as a scanner silently skipping a file it does not recognize.
func TestInteropRunnerFailsClosedWithoutDocker(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	runner := filepath.Join(root, "test", "interop", "run.py")
	if _, err := os.Stat(runner); err != nil {
		t.Fatalf("stat %s: %v", runner, err)
	}
	python := pythonOrSkip(t)

	// An empty directory as the entire PATH makes `docker` unreachable whether or
	// not this host has it installed, so the test is deterministic on a developer
	// laptop with Docker running and on a runner without it. python3 is invoked by
	// absolute path for the same reason.
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, runner)
	cmd.Dir = root
	cmd.Env = probeEnv(t)

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("test/interop/run.py exited 0 with no Docker; it must fail closed.\noutput:\n%s", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run.py did not run: %v\noutput:\n%s", err, out)
	}
	text := strings.ToLower(string(out))
	if !strings.Contains(text, "docker") {
		t.Errorf("run.py's failure must name Docker as the missing prerequisite; got:\n%s", out)
	}
	if strings.Contains(text, "skipping") {
		t.Errorf("run.py must not describe a missing lab as skipping; got:\n%s", out)
	}
}

// aScenarioName returns any scenario directory holding a check.py, so the probe
// below drives the runner's real loop without pinning one scenario's existence.
func aScenarioName(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "test", "interop", "scenarios")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "check.py")); err == nil {
			return e.Name()
		}
	}
	t.Fatalf("no scenario with a check.py under %s", dir)
	return ""
}

// pythonOrSkip resolves the interpreter every test in this package drives.
//
// ONE call site, deliberately. Three copies of this lookup meant three
// `t.Skipf` calls, and `scripts/dev/audit-test-relaxation.py` counts an added
// skip as a weakened test: it cannot tell a duplicated helper from coverage
// being switched off, and it is right not to try.
func pythonOrSkip(t *testing.T) string {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3 not on PATH: %v", err)
	}
	return python
}

// probeEnv builds the environment every probe runs under.
//
// PATH is emptied on purpose, and it is load-bearing rather than tidy.
// `global_cleanup` (test/interop/interop.py) is registered with `atexit` and
// force-removes ten containers plus the network on the way out. Container names
// carry `ZE_INTEROP_SUFFIX` when it is set, so a probe inheriting a real
// environment can delete a CONCURRENT interop run's lab. With no PATH the
// hook's own `shutil.which("docker")` guard returns early and it issues nothing.
// `TestInteropRunnerFailsClosedWithoutDocker` empties PATH for the same reason.
//
// The probes need no PATH themselves: python3 is invoked by absolute path, and
// every Docker call inside the run is stubbed.
func probeEnv(t *testing.T, extra ...string) []string {
	t.Helper()
	empty := t.TempDir()
	env := []string{"PATH=" + empty}
	for _, kv := range os.Environ() {
		switch {
		case strings.HasPrefix(kv, "PATH="),
			strings.HasPrefix(kv, "SESSION_TIMEOUT="),
			strings.HasPrefix(kv, "ZE_INTEROP_SUFFIX="),
			// A malformed `ZE_INTEROP_SUBNET_INDEX` makes
			// `_candidate_subnet_prefixes` raise, and `Scenario.setup` reaches
			// `_create_network` BEFORE it renders, so `absent-network-passes`
			// reds on all three of its assertions on a correct tree: the run
			// never reaches the container start and never renders a copy. It is
			// the only mode affected, because every other one either denies in
			// the pre-clean or overrides `setup` and never creates a network.
			// `ZE_INTEROP_SUBNET_PREFIX` is validated nowhere and reds nothing;
			// both are stripped because the pair is the documented knob.
			// Stripping masks nothing: no assertion reads a subnet, and every
			// docker call is stubbed.
			strings.HasPrefix(kv, "ZE_INTEROP_SUBNET_"):
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

// runProbe drives test/interop/testdata/runner_probe.py in one of its modes and
// returns the combined output.
func runProbe(t *testing.T, mode string) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	probe := filepath.Join(root, "test", "interop", "testdata", "runner_probe.py")
	if _, err := os.Stat(probe); err != nil {
		t.Fatalf("stat %s: %v", probe, err)
	}
	python := pythonOrSkip(t)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, probe, mode, aScenarioName(t, root))
	cmd.Dir = root
	cmd.Env = probeEnv(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("probe %s did not run: %v\noutput:\n%s", mode, err, out)
	}
	return string(out)
}

// assertSummaryPrinted is the shared assertion of the two tests below: the run
// reported itself. Losing the summary discards the tally for every scenario the
// suite already finished, so it is worse than any single red.
func assertSummaryPrinted(t *testing.T, mode, out string) {
	t.Helper()
	if strings.Contains(out, "ESCAPED=") {
		t.Errorf("%s: an exception escaped main() and the run printed no summary; got:\n%s", mode, out)
	}
	if !strings.Contains(out, "0 passed, 1 failed") {
		t.Errorf("%s: the summary must count the failed scenario; got:\n%s", mode, out)
	}
	if !strings.Contains(out, "EXIT=1") {
		t.Errorf("%s: the runner must exit non-zero; got:\n%s", mode, out)
	}
}

// TestInteropRunnerReportsWhenInterruptedMidNote
//
// VALIDATES: a Ctrl-C inside the failure handler's `docker logs` read still ends
// with a counted scenario and a printed summary.
// PREVENTS: the interrupt escaping main() past the summary. The handler reads
// Ze's log for up to 15 seconds after a scenario fails, and an interrupt in that
// window used to discard the whole run's tally, not just this scenario's.
func TestInteropRunnerReportsWhenInterruptedMidNote(t *testing.T) {
	out := runProbe(t, "interrupt-note")
	assertSummaryPrinted(t, "interrupt-note", out)
	if !strings.Contains(out, "INTERRUPTED") {
		t.Errorf("interrupt-note: the run must name the interrupt; got:\n%s", out)
	}
}

// TestInteropRunnerReportsWhenTeardownFails
//
// VALIDATES: a Docker removal that times out during teardown is reported and the
// summary still prints.
// PREVENTS: `Scenario.teardown` raising out of run.py's `finally`. Teardown runs
// after EVERY scenario, passing or failing, so an unguarded timeout there loses
// the summary of an otherwise green suite.
//
// The container removal and the network removal are broken SEPARATELY, and each
// subtest matches only its own half's wording. Breaking both at once and
// matching the bare word "timed out" left the test green with either half of
// the guard deleted, because the surviving half printed the same word.
func TestInteropRunnerReportsWhenTeardownFails(t *testing.T) {
	for _, tc := range []struct{ mode, want string }{
		// Each anchor is a phrase only its own producer emits. "docker network
		// rm" would NOT do: it is a command name that the strict raises print
		// too, so its uniqueness would rest on this mode's setup being a no-op,
		// an invariant nothing states.
		{"teardown-container-timeout", "container can be left behind"},
		{"teardown-container-oserror", "container can be left behind"},
		{"teardown-network-timeout", "network can be left behind"},
		{"teardown-network-oserror", "network can be left behind"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			out := runProbe(t, tc.mode)
			assertSummaryPrinted(t, tc.mode, out)
			if !strings.Contains(out, tc.want) {
				t.Errorf("%s: this half's failure must be reported, not swallowed; got:\n%s", tc.mode, out)
			}
			if strings.HasSuffix(tc.mode, "-timeout") && !strings.Contains(out, "timed out after 30s") {
				t.Errorf("%s: the report must name the timeout; got:\n%s", tc.mode, out)
			}
			if strings.HasSuffix(tc.mode, "-oserror") && !strings.Contains(out, "could not run") {
				t.Errorf("%s: the report must name the unusable binary; got:\n%s", tc.mode, out)
			}
			// This pins the CLEARING, not the `finally`. Replacing the
			// `try/finally` with a plain call leaves every mode green, because
			// `_remove_network` returns rather than raises; the leak it guards
			// against came from the removal being INLINE, where an early
			// `return` exited teardown itself. Deleting the clearing reds all
			// four teardown modes, which is what makes this assertion load
			// bearing.
			// The FILESYSTEM, not teardown's own attribute. `teardown` clears
			// `rendered_dir` whether or not the removal succeeded, so the
			// attribute answers "cleared" for a copy still on disk.
			//
			// BOTH halves, because "gone afterwards" is also true of a copy
			// that was never created: without the precondition, a render that
			// quietly produced nothing would satisfy this.
			if !strings.Contains(out, "RENDERED_EXISTED=True") {
				t.Errorf("%s: the probe must really have rendered a copy; got:\n%s", tc.mode, out)
			}
			if !strings.Contains(out, "RENDERED_ON_DISK=False") {
				t.Errorf("%s: teardown must clear the rendered copy whatever the removal did; got:\n%s", tc.mode, out)
			}
		})
	}
}

// TestInteropRunnerDeniesWhenThePreCleanFails
//
// VALIDATES: the OPPOSITE polarity of the same removal. `Scenario.setup`
// pre-cleans before starting anything, and a failure there fails the scenario
// instead of running it.
// PREVENTS: the cleanup contract being applied to the pre-clean, which would let
// a scenario run beside a leftover daemon at the same address. A container this
// scenario starts collides by name and is caught; a stale peer it does not start
// is invisible, and `_create_network` accepts a network that already exists.
//
// Four cases, because the guard has four ways to be half-built: either removal,
// failing by timeout or by a non-zero exit. A docker that ANSWERS with an error
// leaves the object standing exactly as one that never answers does, and the
// first version of this guard read only the timeout.
func TestInteropRunnerDeniesWhenThePreCleanFails(t *testing.T) {
	// `want` is what pins the four timeout and oserror rows. It is the only
	// assertion that reds when their branch is deleted, because the same words
	// `shape` matches are printed by the CLEANUP report as well: once the strict
	// raise is gone the pre-clean falls through to it, and `run.py`'s `finally`
	// tears down again and prints it a second time. Dropping `want` as redundant
	// would make those four vacuous.
	//
	// The two `-error` rows differ, and the difference is exact rather than a
	// detail: the cleanup contract reads no exit code, so it stays SILENT on a
	// non-zero exit (`docker_rm` for the container rows, `_remove_network` for
	// the network ones). "exit 1" therefore has one producer there and both
	// assertions red. `shape` also pins one producer on the teardown table
	// above.
	for _, tc := range []struct{ mode, want, shape string }{
		{"setup-container-timeout", "a leftover container would race this scenario", "timed out after 30s"},
		{"setup-container-error", "a leftover container would race this scenario", "exit 1"},
		{"setup-container-oserror", "a leftover container would race this scenario", "could not run"},
		{"setup-network-timeout", "a leftover network would race this scenario", "timed out after 30s"},
		{"setup-network-error", "a leftover network would race this scenario", "exit 1"},
		{"setup-network-oserror", "a leftover network would race this scenario", "could not run"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			out := runProbe(t, tc.mode)
			assertSummaryPrinted(t, tc.mode, out)
			if !strings.Contains(out, tc.want) {
				t.Errorf("%s: the pre-clean must deny and name what it found; got:\n%s", tc.mode, out)
			}
			if !strings.Contains(out, tc.shape) {
				t.Errorf("%s: the denial must name HOW the removal failed; got:\n%s", tc.mode, out)
			}
		})
	}
}

// TestInteropRunnerPreCleanExemptsOnlyTheAbsentNetwork
//
// VALIDATES: both sides of the network removal's one exemption. A network that
// is not there is the ordinary pre-clean case and must pass; any other failure
// whose text merely contains "not found" must still deny.
// PREVENTS: the exemption widening into a hole. It is a substring match on
// docker's stderr, and a bare "not found" is not specific to a missing network:
// a misconfigured DOCKER_CONTEXT answers `context ... not found` having removed
// nothing, which would exempt itself and let the scenario run on a live network.
func TestInteropRunnerPreCleanExemptsOnlyTheAbsentNetwork(t *testing.T) {
	const denial = "a leftover network would race this scenario"

	t.Run("absent-network-passes", func(t *testing.T) {
		out := runProbe(t, "setup-network-absent")
		assertSummaryPrinted(t, "setup-network-absent", out)
		if strings.Contains(out, denial) {
			t.Errorf("a network that is simply not there must not deny; got:\n%s", out)
		}
		// It got past the pre-clean, so it reached the container start.
		if !strings.Contains(out, "docker run") {
			t.Errorf("the run must proceed past the pre-clean; got:\n%s", out)
		}
		// This mode renders and then fails, so it also pins the clearing on the
		// setup-failed-after-render path, which no other mode reaches.
		if !strings.Contains(out, "RENDERED_EXISTED=True") ||
			!strings.Contains(out, "RENDERED_ON_DISK=False") {
			t.Errorf("a failed setup must still leave no rendered copy; got:\n%s", out)
		}
	})

	t.Run("other-not-found-denies", func(t *testing.T) {
		out := runProbe(t, "setup-network-notfound")
		assertSummaryPrinted(t, "setup-network-notfound", out)
		if !strings.Contains(out, denial) {
			t.Errorf("a non-network failure saying \"not found\" must still deny; got:\n%s", out)
		}
	})
}
