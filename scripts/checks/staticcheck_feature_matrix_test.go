// Design: docs/architecture/testing/tracked-build-gate.md -- feature-tag type-check boundary

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func buildStaticcheckFeatureMatrix(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "staticcheck-feature-matrix")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, "staticcheck_feature_matrix.go")
	cmd.Env = append(overriddenEnvironment(nil), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build checker: %v\n%s", err, out)
	}
	return binary
}

func runStaticcheckFeatureMatrixBinary(t *testing.T, binary string, manifest *string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	dir := t.TempDir()
	if manifest != nil {
		path := filepath.Join(dir, "feature-gates.txt")
		if err := os.WriteFile(path, []byte(*manifest), 0o600); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Env = overriddenEnvironment(nil)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("run checker: %v", err)
	return "", -1
}

func runStaticcheckFeatureMatrixInDir(
	t *testing.T,
	binary, dir string,
	env map[string]string,
	args ...string,
) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Env = overriddenEnvironment(env)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("run checker: %v", err)
	return "", -1
}

// verifyRunEnvPrefix names the family of variables the verify runner exports to
// every stage it starts: ZE_VERIFY_MODE from execStage
// (scripts/status/verify_run.go), and the run's two scope answers
// ZE_VERIFY_SCOPE_PACKAGES and ZE_VERIFY_SCOPE_TAGS. The prefix is the contract,
// not the three names: a variable added to that family later is subtracted here
// on the day it is added.
const verifyRunEnvPrefix = "ZE_VERIFY_"

// overriddenEnvironment returns the environment a child process of these tests
// gets: this process's own, less every variable the verify runner exports, plus
// the overrides the caller names.
//
// The subtraction is UNCONDITIONAL, and it is what makes a row-count assertion a
// statement about the code rather than about the machine. ZE_PACKAGES puts
// ./scripts/checks inside the scoped unit stage of a verify run, so these tests
// execute with that run's ZE_VERIFY_SCOPE_TAGS already set, and a child
// inheriting it judges the rows the RUN scoped to instead of the rows the
// fixture implies.
//
// An override still wins: it is appended after the subtraction, which is the
// path every scoped case here drives.
func overriddenEnvironment(overrides map[string]string) []string {
	env := os.Environ()
	filtered := env[:0]
	for _, entry := range env {
		key, _, found := strings.Cut(entry, "=")
		if !found {
			filtered = append(filtered, entry)
			continue
		}
		if strings.HasPrefix(key, verifyRunEnvPrefix) {
			continue
		}
		if _, replaced := overrides[key]; replaced {
			continue
		}
		filtered = append(filtered, entry)
	}
	for key, value := range overrides {
		filtered = append(filtered, key+"="+value)
	}
	return filtered
}

func staticcheckFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create fixture directory for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	return dir
}

func writeStaticcheckFixtureFile(t *testing.T, dir, name, content string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create fixture directory for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

const staticcheckFixtureManifest = "" +
	"ze_consumer consumer\n" +
	"ze_provider consumer\n"

func baseStaticcheckFixture() map[string]string {
	return map[string]string{
		"go.mod":            "module example.com/matrix\n\ngo 1.20\n",
		"feature-gates.txt": staticcheckFixtureManifest,
		"consumer/base.go":  "package consumer\n\nconst Base = 1\n",
	}
}

// TestStaticcheckFeatureMatrixDerivesManifestTags
//
// VALIDATES: AC-1 and AC-5, unique sorted feature tags come only from the
// manifest, while a tag repeated for multiple owned packages is valid.
// PREVENTS: a second feature inventory or duplicate matrix rows for sidecars.
func TestStaticcheckFeatureMatrixDerivesManifestTags(t *testing.T) {
	binary := buildStaticcheckFeatureMatrix(t)
	manifest := `
# tag package
ze_web internal/component/web

ze_alpha internal/component/alpha
ze_web internal/plugins/web-cmd
`
	out, code := runStaticcheckFeatureMatrixBinary(t, binary, &manifest, "--print-matrix")
	if code != 0 {
		t.Fatalf("print matrix exit = %d, want 0:\n%s", code, out)
	}
	if strings.Count(out, "without_ze_web:") != 1 {
		t.Fatalf("repeated manifest tag did not de-duplicate to one row:\n%s", out)
	}
	alpha := strings.Index(out, "without_ze_alpha:")
	web := strings.Index(out, "without_ze_web:")
	if alpha < 0 || web < 0 || alpha >= web {
		t.Fatalf("feature rows are not sorted deterministically:\n%s", out)
	}
}

// TestStaticcheckFeatureMatrixEmitsPromisedRows
//
// VALIDATES: AC-1 and AC-7, N tags produce the exact N+2 configurations in
// deterministic order, with legal names, comma-separated tags, and a newline.
// PREVENTS: omitting bare core, an exclusion row, or Staticcheck's final row.
func TestStaticcheckFeatureMatrixEmitsPromisedRows(t *testing.T) {
	binary := buildStaticcheckFeatureMatrix(t)
	manifest := "ze_web internal/component/web\nze_alpha internal/component/alpha\n"
	got, code := runStaticcheckFeatureMatrixBinary(t, binary, &manifest, "--print-matrix")
	if code != 0 {
		t.Fatalf("print matrix exit = %d, want 0:\n%s", code, got)
	}
	want := "all_features: -tags=ze_core,ze_distro,ze_alpha,ze_web\n" +
		"core_only: -tags=ze_core\n" +
		"without_ze_alpha: -tags=ze_core,ze_distro,ze_web\n" +
		"without_ze_web: -tags=ze_core,ze_distro,ze_alpha\n"
	if got != want {
		t.Fatalf("matrix:\n%s\nwant:\n%s", got, want)
	}
	if got[len(got)-1] != '\n' {
		t.Fatalf("matrix is not newline-terminated: %q", got)
	}
}

// TestStaticcheckFeatureMatrixRejectsVacuousInput
//
// VALIDATES: AC-5 and AC-7, missing, empty, malformed, zero-row,
// duplicate-name, wrong-count, and unterminated matrices fail closed.
// PREVENTS: a malformed manifest or impossible matrix reporting success.
func TestStaticcheckFeatureMatrixRejectsVacuousInput(t *testing.T) {
	binary := buildStaticcheckFeatureMatrix(t)
	t.Run("missing manifest", func(t *testing.T) {
		out, code := runStaticcheckFeatureMatrixBinary(t, binary, nil, "--print-matrix")
		assertCheckerFailure(t, out, code, "read feature manifest")
	})

	manifestCases := []struct {
		name    string
		content string
		want    string
	}{
		{name: "empty", content: "", want: "no feature tags"},
		{name: "comments only", content: "# nothing here\n\n", want: "no feature tags"},
		{name: "missing package", content: "ze_web\n", want: "expected <tag> <package>"},
		{name: "extra field", content: "ze_web internal/component/web extra\n", want: "expected <tag> <package>"},
		{name: "invalid tag", content: "ze-web internal/component/web\n", want: "invalid feature tag"},
		{name: "reserved core tag", content: "ze_core internal/core/example\n", want: "reserved feature tag"},
		{name: "reserved distro tag", content: "ze_distro internal/component/example\n", want: "reserved feature tag"},
		{name: "absolute package", content: "ze_web /internal/component/web\n", want: "invalid package path"},
		{name: "parent package", content: "ze_web internal/component/../web\n", want: "invalid package path"},
		{name: "empty package segment", content: "ze_web internal//web\n", want: "invalid package path"},
	}
	for _, tc := range manifestCases {
		t.Run(tc.name, func(t *testing.T) {
			out, code := runStaticcheckFeatureMatrixBinary(t, binary, &tc.content, "--print-matrix")
			assertCheckerFailure(t, out, code, tc.want)
		})
	}

	t.Run("impossible generated matrices", func(t *testing.T) {
		runInjectedMatrixInvariantTests(t)
	})
}

// TestStaticcheckFeatureMatrixReportsBrokenFeatureDependency
//
// VALIDATES: AC-2 and A-2, a retained consumer cannot use a symbol supplied
// only by the omitted provider, and the diagnostic names both defect and row.
// PREVENTS: a fixture that fails only because omission also removes consumer.
func TestStaticcheckFeatureMatrixReportsBrokenFeatureDependency(t *testing.T) {
	binary := buildStaticcheckFeatureMatrix(t)
	files := baseStaticcheckFixture()
	files["consumer/provider.go"] = "//go:build ze_provider\n\npackage consumer\n\nconst ProviderOnly = 1\n"
	files["consumer/consumer.go"] = "//go:build ze_consumer\n\npackage consumer\n\nconst UsesProvider = ProviderOnly\n"
	dir := staticcheckFixture(t, files)

	out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, nil)
	if code != 1 {
		t.Fatalf("broken production fixture exit = %d, want 1:\n%s", code, out)
	}
	for _, want := range []string{"without_ze_provider", "ProviderOnly"} {
		if !strings.Contains(out, want) {
			t.Fatalf("broken production fixture does not name %q:\n%s", want, out)
		}
	}

	writeStaticcheckFixtureFile(
		t,
		dir,
		"consumer/provider.go",
		"//go:build ze_provider || ze_consumer\n\npackage consumer\n\nconst ProviderOnly = 1\n",
		0o600,
	)
	out, code = runStaticcheckFeatureMatrixInDir(t, binary, dir, nil)
	assertStaticcheckMatrixPass(t, out, code, 4)
}

// TestStaticcheckFeatureMatrixIncludesTestFiles
//
// VALIDATES: AC-3 and AC-7, default Staticcheck matrix execution selects
// _test.go files and keeps their compile diagnostics fatal.
// PREVENTS: accidentally passing -tests=false or filtering test diagnostics.
func TestStaticcheckFeatureMatrixIncludesTestFiles(t *testing.T) {
	binary := buildStaticcheckFeatureMatrix(t)
	files := baseStaticcheckFixture()
	files["consumer/provider_test_symbol.go"] = "//go:build ze_provider\n\npackage consumer\n\nconst ProviderTestOnly = 1\n"
	files["consumer/consumer_test.go"] = "//go:build ze_consumer\n\npackage consumer\n\nconst UsesProviderInTest = ProviderTestOnly\n"
	dir := staticcheckFixture(t, files)

	out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, nil)
	if code != 1 {
		t.Fatalf("broken test fixture exit = %d, want 1:\n%s", code, out)
	}
	for _, want := range []string{"consumer_test.go", "without_ze_provider", "ProviderTestOnly"} {
		if !strings.Contains(out, want) {
			t.Fatalf("broken test fixture does not name %q:\n%s", want, out)
		}
	}

	writeStaticcheckFixtureFile(
		t,
		dir,
		"consumer/provider_test_symbol.go",
		"//go:build ze_provider || ze_consumer\n\npackage consumer\n\nconst ProviderTestOnly = 1\n",
		0o600,
	)
	out, code = runStaticcheckFeatureMatrixInDir(t, binary, dir, nil)
	assertStaticcheckMatrixPass(t, out, code, 4)
}

// TestStaticcheckFeatureMatrixAcceptsValidConfigurations
//
// VALIDATES: AC-4 and AC-7, a coherent real-tool fixture passes every row and
// analyzer-only findings remain owned by lint rather than this type-check gate.
// PREVENTS: enabling standalone Staticcheck analyzers in the matrix invocation.
func TestStaticcheckFeatureMatrixAcceptsValidConfigurations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	binary := buildStaticcheckFeatureMatrix(t)
	files := baseStaticcheckFixture()
	files["consumer/coherent.go"] = "package consumer\n\nfunc Same(value int) bool { return value == value }\n"
	dir := staticcheckFixture(t, files)

	staticcheck, err := exec.LookPath("staticcheck")
	if err != nil {
		t.Fatalf("real Staticcheck is required for the fixture: %v", err)
	}
	lint := exec.CommandContext(ctx, staticcheck, "-checks=SA4000", "./...")
	lint.Dir = dir
	lint.Env = overriddenEnvironment(nil)
	lintOut, lintErr := lint.CombinedOutput()
	if lintErr == nil || !strings.Contains(string(lintOut), "SA4000") {
		t.Fatalf("fixture does not discriminate analyzer disabling:\nerror: %v\n%s", lintErr, lintOut)
	}

	out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, nil)
	assertStaticcheckMatrixPass(t, out, code, 4)
	if strings.Contains(out, "SA4000") {
		t.Fatalf("type-check gate emitted an analyzer-only finding:\n%s", out)
	}
}

// TestStaticcheckFeatureMatrixClassifiesToolFailures
//
// VALIDATES: AC-6 and the deadline boundary, missing, unstartable, timed-out,
// malformed, and no-package runs are unable to judge and can never be green.
// PREVENTS: exit zero or a checked-row line without a complete tool verdict.
func TestStaticcheckFeatureMatrixClassifiesToolFailures(t *testing.T) {
	binary := buildStaticcheckFeatureMatrix(t)

	t.Run("missing executable", func(t *testing.T) {
		dir := staticcheckFixture(t, baseStaticcheckFixture())
		emptyPath := t.TempDir()
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, map[string]string{"PATH": emptyPath})
		assertUnableToJudge(t, out, code, "not found on PATH")
	})

	t.Run("cannot start", func(t *testing.T) {
		dir := staticcheckFixture(t, baseStaticcheckFixture())
		binDir := t.TempDir()
		writeStaticcheckFixtureFile(t, binDir, "staticcheck", "#!/no/such/interpreter\n", 0o700)
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, map[string]string{"PATH": binDir})
		assertUnableToJudge(t, out, code, "could not start")
	})

	t.Run("timeout", func(t *testing.T) {
		dir := staticcheckFixture(t, baseStaticcheckFixture())
		binDir := t.TempDir()
		writeStaticcheckFixtureFile(t, binDir, "staticcheck", "#!/bin/sh\nwhile :; do :; done\n", 0o700)
		out, code := runStaticcheckFeatureMatrixInDir(
			t,
			binary,
			dir,
			map[string]string{"PATH": binDir},
			"--deadline=50ms",
		)
		assertUnableToJudge(t, out, code, "deadline")
	})

	t.Run("timeout closes descendant-held pipes", func(t *testing.T) {
		dir := staticcheckFixture(t, baseStaticcheckFixture())
		binDir := t.TempDir()
		writeStaticcheckFixtureFile(
			t,
			binDir,
			"staticcheck",
			"#!/bin/sh\n/bin/sleep 5 &\nwait\n",
			0o700,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "--deadline=200ms")
		cmd.Dir = dir
		cmd.Env = overriddenEnvironment(map[string]string{"PATH": binDir})
		started := time.Now()
		out, runErr := cmd.CombinedOutput()
		elapsed := time.Since(started)
		if ctx.Err() != nil {
			t.Fatalf("checker hit outer bound while descendant held its output pipes: %v", ctx.Err())
		}
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			t.Fatalf("timed-out checker error = %v, want exit 2:\n%s", runErr, out)
		}
		assertUnableToJudge(t, string(out), exitErr.ExitCode(), "deadline")
		if elapsed >= 1500*time.Millisecond {
			t.Fatalf("checker took %s after a 200ms deadline; descendant-held pipes were not closed promptly", elapsed)
		}
	})

	t.Run("no packages", func(t *testing.T) {
		files := baseStaticcheckFixture()
		delete(files, "consumer/base.go")
		dir := staticcheckFixture(t, files)
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, nil)
		assertUnableToJudge(t, out, code, "matched no packages")
	})

	for _, deadline := range []string{"0", "-1s", "not-a-duration"} {
		t.Run("invalid deadline "+deadline, func(t *testing.T) {
			dir := staticcheckFixture(t, baseStaticcheckFixture())
			out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, nil, "--deadline="+deadline)
			assertUnableToJudge(t, out, code, "positive duration")
		})
	}

	t.Run("malformed invocation", func(t *testing.T) {
		dir := staticcheckFixture(t, baseStaticcheckFixture())
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, nil, "--unknown")
		assertUnableToJudge(t, out, code, "usage")
	})
}

func TestStaticcheckFeatureMatrixCompletedVerdictWinsExpiredContext(t *testing.T) {
	t.Parallel()
	binary := buildStaticcheckFeatureMatrix(t)
	const payloadSize = 8 << 20
	deadline := 2 * time.Second

	for _, want := range []int{0, 1} {
		t.Run(fmt.Sprintf("exit %d", want), func(t *testing.T) {
			t.Parallel()
			dir := staticcheckFixture(t, baseStaticcheckFixture())
			binDir := t.TempDir()
			payload := filepath.Join(dir, "staticcheck-output")
			if err := os.WriteFile(payload, []byte(strings.Repeat("x", payloadSize)), 0o600); err != nil {
				t.Fatalf("write backpressure payload: %v", err)
			}
			writeStaticcheckFixtureFile(
				t,
				binDir,
				"staticcheck",
				"#!/bin/sh\n/bin/cat \"$STATICCHECK_FIXTURE_PAYLOAD\"\nexit \"$STATICCHECK_FIXTURE_EXIT\"\n",
				0o700,
			)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "--deadline="+deadline.String())
			cmd.Dir = dir
			cmd.Env = overriddenEnvironment(map[string]string{
				"PATH":                        binDir,
				"STATICCHECK_FIXTURE_EXIT":    strconv.Itoa(want),
				"STATICCHECK_FIXTURE_PAYLOAD": payload,
			})
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatalf("open checker stdout: %v", err)
			}
			var stderr strings.Builder
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatalf("start checker: %v", err)
			}

			time.Sleep(deadline + 100*time.Millisecond)
			out, err := io.ReadAll(stdout)
			if err != nil {
				t.Fatalf("drain checker stdout: %v", err)
			}
			runErr := cmd.Wait()
			if ctx.Err() != nil {
				t.Fatalf("checker exceeded fixture bound: %v", ctx.Err())
			}
			got := 0
			if runErr != nil {
				var exitErr *exec.ExitError
				if !errors.As(runErr, &exitErr) {
					t.Fatalf("wait checker: %v", runErr)
				}
				got = exitErr.ExitCode()
			}
			if len(out) < payloadSize {
				t.Fatalf("checker preserved %d bytes, want at least %d bytes of backpressure", len(out), payloadSize)
			}
			if got != want {
				t.Fatalf("completed Staticcheck exit = %d, want %d after deadline; stderr:\n%s", got, want, stderr.String())
			}
		})
	}
}

func assertStaticcheckMatrixPass(t *testing.T, out string, code, rows int) {
	t.Helper()
	if code != 0 {
		t.Fatalf("coherent fixture exit = %d, want 0:\n%s", code, out)
	}
	want := fmt.Sprintf("checked %d rows", rows)
	if !strings.Contains(out, want) {
		t.Fatalf("success output does not contain %q:\n%s", want, out)
	}
}

func assertUnableToJudge(t *testing.T, out string, code int, want string) {
	t.Helper()
	if code != 2 {
		t.Fatalf("unable-to-judge exit = %d, want 2:\n%s", code, out)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("unable-to-judge output does not contain %q:\n%s", want, out)
	}
	if strings.Contains(out, "checked ") {
		t.Fatalf("unable-to-judge output reported success:\n%s", out)
	}
}

func assertCheckerFailure(t *testing.T, out string, code int, want string) {
	t.Helper()
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output:\n%s", code, out)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("output does not contain %q:\n%s", want, out)
	}
}

func runInjectedMatrixInvariantTests(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	source, err := os.ReadFile("staticcheck_feature_matrix.go")
	if err != nil {
		t.Fatalf("read checker source: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "checker.go"), source, 0o600); err != nil {
		t.Fatalf("copy checker source: %v", err)
	}
	const testSource = `package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectedMatrixInvariants(t *testing.T) {
	assertContains := func(err error, want string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want diagnostic containing %q", err, want)
		}
	}
	if _, err := buildFeatureMatrix(nil); err == nil {
		t.Fatal("zero feature tags produced rows")
	}
	rows := []featureMatrixRow{
		{name: "all_features", tags: []string{"ze_core", "ze_distro", "ze_web"}},
		{name: "core_only", tags: []string{"ze_core"}},
	}
	assertContains(validateFeatureMatrix([]string{"ze_web"}, rows), "row count")
	rows = append(rows, featureMatrixRow{name: "core_only", tags: []string{"ze_core", "ze_distro"}})
	assertContains(validateFeatureMatrix([]string{"ze_web"}, rows), "duplicate row name")
	if _, err := renderFeatureMatrix([]featureMatrixRow{{name: "not-legal", tags: []string{"ze_core"}}}); err == nil {
		t.Fatal("illegal row name rendered")
	}
	if _, err := renderFeatureMatrix([]featureMatrixRow{{name: "empty"}}); err == nil {
		t.Fatal("tagless row rendered")
	}
	assertContains(
		validateRenderedMatrix(
			[]featureMatrixRow{{name: "core_only", tags: []string{"ze_core"}}},
			[]byte("core_only: -tags=ze_core"),
		),
		"final newline",
	)

	// The scoped matrix's floor. all_features and core_only judge the two
	// combinations Ze ships, so no answer subtracts them and a filter that does
	// is refused rather than run (the boundary table of
	// plan/spec-verify-scope-3-selector-consumers.md: 2 rows is the minimum and
	// 1 is an error).
	derived := matrixRowsForTags([]string{"ze_ssh", "ze_web"})
	if len(derived) != 4 {
		t.Fatalf("derived %d rows for 2 tags, want 4", len(derived))
	}
	assertContains(validateScopedMatrix(derived[:1]), "at least 2")
	assertContains(validateScopedMatrix([]featureMatrixRow{derived[0], derived[2]}), "never subtracted")
	assertContains(validateScopedMatrix([]featureMatrixRow{derived[2], derived[3]}), "never subtracted")

	every, err := scopeFeatureMatrix(derived, changeScope{every: true})
	if err != nil || len(every) != 4 {
		t.Fatalf("the widen scope judged %d rows (err %v), want all 4", len(every), err)
	}
	narrow, err := scopeFeatureMatrix(derived, changeScope{tags: map[string]bool{"ze_web": true}})
	if err != nil || len(narrow) != 3 {
		t.Fatalf("a one-tag scope judged %d rows (err %v), want 3", len(narrow), err)
	}
	if narrow[2].omits != "ze_web" {
		t.Fatalf("a one-tag scope kept %q, want the row omitting ze_web", narrow[2].name)
	}
	none, err := scopeFeatureMatrix(derived, changeScope{})
	if err != nil || len(none) != 2 {
		t.Fatalf("an empty scope judged %d rows (err %v), want the 2 shipped combinations", len(none), err)
	}

	if scope, widen := readChangeScope("", []string{"ze_ssh"}); !scope.every || widen != nil {
		t.Fatalf("an unnamed answer scoped to %v (%v), want every row and no complaint", scope, widen)
	}
	if scope, widen := readChangeScope(filepath.Join(t.TempDir(), "absent.txt"), []string{"ze_ssh"}); !scope.every || widen == nil {
		t.Fatalf("an unreadable answer scoped to %v (%v), want every row and a stated reason", scope, widen)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "checker_internal_test.go"), []byte(testSource), 0o600); err != nil {
		t.Fatalf("write injected tests: %v", err)
	}
	cmd := exec.CommandContext(
		ctx,
		"go",
		"test",
		"checker.go",
		"checker_internal_test.go",
		"-run",
		"^TestInjectedMatrixInvariants$",
		"-count=1",
	)
	cmd.Dir = dir
	cmd.Env = append(overriddenEnvironment(nil), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("injected invariant tests: %v\n%s", err, out)
	}
}

// scopeTagsEnvName mirrors scopeTagsEnv in staticcheck_feature_matrix.go. That
// file carries the ignore build constraint, so no test can import the constant;
// the string itself is the contract scripts/status/verify_run.go sets on every
// stage. A rename on either side leaves this test judging every row, which the
// row-count assertions below refuse.
const scopeTagsEnvName = "ZE_VERIFY_SCOPE_TAGS"

// scopedFixtureManifest gates three packages, which is the smallest manifest that
// can tell "the row this change can move" from "a row it cannot".
const scopedFixtureManifest = "" +
	"ze_ssh ssh\n" +
	"ze_vpp vpp\n" +
	"ze_web web\n"

// scopedFixtureRows is every row scopedFixtureManifest implies, in the order the
// matrix renders them. It is the answer of an unscoped run over that fixture.
var scopedFixtureRows = []string{"all_features", "core_only", "without_ze_ssh", "without_ze_vpp", "without_ze_web"}

// scopedRowFixture writes the module those rows are derived from. It holds no Go
// source: --print-matrix renders the rows without analyzing anything.
func scopedRowFixture(t *testing.T) string {
	t.Helper()
	return staticcheckFixture(t, map[string]string{
		"go.mod":            "module example.com/matrix\n\ngo 1.20\n",
		"feature-gates.txt": scopedFixtureManifest,
	})
}

func writeScopeAnswerFixture(t *testing.T, tags []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "verify-scope-tags.txt")
	var body strings.Builder
	for _, tag := range tags {
		body.WriteString(tag)
		body.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(body.String()), 0o600); err != nil {
		t.Fatalf("write the feature-tag answer: %v", err)
	}
	return path
}

// matrixRowNames reads the row names out of a rendered matrix.
func matrixRowNames(t *testing.T, rendered string) []string {
	t.Helper()
	var names []string
	for line := range strings.SplitSeq(rendered, "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		name, rest, found := strings.Cut(line, ": -tags=")
		if !found || rest == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// repositoryFeatureTags reads the real manifest the gate judges.
func repositoryFeatureTags(t *testing.T, root string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		t.Fatalf("read the repository feature manifest: %v", err)
	}
	seen := map[string]bool{}
	var tags []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tag := strings.Fields(line)[0]
		if seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	return tags
}

// TestMatrixTestsJudgeTheFixtureNotTheRunThatStartedThem
//
// VALIDATES: overriddenEnvironment subtracts the verify runner's own variables
// from every child these tests start, so a scoped verify run cannot change what
// they judge.
// PREVENTS: this file going red inside `make ze-precommit-verify` and green in a
// shell. execStage (scripts/status/verify_run.go) exports ZE_VERIFY_SCOPE_TAGS
// to every stage, and ZE_PACKAGES puts ./scripts/checks inside the scoped unit
// stage, so a child inheriting that answer judges the rows the RUN scoped to.
// The second subtest is the one that goes red when the subtraction is removed;
// the first is why it keys on the prefix rather than on a list of names.
func TestMatrixTestsJudgeTheFixtureNotTheRunThatStartedThem(t *testing.T) {
	t.Setenv(scopeTagsEnvName, writeScopeAnswerFixture(t, []string{"ze_ssh"}))
	t.Setenv(verifyRunEnvPrefix+"A_NAME_ADDED_LATER", "1")

	t.Run("no runner variable reaches a child", func(t *testing.T) {
		for _, entry := range overriddenEnvironment(nil) {
			if strings.HasPrefix(entry, verifyRunEnvPrefix) {
				t.Fatalf("a child of this test inherits %q from the run that started it", entry)
			}
		}
	})

	t.Run("an inherited answer narrows no matrix", func(t *testing.T) {
		binary := buildStaticcheckFeatureMatrix(t)
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, scopedRowFixture(t), nil, "--print-matrix")
		if code != 0 {
			t.Fatalf("print matrix exit = %d, want 0:\n%s", code, out)
		}
		got := matrixRowNames(t, out)
		if strings.Join(got, ",") != strings.Join(scopedFixtureRows, ",") {
			t.Fatalf("rows judged = %v, want %v: the run's own answer reached the child:\n%s", got, scopedFixtureRows, out)
		}
	})
}

// TestMatrixRowsScopeToChangedTags
//
// VALIDATES: AC-1 and AC-2 of plan/spec-verify-scope-3-selector-consumers.md.
// The rows are SUBTRACTED from the manifest's own derivation by the change
// set's feature-tag answer, the two rows that omit no tag survive every
// subtraction, and every answer the check cannot trust judges the whole matrix.
// PREVENTS: the 874s the gate paid to judge 38 rows for a one-feature change,
// and the two failure shapes that cost buys -- a narrowed matrix taken from an
// answer that does not parse, and a filter that drops a shipped combination.
func TestMatrixRowsScopeToChangedTags(t *testing.T) {
	binary := buildStaticcheckFeatureMatrix(t)

	fixtureRows := scopedFixtureRows
	fixture := scopedRowFixture(t)

	for _, tc := range []struct {
		name     string
		answer   *[]string
		absent   bool
		wantRows []string
		wantSaid string
	}{
		{
			name:     "no answer at all judges every row",
			wantRows: fixtureRows,
		},
		{
			name:     "an empty answer judges the shipped combinations alone",
			answer:   &[]string{},
			wantRows: []string{"all_features", "core_only"},
		},
		{
			name:     "one reached tag keeps its own row",
			answer:   &[]string{"ze_ssh"},
			wantRows: []string{"all_features", "core_only", "without_ze_ssh"},
		},
		{
			name:     "two reached tags keep two rows",
			answer:   &[]string{"ze_ssh", "ze_web"},
			wantRows: []string{"all_features", "core_only", "without_ze_ssh", "without_ze_web"},
		},
		{
			name:     "blank lines and repeats do not multiply rows",
			answer:   &[]string{"ze_ssh", "", "ze_ssh"},
			wantRows: []string{"all_features", "core_only", "without_ze_ssh"},
		},
		{
			name:     "every tag is the widen answer",
			answer:   &[]string{"ze_ssh", "ze_vpp", "ze_web"},
			wantRows: fixtureRows,
		},
		{
			name:     "a tag the manifest does not declare widens",
			answer:   &[]string{"ze_ssh", "ze_gone"},
			wantRows: fixtureRows,
			wantSaid: "does not declare",
		},
		{
			name:     "an answer that cannot be read widens",
			absent:   true,
			wantRows: fixtureRows,
			wantSaid: "could not be read",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{}
			switch {
			case tc.absent:
				env[scopeTagsEnvName] = filepath.Join(t.TempDir(), "no-such-answer.txt")
			case tc.answer != nil:
				env[scopeTagsEnvName] = writeScopeAnswerFixture(t, *tc.answer)
			}
			out, code := runStaticcheckFeatureMatrixInDir(t, binary, fixture, env, "--print-matrix")
			if code != 0 {
				t.Fatalf("print matrix exit = %d, want 0:\n%s", code, out)
			}
			got := matrixRowNames(t, out)
			if strings.Join(got, ",") != strings.Join(tc.wantRows, ",") {
				t.Fatalf("rows judged = %v, want %v:\n%s", got, tc.wantRows, out)
			}
			if tc.wantSaid != "" && !strings.Contains(out, tc.wantSaid) {
				t.Fatalf("a widened answer said nothing containing %q:\n%s", tc.wantSaid, out)
			}
		})
	}

	root := filepath.Join("..", "..")
	everyTag := repositoryFeatureTags(t, root)

	// AC-1: one SSH-gated file changed. The gate judges the two shipped
	// combinations and the one row SSH can move, out of the real manifest.
	t.Run("an ssh-local change judges at most four of the real rows", func(t *testing.T) {
		env := map[string]string{scopeTagsEnvName: writeScopeAnswerFixture(t, []string{"ze_ssh"})}
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, root, env, "--print-matrix")
		if code != 0 {
			t.Fatalf("print matrix exit = %d, want 0:\n%s", code, out)
		}
		got := matrixRowNames(t, out)
		if len(got) > 4 {
			t.Fatalf("an ssh-local change judges %d rows, want at most 4: %v", len(got), got)
		}
		for _, want := range []string{"all_features", "core_only", "without_ze_ssh"} {
			if !slices.Contains(got, want) {
				t.Fatalf("the scoped matrix dropped %s: %v", want, got)
			}
		}
	})

	// AC-2: an always-on package changed, so the selector answers every feature
	// and nothing can be subtracted.
	t.Run("an always-on change judges every real row", func(t *testing.T) {
		env := map[string]string{scopeTagsEnvName: writeScopeAnswerFixture(t, everyTag)}
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, root, env, "--print-matrix")
		if code != 0 {
			t.Fatalf("print matrix exit = %d, want 0:\n%s", code, out)
		}
		if got := len(matrixRowNames(t, out)); got != len(everyTag)+2 {
			t.Fatalf("the widen answer judges %d rows, want %d", got, len(everyTag)+2)
		}
	})
}

// scopedBreakFixture builds a module whose only type error is invisible to every
// matrix row but without_ze_ssh.
//
// web/transport_plain.go is compiled when ze_web is on and ze_ssh is off, which
// all_features (both on), core_only (both off) and without_ze_web (web off) each
// miss. That is what makes the assertion below discriminate: the scoped matrix
// catches the break only because it kept the row the change set can move.
func scopedBreakFixture(broken bool) map[string]string {
	plain := "//go:build ze_web && !ze_ssh\n\npackage web\n\nfunc transport() string { return \"plain\" }\n"
	if broken {
		plain = "//go:build ze_web && !ze_ssh\n\npackage web\n\nfunc transport() int { return 0 }\n"
	}
	return map[string]string{
		"go.mod":                 "module example.com/matrix\n\ngo 1.20\n",
		"feature-gates.txt":      "ze_ssh ssh\nze_web web\n",
		"ssh/doc.go":             "package ssh\n",
		"ssh/dial.go":            "//go:build ze_ssh\n\npackage ssh\n\nfunc Dial() string { return \"ssh\" }\n",
		"web/doc.go":             "package web\n",
		"web/serve.go":           "//go:build ze_web\n\npackage web\n\nfunc Serve() string { return transport() }\n",
		"web/transport_ssh.go":   "//go:build ze_web && ze_ssh\n\npackage web\n\nimport \"example.com/matrix/ssh\"\n\nfunc transport() string { return ssh.Dial() }\n",
		"web/transport_plain.go": plain,
	}
}

// selectorTagAnswer runs the change-set selector over a fixture module and
// returns the feature-tag answer a verify run would publish for that change.
// Composing the two producers here is what makes the row filter evidence rather
// than a claim about a hand-typed answer: the matrix judges what
// scripts/checks/verify_scope_selector.go says, and nothing else writes that
// file in a real run.
func selectorTagAnswer(t *testing.T, dir string, changed ...string) []string {
	t.Helper()
	selector := buildScopeSelector(t)
	paths := scopePathsFile(t, changed...)
	stdout, stderr, code := runScopeSelector(t, selector, dir, "--print=tags", "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("the selector exited %d over the fixture\nstderr:\n%s", code, stderr)
	}
	return scopeLines(stdout)
}

// TestMatrixRowFilterCatchesAGatedBreak
//
// VALIDATES: AC-3 of plan/spec-verify-scope-3-selector-consumers.md. A type
// error that only the without_ze_ssh row compiles is still caught when the
// matrix is scoped to the change set that introduced it.
// PREVENTS: the whole failure mode the row filter can introduce -- a smaller
// matrix that is also a weaker one. The last subtest is what makes the first
// two evidence rather than coincidence: the changed file is constrained
// !ze_ssh, and the answer the SELECTOR gives for it is what has to keep the
// row. A tag answer built from the changed package's gate alone would name
// ze_web, subtract without_ze_ssh, and let this break ship.
func TestMatrixRowFilterCatchesAGatedBreak(t *testing.T) {
	if _, err := exec.LookPath("staticcheck"); err != nil {
		t.Fatalf("real Staticcheck is required for this fixture: %v", err)
	}
	binary := buildStaticcheckFeatureMatrix(t)

	t.Run("the coherent fixture passes the scoped matrix", func(t *testing.T) {
		dir := staticcheckFixture(t, scopedBreakFixture(false))
		env := map[string]string{scopeTagsEnvName: writeScopeAnswerFixture(t, []string{"ze_ssh"})}
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, env)
		assertStaticcheckMatrixPass(t, out, code, 3)
	})

	t.Run("the ssh-scoped matrix catches the gated break", func(t *testing.T) {
		dir := staticcheckFixture(t, scopedBreakFixture(true))
		env := map[string]string{scopeTagsEnvName: writeScopeAnswerFixture(t, []string{"ze_ssh"})}
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, env)
		if code != 1 {
			t.Fatalf("scoped matrix exit = %d, want 1: the retained without_ze_ssh row must still catch the break:\n%s", code, out)
		}
		if !strings.Contains(out, "without_ze_ssh") {
			t.Fatalf("the verdict names no row, so the catch cannot be attributed:\n%s", out)
		}
	})

	t.Run("the answer the selector gives for that change keeps the row", func(t *testing.T) {
		dir := staticcheckFixture(t, scopedBreakFixture(true))
		tags := selectorTagAnswer(t, dir, "web/transport_plain.go")
		if !slices.Contains(tags, "ze_ssh") {
			t.Fatalf("the selector answered %v for a file constrained !ze_ssh, so the only row that compiles it is subtracted", tags)
		}
		env := map[string]string{scopeTagsEnvName: writeScopeAnswerFixture(t, tags)}
		out, code := runStaticcheckFeatureMatrixInDir(t, binary, dir, env)
		if code != 1 {
			t.Fatalf("matrix scoped to %v exit = %d, want 1: the row the changed file compiles in must be judged:\n%s", tags, code, out)
		}
		if !strings.Contains(out, "without_ze_ssh") {
			t.Fatalf("the verdict names no row, so the catch cannot be attributed:\n%s", out)
		}
	})
}
