// Related: platformvet.go -- the plans, environment, and streamed execution under test
// Related: actions.go -- the two gate claims and multi-action sweep
//
// VALIDATES: both platform gates retain their exact package population, command
// environment, direct output, and exit status.
// PREVENTS: a cross-target check that silently vets the host platform, enables
// CGO, adds feature tags, or succeeds after one package tree disappears.
package platformvet

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/leaction"
)

const platformVetManifest = "ze_bgp\tBGP\n"
const platformVetGoMod = "module example.com/platformvet\n\ngo 1.26\n\ntoolchain go1.26.6\n"

func platformVetFixture(t *testing.T, manifest, goMod string) string {
	t.Helper()
	root := t.TempDir()
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"), []byte(manifest), 0o600); err != nil {
			t.Fatalf("write feature manifest: %v", err)
		}
	}
	if goMod != "" {
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o600); err != nil {
			t.Fatalf("write go.mod: %v", err)
		}
	}
	for _, pattern := range packagePatterns {
		relative := strings.TrimSuffix(strings.TrimPrefix(pattern, "./"), "/...")
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(relative)), 0o750); err != nil {
			t.Fatalf("create package %s: %v", pattern, err)
		}
	}
	return root
}

func platformVetEnvValue(entries []string, key string) (string, bool) {
	value := ""
	found := false
	for _, entry := range entries {
		name, candidate, ok := strings.Cut(entry, "=")
		if ok && name == key {
			value = candidate
			found = true
		}
	}
	return value, found
}

func TestActionsClaimBothReadOnlyGates(t *testing.T) {
	listing := Actions()
	if len(listing.Actions) != 2 {
		t.Fatalf("platform-vet declares %d actions, want 2", len(listing.Actions))
	}
	want := []string{"ze-platform-vet-darwin", "ze-platform-vet-freebsd"}
	if got := Gates(); !slices.Equal(got, want) {
		t.Fatalf("Gates = %v, want %v", got, want)
	}
	for _, action := range listing.Actions {
		if action.Writes {
			t.Errorf("%s is marked as writing", action.Gate)
		}
		if action.Why == "" {
			t.Errorf("%s has no reason", action.Gate)
		}
		if len(action.Forks) != 0 {
			t.Errorf("%s still forks %v", action.Gate, action.Forks)
		}
	}
}

func TestBothPlansUseTheExactCommandAndDistinctGOOS(t *testing.T) {
	root := platformVetFixture(t, platformVetManifest, platformVetGoMod)
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "arm64")
	t.Setenv("CGO_ENABLED", "1")
	t.Setenv("GOMAXPROCS", "91")
	t.Setenv("GOMEMLIMIT", "17GiB")

	runner, err := NewRunner(root)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	wantCommand := []string{
		"go", "vet",
		"./internal/component/host/...",
		"./internal/component/iface/...",
		"./internal/plugins/iface/...",
	}
	for _, test := range []struct {
		platform Platform
		goos     string
		gate     string
	}{
		{PlatformDarwin, "darwin", "ze-platform-vet-darwin"},
		{PlatformFreeBSD, "freebsd", "ze-platform-vet-freebsd"},
	} {
		plan, planErr := runner.Plan(test.platform)
		if planErr != nil {
			t.Fatalf("Plan(%s): %v", test.goos, planErr)
		}
		if plan.Gate != test.gate || plan.Platform != test.goos {
			t.Errorf("Plan(%s) identifies gate=%q platform=%q", test.goos, plan.Gate, plan.Platform)
		}
		if !slices.Equal(plan.Command, wantCommand) {
			t.Errorf("Plan(%s) command = %v, want %v", test.goos, plan.Command, wantCommand)
		}
		if !slices.Equal(plan.Packages, wantCommand[2:]) {
			t.Errorf("Plan(%s) packages = %v, want %v", test.goos, plan.Packages, wantCommand[2:])
		}
		for key, want := range map[string]string{
			"GOCACHE":             filepath.Join(root, "cache", "go-cache"),
			"GOLANGCI_LINT_CACHE": filepath.Join(root, "tmp", "golangci-lint-cache"),
			"CGO_ENABLED":         "0",
			"GOTOOLCHAIN":         "go1.26.6",
			"GOOS":                test.goos,
			"GOARCH":              "arm64",
			"GOMAXPROCS":          "91",
			"GOMEMLIMIT":          "17GiB",
		} {
			if got, found := platformVetEnvValue(plan.Environment, key); !found || got != want {
				t.Errorf("Plan(%s) %s = %q (present=%v), want %q", test.goos, key, got, found, want)
			}
		}
	}
}

func TestRunnerHandsTheExactPlanToTheExecutorAndPropagatesItsCode(t *testing.T) {
	root := platformVetFixture(t, platformVetManifest, platformVetGoMod)
	var seenGate, seenDir string
	var seenCommand, seenEnvironment []string
	execute := func(gate string, command []string, dir string, environment []string) (gaterun.GateReport, int) {
		seenGate = gate
		seenDir = dir
		seenCommand = slices.Clone(command)
		seenEnvironment = slices.Clone(environment)
		return gaterun.GateReport{Gate: gate, Command: command, Code: 23}, 23
	}
	runner, err := newRunner(root, execute)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}

	report, code := runner.Run(PlatformFreeBSD)
	if code != 23 || report.Code != 23 {
		t.Fatalf("Run code=%d report.code=%d, want 23", code, report.Code)
	}
	if seenGate != "ze-platform-vet-freebsd" || seenDir != root {
		t.Errorf("executor received gate=%q dir=%q", seenGate, seenDir)
	}
	if !slices.Equal(seenCommand, report.Command) {
		t.Errorf("executor command=%v report command=%v", seenCommand, report.Command)
	}
	if goos, _ := platformVetEnvValue(seenEnvironment, "GOOS"); goos != "freebsd" {
		t.Errorf("executor GOOS=%q, want freebsd", goos)
	}
}

func TestSweepRunsBothPlatformsAndKeepsTheFirstFailure(t *testing.T) {
	var ran []Platform
	table := actionTable(func(platform Platform) (any, int) {
		ran = append(ran, platform)
		code := 9
		if platform == PlatformDarwin {
			code = 7
		}
		return Report{Platform: "fixture", Code: code}, code
	})

	answer, code := table.Sweep([]string{"darwin", "freebsd"}, leaction.RunEveryAction)
	if code != 7 {
		t.Fatalf("Sweep code=%d, want first failure 7", code)
	}
	if !slices.Equal(ran, []Platform{PlatformDarwin, PlatformFreeBSD}) {
		t.Fatalf("Sweep ran %v, want Darwin then FreeBSD", ran)
	}
	if answer == nil {
		t.Fatal("Sweep returned no structured report")
	}
}

func TestRunnerFailsClosedOnMissingToolchainData(t *testing.T) {
	for _, test := range []struct {
		name     string
		manifest string
		goMod    string
	}{
		{"manifest", "", platformVetGoMod},
		{"go.mod", platformVetManifest, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := platformVetFixture(t, test.manifest, test.goMod)
			if _, err := NewRunner(root); err == nil {
				t.Fatal("NewRunner accepted incomplete toolchain data")
			}
		})
	}
}

func TestRunnerFailsClosedBeforeExecutionWhenAPackageIsMissing(t *testing.T) {
	root := platformVetFixture(t, platformVetManifest, platformVetGoMod)
	missing := filepath.Join(root, "internal", "component", "iface")
	if err := os.Remove(missing); err != nil {
		t.Fatalf("remove fixture package: %v", err)
	}
	called := false
	runner, err := newRunner(root, func(string, []string, string, []string) (gaterun.GateReport, int) {
		called = true
		return gaterun.GateReport{}, 0
	})
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}

	report, code := runner.Run(PlatformDarwin)
	if code != 2 || report.Code != 2 || report.Error == "" {
		t.Fatalf("missing package report=%+v code=%d, want a structured code-2 failure", report, code)
	}
	if called {
		t.Fatal("the vet command ran after its package population became incomplete")
	}
}

func TestDefaultRunnerStreamsChildOutputAndExitCodeUnchanged(t *testing.T) {
	root := platformVetFixture(t, platformVetManifest, platformVetGoMod)
	bin := t.TempDir()
	fakeGo := filepath.Join(bin, "go")
	body := "#!/bin/sh\nprintf 'vet stdout\\n'\nprintf 'vet stderr\\n' >&2\nexit 19\n"
	if err := os.WriteFile(fakeGo, []byte(body), 0o750); err != nil { //nolint:gosec // a test stand-in on PATH must be executable
		t.Fatalf("write fake go: %v", err)
	}
	t.Setenv("PATH", bin)

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create stdout capture: %v", err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("create stderr capture: %v", err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	defer func() {
		os.Stdout, os.Stderr = oldStdout, oldStderr
	}()
	runner, runnerErr := NewRunner(root)
	if runnerErr != nil {
		os.Stdout, os.Stderr = oldStdout, oldStderr
		t.Fatalf("NewRunner: %v", runnerErr)
	}
	_, code := runner.Run(PlatformDarwin)
	os.Stdout, os.Stderr = oldStdout, oldStderr
	if err := stdout.Close(); err != nil {
		t.Fatalf("close stdout capture: %v", err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatalf("close stderr capture: %v", err)
	}

	out, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	errOut, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if code != 19 {
		t.Errorf("Run code=%d, want 19", code)
	}
	if got := string(out); got != "vet stdout\n" {
		t.Errorf("stdout=%q, want child stdout unchanged", got)
	}
	wantErr := "==> ze-platform-vet-darwin\nvet stderr\n"
	if got := string(errOut); got != wantErr {
		t.Errorf("stderr=%q, want %q", got, wantErr)
	}
}
