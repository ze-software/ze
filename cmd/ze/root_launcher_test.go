// VALIDATES: the tracked root launchers build one cmd/ze composition when absent
// and exec the selected personality without changing process semantics, and that
// `./le --name <name>` gives one session a rebuilt binary of its own.
// PREVENTS: Python fallback, freshness rebuilds, tag drift, argv splitting, a
// shell parent masking the binary's exit status or terminating signal, a named
// build writing the shared bin/le, and a name reaching the filesystem unchecked.
package main

import (
	"bufio"
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

	"github.com/ze-software/ze/internal/core/env"
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

			// The same assertion over the one path that does NOT exec.
			//
			// le_sampled sends one call in sixteen through a branch that runs the
			// binary as a CHILD, so it can print the staleness warning after the
			// answer. That branch is chosen by `$$ % 16`, so the run above reaches
			// it on one pid in sixteen and the case this test is named for was
			// covered by luck: it went red in a full verification run and passed
			// on the next fifteen. Forcing the sample is what makes the coverage
			// a fact rather than a coin toss.
			//
			// Only `le` carries the branch: `ze` execs on every call. That is
			// asserted rather than assumed, so a `ze` that grows one, or an `le`
			// that loses it, is not silently left uncovered.
			sampled, samples := forcedSampleLauncher(t, fixture, launcher)
			if samples != (launcher == "le") {
				t.Fatalf("%s samples = %v; the sampling branch belongs to le alone", launcher, samples)
			}
			if !samples {
				return
			}
			signalSampled := exec.CommandContext(t.Context(), sampled, "signal")
			signalSampled.Dir = fixture
			signalSampled.Env = append(launcherEnv(), "ZE_EXEC_RECORD="+execRecord)
			err = signalSampled.Run()
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != -1 {
				t.Fatalf("the sampled launcher branch hid the binary's terminating signal: %v", err)
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
	for _, mode := range []string{"cold", "named", "update"} {
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
				target := filepath.Join(fixture, "bin", "le")
				var args []string
				switch mode {
				case "named":
					target = filepath.Join(fixture, "bin", "le-"+buildNameProbe, "le")
					args = []string{"--name", buildNameProbe}
				case "update":
					args = []string{"--update", "probe"}
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
				cmd := exec.CommandContext(t.Context(), filepath.Join(fixture, "le"), args...)
				cmd.Dir = fixture
				cmd.Env = launcherEnv()
				if output, err := cmd.CombinedOutput(); err == nil {
					t.Errorf("invalid preparation succeeded: %s", output)
				}
				if mode == "cold" {
					if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("preparation failure published a target: %v", err)
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
	writeExecutable(t, filepath.Join(fixture, "fakebin", "go"), `#!/bin/sh
set -eu
"$ZE_REAL_GO" "$@"
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
// launcher is free to name the argv more than once (le passes it to the binary
// under the staleness sample as well as through exec, and its find helper
// forwards its OWN arguments the same way); what it is never free to do is
// expand it unquoted. The count read every one of those as a defect and read a
// single unquoted $@ as correct.
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

// forcedSampleLauncher writes a copy of the fixture's launcher whose le_sampled
// always answers yes, and returns its path. samples is false for a launcher that
// carries no sampling predicate at all, which `ze` does not.
//
// The sample is `$$ % 16` on purpose: a counter would be state several sessions
// write at once, and the only thing the sample has to be is roughly one in
// sixteen (le). That makes the branch unreachable on demand, so the test
// rewrites the predicate in its own copy rather than asking the launcher for a
// seam it should not carry. Only the predicate's body moves; every other line,
// including the branch under test, is the tracked file's.
func forcedSampleLauncher(t *testing.T, fixture, launcher string) (path string, samples bool) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(fixture, launcher))
	if err != nil {
		t.Fatalf("read fixture %s: %v", launcher, err)
	}
	const predicate = "\t[ $(($$ % 16)) -eq 0 ]\n"
	text := string(body)
	if !strings.Contains(text, predicate) {
		return "", false
	}
	path = filepath.Join(fixture, launcher+".sampled")
	writeExecutable(t, path, strings.Replace(text, predicate, "\treturn 0\n", 1))
	return path, true
}

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
	if err := os.WriteFile(filepath.Join(fixture, "go.mod"), []byte("module launcher.test/fixture\n\ngo 1.27\ntoolchain go1.27.0\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	manifest := "# fixture feature gates\nze_alpha internal/alpha\nze_beta internal/beta\nze_alpha internal/alpha/sidecar\n"
	if err := os.WriteFile(filepath.Join(fixture, "feature-gates.txt"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write feature-gates.txt: %v", err)
	}
	return fixture
}

// launcherEnv removes every spelling of the outer launcher's identity and root.
// Fixture-specific overrides are appended only after this isolation boundary.
func launcherEnv() []string {
	return env.Without(os.Environ(), "ze.repo.root", "ze.le.build.name")
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
