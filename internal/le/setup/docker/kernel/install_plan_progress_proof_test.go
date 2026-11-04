package dockerkernel

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

const installProgressFixtureRoot = "INSTALL_PROGRESS_FIXTURE_ROOT"

// TestInstallPlanProgress enters the real signal root and confirmed-plan
// consumer with owned subprocesses performing temporary filesystem changes.
// Interrupted and failed copies follow actual module removal; later commands
// must not run. No sudo, host GRUB read, installation or reboot is performed.
func TestInstallPlanProgress(t *testing.T) {
	if root := os.Getenv(installProgressFixtureRoot); root != "" {
		runInstallProgressFixture(t, root)
		return
	}
	for _, tc := range []struct {
		name      string
		stop      int
		mode      string
		completed int
	}{
		{name: "before first step", stop: 0, mode: "canceled", completed: 0},
		{name: "first step failed", stop: 0, mode: "fail", completed: 0},
		{name: "copy failed after module removal", stop: 3, mode: "fail", completed: 3},
		{name: "copy interrupted after module removal", stop: 3, mode: "interrupt", completed: 3},
		{name: "GRUB read failed", stop: 7, mode: "read", completed: 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv(installProgressFixtureRoot, root)
			modules := filepath.Join(root, "modules")
			if err := os.Mkdir(modules, 0o700); err != nil {
				t.Fatal(err)
			}
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			const release = "7.2.9-ze"
			steps := dockerKernelInstallSteps(filepath.Join(root, "cache"), release)
			for i := range steps {
				mode := "complete"
				if i == tc.stop {
					mode = tc.mode
				}
				steps[i].Argv = []string{self, "-test.run=^TestInstallPlanProgress$", "--", mode, strconv.Itoa(i)}
			}
			stderr, err := os.CreateTemp(root, "diagnostic-")
			if err != nil {
				t.Fatal(err)
			}
			defer stderr.Close() //nolint:errcheck // Read below before closing this temporary output file.
			previousStderr := os.Stderr
			os.Stderr = stderr
			defer func() { os.Stderr = previousStderr }()
			previousInstall := installKernel
			defer func() { installKernel = previousInstall }()
			installKernel = func(ctx context.Context, args leaction.Arguments) (any, int) {
				if tc.mode == "canceled" {
					canceled, cancel := context.WithCancel(ctx)
					cancel()
					ctx = canceled
				}
				return runKernelInstallPlan(ctx, args, release, steps, func(path string) ([]byte, error) {
					if path != grubConfigPath {
						t.Fatalf("plan read unexpected path %q", path)
					}
					return nil, errors.New("controlled GRUB read failure")
				})
			}
			_, code := runDockerKernelInstall(leaction.Arguments{
				dockerKernelBuilderKeyword: {"qemu"}, dockerKernelConfirmKeyword: {release},
			})
			if code != 1 {
				t.Fatalf("unfinished install code = %d, want 1", code)
			}
			output, err := os.ReadFile(stderr.Name())
			if err != nil {
				t.Fatal(err)
			}
			diagnostic := string(output)
			prefix := "Completed install steps (" + strconv.Itoa(tc.completed) + "/8):"
			_, progress, found := strings.Cut(diagnostic, prefix)
			if !found {
				t.Errorf("diagnostic lacks truthful completed prefix %q:\n%s", prefix, diagnostic)
			}
			completed, pending, unfinished := strings.Cut(progress, "Unfinished install work:")
			if !unfinished {
				t.Errorf("diagnostic does not identify unfinished install work:\n%s", diagnostic)
			}
			if !strings.Contains(pending, "read the GRUB menu and select its saved default") {
				t.Errorf("unfinished work omits final GRUB default selection: %s", diagnostic)
			}
			for i := range steps {
				_, statErr := os.Stat(filepath.Join(root, "completed-"+strconv.Itoa(i)))
				if i < tc.completed {
					if statErr != nil {
						t.Errorf("completed step %d did not perform its write: %v", i+1, statErr)
					}
					if !strings.Contains(completed, steps[i].Why) {
						t.Errorf("completed step %d absent from progress prefix: %s", i+1, diagnostic)
					}
					continue
				}
				if !errors.Is(statErr, os.ErrNotExist) {
					t.Errorf("unfinished/later step %d performed its write: %v", i+1, statErr)
				}
				if !strings.Contains(pending, steps[i].Why) {
					t.Errorf("unfinished step %d absent from pending work: %s", i+1, diagnostic)
				}
				if strings.Contains(completed, steps[i].Why) {
					t.Errorf("unfinished step %d claimed as completed: %s", i+1, diagnostic)
				}
			}
			if tc.mode == "canceled" {
				if !strings.Contains(diagnostic, "No install step started") {
					t.Errorf("pre-first-step state is not explicit: %s", diagnostic)
				}
				if strings.Contains(diagnostic, "partly made") {
					t.Errorf("pre-first-step state claims partial changes: %s", diagnostic)
				}
			}
			if tc.completed >= 3 {
				if _, err := os.Stat(modules); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("module removal was not exercised: %v", err)
				}
				for _, want := range []string{"Do not reboot", "No automatic reboot", "./le setup docker kernel install builder qemu confirm " + release} {
					if !strings.Contains(diagnostic, want) {
						t.Errorf("reboot-sensitive diagnostic lacks %q: %s", want, diagnostic)
					}
				}
			}
		})
	}
}

// runInstallProgressFixture is one owned command in the confirmed-plan fixture.
// Its signal wait is bounded, and the parent command runner MUST wait for it.
func runInstallProgressFixture(t *testing.T, root string) {
	t.Helper()
	if len(os.Args) < 3 {
		t.Fatal("missing controlled step arguments")
	}
	mode := os.Args[len(os.Args)-2]
	index := os.Args[len(os.Args)-1]
	if mode == "interrupt" {
		stopped := make(chan os.Signal, 1)
		signal.Notify(stopped, syscall.SIGTERM)
		defer signal.Stop(stopped)
		if err := syscall.Kill(os.Getppid(), syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case <-stopped:
			t.Fatal("controlled copy interrupted before completion")
		case <-time.After(5 * time.Second):
			t.Fatal("installer did not relay cancellation to its owned command")
		}
	}
	if mode == "fail" {
		t.Fatal("controlled step failed before completion")
	}
	if mode != "complete" {
		t.Fatalf("unknown controlled step mode %q", mode)
	}
	if index == "2" {
		if err := os.Remove(filepath.Join(root, "modules")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "completed-"+index), []byte("completed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
