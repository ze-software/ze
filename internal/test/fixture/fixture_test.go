package fixture

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// readOneReport runs body with os.Stderr replaced, and answers what body wrote
// there.
//
// ReportFailure emits the FIRST failure of a process and drops every later one,
// so a test binary that calls Run twice reads an empty stream the second time
// unless the latch is cleared. One test process runs every case here, which is
// what made TestRunConvertsPanicToObserverFailure pass alone and fail after
// TestRunReportsDriverFailure.
func readOneReport(t *testing.T, body func()) string {
	t.Helper()
	reportedOnce.Store(false)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = writer
	body()
	os.Stderr = old
	_ = writer.Close()
	report := make([]byte, 512)
	n, _ := reader.Read(report)
	_ = reader.Close()
	return string(report[:n])
}

func TestRunDispatchesRegisteredDriver(t *testing.T) {
	name := "fixture-test-driver"
	Register(name, func(_ context.Context, args []string) error {
		if strings.Join(args, ",") != "a,b" {
			t.Fatalf("args = %v", args)
		}
		return nil
	})
	if code := Run([]string{name, "a", "b"}); code != 0 {
		t.Fatalf("Run exited %d", code)
	}
}

// TestRunExpandsEnvironmentInDriverArguments checks the one transformation Run
// makes to an argument.
//
// A quote reaches the driver as written. Run stripped surrounding quotes until
// 03f568f969 (2026-09-02), a branch that existed only to undo the argv a
// whitespace splitter mangled; one quote-aware splitter now serves every suite,
// so stripping here would unquote an argument a .ci author quoted on purpose.
// This test asserted the deleted behavior and was red from that commit until
// this one.
func TestRunExpandsEnvironmentInDriverArguments(t *testing.T) {
	name := "fixture-test-environment"
	t.Setenv("FIXTURE_TEST_ROOT", "/fixture/root")
	Register(name, func(_ context.Context, args []string) error {
		if strings.Join(args, ",") != `/fixture/root/kernel,"/fixture/root/quoted",literal` {
			t.Fatalf("args = %v", args)
		}
		return nil
	})
	if code := Run([]string{name, "$FIXTURE_TEST_ROOT/kernel", `"$FIXTURE_TEST_ROOT/quoted"`, "literal"}); code != 0 {
		t.Fatalf("Run exited %d", code)
	}
}

func TestRunReportsDriverFailure(t *testing.T) {
	name := "fixture-test-failure"
	Register(name, func(context.Context, []string) error { return errors.New("broken observation") })

	code := 0
	report := readOneReport(t, func() { code = Run([]string{name}) })
	if code != 1 || !strings.Contains(report, observerFailure+`: broken observation`) {
		t.Fatalf("code=%d stderr=%q", code, report)
	}
}

func TestRunConvertsPanicToObserverFailure(t *testing.T) {
	name := "fixture-test-panic"
	Register(name, func(context.Context, []string) error { panic("broken invariant") })

	code := 0
	report := readOneReport(t, func() { code = Run([]string{name}) })
	if code != 1 || !strings.Contains(report, observerFailure+`: fixture panic: broken invariant`) {
		t.Fatalf("code=%d stderr=%q", code, report)
	}
}
func TestAwaitObserverResultDoesNotLoseALateScenarioFailure(t *testing.T) {
	started := make(chan struct{})
	result := make(chan error, 1)
	close(started)
	go func() {
		time.Sleep(10 * time.Millisecond)
		result <- errors.New("scenario assertion failed")
	}()

	err := awaitObserverResult(started, result, errors.New("transport stopped"))
	if err == nil || !strings.Contains(err.Error(), "scenario assertion failed") ||
		!strings.Contains(err.Error(), "transport stopped") {
		t.Fatalf("joined error = %v", err)
	}
}

func TestAwaitObserverResultAcceptsTransportStopAfterScenarioSuccess(t *testing.T) {
	started := make(chan struct{})
	result := make(chan error, 1)
	close(started)
	result <- nil

	if err := awaitObserverResult(started, result, errors.New("transport stopped")); err != nil {
		t.Fatalf("error = %v, want successful scenario verdict", err)
	}
}

func TestAwaitObserverResultReturnsTransportErrorBeforeScenarioStarts(t *testing.T) {
	started := make(chan struct{})
	result := make(chan error, 1)
	runErr := errors.New("startup transport stopped")
	if err := awaitObserverResult(started, result, runErr); !errors.Is(err, runErr) {
		t.Fatalf("error = %v, want %v", err, runErr)
	}
}

// TestRunRefusesADriverStartedInTheCheckoutRoot drives the working-directory
// guard from the entry point a .ci step reaches, `ze-test fixture <name>`.
//
// VALIDATES: a driver whose relative file names would land beside tracked
// source does not run at all, and the report names the directory.
// PREVENTS: the 2026-09-05 run that left two ed25519 private keys and five
// config files at the repository root, none of them ignored, so a `git add -A`
// from any session would have committed a private key. See
// plan/journal/test-artifacts-land-in-the-repository-root.md.
func TestRunRefusesADriverStartedInTheCheckoutRoot(t *testing.T) {
	checkout := t.TempDir()
	if err := os.WriteFile(filepath.Join(checkout, "go.mod"), []byte("module github.com/ze-software/ze\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	name := "fixture-test-checkout-root"
	ran := false
	Register(name, func(context.Context, []string) error {
		ran = true
		return nil
	})

	t.Chdir(checkout)
	code := 0
	report := readOneReport(t, func() { code = Run([]string{name}) })
	if code != 1 {
		t.Fatalf("Run exited %d, want 1", code)
	}
	if ran {
		t.Fatal("the driver ran in the checkout root")
	}
	if !strings.Contains(report, "refusing to run a fixture in the ze checkout root") {
		t.Fatalf("stderr = %q", report)
	}
}

// TestRunDispatchesADriverStartedOutsideTheCheckoutRoot is the other polarity:
// the per-test working directory the .ci runner creates holds no ze go.mod, so
// the guard passes and the driver runs.
func TestRunDispatchesADriverStartedOutsideTheCheckoutRoot(t *testing.T) {
	name := "fixture-test-work-directory"
	ran := false
	Register(name, func(context.Context, []string) error {
		ran = true
		return nil
	})

	t.Chdir(t.TempDir())
	if code := Run([]string{name}); code != 0 {
		t.Fatalf("Run exited %d, want 0", code)
	}
	if !ran {
		t.Fatal("the driver did not run in a per-test working directory")
	}
}

func TestRunRefusesUnknownFixture(t *testing.T) {
	if code := Run([]string{"not-registered"}); code != 2 {
		t.Fatalf("Run exited %d", code)
	}
}

// VALIDATES: an override replaces every SPELLING of its key, so a child is
// handed one value for it however the caller's environment spelled it.
// PREVENTS: the shape that made five `le-*` ui fixtures pass standalone and
// fail inside the sweep. internal/core/env reads a ze variable
// case-insensitively and treats a dot and an underscore as one separator, and
// env.Set writes the dot spelling, so a fixture that replaced `ZE_REPO_ROOT`
// alone left the inherited `ze.repo.root` beside it and the child read whichever
// of the two ensureCache met last.
func TestChildEnvironmentReplacesEverySpellingOfAKey(t *testing.T) {
	base := []string{
		"ze.repo.root=/inherited",
		"ZE_REPO_ROOT=/also-inherited",
		"Ze_Repo_Root=/third",
		"PATH=/bin",
	}
	answer := childEnvironment(base, map[string]string{"ZE_REPO_ROOT": "/stand-in"})

	var roots []string
	for _, entry := range answer {
		key, value, _ := strings.Cut(entry, "=")
		if environmentKeyReading(key) == "ze_repo_root" {
			roots = append(roots, key+"="+value)
		}
	}
	if len(roots) != 1 || roots[0] != "ZE_REPO_ROOT=/stand-in" {
		t.Fatalf("the root survives as %q, want exactly ZE_REPO_ROOT=/stand-in", roots)
	}
	if !slices.Contains(answer, "PATH=/bin") {
		t.Fatalf("an unrelated variable was dropped: %q", answer)
	}
}

// VALIDATES: the `KEY=value` entry point strips every spelling too, and an
// entry carrying no `=` is left alone rather than read as an override.
// PREVENTS: a caller that holds its overrides as assignments keeping the
// exact-name comparison the map entry point no longer has.
func TestChildEnvironmentAssignmentsStripEverySpelling(t *testing.T) {
	base := []string{"ze.repo.root=/inherited", "PATH=/bin"}
	answer := childEnvironmentAssignments(base, "ZE_REPO_ROOT=/stand-in", "bare-name-no-value")

	var roots []string
	for _, entry := range answer {
		key, _, _ := strings.Cut(entry, "=")
		if environmentKeyReading(key) == "ze_repo_root" {
			roots = append(roots, entry)
		}
	}
	if len(roots) != 1 || roots[0] != "ZE_REPO_ROOT=/stand-in" {
		t.Fatalf("the root survives as %q, want exactly ZE_REPO_ROOT=/stand-in", roots)
	}
	if slices.ContainsFunc(answer, func(entry string) bool {
		return strings.HasPrefix(entry, "bare-name-no-value")
	}) {
		t.Fatalf("an assignment with no value was applied: %q", answer)
	}
	if !slices.Contains(answer, "PATH=/bin") {
		t.Fatalf("an unrelated variable was dropped: %q", answer)
	}
}
