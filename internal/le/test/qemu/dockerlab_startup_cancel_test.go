// Design: docs/architecture/testing/qemu-integration.md -- cancellation owns VM teardown.

package testqemu

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestDockerLabStartupCancellationStopsOnlyOwnedVM reaches the first SSH
// readiness attempt after a real fixture QEMU process has started and acquired
// its VM record. A channel rendezvous then cancels the root, before any check,
// lab, forward or own-system command. The producer MUST join its QEMU and remove
// its transient files while retaining the reported console and preserving the
// foreign sentinel in the same record directory.
// MUTATION: detach the readiness context or omit the deferred VM stop/removal.
func TestDockerLabStartupCancellationStopsOnlyOwnedVM(t *testing.T) {
	for _, mode := range []string{"lab", "command"} {
		t.Run(mode, func(t *testing.T) {
			run, _, logPath := fakeDockerLabRun(t)
			var kernel *dockerLabKernel
			image := "/img"
			if mode == "command" {
				runtime := dockerLabRuntimeFixture(t)
				var err error
				kernel, err = dockerLabCommandKernel(filepath.Join(runtime, "vmlinuz"), runtime)
				if err != nil {
					t.Fatal(err)
				}
				image = dockerLabImageFixture(t, "runtime-kernel", "root=PARTUUID=fixture-02 ro init=/gokrazy/init\n")
				run.Options.Kernel = kernel.Selected
				run.Options.Command = "must-not-execute-during-startup"
			}
			if err := os.MkdirAll(run.vmRecordDir(), 0o700); err != nil {
				t.Fatal(err)
			}
			foreign := filepath.Join(run.vmRecordDir(), "foreign-startup-sentinel")
			if err := os.WriteFile(foreign, []byte("foreign owner\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(foreign)
			if err != nil {
				t.Fatal(err)
			}

			budget, endBudget := context.WithTimeout(t.Context(), 20*time.Second)
			defer endBudget()
			root, cancel := context.WithCancel(budget)
			attempt := make(chan struct{})
			joined := make(chan struct{})
			go cancelDockerLabAtSSH(root, attempt, cancel, joined)
			// The test MUST cancel and join its rendezvous worker on every exit.
			defer func() {
				cancel()
				<-joined
			}()

			var qemu *exec.Cmd
			// A broken producer MUST still leave this test responsible for its
			// own child. Assertions run before this failure-only cleanup.
			// Both production calls return only after their synchronous stop
			// joins its waiter. Omitting that entire stop creates no waiter.
			// This fallback is not safe for an asynchronous or unjoined-stop
			// mutation: nil ProcessState alone does not transfer Wait ownership.
			t.Cleanup(func() {
				if qemu == nil {
					return
				}
				if qemu.ProcessState != nil {
					return
				}
				stopVM(qemu)
			})
			starts := 0
			start := run.ops.Start
			run.ops.Start = func(command *exec.Cmd) error {
				starts++
				if starts != 1 {
					t.Fatalf("startup cancellation reached another child: %v", command.Args)
				}
				if err := start(command); err != nil {
					return err
				}
				qemu = command
				return nil
			}
			keygen := run.ops.Run
			attempts := 0
			run.ops.Run = func(ctx context.Context, spec commandSpec) (commandResult, error) {
				if spec.Name == "ssh-keygen" {
					return keygen(ctx, spec)
				}
				if spec.Name != sshProgram {
					t.Fatalf("startup reached unexpected command %s", spec.Name)
				}
				if spec.Args[len(spec.Args)-1] != "true" {
					t.Fatalf("startup advanced past SSH readiness: %v", spec.Args)
				}
				attempts++
				if attempts != 1 {
					t.Fatal("canceled startup retried SSH")
				}
				if qemu == nil {
					t.Fatal("SSH readiness preceded QEMU start")
				}
				if _, err := os.Stat(filepath.Join(run.vmRecordDir(), vmRecordName(qemu.Process.Pid))); err != nil {
					t.Fatalf("SSH readiness preceded owned VM record: %v", err)
				}
				if err := ctx.Err(); err != nil {
					t.Fatalf("readiness was already canceled before rendezvous: %v", err)
				}
				close(attempt)
				select {
				case <-ctx.Done():
					return commandResult{}, ctx.Err()
				case <-budget.Done():
					t.Error("readiness detached from the canceled root")
					return commandResult{}, budget.Err()
				}
			}
			var report DockerLabReport
			switch mode {
			case "command":
				report, err = run.dockerLabCommand(root, image, kernel)
				if !errors.Is(err, context.Canceled) {
					t.Errorf("own-system startup lost cancellation: %v", err)
				}
			case "lab":
				report, err = run.dockerLab(root, dockerLabRequest{image: image, linuxZe: "/ze", lab: []string{"test", "lab"}})
				if !report.Interrupted {
					t.Error("lab startup did not report interruption")
				}
				if dockerLabExitCode(&report, true) != 1 {
					t.Errorf("interrupted lab has success verdict: %+v", report)
				}
			}
			if err == nil {
				t.Error("canceled startup answered success")
			}
			if attempts != 1 {
				t.Fatalf("SSH attempts=%d, want the observed startup rendezvous", attempts)
			}
			if qemu.ProcessState == nil {
				t.Fatal("startup returned before joining owned QEMU")
			}
			if strings.Contains(readLog(t, logPath), "le ") {
				t.Error("a host check or lab ran during canceled startup")
			}
			entries, err := os.ReadDir(run.vmRecordDir())
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 {
				t.Fatalf("startup left owned files behind: %v", entries)
			}
			// dockerLabPrepare reserves a console outside the transient run
			// directory so a failed boot's returned diagnostic remains readable.
			if filepath.Dir(report.Console) != run.vmRecordDir() {
				t.Fatalf("reported console is not owned by this run: %q", report.Console)
			}
			if !slices.Contains(qemu.Args, "file,id=console0,path="+report.Console) {
				t.Fatalf("reported console is not QEMU's selected console: %q", report.Console)
			}
			for _, entry := range entries {
				if entry.Name() == filepath.Base(foreign) {
					continue
				}
				if filepath.Join(run.vmRecordDir(), entry.Name()) != report.Console {
					t.Fatalf("startup retained an unreported file or directory: %s", entry.Name())
				}
			}
			console, err := os.Lstat(report.Console)
			if err != nil {
				t.Fatal(err)
			}
			if !console.Mode().IsRegular() {
				t.Fatalf("retained console is not a regular file: %s", console.Mode())
			}
			after, err := os.Stat(foreign)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) {
				t.Error("startup replaced the foreign owner's inode")
			}
			data, err := os.ReadFile(foreign)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "foreign owner\n" {
				t.Errorf("startup changed foreign bytes: %q", data)
			}
		})
	}
}

// cancelDockerLabAtSSH owns one test rendezvous. Its caller MUST cancel root
// and join done even if preparation fails before the first SSH attempt.
func cancelDockerLabAtSSH(root context.Context, attempt <-chan struct{}, cancel context.CancelFunc, done chan<- struct{}) {
	defer close(done)
	select {
	case <-attempt:
		cancel()
	case <-root.Done():
	}
}
