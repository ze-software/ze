package testqemu

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: the crash listing reader picks out kernel-kind artifacts by name
// and ignores userspace crash files and malformed entries.
// PREVENTS: a panic proof passing on a userspace crash file, or an OTA proof
// failing on one.
func TestCrashListingKernelArtifacts(t *testing.T) {
	listing := crashListing{crashes: []any{
		map[string]any{"name": "crash-1.log", "kind": "panic"},
		map[string]any{"name": "crash-2-dmesg-ramoops-0-kernel.log", "kind": "kernel"},
		"not an object",
		map[string]any{"kind": "kernel"},
	}}
	got := listing.kernelArtifacts()
	want := []string{"crash-2-dmesg-ramoops-0-kernel.log"}
	if !slices.Equal(got, want) {
		t.Fatalf("kernel artifacts = %v, want %v", got, want)
	}
}

// VALIDATES: findKey reaches a key nested inside objects and arrays, and stops
// at its depth bound.
// PREVENTS: a readiness block wrapped by the daemon's envelope being missed.
func TestCrashFindKeyIsBounded(t *testing.T) {
	document := map[string]any{"data": []any{map[string]any{"readiness": "here"}}}
	if got := findKey(document, "readiness", findDepthMax); got != "here" {
		t.Fatalf("findKey = %v, want here", got)
	}
	if got := findKey(document, "readiness", 2); got != nil {
		t.Fatalf("findKey at depth 2 = %v, want nil", got)
	}
}

// VALIDATES: the boot time is read from `show host kernel | json`, and an
// answer without one is refused rather than read as zero.
// PREVENTS: a missing field reading as a boot time that never changes.
func TestCrashBootTimeOf(t *testing.T) {
	booted, ok := bootTimeOf(`{"kernel":{"release":"7.2","boot-time-unix":1760000000}}`)
	if !ok || booted != 1760000000 {
		t.Fatalf("bootTimeOf = %d, %v; want 1760000000, true", booted, ok)
	}
	if _, ok := bootTimeOf(`{"kernel":{"release":"7.2"}}`); ok {
		t.Fatal("bootTimeOf accepted an answer with no boot time")
	}
	if _, ok := bootTimeOf("not json"); ok {
		t.Fatal("bootTimeOf accepted text that is not JSON")
	}
}

// VALIDATES: the VM arguments add the update-server forward to the SSH NIC and
// a monitor on its own port.
// PREVENTS: an OTA proof whose POST reaches nothing, or an NMI with no monitor.
func TestCrashQEMUArgsAddMonitorAndForward(t *testing.T) {
	lab := &CrashCapture{run: &Hugepages{Arch: ArchAMD64}, lab: CrashLabOTAUnaffected}
	argv := lab.qemuArgs("image.img", crashPorts{ssh: 2201, monitor: 2202, http: 2203}, HugepagesReport{Accelerator: "kvm", MemoryMiB: 1024})
	joined := strings.Join(argv, " ")
	for _, want := range []string{"hostfwd=tcp::2201-:22,hostfwd=tcp::2203-:80", "-monitor tcp:127.0.0.1:2202,server=on,wait=off"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv %q lacks %q", joined, want)
		}
	}
}

// VALIDATES: the panic line is found inside the artifact text.
// PREVENTS: an artifact without the panic passing the harvest proof.
func TestCrashLineWith(t *testing.T) {
	text := "=== Kernel Crash ===\nKind: kernel\n\n  Kernel panic - not syncing: NMI: Not continuing\n"
	if got := lineWith(text, panicText); got != "Kernel panic - not syncing: NMI: Not continuing" {
		t.Fatalf("lineWith = %q", got)
	}
	if got := lineWith("Kind: kernel\n", panicText); got != "" {
		t.Fatalf("lineWith on text without a panic = %q, want empty", got)
	}
}
