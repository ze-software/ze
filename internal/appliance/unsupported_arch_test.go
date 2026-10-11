package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestUnsupportedArchRefusedBeforeAnyHost proves an arch Ze builds no kernel
// for is refused as unsupported at each entry point an operator types, before
// any kernel build or image build starts, and that no refusal sends the
// operator to a host of that arch: none exists that could build it. Each
// refusal names the arches Ze builds and ends with a fix for this host's arch.
//
// VALIDATES: round-4 NOTE 1, round-5 ISSUE 2, round-6 ISSUE 1.
// PREVENTS: "build it on a riscv64 host", advice no host can follow, and an
// arch refusal with no fix.
func TestUnsupportedArchRefusedBeforeAnyHost(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	const unsupported = "riscv64"
	host := runtime.GOARCH
	setTestKernelBuild(t, func(kernelBuildSpec) error {
		t.Fatal("a kernel build started for an unsupported arch")
		return nil
	})
	oldGok := gokBuildFn
	gokBuildFn = func([]string) error { t.Fatal("the image builder ran for an unsupported arch"); return nil }
	defer func() { gokBuildFn = oldGok }()

	// ze appliance kernel --arch riscv64.
	var code int
	kernel := captureStderr(t, func() { code = runKernel([]string{"--target", "runtime", "--arch", unsupported}) })
	if code != exitError {
		t.Errorf("runKernel(--arch %s) = %d, want %d", unsupported, code, exitError)
	}

	// ze appliance build lab, whose config asks for riscv64.
	oldBase := baseDir
	baseDir = t.TempDir()
	defer func() { baseDir = oldBase }()
	if err := os.MkdirAll(filepath.Join(baseDir, "lab"), 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig("lab")
	cfg.Image.Arch = unsupported
	if err := saveConfig(ConfigPath(baseDir, "lab"), &cfg); err != nil {
		t.Fatal(err)
	}
	image := captureStderr(t, func() { code = buildOne("lab") })
	if code != exitError {
		t.Errorf("buildOne with image.arch %s = %d, want %d", unsupported, code, exitError)
	}

	// ze appliance kernel lab: the arch comes from the same config, so the
	// refusal names the config field and file, never an --arch nobody typed.
	named := captureStderr(t, func() { code = runKernel([]string{"lab"}) })
	if code != exitError {
		t.Errorf("runKernel(lab) with image.arch %s = %d, want %d", unsupported, code, exitError)
	}

	// GOARCH=riscv64 ./le build gokrazy reaches runtimeKernelReachable unchecked.
	reach := runtimeKernelReachable(unsupported)
	if foreign, ok := errors.AsType[*foreignArchKernelError](reach); !ok || !foreign.unsupported {
		t.Fatalf("runtimeKernelReachable(%s) = %v, want the unsupported-arch refusal", unsupported, reach)
	}
	gokrazy := GokrazyRuntimeKernelReachable(unsupported, []string{"overwrite"}).Error()

	refusals := map[string]string{"kernel": kernel, "named": named, "image": image, "gokrazy": gokrazy}
	for caller, refusal := range refusals {
		if !strings.Contains(refusal, "Ze builds kernels for amd64 and arm64 only") {
			t.Errorf("%s refusal does not say the arch is unsupported:\n%s", caller, refusal)
		}
		if strings.Contains(refusal, unsupported+" host") || strings.Contains(refusal, "--arch "+unsupported) {
			t.Errorf("%s refusal sends the operator to a %s host:\n%s", caller, unsupported, refusal)
		}
	}
	lastLine := func(text string) string {
		lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
		return lines[len(lines)-1]
	}
	if !strings.Contains(kernel, `--arch "riscv64" names an architecture Ze builds no kernel for`) {
		t.Errorf("kernel refusal does not name the flag the operator typed:\n%s", kernel)
	}
	if got, want := lastLine(kernel), "ze appliance kernel --target runtime --arch "+host; got != want {
		t.Errorf("kernel refusal ends %q, want %q", got, want)
	}
	if !strings.Contains(image, `image.arch "riscv64" names an architecture Ze builds no kernel for`) {
		t.Errorf("image refusal does not name the config field:\n%s", image)
	}
	if !strings.Contains(image, ConfigPath(baseDir, "lab")) {
		t.Errorf("image refusal does not name the config file %s:\n%s", ConfigPath(baseDir, "lab"), image)
	}
	if got, want := lastLine(image), `"image": {"arch": "`+host+`"}`; got != want {
		t.Errorf("image refusal ends %q, want %q", got, want)
	}
	if !strings.Contains(named, `image.arch "riscv64" names an architecture Ze builds no kernel for`) {
		t.Errorf("named kernel refusal does not name the config field:\n%s", named)
	}
	if !strings.Contains(named, ConfigPath(baseDir, "lab")) {
		t.Errorf("named kernel refusal does not name the config file %s:\n%s", ConfigPath(baseDir, "lab"), named)
	}
	if got, want := lastLine(named), `"image": {"arch": "`+host+`"}`; got != want {
		t.Errorf("named kernel refusal ends %q, want %q", got, want)
	}
	if got, want := lastLine(gokrazy), "GOARCH="+host+" ./le build gokrazy overwrite"; got != want {
		t.Errorf("gokrazy refusal ends %q, want %q", got, want)
	}
}
