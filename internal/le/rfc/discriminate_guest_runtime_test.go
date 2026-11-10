// Design: docs/contributing/rfc-conformance-gates.md -- explicit-kernel placement.
// Related: discriminate_guest_selection_test.go -- the same placement on a plain unit
package rfc

import (
	"runtime"
	"slices"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// gtsmCapabilityUnits are the two Linux-capability units P-39 records in the
// guest. Their file is `//go:build linux`, so on a Linux host it is
// host-buildable, and only an explicit `kernel` may move it to the guest.
var gtsmCapabilityUnits = []string{
	"TestGTSMTransmittedICMPErrorCarriesTTL255",
	"TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255",
}

const gtsmCapabilityFile = "internal/component/gtsm/rfc5082_gtsm_linux_test.go"

// TestRecordLinuxCapabilityUnitInGuest builds the recording entry's runner for
// each real GTSM unit, without starting a compiler or a VM. With a kernel the
// runner must place the unit in the guest, boot exactly that kernel, and run
// exactly that unit from the unit's own package. Without one, a Linux host
// keeps host-first placement, and any other host refuses for want of a kernel
// rather than running nothing.
//
// MUTATION: discriminate_guest.go unitNeedsGuest `if !explicitGuest {` ->
// `if true {` reddens the kernel cases on Linux: "runs on this host, so
// `kernel` names a guest nothing would boot".
func TestRecordLinuxCapabilityUnitInGuest(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	const kernel = "tmp/kernel/build/vmlinuz"
	for _, unit := range gtsmCapabilityUnits {
		tag := Tag{File: gtsmCapabilityFile}
		record := DiscriminationRecord{Unit: gtsmCapabilityFile + "::" + unit}
		t.Run(unit+"/kernel", func(t *testing.T) {
			runner, err := newObservationRunner(root, newSourceReader(root), newScopeIndex(),
				Carrier{Kind: kindUnit}, tag, &record, kernel)
			if err != nil {
				t.Fatalf("explicit kernel placement refused: %v", err)
			}
			if !runner.guest {
				t.Fatalf("explicit kernel left %s on the host", unit)
			}
			if runner.kernel != kernel {
				t.Fatalf("guest boots %q, want %q", runner.kernel, kernel)
			}
			if runner.unitName != unit {
				t.Fatalf("runner selects %q, want %q", runner.unitName, unit)
			}
			argv := runner.guestRunArgv("probe.sh")
			at := slices.Index(argv, keyKernel)
			if at < 0 {
				t.Fatalf("guest command names no kernel: %q", argv)
			}
			if at+1 >= len(argv) {
				t.Fatalf("guest command names no kernel: %q", argv)
			}
			if argv[at+1] != kernel {
				t.Fatalf("guest command boots %q, want %q: %q", argv[at+1], kernel, argv)
			}
			script := runner.guestScript("/scratch/guest.test", "")
			if !strings.Contains(script, "-test.run '^"+unit+"$'") {
				t.Fatalf("guest script does not select exactly %s:\n%s", unit, script)
			}
			build := strings.Join(runner.guestBuildArgv("/scratch/guest.test", "", ""), " ")
			if !strings.Contains(build, "./internal/component/gtsm") {
				t.Fatalf("guest build does not compile the unit's package: %s", build)
			}
		})
		t.Run(unit+"/no-kernel", func(t *testing.T) {
			runner, err := newObservationRunner(root, newSourceReader(root), newScopeIndex(),
				Carrier{Kind: kindUnit}, tag, &record, "")
			if runtime.GOOS != "linux" {
				if err == nil {
					t.Fatalf("a %s host placed a Linux-only unit with no kernel: guest=%v", runtime.GOOS, runner.guest)
				}
				if !strings.Contains(err.Error(), "runs only in the Linux guest") {
					t.Fatalf("refusal does not ask for the guest kernel: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("host-first placement refused on Linux: %v", err)
			}
			if runner.guest {
				t.Fatalf("no kernel moved host-buildable %s to the guest", unit)
			}
		})
	}
}
