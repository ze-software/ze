package dockerhost

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance"
)

// TestDockerHostBuildKeepsSelectedRoot exercises the actual initial kernel
// resolver. Distinct missing manifests distinguish the selected and decoy roots
// before any fetch, native kernel build or image build can start.
func TestDockerHostBuildKeepsSelectedRoot(t *testing.T) {
	selected := fakeRepo(t)
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
	recorder := newRecordingBuilder(t)
	recorder.kernelTree = dockerHostRuntimeKernel
	req := buildRequest{repo: selected, arch: runtime.GOARCH, pins: fullPins(t), sizeGiB: imageSizeGiBDefault}
	_, err := recorder.build(context.Background(), req)
	want := filepath.Join(selected, "gokrazy", "kernel", "runtime.require")
	if err == nil {
		t.Fatal("selected profile without runtime.require was accepted")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("kernel refusal = %v, want missing selected manifest %s", err, want)
	}
	if strings.Contains(err.Error(), decoy) {
		t.Fatalf("kernel refusal selected decoy root: %v", err)
	}
	if len(recorder.gokRuns) != 0 || len(recorder.containerRuns) != 0 {
		t.Fatal("kernel profile refusal reached image construction")
	}
}

// TestDockerHostNestedGokKeepsSelectedRoot exercises builder.build -> runGok ->
// Run source preflight. The initial kernel lookup alone is a fixture artifact;
// the nested producer and its selected-module refusal are real. No Go runs.
func TestDockerHostNestedGokKeepsSelectedRoot(t *testing.T) {
	selected := fakeRepo(t)
	decoy := t.TempDir()
	if err := os.WriteFile(filepath.Join(selected, "go.mod"), []byte("module example.invalid/selected\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decoy, "go.mod"), []byte("module github.com/ze-software/ze\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	goroot := t.TempDir()
	marker := filepath.Join(goroot, filepath.FromSlash(appliance.StdpatchMarker))
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("package tls\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(goroot, "bin")
	if err := os.MkdirAll(bin, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("must not execute"), 0o755); err != nil { //nolint:gosec // Executable fixture is inspected only by LookPath.
		t.Fatal(err)
	}
	t.Chdir(decoy)
	t.Setenv("ZE_REPO_ROOT", decoy)
	t.Setenv("GOROOT", goroot)
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("PATH", bin)
	t.Setenv("GOMODCACHE", "")
	recorder := newRecordingBuilder(t)
	recorder.gok = runGok
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close() //nolint:errcheck // Test pipe cleanup.
	stderr := os.Stderr
	os.Stderr = write
	req := buildRequest{repo: selected, arch: "arm64", pins: fullPins(t), sizeGiB: imageSizeGiBDefault}
	_, buildErr := recorder.build(context.Background(), req)
	os.Stderr = stderr
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if buildErr == nil {
		t.Fatalf("image build = %v, want nested gok refusal", buildErr)
	}
	want := filepath.Join(selected, "go.mod")
	if !strings.Contains(string(output), want) || !strings.Contains(string(output), "module example.invalid/selected") {
		t.Fatalf("nested preflight output %q lacks selected-root cause %q", output, want)
	}
	if _, err := os.Stat(filepath.Join(selected, "gokrazy", "modcache")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(decoy, "gokrazy", "modcache")); !os.IsNotExist(err) {
		t.Fatalf("decoy modcache inspection = %v, want absent", err)
	}
}
