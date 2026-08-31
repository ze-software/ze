// VALIDATES: the tracked root launchers build one cmd/ze composition when absent
// and exec the selected personality without changing process semantics, and that
// `./le --name <name>` gives one session a rebuilt binary of its own.
// PREVENTS: Python fallback, freshness rebuilds, tag drift, argv splitting, a
// shell parent masking the binary's exit status or terminating signal, a named
// build writing the shared bin/le, and a name reaching the filesystem unchecked.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRootLaunchersBuildMissingBinary(t *testing.T) {
	root := personalityRepoRoot(t)
	const zeTags = "ze_core,ze_distro,ze_alpha,ze_beta"

	tests := []struct {
		name      string
		launcher  string
		wantTags  string
		admitWith bool
	}{
		{name: "le cold start", launcher: "le", wantTags: "ze_le,ze_alpha,ze_beta"},
		{name: "ze cold start", launcher: "ze", wantTags: zeTags},
		{name: "ze native admission", launcher: "ze", wantTags: zeTags, admitWith: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := launcherFixture(t, root, tt.launcher)
			goRecord := filepath.Join(fixture, "go.record")
			execRecord := filepath.Join(fixture, "exec.record")
			writeFakeGo(t, fixture)
			if tt.admitWith {
				writeFakeAdmissionLe(t, fixture)
			}

			working := filepath.Join(fixture, "working directory")
			if err := os.MkdirAll(working, 0o755); err != nil {
				t.Fatalf("create working directory: %v", err)
			}
			if err := os.WriteFile(filepath.Join(working, "glob-expanded"), nil, 0o600); err != nil {
				t.Fatalf("create glob probe: %v", err)
			}

			args := []string{"two words", "*", "", "semi;colon"}
			cmd := exec.CommandContext(t.Context(), filepath.Join(fixture, tt.launcher), args...)
			cmd.Dir = working
			cmd.Env = append(launcherEnv(),
				"PATH="+filepath.Join(fixture, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"),
				"ZE_GO_RECORD="+goRecord,
				"ZE_EXEC_RECORD="+execRecord,
				"ZE_EXEC_EXIT=37",
				"ZE_LAUNCHER_ROOT="+fixture,
				"ZE_NATIVE_LE_RECORD="+filepath.Join(fixture, "admission.record"),
			)
			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 37 {
				t.Fatalf("launcher exit = %v, want built binary's 37", err)
			}

			wantBuild := []string{
				"cwd=" + fixture,
				"GOCACHE=" + filepath.Join(fixture, "cache", "go-cache"),
				"GOLANGCI_LINT_CACHE=" + filepath.Join(fixture, "tmp", "golangci-lint-cache"),
				"CGO_ENABLED=0",
				"GOTOOLCHAIN=go1.27.0",
				"arg=build",
				"arg=-tags",
				"arg=" + tt.wantTags,
				"arg=-o",
				"arg=" + filepath.Join(fixture, "bin", tt.launcher),
				"arg=./cmd/ze",
			}
			assertLauncherRecord(t, goRecord, wantBuild)
			assertLauncherRecord(t, execRecord, args)

			admissionRecord := filepath.Join(fixture, "admission.record")
			if tt.admitWith {
				wantAdmission := []string{
					"job", "run", "label", "ze-build", "command",
					"go", "build", "-tags", tt.wantTags, "-o",
					filepath.Join(fixture, "bin", "ze"), "./cmd/ze",
				}
				assertLauncherRecord(t, admissionRecord, wantAdmission)
			} else if _, statErr := os.Stat(admissionRecord); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("cold launcher unexpectedly used native admission: %v", statErr)
			}
		})
	}
}

func TestRootLaunchersExecExistingBinaryWithoutRebuild(t *testing.T) {
	root := personalityRepoRoot(t)
	for _, launcher := range []string{"le", "ze"} {
		t.Run(launcher, func(t *testing.T) {
			fixture := launcherFixture(t, root, launcher)
			writeExecutable(t, filepath.Join(fixture, "bin", launcher), fakeBuiltBinary)
			writeExecutable(t, filepath.Join(fixture, "fakebin", "go"), "#!/bin/sh\ntouch \"$ZE_REBUILD_RECORD\"\nexit 99\n")

			working := filepath.Join(fixture, "working")
			if err := os.MkdirAll(working, 0o755); err != nil {
				t.Fatalf("create working directory: %v", err)
			}
			if err := os.WriteFile(filepath.Join(working, "glob-expanded"), nil, 0o600); err != nil {
				t.Fatalf("create glob probe: %v", err)
			}
			rebuildRecord := filepath.Join(fixture, "rebuilt")
			execRecord := filepath.Join(fixture, "exec.record")
			args := []string{"two words", "*", "", "semi;colon"}
			cmd := exec.CommandContext(t.Context(), filepath.Join(fixture, launcher), args...)
			cmd.Dir = working
			cmd.Env = append(launcherEnv(),
				"PATH="+filepath.Join(fixture, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"),
				"ZE_REBUILD_RECORD="+rebuildRecord,
				"ZE_EXEC_RECORD="+execRecord,
				"ZE_EXEC_EXIT=37",
			)
			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 37 {
				t.Fatalf("launcher exit = %v, want existing binary's 37", err)
			}
			assertLauncherRecord(t, execRecord, args)
			if _, statErr := os.Stat(rebuildRecord); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("existing binary triggered a rebuild: %v", statErr)
			}

			signal := exec.CommandContext(t.Context(), filepath.Join(fixture, launcher), "signal")
			signal.Env = append(launcherEnv(), "ZE_EXEC_RECORD="+execRecord)
			err = signal.Run()
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != -1 {
				t.Fatalf("launcher hid existing binary's terminating signal: %v", err)
			}
		})
	}
}

// buildNameProbe exercises every character class the launcher accepts.
const buildNameProbe = "probe.1_2-3"

// TestLeLauncherNameBuildsPrivateBinary drives `./le --name <name>` through both
// option spellings and proves the three things the option promises: the build
// lands beside the shared binary rather than on it, it happens on every call so
// a name cannot go stale, and the option itself never reaches the binary.
func TestLeLauncherNameBuildsPrivateBinary(t *testing.T) {
	root := personalityRepoRoot(t)
	tests := []struct {
		name     string
		option   []string
		prebuilt bool
	}{
		{name: "separate value", option: []string{"--name", buildNameProbe}},
		{name: "joined value", option: []string{"--name=" + buildNameProbe}},
		{name: "existing named build is rebuilt", option: []string{"--name", buildNameProbe}, prebuilt: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := launcherFixture(t, root, "le")
			writeFakeGo(t, fixture)
			goRecord := filepath.Join(fixture, "go.record")
			execRecord := filepath.Join(fixture, "exec.record")
			shared := filepath.Join(fixture, "bin", "le")
			writeExecutable(t, shared, sharedStubBinary)
			named := filepath.Join(fixture, "bin", "le-"+buildNameProbe, "le")
			if tt.prebuilt {
				writeExecutable(t, named, sharedStubBinary)
			}

			args := []string{"two words", "*", "", "semi;colon"}
			argv := append(slices.Clone(tt.option), args...)
			exitCode := runLauncher(t, fixture, "le", argv, []string{
				"ZE_GO_RECORD=" + goRecord,
				"ZE_EXEC_RECORD=" + execRecord,
				"ZE_EXEC_EXIT=37",
			})
			if exitCode != 37 {
				t.Fatalf("launcher exit = %d, want the named build's 37", exitCode)
			}

			assertLauncherRecord(t, goRecord, wantNamedBuild(fixture, named))
			assertLauncherRecord(t, execRecord, args)
			assertSharedStubUntouched(t, shared)
		})
	}
}

// TestLeLauncherCarriesTheBuildNameToNestedCalls covers the propagation half. A
// named session shells out to ./le without repeating the option, and a nested
// call that reached the shared binary would reintroduce the staleness the
// option exists to remove.
func TestLeLauncherCarriesTheBuildNameToNestedCalls(t *testing.T) {
	root := personalityRepoRoot(t)

	t.Run("inherited name selects the named build", func(t *testing.T) {
		fixture := launcherFixture(t, root, "le")
		writeFakeGo(t, fixture)
		goRecord := filepath.Join(fixture, "go.record")
		execRecord := filepath.Join(fixture, "exec.record")
		shared := filepath.Join(fixture, "bin", "le")
		writeExecutable(t, shared, sharedStubBinary)

		args := []string{"repository", "status"}
		exitCode := runLauncher(t, fixture, "le", args, []string{
			"ZE_GO_RECORD=" + goRecord,
			"ZE_EXEC_RECORD=" + execRecord,
			"ZE_EXEC_EXIT=37",
			"ZE_LE_BUILD_NAME=" + buildNameProbe,
		})
		if exitCode != 37 {
			t.Fatalf("launcher exit = %d, want the named build's 37", exitCode)
		}
		named := filepath.Join(fixture, "bin", "le-"+buildNameProbe, "le")
		assertLauncherRecord(t, goRecord, wantNamedBuild(fixture, named))
		assertLauncherRecord(t, execRecord, args)
		assertSharedStubUntouched(t, shared)
	})

	t.Run("the option exports the name it built", func(t *testing.T) {
		fixture := launcherFixture(t, root, "le")
		writeFakeGoProducing(t, fixture, buildNameEchoBinary)
		execRecord := filepath.Join(fixture, "exec.record")

		exitCode := runLauncher(t, fixture, "le", []string{"--name", buildNameProbe, "repository"}, []string{
			"ZE_GO_RECORD=" + filepath.Join(fixture, "go.record"),
			"ZE_EXEC_RECORD=" + execRecord,
		})
		if exitCode != 0 {
			t.Fatalf("launcher exit = %d, want 0", exitCode)
		}
		assertLauncherRecord(t, execRecord, []string{buildNameProbe})
	})
}

// TestLeLauncherRefusesUnsafeBuildNames proves the name is refused rather than
// repaired. Every case below would otherwise reach the filesystem as a path
// component.
func TestLeLauncherRefusesUnsafeBuildNames(t *testing.T) {
	root := personalityRepoRoot(t)
	const wantMessage = "--name accepts letters, digits, dot, underscore and hyphen"
	tests := []struct {
		name   string
		option []string
	}{
		{name: "path separator", option: []string{"--name", "foo/bar"}},
		{name: "parent directory", option: []string{"--name", ".."}},
		{name: "embedded parent directory", option: []string{"--name", "a..b"}},
		{name: "absolute path", option: []string{"--name", "/etc/le"}},
		{name: "empty joined value", option: []string{"--name="}},
		{name: "missing value", option: []string{"--name"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := launcherFixture(t, root, "le")
			writeFakeGo(t, fixture)
			goRecord := filepath.Join(fixture, "go.record")
			execRecord := filepath.Join(fixture, "exec.record")
			shared := filepath.Join(fixture, "bin", "le")
			writeExecutable(t, shared, sharedStubBinary)

			cmd := exec.CommandContext(t.Context(), filepath.Join(fixture, "le"), tt.option...)
			cmd.Dir = fixture
			cmd.Env = append(launcherEnv(),
				"PATH="+filepath.Join(fixture, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"),
				"ZE_GO_RECORD="+goRecord,
				"ZE_EXEC_RECORD="+execRecord,
			)
			var stderr strings.Builder
			cmd.Stderr = &stderr
			err := cmd.Run()

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
				t.Fatalf("launcher exit = %v, want the refusal's 2", err)
			}
			if !strings.Contains(stderr.String(), wantMessage) {
				t.Errorf("refusal message = %q, want it to contain %q", stderr.String(), wantMessage)
			}
			for _, record := range []string{goRecord, execRecord} {
				if _, statErr := os.Stat(record); !errors.Is(statErr, os.ErrNotExist) {
					t.Errorf("a refused name still reached %s: %v", filepath.Base(record), statErr)
				}
			}
			assertSharedStubUntouched(t, shared)
		})
	}
}

func TestRootLaunchersArePOSIXShellWithoutPython(t *testing.T) {
	root := personalityRepoRoot(t)
	for _, launcher := range []string{"le", "ze"} {
		t.Run(launcher, func(t *testing.T) {
			path := filepath.Join(root, launcher)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", launcher, err)
			}
			source := string(body)
			if !strings.HasPrefix(source, "#!/bin/sh\n") {
				t.Errorf("%s is not a POSIX shell launcher", launcher)
			}
			if strings.Contains(strings.ToLower(source), "python") {
				t.Errorf("%s retains a Python path", launcher)
			}
			if strings.Count(source, "$@") != 1 || !strings.Contains(source, `exec "$binary" "$@"`) {
				t.Errorf("%s does not pass user argv once, quoted, through exec", launcher)
			}
			check := exec.CommandContext(t.Context(), "sh", "-n", path)
			if out, err := check.CombinedOutput(); err != nil {
				t.Fatalf("sh -n %s: %v\n%s", launcher, err, out)
			}
		})
	}
}

func launcherFixture(t *testing.T, root, launcher string) string {
	t.Helper()
	fixture := t.TempDir()
	body, err := os.ReadFile(filepath.Join(root, launcher))
	if err != nil {
		t.Fatalf("read root %s: %v", launcher, err)
	}
	writeExecutable(t, filepath.Join(fixture, launcher), string(body))
	if err := os.WriteFile(filepath.Join(fixture, "go.mod"), []byte("module launcher.test/fixture\n\ngo 1.27\ntoolchain go1.27.0\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	manifest := "# fixture feature gates\nze_alpha internal/alpha\nze_beta internal/beta\nze_alpha internal/alpha/sidecar\n"
	if err := os.WriteFile(filepath.Join(fixture, "feature-gates.txt"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write feature-gates.txt: %v", err)
	}
	return fixture
}

// launcherEnv is the parent environment with the launcher's own variable
// cleared. A session that runs the whole suite under `./le --name <name>`
// exports it, and an inherited value would change what every case measures.
func launcherEnv() []string {
	return append(os.Environ(), "ZE_LE_BUILD_NAME=")
}

// runLauncher runs one launcher inside its fixture and answers the exit code.
func runLauncher(t *testing.T, fixture, launcher string, args, extraEnv []string) int {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), filepath.Join(fixture, launcher), args...)
	cmd.Dir = fixture
	cmd.Env = append(launcherEnv(),
		"PATH="+filepath.Join(fixture, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run %s %v: %v", launcher, args, err)
		}
		return exitErr.ExitCode()
	}
	return 0
}

// wantNamedBuild is the go invocation a named build owes: the same tags, caches
// and toolchain pin as the shared build, with the named output path.
func wantNamedBuild(fixture, output string) []string {
	return []string{
		"cwd=" + fixture,
		"GOCACHE=" + filepath.Join(fixture, "cache", "go-cache"),
		"GOLANGCI_LINT_CACHE=" + filepath.Join(fixture, "tmp", "golangci-lint-cache"),
		"CGO_ENABLED=0",
		"GOTOOLCHAIN=go1.27.0",
		"arg=build",
		"arg=-tags",
		"arg=ze_le,ze_alpha,ze_beta",
		"arg=-o",
		"arg=" + output,
		"arg=./cmd/ze",
	}
}

func assertSharedStubUntouched(t *testing.T, shared string) {
	t.Helper()
	body, err := os.ReadFile(shared)
	if err != nil {
		t.Fatalf("read shared binary: %v", err)
	}
	if string(body) != sharedStubBinary {
		t.Errorf("a named build rewrote the shared binary:\n%s", body)
	}
}

func writeFakeGo(t *testing.T, root string) {
	t.Helper()
	writeFakeGoProducing(t, root, fakeBuiltBinary)
}

func writeFakeGoProducing(t *testing.T, root, produced string) {
	t.Helper()
	writeExecutable(t, filepath.Join(root, "fakebin", "go"), `#!/bin/sh
{
	printf 'cwd=%s\n' "$PWD"
	printf 'GOCACHE=%s\n' "$GOCACHE"
	printf 'GOLANGCI_LINT_CACHE=%s\n' "$GOLANGCI_LINT_CACHE"
	printf 'CGO_ENABLED=%s\n' "$CGO_ENABLED"
	printf 'GOTOOLCHAIN=%s\n' "$GOTOOLCHAIN"
	for arg do
		printf 'arg=%s\n' "$arg"
	done
} > "$ZE_GO_RECORD"
out=
previous=
for arg do
	if [ "$previous" = -o ]; then
		out=$arg
	fi
	previous=$arg
done
mkdir -p "$(dirname "$out")"
cat > "$out" <<'EOF_BINARY'
`+produced+`EOF_BINARY
chmod +x "$out"
`)
}

func writeFakeAdmissionLe(t *testing.T, root string) {
	t.Helper()
	writeExecutable(t, filepath.Join(root, "bin", "le"), `#!/bin/sh
printf '%s\n' "$@" > "$ZE_NATIVE_LE_RECORD"
shift 5
cd "$ZE_LAUNCHER_ROOT"
exec "$@"
`)
}

// sharedStubBinary stands in for bin/le. It exits with a code no other stub
// uses, so a test that reaches it by mistake fails on the exit code rather than
// passing on the wrong binary's output.
const sharedStubBinary = `#!/bin/sh
printf 'shared\n' > "$ZE_EXEC_RECORD"
exit 12
`

// buildNameEchoBinary reports the build name the launcher exported to it.
const buildNameEchoBinary = `#!/bin/sh
printf '%s\n' "${ZE_LE_BUILD_NAME-unset}" > "$ZE_EXEC_RECORD"
exit 0
`

const fakeBuiltBinary = `#!/bin/sh
if [ "${1-}" = signal ]; then
	kill -TERM "$$"
fi
printf '%s\n' "$@" > "$ZE_EXEC_RECORD"
exit "${ZE_EXEC_EXIT:-0}"
`

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s parent: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertLauncherRecord(t *testing.T, path string, want []string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	got := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
	if !slices.Equal(got, want) {
		t.Fatalf("%s =\n%s\nwant:\n%s", filepath.Base(path), formatLauncherRecord(got), formatLauncherRecord(want))
	}
}

func formatLauncherRecord(lines []string) string {
	formatted := make([]string, len(lines))
	for index, line := range lines {
		formatted[index] = fmt.Sprintf("%d: %q", index, line)
	}
	return strings.Join(formatted, "\n")
}
