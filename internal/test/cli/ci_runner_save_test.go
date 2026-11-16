// Design: docs/functional-tests.md -- shared .ci runners save captured output.
// Related: ci_runner.go -- the actual suite consumer parses and forwards save paths.
// Related: ../runner/runner_output.go -- the artifact writer exercised here.

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCISubcommandSaveArtifacts invokes the shared consumer, then reads its real
// artifacts. Separate consumer processes own their runner signal listeners and
// keep temporary directory and environment changes out of the parent process.
func TestCISubcommandSaveArtifacts(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, saveFlag := range []string{"--save", "-s"} {
		for _, outcome := range []string{"pass", "fail"} {
			t.Run(saveFlag+"/"+outcome, func(t *testing.T) {
				base := t.TempDir()
				if err := os.WriteFile(filepath.Join(base, "go.mod"), []byte("module ci-save-probe\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				suite := filepath.Join(base, "test", "save-probe")
				if err := os.MkdirAll(suite, 0o700); err != nil {
					t.Fatal(err)
				}
				body := "cmd=foreground:seq=1:exec=ze -test.run=^TestCISubcommandSaveOutput$ -- ci-save-output " + outcome + ":exit=0\n" +
					"expect=stdout:contains=shared-save-stdout\nexpect=stderr:contains=shared-save-stderr\nexpect=exit:code=0\n"
				if err := os.WriteFile(filepath.Join(suite, "artifact-output.ci"), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				saved := filepath.Join(base, "saved output")
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				child := exec.CommandContext(ctx, binary, "-test.run=^TestCISubcommandSaveConsumer$",
					"--", "ci-save-consumer", base, saveFlag, saved, outcome)
				child.WaitDelay = time.Second
				output, err := child.CombinedOutput()
				if err != nil {
					t.Fatalf("shared consumer failed: %v\n%s", err, output)
				}
				entries, err := os.ReadDir(saved)
				if err != nil {
					t.Fatalf("shared consumer did not save artifacts: %v", err)
				}
				if len(entries) != 1 {
					t.Fatalf("saved %d test directories, want the one selected test", len(entries))
				}
				if !entries[0].IsDir() {
					t.Fatalf("saved artifact is not a test directory: %s", entries[0].Name())
				}
				for _, file := range []string{"peer-stdout.log", "peer-stderr.log", "client-stdout.log",
					"client-stderr.log", "expected.txt", "received.txt"} {
					content, err := os.ReadFile(filepath.Join(saved, entries[0].Name(), file))
					if err != nil {
						t.Fatalf("missing saved %s: %v", file, err)
					}
					if file == "client-stdout.log" {
						if !strings.Contains(string(content), "shared-save-stdout") {
							t.Fatalf("saved stdout lost the child output: %q", content)
						}
					}
					if file == "client-stderr.log" {
						if !strings.Contains(string(content), "shared-save-stderr") {
							t.Fatalf("saved stderr lost the child output: %q", content)
						}
					}
				}
			})
		}
	}
}

// TestCISubcommandSaveConsumer is a child fixture for the actual shared consumer.
// No argument marker means the package test is not invoking that fixture.
func TestCISubcommandSaveConsumer(t *testing.T) {
	args := flag.Args()
	if len(args) == 0 {
		return
	}
	if args[0] != "ci-save-consumer" {
		return
	}
	if len(args) != 5 {
		t.Fatal("save consumer fixture requires root, flag, directory and outcome")
	}
	t.Chdir(args[1])
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	setBuildEnv(t, "ZE_BIN", binary)
	setBuildEnv(t, "LE_TEST_NO_BUILD", "1")
	cfg := CIRunnerConfig{Name: "save-probe", TestSubdir: "save-probe",
		Description: "saved output", DefaultParallel: 1}
	err = runCISubcommandInner(cfg, []string{args[2], args[3], "-q", "--pattern", "artifact-output"})
	switch args[4] {
	case "pass":
		if err != nil {
			t.Fatal(err)
		}
	case "fail":
		if !errors.Is(err, ErrTestsFailed) {
			t.Fatalf("shared consumer error = %v, want its test failure", err)
		}
	default:
		t.Fatalf("unknown save consumer outcome %q", args[4])
	}
}

// TestCISubcommandSaveOutput emits both streams as the prebuilt command fixture.
// Its failing case exits through testing, so the runner must retain early output
// while preserving the command failure rather than treating saving as success.
func TestCISubcommandSaveOutput(t *testing.T) {
	args := flag.Args()
	if len(args) == 0 {
		return
	}
	if args[0] != "ci-save-output" {
		return
	}
	if len(args) != 2 {
		t.Fatal("save output fixture requires one outcome")
	}
	if _, err := fmt.Fprintln(os.Stdout, "shared-save-stdout"); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintln(os.Stderr, "shared-save-stderr"); err != nil {
		t.Fatal(err)
	}
	switch args[1] {
	case "pass":
	case "fail":
		t.Fail()
	default:
		t.Fatalf("unknown save output outcome %q", args[1])
	}
}
