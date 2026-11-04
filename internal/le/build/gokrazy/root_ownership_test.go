package buildgokrazy

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance"
)

// TestPrepareArgsKeepsSelectedRoot exercises the real resolver aliases. Native
// reachability passes into RuntimeKernelTree; foreign reachability reads the
// same selected profile itself. Different missing manifests identify each root.
func TestPrepareArgsKeepsSelectedRoot(t *testing.T) {
	selected, parent := writeInstanceFixture(t)
	// The shared fixture installs kernel fakes; this test consumes the real aliases.
	runtimeKernelTreeFn = appliance.RuntimeKernelTree
	runtimeKernelReachableFn = appliance.GokrazyRuntimeKernelReachable
	decoy := t.TempDir()
	for root, missing := range map[string]string{selected: "runtime.require", decoy: "kernel.require"} {
		dir := filepath.Join(root, "gokrazy", "kernel")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"kernel.config", "kernel.require", "runtime.config", "runtime.require"} {
			if name == missing {
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte("# fixture\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Chdir(decoy)
	t.Setenv("ZE_REPO_ROOT", decoy)
	foreign := "arm64"
	if runtime.GOARCH == foreign {
		foreign = "amd64"
	}
	for _, arch := range []string{runtime.GOARCH, foreign} {
		t.Run(arch, func(t *testing.T) {
			t.Setenv("GOARCH", arch)
			_, cleanup, err := prepareArgs(selected, []string{"--parent_dir", parent, "-i", "ze", "overwrite"})
			defer cleanup()
			want := filepath.Join(selected, "gokrazy", "kernel", "runtime.require")
			if err == nil {
				t.Fatal("selected profile without runtime.require was accepted")
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("profile refusal = %v, want missing selected manifest %s", err, want)
			}
			if strings.Contains(err.Error(), decoy) {
				t.Fatalf("profile refusal selected decoy root: %v", err)
			}
		})
	}
}

// TestRunSourcePreflightKeepsSelectedRoot exercises real Run and source preflight
// without a Go subprocess: both selected modules refuse before hostGoVersion.
// A Ze module without a go directive differs from a foreign module; the decoy
// has the opposite module identity.
func TestRunSourcePreflightKeepsSelectedRoot(t *testing.T) {
	for _, valid := range []bool{true, false} {
		name := "selected-invalid"
		if valid {
			name = "selected-valid"
		}
		t.Run(name, func(t *testing.T) {
			selected := t.TempDir()
			decoy := t.TempDir()
			good := "module github.com/ze-software/ze\n"
			bad := "module example.invalid/decoy\n\ngo 1.27.0\n"
			selectedModule, decoyModule := bad, good+"\ngo 1.27.0\n"
			if valid {
				selectedModule, decoyModule = good, bad
			}
			for root, module := range map[string]string{selected: selectedModule, decoy: decoyModule} {
				if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			goroot := t.TempDir()
			marker := filepath.Join(goroot, filepath.FromSlash(appliance.StdpatchMarker))
			if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(marker, []byte("package tls\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Chdir(decoy)
			t.Setenv("ZE_REPO_ROOT", decoy)
			t.Setenv("GOROOT", goroot)
			bin := filepath.Join(goroot, "bin")
			if err := os.MkdirAll(bin, 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, "go"), []byte("must not execute"), 0o755); err != nil { //nolint:gosec // Executable fixture is only inspected by LookPath.
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			for _, key := range []string{"GOMODCACHE", "GOFLAGS", "GOPROXY", "CGO_ENABLED", "GOTOOLCHAIN"} {
				t.Setenv(key, "")
			}
			t.Setenv("GOTOOLCHAIN", "local")
			read, write, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer read.Close() //nolint:errcheck // Test pipe cleanup.
			stderr := os.Stderr
			os.Stderr = write
			code := Run(selected, nil)
			os.Stderr = stderr
			if err := write.Close(); err != nil {
				t.Fatal(err)
			}
			output, err := io.ReadAll(read)
			if err != nil {
				t.Fatal(err)
			}
			if code == 0 {
				t.Fatal("source preflight unexpectedly succeeded")
			}
			want := filepath.Join(selected, "go.mod") + " is not the Ze source tree's go.mod"
			if valid {
				want = filepath.Join(selected, "go.mod") + " has no go directive"
			}
			if !strings.Contains(string(output), want) {
				t.Fatalf("preflight output %q lacks %q", output, want)
			}
			cache := filepath.Join(selected, "gokrazy", "modcache")
			if got := os.Getenv("GOMODCACHE"); got != cache {
				t.Errorf("GOMODCACHE = %q, want %q", got, cache)
			}
			if _, err := os.Stat(cache); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(decoy, "gokrazy", "modcache")); !os.IsNotExist(err) {
				t.Fatalf("decoy modcache inspection = %v, want absent", err)
			}
		})
	}
}
