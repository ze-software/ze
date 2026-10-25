// VALIDATES: the tracked root launchers build one cmd/ze composition when absent
// and exec the selected personality without changing process semantics, and that
// `./le --name <name>` gives one session a rebuilt binary of its own.
// PREVENTS: Python fallback, freshness rebuilds, tag drift, argv splitting, a
// shell parent masking the binary's exit status or terminating signal, a named
// build writing the shared bin/le, and a name reaching the filesystem unchecked.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/modfile"

	"github.com/ze-software/ze/internal/core/env"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
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

			if tt.launcher == "ze" {
				wantBuild := []string{
					"cwd=" + fixture,
					"GOCACHE=" + filepath.Join(fixture, "cache", "go-cache"),
					"GOLANGCI_LINT_CACHE=" + filepath.Join(fixture, "cache", "golangci-lint-cache"),
					"CGO_ENABLED=0",
					"GOTOOLCHAIN=" + checkoutToolchain(t, root),
					"arg=build",
					"arg=-tags",
					"arg=" + tt.wantTags,
					"arg=-o",
					"arg=" + filepath.Join(fixture, "bin", tt.launcher),
					"arg=./cmd/ze",
				}
				assertLauncherRecord(t, goRecord, wantBuild)
			}
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
			if launcher == "le" {
				writeHeadBuild(t, fixture, fakeBuiltBinary)
			} else {
				writeExecutable(t, filepath.Join(fixture, "bin", launcher), fakeBuiltBinary)
			}
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
			// Both launchers exec on every call now: le's staleness sample, the
			// one branch that ran the binary as a child, is gone with the walk it
			// paid for. A launcher that grows a child branch again is caught here.
			body, err := os.ReadFile(filepath.Join(fixture, launcher))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), `"$binary" "$@" ||`) {
				t.Fatalf("%s runs the binary as a child on some path, which hides its signal", launcher)
			}
		})
	}
}

// buildNameProbe exercises every character class the launcher accepts.
const buildNameProbe = "probe.1_2-3"

// TestLeLauncherBuildsForTheHostUnderACrossTarget: a caller's GOOS and GOARCH
// reach the launcher when a tool shim starts it from a cross-target run
// (golangci-lint under GOOS=linux on a Mac), and the launcher still builds the
// generator and le for the host. The fake go refuses either step under a
// target, so the run reaches the built binary only when the launcher dropped
// it.
// VALIDATES: le's build_le runs the generator and the le build with no
// inherited GOOS or GOARCH.
// PREVENTS: `exec format error` from a generator built for another system,
// which stopped a linux lint pass whose golangci-lint went through its shim.
func TestLeLauncherBuildsForTheHostUnderACrossTarget(t *testing.T) {
	root := personalityRepoRoot(t)
	fixture := launcherFixture(t, root, "le")
	writeFakeGo(t, fixture)
	execRecord := filepath.Join(fixture, "exec.record")
	code := runLauncher(t, fixture, []string{"probe"}, []string{
		"GOOS=linux",
		"GOARCH=amd64",
		"ZE_GO_RECORD=" + filepath.Join(fixture, "go.record"),
		"ZE_EXEC_RECORD=" + execRecord,
		"ZE_EXEC_EXIT=37",
	})
	if code != 37 {
		t.Fatalf("launcher exit = %d, want the built binary's 37", code)
	}
	assertLauncherRecord(t, execRecord, []string{"probe"})
}

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
			shared := writeHeadBuild(t, fixture, sharedStubBinary)
			named := filepath.Join(fixture, "bin", "le-"+buildNameProbe, "le")
			if tt.prebuilt {
				writeExecutable(t, named, sharedStubBinary)
			}

			args := []string{"two words", "*", "", "semi;colon"}
			argv := append(slices.Clone(tt.option), args...)
			exitCode := runLauncher(t, fixture, argv, []string{
				"ZE_GO_RECORD=" + goRecord,
				"ZE_EXEC_RECORD=" + execRecord,
				"ZE_EXEC_EXIT=37",
			})
			if exitCode != 37 {
				t.Fatalf("launcher exit = %d, want the named build's 37", exitCode)
			}

			assertLauncherRecord(t, execRecord, args)
			assertSharedStubUntouched(t, shared)
		})
	}
}

// TestLeLauncherPreparationFailureNeverPublishes uses a real compiled feature
// probe: ignoring manifest refusal would publish a feature-stripped generation.
func TestLeLauncherPreparationFailureNeverPublishes(t *testing.T) {
	root := personalityRepoRoot(t)
	cache := t.TempDir()
	// A cold start builds HEAD, so its failure is committed before the run; a
	// named build compiles the working tree, so its failure is only written.
	for _, mode := range []string{"cold", "named"} {
		for _, failure := range []string{"empty manifest", "missing manifest", "missing module", "unusable caches"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				fixture := launcherFixture(t, root, "le")
				if err := os.Symlink(cache, filepath.Join(fixture, "cache")); err != nil {
					t.Fatal(err)
				}
				writeProbe := func(generation string) {
					t.Helper()
					writeExecutable(t, filepath.Join(fixture, "cmd", "ze", "main.go"), `package main
import "fmt"
var feature = "feature-disabled"
func main() { fmt.Println("`+generation+`", feature) }
`)
					writeExecutable(t, filepath.Join(fixture, "cmd", "ze", "feature.go"), `//go:build ze_alpha && ze_beta

package main
func init() { feature = "feature-enabled" }
`)
				}
				writeProbe("old")
				var target string
				var args []string
				if mode == "named" {
					target = filepath.Join(fixture, "bin", "le-"+buildNameProbe, "le")
					args = []string{"--name", buildNameProbe}
				}
				if mode != "cold" {
					initial := exec.CommandContext(t.Context(), filepath.Join(fixture, "le"), args...)
					initial.Dir = fixture
					initial.Env = launcherEnv()
					if output, err := initial.CombinedOutput(); err != nil || string(output) != "old feature-enabled\n" {
						t.Fatalf("initial published feature probe: %v, %q", err, output)
					}
				}
				writeProbe("new")
				switch failure {
				case "empty manifest":
					if err := os.WriteFile(filepath.Join(fixture, "feature-gates.txt"), nil, 0o600); err != nil {
						t.Fatal(err)
					}
				case "missing manifest":
					if err := os.Remove(filepath.Join(fixture, "feature-gates.txt")); err != nil {
						t.Fatal(err)
					}
				case "missing module":
					if err := os.Remove(filepath.Join(fixture, "go.mod")); err != nil {
						t.Fatal(err)
					}
				case "unusable caches":
					if err := os.Remove(filepath.Join(fixture, "cache")); err != nil {
						t.Fatal(err)
					}
					for _, name := range []string{"cache", "tmp"} {
						if err := os.WriteFile(filepath.Join(fixture, name), nil, 0o600); err != nil {
							t.Fatal(err)
						}
					}
				}
				if mode == "cold" {
					commitFixture(t, fixture, "a broken preparation")
				}
				cmd := exec.CommandContext(t.Context(), filepath.Join(fixture, "le"), args...)
				cmd.Dir = fixture
				cmd.Env = launcherEnv()
				if output, err := cmd.CombinedOutput(); err == nil {
					t.Errorf("invalid preparation succeeded: %s", output)
				}
				if mode == "cold" {
					entries, err := os.ReadDir(filepath.Join(fixture, "bin", "builds"))
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						t.Fatal(err)
					}
					for _, entry := range entries {
						t.Errorf("preparation failure left bin/builds/%s", entry.Name())
					}
				} else {
					consumer := exec.CommandContext(t.Context(), target)
					consumer.Env = launcherEnv()
					if output, err := consumer.CombinedOutput(); err != nil || string(output) != "old feature-enabled\n" {
						t.Fatalf("published feature probe after refusal: %v, %q", err, output)
					}
				}
			})
		}
	}
}

// TestLeLauncherPublishesOverRunningELF exercises the tracked launcher and the
// real Go compiler, not a script standing in for its output. The first named
// process stays alive on stdin while the second invocation rebuilds its path.
func TestLeLauncherPublishesOverRunningELF(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux enforces ETXTBSY on an executing ELF inode")
	}
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	source := filepath.Join(fixture, "cmd", "ze", "main.go")
	writeProbe := func(generation string) {
		t.Helper()
		writeExecutable(t, source, `//go:build ze_le && ze_alpha && ze_beta

package main
import ("fmt"; "os"; "path/filepath")
func main() {
	fmt.Println("`+generation+`", filepath.Base(os.Args[0]), os.Getenv("ZE_LE_BUILD_NAME"))
	var release [1]byte
	if _, err := os.Stdin.Read(release[:]); err != nil { os.Exit(2) }
	fmt.Println("`+generation+` released")
}
`)
	}
	writeProbe("first")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	first := exec.CommandContext(ctx, filepath.Join(fixture, "le"), "--name", buildNameProbe)
	first.Dir = fixture
	first.Env = launcherEnv()
	stdin, err := first.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := stdin.Close()
		if errors.Is(err, os.ErrClosed) {
			return
		}
		if err != nil {
			t.Errorf("close first launcher input: %v", err)
		}
	})
	stdout, err := first.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	first.Stderr = &stderr
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	// The process MUST be reaped even when a publication assertion fails.
	defer func() {
		cancel()
		if first.ProcessState == nil {
			_ = first.Wait()
		}
	}()
	reader := bufio.NewReader(stdout)
	if line, err := reader.ReadString('\n'); err != nil || line != "first le "+buildNameProbe+"\n" {
		t.Fatalf("first named process: %q, %v", line, err)
	}
	binary := filepath.Join(fixture, "bin", "le-"+buildNameProbe, "le")
	before, err := os.Stat(binary)
	if err != nil {
		t.Fatal(err)
	}
	writeProbe("second")
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	// Hold the real compiler's output open at its publication boundary. No
	// output is fabricated: this only makes the writable-inode window stable.
	// le builds with the go of the tree its goroot step printed, not the go on
	// PATH, so the hold sits in that tree: the goroot step runs for real and its
	// answer is swapped for a tree whose bin/go runs the real tree's compiler
	// and then holds. Every other call reaches the real go unchanged.
	writeExecutable(t, filepath.Join(fixture, "fakebin", "go"), `#!/bin/sh
set -eu
if [ "${1-}" != run ]; then
	exec "$ZE_REAL_GO" "$@"
fi
real_tree=$("$ZE_REAL_GO" "$@")
tree=$PWD/barriergoroot
mkdir -p "$tree/bin"
{
	printf '#!/bin/sh\nset -eu\nreal_tree=%s\n' "'$real_tree'"
	cat <<'EOF_BARRIER'
GOROOT=$real_tree "$real_tree/bin/go" "$@"
out=
previous=
for arg do
	if [ "$previous" = -o ]; then out=$arg; fi
	previous=$arg
done
exec 3>>"$out"
printf 'publication pending\n'
read release
exec 3>&-
EOF_BARRIER
} > "$tree/bin/go"
chmod +x "$tree/bin/go"
printf '%s\n' "$tree"
`)
	second := exec.CommandContext(ctx, filepath.Join(fixture, "le"), "--name", buildNameProbe)
	second.Dir = fixture
	second.Env = append(launcherEnv(),
		"PATH="+filepath.Join(fixture, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_REAL_GO="+realGo,
	)
	second.Stderr = os.Stderr
	secondInput, err := second.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := secondInput.Close()
		if errors.Is(err, os.ErrClosed) {
			return
		}
		if err != nil {
			t.Errorf("close second launcher input: %v", err)
		}
	})
	secondOutput, err := second.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		if second.ProcessState == nil {
			_ = second.Wait()
		}
	}()
	secondReader := bufio.NewReader(secondOutput)
	if line, err := secondReader.ReadString('\n'); err != nil || line != "publication pending\n" {
		t.Fatalf("compiler publication barrier: %q, %v", line, err)
	}
	consumer := exec.CommandContext(ctx, binary)
	consumer.Env = append(launcherEnv(), "ZE_LE_BUILD_NAME="+buildNameProbe)
	consumer.Stdin = strings.NewReader("x")
	if output, err := consumer.CombinedOutput(); err != nil || string(output) != "first le "+buildNameProbe+"\nfirst released\n" {
		t.Fatalf("consumer during publication: %v, %q", err, output)
	}
	if _, err := secondInput.Write([]byte("publish\nx")); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(secondReader)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Wait(); err != nil {
		t.Fatalf("rebuild while ELF is running: %v\n%s", err, output)
	}
	if string(output) != "second le "+buildNameProbe+"\nsecond released\n" {
		t.Fatalf("rebuilt named process output = %q", output)
	}
	after, err := os.Stat(binary)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("publication reused the executing inode")
	}
	if _, err := stdin.Write([]byte("x")); err != nil {
		t.Fatalf("release original process: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "first released\n" {
		t.Fatalf("original process after publication: %q, %v", line, err)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("original process exit: %v; stderr: %s", err, stderr.String())
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
		shared := writeHeadBuild(t, fixture, sharedStubBinary)

		args := []string{"repository", "status"}
		exitCode := runLauncher(t, fixture, args, []string{
			"ZE_GO_RECORD=" + goRecord,
			"ZE_EXEC_RECORD=" + execRecord,
			"ZE_EXEC_EXIT=37",
			"ZE_LE_BUILD_NAME=" + buildNameProbe,
		})
		if exitCode != 37 {
			t.Fatalf("launcher exit = %d, want the named build's 37", exitCode)
		}
		assertLauncherRecord(t, execRecord, args)
		assertSharedStubUntouched(t, shared)
	})

	t.Run("the option exports the name it built", func(t *testing.T) {
		fixture := launcherFixture(t, root, "le")
		writeFakeGoProducing(t, fixture, buildNameEchoBinary)
		execRecord := filepath.Join(fixture, "exec.record")

		exitCode := runLauncher(t, fixture, []string{"--name", buildNameProbe, "repository"}, []string{
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
			shared := writeHeadBuild(t, fixture, sharedStubBinary)

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
			if !strings.Contains(source, `exec "$binary" "$@"`) {
				t.Errorf("%s does not reach the binary through exec with the whole argv", launcher)
			}
			for _, split := range unquotedArgvExpansions(source) {
				t.Errorf("%s expands argv as %s, which re-splits an argument on whitespace", launcher, split)
			}
			check := exec.CommandContext(t.Context(), "sh", "-n", path)
			if out, err := check.CombinedOutput(); err != nil {
				t.Fatalf("sh -n %s: %v\n%s", launcher, err, out)
			}
		})
	}
}

// unquotedArgvExpansions names every argv expansion in a launcher that a word
// split can reach: a bare $@, and $* in any form. Both join the arguments and
// re-split them on IFS, so `le commit create message "two words"` arrives as two.
//
// This replaces a count of the string "$@", which measured the wrong thing. A
// launcher is free to name the argv more than once (le execs it from the named
// path, the completion path and the newest-build path, and execs the legacy
// binary with it too); what it is never free to do is expand it unquoted. The
// count read every one of those as a defect and read a single unquoted $@ as
// correct.
func unquotedArgvExpansions(source string) []string {
	splits := make([]string, 0)
	if strings.Contains(source, "$*") {
		splits = append(splits, "$*")
	}
	for offset := 0; ; {
		at := strings.Index(source[offset:], "$@")
		if at < 0 {
			return splits
		}
		at += offset
		offset = at + 2
		quoted := at > 0 && source[at-1] == '"' && offset < len(source) && source[offset] == '"'
		if !quoted {
			splits = append(splits, "$@")
		}
	}
}

// launcherFixture copies one launcher and the files it reads into a fresh
// directory. An le fixture is also a git checkout with everything committed,
// because le builds HEAD, never the working tree.
func launcherFixture(t *testing.T, root, launcher string) string {
	t.Helper()
	fixture := t.TempDir()
	body, err := os.ReadFile(filepath.Join(root, launcher))
	if err != nil {
		t.Fatalf("read root %s: %v", launcher, err)
	}
	writeExecutable(t, filepath.Join(fixture, launcher), string(body))
	// The launchers source feature-tags to learn which gates to compile, so a
	// fixture without it is a launcher that cannot build at all. The real file is
	// copied rather than restated here: it is the reader under test, and a second
	// copy of its walk would let the two disagree without anything going red.
	reader, err := os.ReadFile(filepath.Join(root, "feature-tags"))
	if err != nil {
		t.Fatalf("read root feature-tags: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "feature-tags"), reader, 0o600); err != nil {
		t.Fatalf("write feature-tags: %v", err)
	}
	writeFakeStdpatchGoroot(t, fixture)
	gomod := "module launcher.test/fixture\n\ngo 1.27\ntoolchain " + checkoutToolchain(t, root) + "\n"
	if err := os.WriteFile(filepath.Join(fixture, "go.mod"), []byte(gomod), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	manifest := "# fixture feature gates\nze_alpha internal/alpha\nze_beta internal/beta\nze_alpha internal/alpha/sidecar\n"
	if err := os.WriteFile(filepath.Join(fixture, "feature-gates.txt"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write feature-gates.txt: %v", err)
	}
	if launcher == "le" {
		ignored := "/bin/\n/fakebin/\n/cache\n/tmp\n*.record\n*.count\n*.gate\n"
		if err := os.WriteFile(filepath.Join(fixture, ".gitignore"), []byte(ignored), 0o600); err != nil {
			t.Fatalf("write .gitignore: %v", err)
		}
		fixtureGit(t, fixture, "init", "--quiet")
		commitFixture(t, fixture, "fixture")
	}
	return fixture
}

// checkoutToolchain answers the toolchain the checkout's go.mod selects: its
// toolchain line, else the go directive. The fixture pins this one so that the
// launcher's GOTOOLCHAIN pin names a Go the machine already holds. Every
// workflow installs that version through actions/setup-go, so a fixture pinned
// to any other release made the go command fetch it on a fresh runner, and its
// "go: downloading go1.27.0 (linux/amd64)" line landed in the probe output the
// preparation tests compare exactly.
func checkoutToolchain(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checkout go.mod: %v", err)
	}
	file, err := modfile.ParseLax(path, data, nil)
	if err != nil {
		t.Fatalf("parse checkout go.mod: %v", err)
	}
	if file.Toolchain != nil {
		return file.Toolchain.Name
	}
	if file.Go == nil {
		t.Fatalf("checkout go.mod %s has neither a toolchain line nor a go directive", path)
	}
	return "go" + file.Go.Version
}

// writeFakeStdpatchGoroot gives the fixture module the program le's build_le
// runs first, `go run ./internal/le/go/stdpatch/goroot`, for the tests that
// build with the real compiler. The real program builds Ze's patched standard
// library from the checkout's patch set; the fixture has no patch set and no Ze
// import path, so this one keeps the contract the launcher relies on and no
// more: print a GOROOT holding bin/go on standard output, or fail. It answers
// the toolchain the go.mod pin selected, so the build that follows compiles
// with the same Go as the goroot step did.
func writeFakeStdpatchGoroot(t *testing.T, fixture string) {
	t.Helper()
	writeExecutable(t, filepath.Join(fixture, "internal", "le", "go", "stdpatch", "goroot", "main.go"), `package main

import (
	"os"
	"os/exec"
)

func main() {
	tree, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	os.Stdout.Write(tree)
}
`)
}

// launcherEnv removes every spelling of the outer launcher's identity and root.
// Fixture-specific overrides are appended only after this isolation boundary.
func launcherEnv() []string {
	return env.Without(os.Environ(), "ze.repo.root", "ze.le.build.name")
}

// runLauncher runs the fixture's le launcher and answers the exit code.
func runLauncher(t *testing.T, fixture string, args, extraEnv []string) int {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), filepath.Join(fixture, "le"), args...)
	cmd.Dir = fixture
	cmd.Env = append(launcherEnv(),
		"PATH="+filepath.Join(fixture, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run le %v: %v", args, err)
		}
		return exitErr.ExitCode()
	}
	return 0
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

// writeFakeGoProducing installs a fake go that answers the two calls the
// launchers make. `go run ./internal/le/go/stdpatch/goroot` is le's first step
// (build_le): the real program prints the patched GOROOT, a tree holding bin/go,
// so the fake prints a tree whose bin/go is a copy of itself. Every other call
// is the build: it records its environment and arguments and writes produced at
// its -o path. le's build therefore runs through the tree's go, as it does in
// the checkout, and ze's through the go on PATH.
func writeFakeGoProducing(t *testing.T, root, produced string) {
	t.Helper()
	writeExecutable(t, filepath.Join(root, "fakebin", "go"), `#!/bin/sh
if [ "${1-}" = run ]; then
	if [ "${2-}" != ./internal/le/go/stdpatch/goroot ]; then
		echo "fake go: unexpected go run $*" >&2
		exit 2
	fi
	if [ -n "${GOOS-}${GOARCH-}" ]; then
		echo "fake go: go run under GOOS=${GOOS-} GOARCH=${GOARCH-}, but the generator is a host program" >&2
		exit 3
	fi
	tree=$PWD/fakegoroot
	mkdir -p "$tree/bin"
	cp "$0" "$tree/bin/go"
	printf '%s\n' "$tree"
	exit 0
fi
if [ -n "${GOOS-}${GOARCH-}" ]; then
	echo "fake go: go build under GOOS=${GOOS-} GOARCH=${GOARCH-}, but le is a host program" >&2
	exit 3
fi
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

// fakeBuiltBinary stands in for a build of le. It answers the launcher's smoke
// test (help, spec) with success and records nothing for it, so the record a
// test reads is the call the test made.
const fakeBuiltBinary = `#!/bin/sh
case "${1-}" in
help | spec) exit 0 ;;
esac
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

// TestLeLauncherAcceptsTestNames proves `./le --name` builds under the names
// the D-3 refusal once held back (AC-43): no standalone harness artifact
// remains under bin/ for a build directory to collide with.
//
// Method: run the launcher over a fixture with a fake go, and read that each
// name reached the build.
//
// VALIDATES: AC-43, the launcher accepts --name test again.
// PREVENTS: a refusal left behind after the artifact it protected is gone.
func TestLeLauncherAcceptsTestNames(t *testing.T) {
	root := personalityRepoRoot(t)
	run := func(t *testing.T, name string) (fixture string, stderr string, err error) {
		t.Helper()
		fixture = launcherFixture(t, root, "le")
		writeFakeGo(t, fixture)
		writeExecutable(t, filepath.Join(fixture, "bin", "le"), sharedStubBinary)
		cmd := exec.CommandContext(t.Context(), filepath.Join(fixture, "le"), "--name", name, "spec")
		cmd.Dir = fixture
		cmd.Env = append(launcherEnv(),
			"PATH="+filepath.Join(fixture, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"),
			"ZE_GO_RECORD="+filepath.Join(fixture, "go.record"),
			"ZE_EXEC_RECORD="+filepath.Join(fixture, "exec.record"),
		)
		var output strings.Builder
		cmd.Stderr = &output
		err = cmd.Run()
		return fixture, output.String(), err
	}

	for _, name := range []string{"test", "test-linux-amd64", "test-linux-arm64", "testbed"} {
		t.Run(name, func(t *testing.T) {
			fixture, stderr, err := run(t, name)
			if err != nil {
				t.Fatalf("launcher exit = %v, stderr %q, want a build", err, stderr)
			}
			if _, statErr := os.Stat(filepath.Join(fixture, "go.record")); statErr != nil {
				t.Errorf("the name %s did not build: %v", name, statErr)
			}
		})
	}
}

// fixtureGit runs one git command in a fixture checkout and answers its
// trimmed output. The user's and the system's git configuration are shut out,
// so a signing or hook setting on the host cannot change a fixture commit, and
// every date is read in UTC, as the launcher reads it.
func fixtureGit(t *testing.T, fixture string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", fixture}, args...)...) //nolint:gosec // the test's own git arguments
	cmd.Env = append(launcherEnv(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"TZ=UTC",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// commitFixture commits the whole fixture tree. Each commit is dated one
// minute after the one before it, so builds of successive commits sort in
// commit order, as they do in a real checkout.
func commitFixture(t *testing.T, fixture, message string) {
	t.Helper()
	fixtureGit(t, fixture, "add", "-A")
	commits := 0
	if out := fixtureGit(t, fixture, "rev-list", "--all", "--count"); out != "" {
		if _, err := fmt.Sscan(out, &commits); err != nil {
			t.Fatalf("count commits %q: %v", out, err)
		}
	}
	date := fmt.Sprintf("@%d +0000", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()+int64(60*commits))
	cmd := exec.CommandContext(t.Context(), "git", "-C", fixture, "commit", "--quiet", "--allow-empty", "-m", message) //nolint:gosec // the test's own git arguments
	cmd.Env = append(launcherEnv(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, output)
	}
}

// headBuildName answers the name the launcher gives a build of the fixture's
// HEAD on this checkout's own platform: <UTC commit time>-<short sha>.
func headBuildName(t *testing.T, fixture string) string {
	t.Helper()
	stamp := fixtureGit(t, fixture, "log", "-1", "--date=format-local:%Y%m%dT%H%M%SZ", "--format=%cd")
	return stamp + "-" + fixtureGit(t, fixture, "rev-parse", "--short", "HEAD")
}

// headInputKey answers the input key the launcher records for the fixture's
// HEAD. The list of inputs is read from the script itself, so this test holds
// no second copy of it.
func headInputKey(t *testing.T, fixture string) string {
	t.Helper()
	script, err := os.ReadFile(filepath.Join(fixture, "le"))
	if err != nil {
		t.Fatal(err)
	}
	const assignment = "\nle_inputs='"
	_, rest, found := strings.Cut(string(script), assignment)
	if !found {
		t.Fatalf("the le script no longer assigns %q", strings.TrimSpace(assignment))
	}
	inputs, _, _ := strings.Cut(rest, "'")
	return fixtureGit(t, fixture, append([]string{"ls-tree", "HEAD", "--"}, strings.Fields(inputs)...)...) + "\n"
}

// writeBuild publishes a build into the fixture's bin/builds/<name>/ the way
// the launcher does: its le, the input key it was built from, and the smoke
// record that says it passed. It answers the le path.
func writeBuild(t *testing.T, fixture, name, body, key string) string {
	t.Helper()
	dir := filepath.Join(fixture, "bin", "builds", name)
	writeExecutable(t, filepath.Join(dir, "le"), body)
	if err := os.WriteFile(filepath.Join(dir, "inputs"), []byte(key), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "smoke"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "le")
}

// writeHeadBuild publishes a build of the fixture's HEAD, current for its
// inputs, and answers its le path.
func writeHeadBuild(t *testing.T, fixture, body string) string {
	t.Helper()
	return writeBuild(t, fixture, headBuildName(t, fixture), body, headInputKey(t, fixture))
}

// recordingBuild is a build that writes its label and then its argv to
// ZE_EXEC_RECORD, so a test reads which build ran and with what.
func recordingBuild(label string) string {
	return "#!/bin/sh\ncase \"${1-}\" in\nhelp | spec) exit 0 ;;\nesac\nprintf '%s\\n' '" + label + "' \"$@\" > \"$ZE_EXEC_RECORD\"\n"
}

// writeHeadGo installs a fake go for builds of HEAD. It answers the goroot
// step as writeFakeGoProducing does, appends the directory it built in to
// ZE_GO_COUNT, waits for ZE_GO_GATE when one is named (at most 600 checks a
// tenth of a second apart), and fails when the tree it compiles carries
// cmd/ze/marker "fail". The le it produces records the marker it was built from
// and then its argv, and fails the smoke test when the marker is "smoke-fails".
func writeHeadGo(t *testing.T, fixture string) {
	t.Helper()
	writeExecutable(t, filepath.Join(fixture, "fakebin", "go"), `#!/bin/sh
if [ "${1-}" = run ]; then
	tree=$PWD/fakegoroot
	mkdir -p "$tree/bin"
	cp "$0" "$tree/bin/go"
	printf '%s\n' "$tree"
	exit 0
fi
printf '%s\n' "$PWD" >> "${ZE_GO_COUNT:-/dev/null}"
if [ -n "${ZE_GO_GATE-}" ]; then
	checks=0
	while [ ! -e "$ZE_GO_GATE" ] && [ "$checks" -lt 600 ]; do
		sleep 0.1
		checks=$((checks + 1))
	done
fi
marker=$(cat cmd/ze/marker 2>/dev/null || echo none)
if [ "$marker" = fail ]; then
	echo "fake go: the tree says fail" >&2
	exit 1
fi
out=
previous=
for arg do
	if [ "$previous" = -o ]; then
		out=$arg
	fi
	previous=$arg
done
mkdir -p "$(dirname "$out")"
{
	printf '#!/bin/sh\nmarker=%s\n' "$marker"
	cat <<'EOF_BUILT'
case "${1-}" in
help | spec)
	if [ "$marker" = smoke-fails ]; then
		exit 3
	fi
	exit 0
	;;
esac
printf '%s\n' "$marker" "$@" > "$ZE_EXEC_RECORD"
EOF_BUILT
} > "$out"
chmod +x "$out"
`)
}

// writeMarker writes the fixture's cmd/ze/marker, the value writeHeadGo's
// builds report.
func writeMarker(t *testing.T, fixture, marker string) {
	t.Helper()
	writeExecutable(t, filepath.Join(fixture, "cmd", "ze", "marker"), marker+"\n")
}

// launcherRun runs a launcher at path from the fixture with exactly env, and
// answers the exit code and stderr.
func launcherRun(t *testing.T, fixture, path string, env, args []string) (int, string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), path, args...) //nolint:gosec // the fixture's launcher
	cmd.Dir = fixture
	cmd.Env = env
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stderr.String()
	}
	if err != nil {
		t.Fatalf("run %s %v: %v", path, args, err)
	}
	return 0, stderr.String()
}

// headEnv is the environment of the HEAD-build tests: the fake go first on
// PATH, its count and gate files, and the exec record.
func headEnv(fixture string, extra ...string) []string {
	env := append(launcherEnv(),
		"PATH="+filepath.Join(fixture, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_GO_COUNT="+filepath.Join(fixture, "go.count"),
		"ZE_EXEC_RECORD="+filepath.Join(fixture, "exec.record"),
	)
	return append(env, extra...)
}

// goCount answers the directories the fake go built in, one per build.
func goCount(t *testing.T, fixture string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(fixture, "go.count"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(body))
}

// waitFor polls cond every 20ms for at most a minute, the bound on a fixture
// build, and fails naming what never happened.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("after a minute: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// lockGone answers whether no candidate build holds the fixture's lock.
func lockGone(fixture string) func() bool {
	return func() bool {
		_, err := os.Stat(filepath.Join(fixture, "bin", "builds", ".building"))
		return errors.Is(err, os.ErrNotExist)
	}
}

// TestLauncherRunsNewestBuild: with two builds for this platform, ./le and its
// installed copy bin/le both exec the newer, and a HEAD whose inputs match it
// starts no build.
//
// VALIDATES: AC-1, AC-2, and that bin/le is the dispatcher, choosing as ./le.
// PREVENTS: an older build answering, or a build started for a current HEAD.
func TestLauncherRunsNewestBuild(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadGo(t, fixture)
	writeBuild(t, fixture, "20200101T000000Z-aaaaaaa", recordingBuild("A"), "older inputs\n")
	writeHeadBuild(t, fixture, recordingBuild("B"))
	execRecord := filepath.Join(fixture, "exec.record")

	if code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "le"), headEnv(fixture), []string{"two words"}); code != 0 {
		t.Fatalf("./le exit %d: %s", code, stderr)
	}
	assertLauncherRecord(t, execRecord, []string{"B", "two words"})

	script, err := os.ReadFile(filepath.Join(fixture, "le"))
	if err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile(filepath.Join(fixture, "bin", "le"))
	if err != nil {
		t.Fatalf("./le did not install itself as bin/le: %v", err)
	}
	if !bytes.Equal(installed, script) {
		t.Fatal("bin/le is not a copy of ./le")
	}
	if code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "bin", "le"), headEnv(fixture), []string{"from bin"}); code != 0 {
		t.Fatalf("bin/le exit %d: %s", code, stderr)
	}
	assertLauncherRecord(t, execRecord, []string{"B", "from bin"})
	if builds := goCount(t, fixture); len(builds) != 0 {
		t.Fatalf("a current HEAD started builds in %v", builds)
	}
	if !lockGone(fixture)() {
		t.Fatal("a current HEAD took the build lock")
	}
}

// TestLauncherBuildsCandidateFromHEAD walks a commit through the launcher: a
// commit outside le's inputs builds nothing, a commit to an input starts one
// background build of HEAD while every caller keeps running the current build,
// and the next call after the build lands runs it. An uncommitted edit made
// after the commit never reaches the build.
//
// VALIDATES: AC-3, AC-4, AC-8.
// PREVENTS: a caller waiting on a build, two builds of one HEAD, and a peer's
// half-written edit reaching the shared le.
func TestLauncherBuildsCandidateFromHEAD(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadGo(t, fixture)
	writeMarker(t, fixture, "one")
	commitFixture(t, fixture, "marker one")
	writeHeadBuild(t, fixture, recordingBuild("B"))
	execRecord := filepath.Join(fixture, "exec.record")
	launcher := filepath.Join(fixture, "le")
	gate := filepath.Join(fixture, "build.gate")
	env := headEnv(fixture, "ZE_GO_GATE="+gate)

	// AC-8: a commit that changes no input.
	if err := os.WriteFile(filepath.Join(fixture, "README"), []byte("prose\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commitFixture(t, fixture, "prose only")
	launcherRun(t, fixture, launcher, env, []string{"probe"})
	assertLauncherRecord(t, execRecord, []string{"B", "probe"})
	if builds := goCount(t, fixture); len(builds) != 0 {
		t.Fatalf("a commit outside the inputs started builds in %v", builds)
	}

	// AC-3 and AC-4: a commit to an input, then an uncommitted edit on top.
	writeMarker(t, fixture, "two")
	commitFixture(t, fixture, "marker two")
	want := headBuildName(t, fixture)
	writeMarker(t, fixture, "uncommitted")

	launcherRun(t, fixture, launcher, env, []string{"first"})
	assertLauncherRecord(t, execRecord, []string{"B", "first"})
	waitFor(t, "the candidate build started", func() bool { return len(goCount(t, fixture)) == 1 })
	launcherRun(t, fixture, launcher, env, []string{"second"})
	assertLauncherRecord(t, execRecord, []string{"B", "second"})
	if builds := goCount(t, fixture); len(builds) != 1 {
		t.Fatalf("a call during the candidate build started another: %v", builds)
	}
	if built := goCount(t, fixture)[0]; !strings.HasSuffix(built, filepath.Join("bin", "builds", ".building", "src")) {
		t.Fatalf("the candidate compiled %s, not the HEAD export", built)
	}

	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the candidate build finished", lockGone(fixture))
	if _, err := os.Stat(filepath.Join(fixture, "bin", "builds", want, "le")); err != nil {
		t.Fatalf("the candidate was not published as bin/builds/%s: %v", want, err)
	}
	launcherRun(t, fixture, launcher, env, []string{"third"})
	assertLauncherRecord(t, execRecord, []string{"two", "third"})
	if builds := goCount(t, fixture); len(builds) != 1 {
		t.Fatalf("the published HEAD was built again: %v", builds)
	}
}

// TestLauncherRefusesAFailedCandidate: a HEAD that does not compile, and one
// that compiles and fails the smoke test, are never published; the next caller
// is told once, naming the commit and the log, and the build is not retried
// for that commit.
//
// VALIDATES: AC-5, A-2.
// PREVENTS: a broken HEAD becoming the shared le, and a warning or a rebuild
// on every call.
func TestLauncherRefusesAFailedCandidate(t *testing.T) {
	for _, marker := range []string{"fail", "smoke-fails"} {
		t.Run(marker, func(t *testing.T) {
			fixture := launcherFixture(t, personalityRepoRoot(t), "le")
			writeHeadGo(t, fixture)
			writeHeadBuild(t, fixture, recordingBuild("B"))
			writeMarker(t, fixture, marker)
			commitFixture(t, fixture, "a broken HEAD")
			head := fixtureGit(t, fixture, "rev-parse", "HEAD")
			execRecord := filepath.Join(fixture, "exec.record")
			launcher := filepath.Join(fixture, "le")
			env := headEnv(fixture)

			launcherRun(t, fixture, launcher, env, []string{"first"})
			assertLauncherRecord(t, execRecord, []string{"B", "first"})
			waitFor(t, "the candidate build finished", lockGone(fixture))
			if _, err := os.Stat(filepath.Join(fixture, "bin", "builds", headBuildName(t, fixture))); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a broken HEAD was published: %v", err)
			}
			if _, err := os.Stat(filepath.Join(fixture, "bin", "builds", ".failed-"+head+".log")); err != nil {
				t.Fatalf("the failure left no log: %v", err)
			}

			_, stderr := launcherRun(t, fixture, launcher, env, []string{"second"})
			assertLauncherRecord(t, execRecord, []string{"B", "second"})
			if !strings.Contains(stderr, head) || !strings.Contains(stderr, ".failed-"+head+".log") {
				t.Fatalf("the next caller was not told the commit and the log: %q", stderr)
			}
			_, stderr = launcherRun(t, fixture, launcher, env, []string{"third"})
			if strings.Contains(stderr, head) {
				t.Fatalf("the failure was reported twice: %q", stderr)
			}
			if builds := goCount(t, fixture); len(builds) != 1 {
				t.Fatalf("a rejected HEAD was built again: %v", builds)
			}
		})
	}
}

// TestLauncherRollsBackAFailingBuild: a newest build that cannot start (not
// executable, or crashing in its first smoke test) is moved aside and the one
// before it runs, in the same call; `./le rollback` does the same on demand.
// Neither rebuilds the HEAD whose build was moved aside.
//
// VALIDATES: AC-6, AC-7.
// PREVENTS: a dead le after a bad build, a deleted build a command still runs,
// and a rollback the next call silently undoes.
func TestLauncherRollsBackAFailingBuild(t *testing.T) {
	cases := []struct {
		name       string
		breakBuild func(t *testing.T, le string)
		args       []string
	}{
		{name: "not executable", breakBuild: func(t *testing.T, le string) {
			t.Helper()
			if err := os.Chmod(le, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "crashes at start", breakBuild: func(t *testing.T, le string) {
			t.Helper()
			writeExecutable(t, le, "#!/bin/sh\nexit 2\n")
			if err := os.Remove(filepath.Join(filepath.Dir(le), "smoke")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "rollback command", breakBuild: func(*testing.T, string) {}, args: []string{"rollback"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fixture := launcherFixture(t, personalityRepoRoot(t), "le")
			writeHeadGo(t, fixture)
			const older = "20200101T000000Z-aaaaaaa"
			writeBuild(t, fixture, older, recordingBuild("A"), "older inputs\n")
			newest := headBuildName(t, fixture)
			tt.breakBuild(t, writeHeadBuild(t, fixture, recordingBuild("B")))
			execRecord := filepath.Join(fixture, "exec.record")
			launcher := filepath.Join(fixture, "le")
			env := headEnv(fixture)

			if tt.args != nil {
				code, stderr := launcherRun(t, fixture, launcher, env, tt.args)
				if code != 0 {
					t.Fatalf("rollback exit %d: %s", code, stderr)
				}
				if !strings.Contains(stderr, ".failed-"+newest) || !strings.Contains(stderr, older) {
					t.Fatalf("rollback did not name what it moved and what runs next: %q", stderr)
				}
			}
			_, stderr := launcherRun(t, fixture, launcher, env, []string{"probe"})
			assertLauncherRecord(t, execRecord, []string{"A", "probe"})
			if tt.args == nil && (!strings.Contains(stderr, newest) || !strings.Contains(stderr, older)) {
				t.Fatalf("the rollback line did not name the failed build and the next: %q", stderr)
			}
			if _, err := os.Stat(filepath.Join(fixture, "bin", "builds", ".failed-"+newest, "le")); err != nil {
				t.Fatalf("the failing build was not moved aside whole: %v", err)
			}
			if builds := goCount(t, fixture); len(builds) != 0 {
				t.Fatalf("the rolled-back HEAD was built again: %v", builds)
			}
		})
	}

	t.Run("nothing to roll back", func(t *testing.T) {
		fixture := launcherFixture(t, personalityRepoRoot(t), "le")
		code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "le"), headEnv(fixture), []string{"rollback"})
		if code != 2 || !strings.Contains(stderr, "no build of le") {
			t.Fatalf("rollback with no build: exit %d, %q", code, stderr)
		}
	})
}

// TestLauncherFreshCheckoutBuildsInForeground: with no build at all, the first
// call builds HEAD itself and runs it, and the installed bin/le then runs the
// same build without building again.
//
// VALIDATES: AC-9.
// PREVENTS: a fresh checkout with no working le.
func TestLauncherFreshCheckoutBuildsInForeground(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadGo(t, fixture)
	writeMarker(t, fixture, "fresh")
	commitFixture(t, fixture, "fresh")
	execRecord := filepath.Join(fixture, "exec.record")

	if code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "le"), headEnv(fixture), []string{"probe"}); code != 0 {
		t.Fatalf("./le exit %d: %s", code, stderr)
	}
	assertLauncherRecord(t, execRecord, []string{"fresh", "probe"})
	dir := filepath.Join(fixture, "bin", "builds", headBuildName(t, fixture))
	for _, name := range []string{"le", "inputs", "smoke"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("the foreground build lacks %s: %v", name, err)
		}
	}
	if !lockGone(fixture)() {
		t.Error("the foreground build left its lock")
	}
	if code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "bin", "le"), headEnv(fixture), []string{"again"}); code != 0 {
		t.Fatalf("bin/le exit %d: %s", code, stderr)
	}
	assertLauncherRecord(t, execRecord, []string{"fresh", "again"})
	if builds := goCount(t, fixture); len(builds) != 1 {
		t.Fatalf("builds = %v, want the one foreground build", builds)
	}
}

// TestLauncherRefusesAnUnwritableBuildsDirectory: with no build and a bin/
// that cannot hold bin/builds, the foreground call refuses at once, naming the
// directory, the cause and the fix, rather than reading the failed mkdir as a
// lock another le holds and waiting fifteen minutes for a build that never
// starts.
//
// Method: bin/ is made read-only. Root ignores directory permission bits, so
// under root the same failure is made by a regular file standing where bin/
// goes; either way `mkdir -p bin/builds` fails and the same path refuses.
//
// VALIDATES: spec-le-built-from-head B-4, take_lock telling a busy lock from
// one that cannot be made.
// PREVENTS: a fresh checkout whose bin/ is unwritable hanging every caller
// with a message that blames another le.
func TestLauncherRefusesAnUnwritableBuildsDirectory(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadGo(t, fixture)
	writeMarker(t, fixture, "fresh")
	commitFixture(t, fixture, "fresh")
	bin := filepath.Join(fixture, "bin")
	wants := []string{"cannot create " + filepath.Join(bin, "builds"), "then run ./le again"}
	if os.Geteuid() == 0 {
		if err := os.WriteFile(bin, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		wants = append(wants, "Not a directory")
	} else {
		if err := os.MkdirAll(bin, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(bin, 0o550); err != nil { //nolint:gosec // a read-only directory is the case under test
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(bin, 0o750); err != nil { //nolint:gosec // restored so the temporary directory can be removed
				t.Errorf("restore bin/ permissions: %v", err)
			}
		})
		wants = append(wants, "Permission denied", "chmod u+w '"+bin+"'")
	}

	// The bound is far below the fifteen-minute wait, so a launcher that waits
	// fails here instead of hanging the run.
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(fixture, "le"), "probe")
	cmd.Dir = fixture
	cmd.Env = headEnv(fixture)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("the launcher was still running after a minute, waiting on a lock that cannot be made: %s", stderr.String())
	}
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("launcher answered %v, want a refusal; stderr: %s", err, stderr.String())
	}
	if exitErr.ExitCode() == 0 {
		t.Fatalf("launcher exit 0, want a refusal; stderr: %s", stderr.String())
	}
	message := stderr.String()
	for _, want := range wants {
		if !strings.Contains(message, want) {
			t.Errorf("refusal %q does not carry %q", message, want)
		}
	}
	if strings.Contains(message, "another le is building") {
		t.Errorf("refusal %q blames another le for a directory that cannot be created", message)
	}
	if builds := goCount(t, fixture); len(builds) != 0 {
		t.Errorf("a refused call still built: %v", builds)
	}
	if _, err := os.Stat(filepath.Join(fixture, "exec.record")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused call still ran a build: %v", err)
	}
}

// TestLauncherMigrationKeepsTheLegacyBuild: right after the layout changed,
// bin/builds holds only the old binary the migration kept. Every caller runs
// it at once while HEAD builds in the background, and the build that lands
// takes over; a process already running the legacy binary keeps its file.
//
// VALIDATES: AC-16, and that no caller waits on the first build of a migrated
// checkout.
// PREVENTS: every session's hooks building le in the foreground at once, or a
// moment with no le to run.
func TestLauncherMigrationKeepsTheLegacyBuild(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadGo(t, fixture)
	writeMarker(t, fixture, "head")
	commitFixture(t, fixture, "head")
	legacy := filepath.Join(fixture, "bin", "builds", ".previous-legacy", "le")
	writeExecutable(t, legacy, recordingBuild("legacy"))
	execRecord := filepath.Join(fixture, "exec.record")
	gate := filepath.Join(fixture, "build.gate")
	env := headEnv(fixture, "ZE_GO_GATE="+gate)

	launcherRun(t, fixture, filepath.Join(fixture, "le"), env, []string{"first"})
	assertLauncherRecord(t, execRecord, []string{"legacy", "first"})
	waitFor(t, "the background build started", func() bool { return len(goCount(t, fixture)) == 1 })
	launcherRun(t, fixture, filepath.Join(fixture, "le"), env, []string{"second"})
	assertLauncherRecord(t, execRecord, []string{"legacy", "second"})

	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the background build finished", lockGone(fixture))
	launcherRun(t, fixture, filepath.Join(fixture, "le"), env, []string{"third"})
	assertLauncherRecord(t, execRecord, []string{"head", "third"})
	if builds := goCount(t, fixture); len(builds) != 1 {
		t.Fatalf("builds = %v, want one", builds)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("the legacy binary was removed while it may still run: %v", err)
	}
}

// foreignHeader answers the first bytes of an executable for a platform that
// is not this one: Mach-O arm64 on Linux, ELF x86-64 elsewhere.
func foreignHeader() string {
	if runtime.GOOS == "linux" {
		return "\xcf\xfa\xed\xfe\x0c\x00\x00\x01" + strings.Repeat("\x00", 24)
	}
	return "\x7fELF" + strings.Repeat("\x00", 14) + "\x3e\x00" + strings.Repeat("\x00", 12)
}

// TestLauncherPicksItsPlatform: a machine that shares the checkout with another
// platform (a QEMU guest) finds the checkout's own build unrunnable, builds HEAD
// under a name carrying its GOOS and GOARCH, and runs that build from then on.
//
// VALIDATES: AC-10, and that the script's platform words are Go's.
// PREVENTS: a guest execing the host's binary, or rebuilding over it.
func TestLauncherPicksItsPlatform(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadGo(t, fixture)
	writeMarker(t, fixture, "guest")
	commitFixture(t, fixture, "guest")
	foreign := writeHeadBuild(t, fixture, foreignHeader())
	execRecord := filepath.Join(fixture, "exec.record")

	for _, call := range []string{"first", "second"} {
		if code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "le"), headEnv(fixture), []string{call}); code != 0 {
			t.Fatalf("./le %s exit %d: %s", call, code, stderr)
		}
		assertLauncherRecord(t, execRecord, []string{"guest", call})
	}
	platform := headBuildName(t, fixture) + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if _, err := os.Stat(filepath.Join(fixture, "bin", "builds", platform, "le")); err != nil {
		t.Fatalf("no build named for this platform: %v", err)
	}
	if body, err := os.ReadFile(foreign); err != nil || string(body) != foreignHeader() {
		t.Fatalf("the other platform's build was touched: %v", err)
	}
	if builds := goCount(t, fixture); len(builds) != 1 {
		t.Fatalf("builds = %v, want one", builds)
	}
}

// TestLauncherJudgesALockByItsHost: the build lock names the host that took
// it. A lock from another host, whose pid means nothing here, holds until it is
// thirty minutes old and is then reclaimed; a lock from this host whose pid is
// dead is reclaimed at once. The local identity is read from a lock the
// launcher itself wrote, so the test holds no second copy of how it is formed.
//
// VALIDATES: R-8, that a QEMU guest sharing bin/builds over 9p cannot judge a
// live host build dead.
// PREVENTS: a guest reclaiming the host's lock and removing its export
// mid-build, or a dead lock from this host blocking every candidate.
func TestLauncherJudgesALockByItsHost(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadGo(t, fixture)
	writeMarker(t, fixture, "one")
	commitFixture(t, fixture, "marker one")
	writeHeadBuild(t, fixture, recordingBuild("B"))
	execRecord := filepath.Join(fixture, "exec.record")
	launcher := filepath.Join(fixture, "le")
	lock := filepath.Join(fixture, "bin", "builds", ".building")
	gate := filepath.Join(fixture, "build.gate")
	env := headEnv(fixture, "ZE_GO_GATE="+gate)
	// No process here has this pid: it is above every system's pid limit.
	const deadPid = "2147483646\n"

	writeMarker(t, fixture, "two")
	commitFixture(t, fixture, "marker two")
	launcherRun(t, fixture, launcher, env, []string{"first"})
	waitFor(t, "the candidate build started", func() bool { return len(goCount(t, fixture)) == 1 })
	localHost, err := os.ReadFile(filepath.Join(lock, "host"))
	if err != nil {
		t.Fatalf("the launcher's lock names no host: %v", err)
	}
	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the candidate build finished", lockGone(fixture))

	writeLock := func(host, pid string, age time.Duration) {
		t.Helper()
		if err := os.MkdirAll(lock, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lock, "host"), []byte(host), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lock, "pid"), []byte(pid), 0o600); err != nil {
			t.Fatal(err)
		}
		then := time.Now().Add(-age)
		if err := os.Chtimes(lock, then, then); err != nil {
			t.Fatal(err)
		}
	}
	foreignHost := "another-host 00000000-0000-0000-0000-000000000000\n"

	// Another host's lock with a pid dead here holds.
	writeMarker(t, fixture, "three")
	commitFixture(t, fixture, "marker three")
	writeLock(foreignHost, deadPid, 29*time.Minute)
	launcherRun(t, fixture, launcher, env, []string{"guest"})
	assertLauncherRecord(t, execRecord, []string{"two", "guest"})
	if builds := goCount(t, fixture); len(builds) != 1 {
		t.Fatalf("another host's live lock was reclaimed: builds %v", builds)
	}
	if body, err := os.ReadFile(filepath.Join(lock, "host")); err != nil || string(body) != foreignHost {
		t.Fatalf("another host's live lock was replaced: %q, %v", body, err)
	}

	// The same lock past thirty minutes is reclaimed.
	writeLock(foreignHost, deadPid, 31*time.Minute)
	launcherRun(t, fixture, launcher, env, []string{"stale"})
	assertLauncherRecord(t, execRecord, []string{"two", "stale"})
	waitFor(t, "a candidate took over the stale lock", func() bool { return len(goCount(t, fixture)) == 2 })
	waitFor(t, "the candidate build finished", lockGone(fixture))
	launcherRun(t, fixture, launcher, env, []string{"after"})
	assertLauncherRecord(t, execRecord, []string{"three", "after"})

	// This host's lock with a dead pid is reclaimed at once.
	writeMarker(t, fixture, "four")
	commitFixture(t, fixture, "marker four")
	writeLock(string(localHost), deadPid, 0)
	launcherRun(t, fixture, launcher, env, []string{"local"})
	waitFor(t, "a candidate took over the dead local lock", func() bool { return len(goCount(t, fixture)) == 3 })
	waitFor(t, "the candidate build finished", lockGone(fixture))
	launcherRun(t, fixture, launcher, env, []string{"last"})
	assertLauncherRecord(t, execRecord, []string{"four", "last"})
}

// TestLauncherMigrationPassesOverAForeignLegacy: a machine sharing a migrated
// checkout (a QEMU guest) finds no build and a legacy binary for the other
// platform. It never execs that binary: it builds HEAD in the foreground, as a
// fresh checkout does, and leaves the legacy binary to the host.
//
// VALIDATES: R-9.
// PREVENTS: a guest failing with an exec format error during the migration.
func TestLauncherMigrationPassesOverAForeignLegacy(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadGo(t, fixture)
	writeMarker(t, fixture, "head")
	commitFixture(t, fixture, "head")
	legacy := filepath.Join(fixture, "bin", "builds", ".previous-legacy", "le")
	writeExecutable(t, legacy, foreignHeader())

	if code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "le"), headEnv(fixture), []string{"probe"}); code != 0 {
		t.Fatalf("./le exit %d: %s", code, stderr)
	}
	assertLauncherRecord(t, filepath.Join(fixture, "exec.record"), []string{"head", "probe"})
	if builds := goCount(t, fixture); len(builds) != 1 {
		t.Fatalf("builds = %v, want the one foreground build", builds)
	}
	if !lockGone(fixture)() {
		t.Error("the foreground build left its lock")
	}
	if body, err := os.ReadFile(legacy); err != nil || string(body) != foreignHeader() {
		t.Fatalf("the other platform's legacy binary was touched: %v", err)
	}
}

// TestLauncherSessionName: `--name session` builds the working tree under the
// session's own name, and refuses, naming the variable, when no session id is
// set.
//
// VALIDATES: AC-11.
// PREVENTS: two sessions sharing a private build, or one built under "session".
func TestLauncherSessionName(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeFakeGoProducing(t, fixture, buildNameEchoBinary)
	execRecord := filepath.Join(fixture, "exec.record")
	env := append(launcherEnv(),
		"PATH="+filepath.Join(fixture, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_GO_RECORD="+filepath.Join(fixture, "go.record"),
		"ZE_EXEC_RECORD="+execRecord,
	)
	withSession := append(slices.Clone(env), "CLAUDE_CODE_SESSION_ID=0123abcd-4567-89ef-0123-456789abcdef")
	if code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "le"), withSession, []string{"--name", "session", "spec"}); code != 0 {
		t.Fatalf("--name session exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(fixture, "bin", "le-s-0123abcd", "le")); err != nil {
		t.Fatalf("--name session did not build bin/le-s-0123abcd/le: %v", err)
	}
	assertLauncherRecord(t, execRecord, []string{"s-0123abcd"})

	without := slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		return strings.HasPrefix(entry, "CLAUDE_CODE_SESSION_ID=")
	})
	code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "le"), without, []string{"--name", "session", "spec"})
	if code != 2 || !strings.Contains(stderr, "CLAUDE_CODE_SESSION_ID") {
		t.Fatalf("--name session with no id: exit %d, %q", code, stderr)
	}
}

// TestLauncherRefusesUpdate: `--update` is gone, and says what replaced it.
//
// VALIDATES: AC-15.
// PREVENTS: a caller believing it refreshed le.
func TestLauncherRefusesUpdate(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadBuild(t, fixture, recordingBuild("B"))
	code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "le"), headEnv(fixture), []string{"--update", "probe"})
	if code != 2 || !strings.Contains(stderr, "follows HEAD") || !strings.Contains(stderr, "./le rollback") {
		t.Fatalf("--update: exit %d, %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(fixture, "exec.record")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("--update still ran a build: %v", err)
	}
}

// TestLauncherCompletionRunsTheNewestWithoutAKeyCheck: a Tab press runs
// `bin/le completion words ...`, which execs the newest build even when HEAD
// has moved past it, and starts no build.
//
// VALIDATES: AC-17.
// PREVENTS: a build started, or git consulted for a key, on every keystroke.
func TestLauncherCompletionRunsTheNewestWithoutAKeyCheck(t *testing.T) {
	fixture := launcherFixture(t, personalityRepoRoot(t), "le")
	writeHeadGo(t, fixture)
	writeBuild(t, fixture, headBuildName(t, fixture), recordingBuild("B"), "stale inputs\n")
	script, err := os.ReadFile(filepath.Join(fixture, "le"))
	if err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(fixture, "bin", "le"), string(script))
	if code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "bin", "le"), headEnv(fixture), []string{"completion", "words", "le", "sp"}); code != 0 {
		t.Fatalf("completion exit %d: %s", code, stderr)
	}
	assertLauncherRecord(t, filepath.Join(fixture, "exec.record"), []string{"B", "completion", "words", "le", "sp"})
	if !lockGone(fixture)() {
		t.Fatal("a completion call took the build lock")
	}
	if builds := goCount(t, fixture); len(builds) != 0 {
		t.Fatalf("a completion call started builds: %v", builds)
	}
}

// TestLauncherAgreesWithTheResolver feeds the script and gotoolchain's name
// grammar the same build names, and the same mix of platforms, and requires the
// same newest build from both. The script's choice is read from the build it
// execs; the Go side is resolverNewest over gotoolchain.ParseBuildName and
// gotoolchain.RunnableBuild.
//
// VALIDATES: the shell name grammar, its platform filter and its runnable test
// match ParseBuildName and RunnableBuild, the newest is the last sorted name,
// and the script's builds directory is gotoolchain.BuildsDir.
// PREVENTS: the store trim, which judges builds by ParseBuildName and
// RunnableBuild, and the launcher disagreeing on which entries are builds.
func TestLauncherAgreesWithTheResolver(t *testing.T) {
	root := personalityRepoRoot(t)
	script, err := os.ReadFile(filepath.Join(root, "le"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "builds=$root/" + strings.TrimPrefix(gotoolchain.BuildsDir(""), string(filepath.Separator)) + "\n"; !strings.Contains(string(script), want) {
		t.Fatalf("the le script does not assign %q, the directory gotoolchain.BuildsDir names", strings.TrimSpace(want))
	}

	const base = "20200101T000000Z-0000000"
	sha40 := strings.Repeat("abcdef0123", 4)
	names := []string{
		"20261025T101010Z-abcdef0",
		"20261025T101010Z-" + sha40,
		"20261025T101010Z-" + sha40 + "a",
		"20261025T101010Z-abcdef",
		"20261025T101010Z-ABCDEF0",
		"20240229T000000Z-abcdef0",
		"20000229T000000Z-abcdef0",
		"20260229T000000Z-abcdef0",
		"21000229T000000Z-abcdef0",
		"20260230T000000Z-abcdef0",
		"20260431T000000Z-abcdef0",
		"20260531T000000Z-abcdef0",
		"20261231T235959Z-abcdef0",
		"20261325T101010Z-abcdef0",
		"20260025T101010Z-abcdef0",
		"20261000T101010Z-abcdef0",
		"20261025T241010Z-abcdef0",
		"20261025T106010Z-abcdef0",
		"20261025T101060Z-abcdef0",
		"2026102T101010Z-abcdef0",
		"20261025T101010-abcdef0",
		"20261025T101010Z",
		"20261025T101010Z-abcdef0-linux",
		"20261025T101010Z-abcdef0-linux-arm64",
		"20261025T101010Z-abcdef0-linux-arm64-x",
		"20261025T101010Z-abcdef0-Linux-arm64",
		".failed-20261025T101010Z-abcdef0",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			fixture := launcherFixture(t, root, "le")
			writeBuild(t, fixture, base, recordingBuild(base), "")
			writeBuild(t, fixture, name, recordingBuild(name), "")
			assertScriptChooses(t, fixture, resolverNewest(t, fixture, ""))
		})
	}

	t.Run("another platform", func(t *testing.T) {
		fixture := launcherFixture(t, root, "le")
		platform := "-" + runtime.GOOS + "-" + runtime.GOARCH
		writeBuild(t, fixture, "20261025T101010Z-abcdef0", foreignHeader(), "")
		writeBuild(t, fixture, "20261024T101010Z-abcdef0"+platform, recordingBuild("older"), "")
		writeBuild(t, fixture, "20261025T090909Z-abcdef0"+platform, recordingBuild("newer"), "")
		writeBuild(t, fixture, "20261026T101010Z-abcdef0-plan9-mips", recordingBuild("other"), "")
		if got := resolverNewest(t, fixture, runtime.GOOS+"-"+runtime.GOARCH); got != "20261025T090909Z-abcdef0"+platform {
			t.Fatalf("resolverNewest = %s", got)
		}
		assertScriptChooses(t, fixture, "newer")
	})
}

// resolverNewest answers the newest build of one platform ("" is the
// checkout's own) as gotoolchain's grammar reads bin/builds: of the sorted
// names, the last that ParseBuildName reads with that platform, that is a
// directory and not a symbolic link, and that RunnableBuild accepts.
func resolverNewest(t *testing.T, fixture, platform string) string {
	t.Helper()
	dir := gotoolchain.BuildsDir(fixture)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	for _, name := range slices.Backward(names) {
		parsed, ok := gotoolchain.ParseBuildName(name)
		if !ok {
			continue
		}
		if parsed.Platform() != platform {
			continue
		}
		build := filepath.Join(dir, name)
		info, err := os.Lstat(build)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			continue
		}
		runnable, err := gotoolchain.RunnableBuild(build)
		if err != nil {
			t.Fatal(err)
		}
		if runnable {
			return name
		}
	}
	t.Fatalf("no build of platform %q in %s", platform, dir)
	return ""
}

// assertScriptChooses runs the fixture's launcher through its completion path,
// which execs the newest build and nothing else, and requires the build that
// ran to record label.
func assertScriptChooses(t *testing.T, fixture, label string) {
	t.Helper()
	code, stderr := launcherRun(t, fixture, filepath.Join(fixture, "le"), headEnv(fixture), []string{"completion", "words"})
	if code != 0 {
		t.Fatalf("launcher exit %d: %s", code, stderr)
	}
	assertLauncherRecord(t, filepath.Join(fixture, "exec.record"), []string{label, "completion", "words"})
}
