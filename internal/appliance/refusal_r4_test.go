package appliance

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestUnsupportedArchRefusedBeforeAnyHost proves an image build for an arch
// Ze builds no kernel for is refused as unsupported, before the cache is
// asked, and that no refusal sends the operator to a host of that arch:
// none exists that could build it. Each caller's fix names this host's arch.
//
// VALIDATES: round-4 NOTE 1.
// PREVENTS: "build it on a riscv64 host", advice no host can follow.
func TestUnsupportedArchRefusedBeforeAnyHost(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	const unsupported = "riscv64"
	host := runtime.GOARCH

	err := runtimeKernelReachable(unsupported)
	foreign, ok := errors.AsType[*foreignArchKernelError](err)
	if !ok || !foreign.unsupported {
		t.Fatalf("runtimeKernelReachable(%s) = %v, want the unsupported-arch refusal", unsupported, err)
	}

	dir := filepath.Join(t.TempDir(), "my lab")
	kernel := foreign.Error()
	image := foreign.forInstance(dir, "lab").Error()
	gokrazy := GokrazyRuntimeKernelReachable(unsupported, []string{"overwrite"}).Error()
	refusals := map[string]string{"kernel": kernel, "image": image, "gokrazy": gokrazy}
	for caller, refusal := range refusals {
		if !strings.Contains(refusal, "for riscv64 cannot be built on any host: Ze builds kernels for amd64 and arm64 only") {
			t.Errorf("%s refusal does not say the arch is unsupported:\n%s", caller, refusal)
		}
		if strings.Contains(refusal, unsupported+" host") || strings.Contains(refusal, "--arch "+unsupported) {
			t.Errorf("%s refusal sends the operator to a %s host:\n%s", caller, unsupported, refusal)
		}
	}
	lastLine := func(text string) string {
		lines := strings.Split(text, "\n")
		return lines[len(lines)-1]
	}
	if got, want := lastLine(kernel), "ze appliance kernel --target runtime --arch "+host; got != want {
		t.Errorf("kernel refusal ends %q, want %q", got, want)
	}
	if got, want := lastLine(image), "ze appliance --dir '"+dir+"' build lab"; got != want {
		t.Errorf("image refusal ends %q, want %q", got, want)
	}
	if !strings.Contains(image, `"arch": "`+host+`"} in `+ConfigPath(dir, "lab")) {
		t.Errorf("image refusal does not name the instance file and this host's arch:\n%s", image)
	}
	if got, want := lastLine(gokrazy), "GOARCH="+host+" ./le build gokrazy overwrite"; got != want {
		t.Errorf("gokrazy refusal ends %q, want %q", got, want)
	}
}
