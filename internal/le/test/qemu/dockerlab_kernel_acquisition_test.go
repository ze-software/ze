// Design: docs/architecture/testing/qemu-integration.md -- supplied kernel payload ownership.
// Related: dockerlab_command.go -- validation before the VM consumes a kernel.

package testqemu

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDockerLabCommandOwnsValidatedKernelPayload replaces the requested stage
// at the actual process-start boundary. The QEMU argument must still name the
// validated requested bytes, held privately until the failed start is cleaned up.
func TestDockerLabCommandOwnsValidatedKernelPayload(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "unchanged request"
		if replace {
			name = "request replaced before QEMU consumes it"
		}
		t.Run(name, func(t *testing.T) {
			run, _, _ := fakeDockerLabRun(t)
			tree := dockerLabRuntimeFixture(t)
			requested := filepath.Join(t.TempDir(), "requested-vmlinuz")
			const payload = "runtime-kernel"
			if err := os.WriteFile(requested, []byte(payload), 0o600); err != nil {
				t.Fatal(err)
			}
			kernel, err := dockerLabCommandKernel(requested, tree)
			if err != nil {
				t.Fatal(err)
			}
			image := dockerLabImageFixture(t, payload, "root=PARTUUID=fixture-02 ro init=/gokrazy/init\n")
			original := *kernel
			run.Options.Kernel = kernel.Selected
			run.Options.Command = "must-not-run-after-start-failure"
			startFailure := errors.New("owned fixture stops at QEMU consumption")
			var selected string
			run.ops.Start = func(command *exec.Cmd) error {
				for i, arg := range command.Args {
					if arg != "-kernel" {
						continue
					}
					if i+1 == len(command.Args) {
						t.Fatal("QEMU kernel argument has no payload")
					}
					selected = command.Args[i+1]
					break
				}
				if selected == "" {
					t.Fatal("QEMU did not select a kernel payload")
				}
				if replace {
					if err := os.WriteFile(requested, []byte("foreign-kernel"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				data, err := os.ReadFile(selected)
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != payload {
					t.Errorf("QEMU consumed %q, want validated request %q", data, payload)
				}
				if selected == requested {
					t.Error("QEMU reopens the mutable requested pathname")
				}
				if selected == filepath.Join(tree, "vmlinuz") {
					t.Error("QEMU substituted the runtime cache for the requested payload")
				}
				rel, err := filepath.Rel(run.vmRecordDir(), selected)
				if err != nil {
					t.Fatal(err)
				}
				if !filepath.IsLocal(rel) {
					t.Errorf("kernel payload %s is outside the run-owned VM directory", selected)
				}
				info, err := os.Stat(selected)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0o600 {
					t.Errorf("private kernel mode=%04o, want 0600", info.Mode().Perm())
				}
				return startFailure
			}
			report, err := run.dockerLabCommand(t.Context(), image, kernel)
			if *kernel != original {
				t.Errorf("command mutated the caller's kernel identity: got %+v, want %+v", *kernel, original)
			}
			if !errors.Is(err, startFailure) {
				t.Fatalf("did not reach selected kernel consumption: %v", err)
			}
			if report.Kernel.Requested != requested {
				t.Errorf("report lost requested path: %+v", report.Kernel)
			}
			if report.Kernel.Selected != selected {
				t.Errorf("report selected %s, QEMU consumed %s", report.Kernel.Selected, selected)
			}
			if _, err := os.Stat(selected); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("failed start left private kernel payload: %v", err)
			}
			if _, err := os.Stat(requested); err != nil {
				t.Errorf("cleanup removed the caller's requested kernel: %v", err)
			}
		})
	}
}

// TestDockerLabCommandRefusesChangedKernelBeforeAcquisition changes the request
// after preflight. A valid cached/image kernel cannot excuse changed input bytes.
func TestDockerLabCommandRefusesChangedKernelBeforeAcquisition(t *testing.T) {
	run, _, _ := fakeDockerLabRun(t)
	tree := dockerLabRuntimeFixture(t)
	requested := filepath.Join(t.TempDir(), "requested-vmlinuz")
	if err := os.WriteFile(requested, []byte("runtime-kernel"), 0o600); err != nil {
		t.Fatal(err)
	}
	kernel, err := dockerLabCommandKernel(requested, tree)
	if err != nil {
		t.Fatal(err)
	}
	image := dockerLabImageFixture(t, "runtime-kernel", "root=PARTUUID=fixture-02 ro init=/gokrazy/init\n")
	original := *kernel
	run.Options.Kernel = kernel.Selected
	if err := os.WriteFile(requested, []byte("foreign-kernel"), 0o600); err != nil {
		t.Fatal(err)
	}
	started := false
	run.ops.Start = func(*exec.Cmd) error {
		started = true
		return errors.New("changed request reached QEMU")
	}
	_, err = run.dockerLabCommand(t.Context(), image, kernel)
	if *kernel != original {
		t.Errorf("refusal mutated the caller's kernel identity: got %+v, want %+v", *kernel, original)
	}
	if started {
		t.Error("changed request started QEMU")
	}
	if err == nil {
		t.Fatal("changed request was accepted")
	}
	if !strings.Contains(err.Error(), kernel.SHA256) {
		t.Errorf("refusal lacks the expected validated payload identity: %v", err)
	}
	for path, want := range map[string]string{
		requested:                      "foreign-kernel",
		filepath.Join(tree, "vmlinuz"): "runtime-kernel",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Errorf("refusal changed %s: got %q, want %q", path, data, want)
		}
	}
	entries, err := os.ReadDir(run.vmRecordDir())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Errorf("refusal left run-owned directory or partial payload: %s", entry.Name())
		}
	}
}
