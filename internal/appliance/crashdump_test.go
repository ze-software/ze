// Design: docs/architecture/diagnostics/crash-capture.md -- appliance crash reservation tests
//
// VALIDATES: AC-10 and assumptions A-2 and A-4: the runtime kernel floor names
//            the pstore symbols, the pinned kernel is new enough for a named
//            size-only reservation, and the build renders both cmdline tokens.
// PREVENTS:  shipping a kernel that accepts the reservation, takes the RAM on
//            every boot, and stores nothing readable -- the CONFIG_INET_ESP
//            failure one subsystem over.

package appliance

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/crashlog"
)

func TestRuntimeKernelRequirementsIncludePstore(t *testing.T) {
	for _, symbol := range []string{"CONFIG_PSTORE", "CONFIG_PSTORE_RAM"} {
		if !slices.Contains(runtimeKernelRequirements, symbol) {
			t.Fatalf("runtimeKernelRequirements does not pin %s: %v", symbol, runtimeKernelRequirements)
		}
	}
}

func TestPinnedKernelMeetsReserveMemFloor(t *testing.T) {
	// A-2: size-named reservation needs 6.12 or newer. The pinned version is the
	// one the runtime kernel is built from, so the floor is checked against it
	// rather than against a number written down twice.
	if err := enforceReserveMemKernelFloor(defaultKernelVersion); err != nil {
		t.Fatalf("pinned kernel %s: %v", defaultKernelVersion, err)
	}
}

func TestReserveMemKernelFloorRefusesOlderKernels(t *testing.T) {
	cases := []struct {
		version string
		ok      bool
	}{
		{"6.11", false},
		{"6.12", true},
		{"6.13.4", true},
		{"5.15", false},
		{"7.2", true},
		{"7", false},
		{"seven.two", false},
	}
	for _, tc := range cases {
		err := enforceReserveMemKernelFloor(tc.version)
		if tc.ok && err != nil {
			t.Fatalf("enforceReserveMemKernelFloor(%q) = %v, want nil", tc.version, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("enforceReserveMemKernelFloor(%q) = nil, want a refusal", tc.version)
		}
	}
}

func TestApplianceBuildRendersCrashReservation(t *testing.T) {
	// Both tokens, naming one region. Either one alone captures nothing, so the
	// assertion is on the pair rather than on the presence of reserve_mem.
	args, err := crashDumpKernelArgs(ImageConfig{CrashDump: &CrashDump{Reserve: "16mb"}})
	if err != nil {
		t.Fatalf("crashDumpKernelArgs: %v", err)
	}
	want := []string{
		"reserve_mem=16M:4096:" + crashlog.ReserveRegionName,
		"ramoops.mem_name=" + crashlog.ReserveRegionName,
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestApplianceBuildLeavesCmdlineAloneWithoutCrashDump(t *testing.T) {
	// nil means the built image's /cmdline.txt is unchanged, which is the same
	// contract the hugepage reservation beside it carries.
	args, err := crashDumpKernelArgs(ImageConfig{})
	if err != nil {
		t.Fatalf("crashDumpKernelArgs: %v", err)
	}
	if args != nil {
		t.Fatalf("args = %v, want nil", args)
	}
}

func TestCrashDumpReserveBounds(t *testing.T) {
	// The reservation is taken from RAM on every boot, so an out-of-range value
	// is refused at the build rather than paid for at run time. The bounds are
	// crashlog's, so this also proves the appliance and the YANG schema agree.
	cases := []struct {
		reserve string
		ok      bool
		want    string
	}{
		{reserve: "4mb", ok: true},
		{reserve: "16mb", ok: true},
		{reserve: "256mb", ok: true},
		{reserve: "3mb", ok: false, want: "4mb to 256mb"},
		{reserve: "257mb", ok: false, want: "4mb to 256mb"},
		{reserve: "1536kb", ok: false, want: "whole number of megabytes"},
		{reserve: "16", ok: false, want: "must end in"},
		{reserve: "", ok: false, want: "must end in"},
	}
	for _, tc := range cases {
		cfg := &applianceConfig{Image: ImageConfig{CrashDump: &CrashDump{Reserve: tc.reserve}}}
		err := cfg.validateCrashDump()
		if tc.ok {
			if err != nil {
				t.Fatalf("reserve %q: %v, want accepted", tc.reserve, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("reserve %q was accepted, want a refusal", tc.reserve)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("reserve %q error = %q, want it to contain %q", tc.reserve, err, tc.want)
		}
	}
}
