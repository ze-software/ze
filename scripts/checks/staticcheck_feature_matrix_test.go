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
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
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

func overriddenEnvironment(overrides map[string]string) []string {
	env := os.Environ()
	filtered := env[:0]
	for _, entry := range env {
		key, _, found := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; found && replaced {
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
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("injected invariant tests: %v\n%s", err, out)
	}
}
