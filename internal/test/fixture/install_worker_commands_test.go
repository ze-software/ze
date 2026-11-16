// Design: docs/functional-tests.md -- synthetic kernel worker command boundaries
// Related: install_fixture_test.go -- end-to-end worker and manifest assertions

package fixture

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallWorkerMakeWriteFailure proves a failed command record cannot be
// reported as a successful fake make invocation.
func TestInstallWorkerMakeWriteFailure(t *testing.T) {
	err := installWorkerMake([]string{"ARCH=x86_64", "defconfig"}, installWorkerClosedWriter{})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("make write failure = %v, want %v", err, io.ErrClosedPipe)
	}
}

type installWorkerClosedWriter struct{}

func (installWorkerClosedWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

// TestInstallWorkerMakeExecutable exercises the compiled make alias without a
// host make or echo, accepting only the synthetic worker's actual command forms.
func TestInstallWorkerMakeExecutable(t *testing.T) {
	program := installWorkerTestExecutable(t, "make")
	// An undispatched Go test binary accepts this flag and runs no tests. Stop
	// here unless dispatch refuses it, before ordinary make argv could run the
	// entire test binary recursively on the pre-fix implementation.
	output, err := exec.CommandContext(t.Context(), program, "-test.run=^$").CombinedOutput()
	if err == nil {
		t.Fatalf("make alias accepted an unsupported test flag: %s", output)
	}
	if !strings.Contains(string(output), "unexpected synthetic worker make arguments") {
		t.Fatalf("make alias refusal = %q, want unsupported-argument diagnostic", output)
	}

	installPath := filepath.Join(t.TempDir(), "out")
	cases := []struct {
		name string
		args []string
	}{
		{"defconfig", []string{"ARCH=x86_64", "defconfig"}},
		{"olddefconfig", []string{"ARCH=x86_64", "olddefconfig"}},
		{"installer", []string{"ARCH=x86_64", "-j1", "bzImage"}},
		{"runtime", []string{"ARCH=x86_64", "-j1", "bzImage", "modules"}},
		{"modules-install", []string{"ARCH=x86_64", "INSTALL_MOD_PATH=" + installPath, "modules_install"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := exec.CommandContext(t.Context(), program, tc.args...).CombinedOutput()
			if err != nil {
				t.Fatalf("make %q: %v: %s", tc.args, err, output)
			}
			if want := strings.Join(tc.args, " ") + "\n"; string(output) != want {
				t.Fatalf("make output = %q, want %q", output, want)
			}
		})
	}
	for _, args := range [][]string{
		nil,
		{"ARCH=x86", "defconfig"},
		{"ARCH=x86_64", "-j2", "bzImage"},
		{"ARCH=x86_64", "-j1", "bzImage", "unknown"},
		{"ARCH=x86_64", "INSTALL_MOD_PATH=", "modules_install"},
		{"ARCH=x86_64", "INSTALL_MOD_PATH=relative", "modules_install"},
		{"ARCH=x86_64", "INSTALL_MOD_PATH=" + installPath, "unknown"},
	} {
		output, err := exec.CommandContext(t.Context(), program, args...).CombinedOutput()
		if err == nil {
			t.Fatalf("make accepted unsupported arguments %q: %s", args, output)
		}
		if !strings.Contains(string(output), "unexpected synthetic worker make arguments") {
			t.Fatalf("make %q refusal = %q, want unsupported-argument diagnostic", args, output)
		}
	}
}

// TestInstallWorkerMergeExecutable proves the compiled merge alias preserves the
// seeded resolved config, while refusing argv the worker never produces.
func TestInstallWorkerMergeExecutable(t *testing.T) {
	program := installWorkerTestExecutable(t, "merge_config.sh")
	// As above, this first check safely discriminates absent dispatch without
	// accidentally starting the entire fixture suite as a helper subprocess.
	output, err := exec.CommandContext(t.Context(), program, "-test.run=^$").CombinedOutput()
	if err == nil {
		t.Fatalf("merge alias accepted an unsupported test flag: %s", output)
	}
	if !strings.Contains(string(output), "unexpected synthetic worker merge arguments") {
		t.Fatalf("merge alias refusal = %q, want unsupported-argument diagnostic", output)
	}

	dir := t.TempDir()
	fragment := filepath.Join(dir, "custom.config")
	seeded := map[string]string{
		filepath.Join(dir, ".config"): "CONFIG_REQUIRED=m\nCONFIG_EQUALS=y\n",
		fragment:                      "CONFIG_REQUIRED=y\n",
	}
	for path, body := range seeded {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.CommandContext(t.Context(), program, "-m", ".config", fragment)
	command.Dir = dir
	output, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("merge: %v: %s", err, output)
	}
	if len(output) != 0 {
		t.Fatalf("merge output = %q, want no output", output)
	}
	for path, want := range seeded {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != want {
			t.Fatalf("merge changed seeded %s: got %q, want %q", filepath.Base(path), body, want)
		}
	}
	for _, args := range [][]string{
		nil,
		{"-m", ".config"},
		{"-n", ".config", fragment},
		{"-m", "other.config", fragment},
		{"-m", ".config", ""},
		{"-m", ".config", "relative.config"},
		{"-m", ".config", fragment, fragment},
	} {
		output, err := exec.CommandContext(t.Context(), program, args...).CombinedOutput()
		if err == nil {
			t.Fatalf("merge accepted unsupported arguments %q: %s", args, output)
		}
		if !strings.Contains(string(output), "unexpected synthetic worker merge arguments") {
			t.Fatalf("merge %q refusal = %q, want unsupported-argument diagnostic", args, output)
		}
	}
}

func installWorkerTestExecutable(t *testing.T, name string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), name)
	if err := os.Symlink(self, program); err != nil {
		t.Fatal(err)
	}
	return program
}
