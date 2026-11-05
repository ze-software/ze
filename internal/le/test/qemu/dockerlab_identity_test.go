// Design: docs/architecture/testing/qemu-integration.md -- One command in Ze's own system.

package testqemu

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestDockerLabCommandNamesTheFailedIdentity checks each identity predicate at
// the command consumer. A refusal must name the failed primitive and must not
// execute the requested command.
func TestDockerLabCommandNamesTheFailedIdentity(t *testing.T) {
	for _, test := range []struct {
		name      string
		primitive string
	}{
		{"kernel release", "uname -r"},
		{"PID 1 executable", "readlink /proc/1/exe"},
		{"root filesystem", "stat -f -c %T /"},
		{"runtime modules", "test -d"},
		{"Docker socket absent", "test ! -S"},
	} {
		t.Run(test.name, func(t *testing.T) {
			run, _, _ := fakeDockerLabRun(t)
			tree := dockerLabRuntimeFixture(t)
			kernel, err := dockerLabCommandKernel(filepath.Join(tree, "vmlinuz"), tree)
			if err != nil {
				t.Fatal(err)
			}
			image := dockerLabImageFixture(t, "runtime-kernel", "root=PARTUUID=fixture-02 ro init=/gokrazy/init\n")
			run.Options.Kernel = kernel.Selected
			run.Options.Command = "requested-command-must-not-run"
			originalRun := run.ops.Run
			refused := false
			run.ops.Run = func(ctx context.Context, spec commandSpec) (commandResult, error) {
				if spec.Name != sshProgram {
					return originalRun(ctx, spec)
				}
				remote := spec.Args[len(spec.Args)-1]
				if strings.Contains(remote, run.Options.Command) {
					t.Error("requested command ran after an identity refusal")
					return commandResult{}, errors.New("identity guard was bypassed")
				}
				if strings.Contains(remote, test.primitive) {
					refused = true
					return commandResult{Code: 1}, nil
				}
				return originalRun(ctx, spec)
			}
			_, err = run.dockerLabCommand(t.Context(), image, kernel)
			if !refused {
				t.Fatal("the identity primitive was never checked")
			}
			if err == nil {
				t.Fatal("a failed identity primitive was accepted")
			}
			if !strings.Contains(err.Error(), "own-system identity refused: "+test.name) {
				t.Fatalf("refusal does not name the failed %s: %v", test.name, err)
			}
		})
	}
}

// TestDockerLabCommandReadsRootPID1ThroughSudo drives the actual identity and
// command consumer with a non-root SSH session. The fake kernel refuses an
// unprivileged readlink of root's PID 1, matching procfs's credential check.
// The requested command must run only after the privileged primitive succeeds.
func TestDockerLabCommandReadsRootPID1ThroughSudo(t *testing.T) {
	run, _, _ := fakeDockerLabRun(t)
	tree := dockerLabRuntimeFixture(t)
	kernel, err := dockerLabCommandKernel(filepath.Join(tree, "vmlinuz"), tree)
	if err != nil {
		t.Fatal(err)
	}
	image := dockerLabImageFixture(t, "runtime-kernel", "root=PARTUUID=fixture-02 ro init=/gokrazy/init\n")
	run.Options.Kernel = kernel.Selected
	run.Options.Command = "requested-command-exits-23"
	originalRun := run.ops.Run
	checkedPID1 := false
	ranCommand := false
	run.ops.Run = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		if spec.Name != sshProgram {
			return originalRun(ctx, spec)
		}
		remote := spec.Args[len(spec.Args)-1]
		if strings.Contains(remote, "readlink /proc/1/exe") {
			if !strings.Contains(strings.Join(spec.Args, " "), "ze@127.0.0.1") {
				t.Error("PID 1 probe bypassed the unprivileged SSH session")
			}
			if !strings.Contains(remote, "executable=$(sudo -n readlink /proc/1/exe)") {
				return commandResult{Code: 1, Stderr: "root PID 1 requires ptrace read credentials"}, nil
			}
			if !strings.Contains(remote, "test \"$executable\" = /gokrazy/init") {
				t.Error("PID 1 probe no longer checks the exact gokrazy executable")
			}
			checkedPID1 = true
			return commandResult{Stdout: "/gokrazy/init\n"}, nil
		}
		if strings.Contains(remote, run.Options.Command) {
			if !checkedPID1 {
				t.Error("requested command ran before the PID 1 identity check")
			}
			ranCommand = true
			return commandResult{Code: 23}, nil
		}
		return originalRun(ctx, spec)
	}
	report, err := run.dockerLabCommand(t.Context(), image, kernel)
	if err != nil {
		t.Fatalf("root PID 1 could not be inspected through the guest's sudo: %v", err)
	}
	if !checkedPID1 {
		t.Fatal("the privileged PID 1 primitive was never checked")
	}
	if !ranCommand {
		t.Fatal("the checked guest command never ran")
	}
	if report.CommandExit != 23 {
		t.Fatalf("guest command exit = %d, want 23", report.CommandExit)
	}
}
