package qemu

// The hugepage proof's planning half.
//
// Goal: verify the values that the run derives BEFORE it performs an action.
// Every later step and assertion uses these values. A wrong page token or page
// count reaches a kernel command line. Method: call plan() for a run configured
// as an operator would configure it. The method uses no QEMU.

import (
	"io"
	"testing"
)

// fixtureHugepages answers a run with the defaults an operator gets, over a
// tree that needs to exist only if a later step reads it.
func fixtureHugepages(t *testing.T) *Hugepages {
	t.Helper()

	return &Hugepages{
		Tree:        t.TempDir(),
		Arch:        ArchAMD64,
		PageSize:    DefaultPageSize,
		Reservation: DefaultReservation,
		Memory:      DefaultMemory,
		SSHPass:     DefaultSSHPass,
		Bios:        DefaultBios,
		Deadline:    AnswerDeadline,
		Progress:    io.Discard,
	}
}

// VALIDATES: the default configuration derives 64 pages of 2 MB, spelled 2M on
// the kernel command line, in a machine of 1024 MiB.
// PREVENTS: a page token or a count that no longer matches what the appliance
// bakes, which would make every run red for a reason that is not the appliance's.
func TestThePlanDerivesTheTokenTheCountAndTheMemory(t *testing.T) {
	run := fixtureHugepages(t)

	report, err := run.plan()
	if err != nil {
		t.Fatalf("plan the default run: %v", err)
	}
	if report.PageToken != "2M" {
		t.Errorf("the page token is %q, want %q", report.PageToken, "2M")
	}
	if report.Pages != 64 {
		t.Errorf("the reservation is %d pages, want 64", report.Pages)
	}
	if report.MemoryMiB != 1024 {
		t.Errorf("the machine gets %d MiB, want 1024", report.MemoryMiB)
	}
	if report.Accelerator == "" {
		t.Error("the plan named no accelerator")
	}
}

// VALIDATES: the 1 GB page size derives its own token and count.
// PREVENTS: a second page size that silently reuses the first one's token, which
// would ask the kernel for pages of one size and assert the other.
func TestTheOtherPageSizeDerivesItsOwnToken(t *testing.T) {
	run := fixtureHugepages(t)
	run.PageSize = "1gb"
	run.Reservation = "4gb"

	report, err := run.plan()
	if err != nil {
		t.Fatalf("plan a 1 GB run: %v", err)
	}
	if report.PageToken != "1G" {
		t.Errorf("the page token is %q, want %q", report.PageToken, "1G")
	}
	if report.Pages != 4 {
		t.Errorf("the reservation is %d pages, want 4", report.Pages)
	}
}

// VALIDATES: a page size the kernel has no spelling for stops the run.
// PREVENTS: a guess reaching a boot argument. The kernel accepts 2M and 1G and
// nothing else, so a third size has no correct kernel command line at all.
func TestAPageSizeTheKernelCannotSpellStopsTheRun(t *testing.T) {
	for _, size := range []string{"4mb", "512kb", "", "2MB "} {
		run := fixtureHugepages(t)
		run.PageSize = size
		if _, err := run.plan(); err == nil {
			t.Errorf("a page size of %q was planned, want a refusal", size)
		}
	}
}

// VALIDATES: a machine is never given less memory than a Linux kernel needs to
// finish booting, whatever the configuration asked for.
// PREVENTS: a VM that never starts being reported as an appliance that never
// answered, which under a hardware accelerator is a FAILURE and would read as a
// defect in the image.
func TestTheMachineNeverGetsLessThanTheFloor(t *testing.T) {
	cases := []struct {
		memory string
		want   uint64
	}{
		{"1b", memoryMiBMin},
		{"128mb", memoryMiBMin},
		{"256mb", memoryMiBMin},
		{"257mb", 257},
		{"1gb", 1024},
	}
	for _, one := range cases {
		run := fixtureHugepages(t)
		run.Memory = one.memory
		report, err := run.plan()
		if err != nil {
			t.Errorf("plan a run with %q of memory: %v", one.memory, err)
			continue
		}
		if report.MemoryMiB != one.want {
			t.Errorf("%q of memory became %d MiB, want %d", one.memory, report.MemoryMiB, one.want)
		}
	}
}

// VALIDATES: a reservation that is not a quantity is an ERROR rather than a skip.
// PREVENTS: the one thing the self-skip contract must not swallow. A missing
// QEMU means that the machine cannot run the proof. A configuration that is not
// a size is a mistake somebody made. A skip would report a pass over a proof
// that was never attempted.
func TestAReservationThatIsNotAQuantityIsAnError(t *testing.T) {
	for _, reservation := range []string{"", "lots", "-1gb", "1kb"} {
		run := fixtureHugepages(t)
		run.Reservation = reservation
		report, err := run.plan()
		if err == nil {
			t.Errorf("a reservation of %q was planned as %d pages, want a refusal", reservation, report.Pages)
		}
		if report.Verdict == VerdictSkip {
			t.Errorf("a reservation of %q answered a skip, want an error", reservation)
		}
	}
}

// VALIDATES: each architecture names its own emulator, and the host's is one of
// the two the appliance configuration accepts.
// PREVENTS: a prerequisite probe that looks for the wrong binary, which would
// skip on a machine that has QEMU.
func TestEachArchitectureNamesItsOwnEmulator(t *testing.T) {
	if got := qemuBinary(ArchARM64); got != "qemu-system-aarch64" {
		t.Errorf("arm64 names %q", got)
	}
	if got := qemuBinary(ArchAMD64); got != "qemu-system-x86_64" {
		t.Errorf("amd64 names %q", got)
	}
	if host := hostArch(); host != ArchAMD64 && host != ArchARM64 {
		t.Errorf("the host architecture is %q, want one of the two the appliance accepts", host)
	}
}

// VALIDATES: only a hypervisor counts as hardware.
// PREVENTS: software emulation being read as hardware, which would turn the
// self-skip on a slow boot into a failure and make the gate red on every CI
// machine without KVM.
func TestOnlyAHypervisorCountsAsHardware(t *testing.T) {
	for accelerator, want := range map[string]bool{
		"kvm": true, "hvf": true, "tcg": false, "": false, "KVM": false,
	} {
		if got := Hardware(accelerator); got != want {
			t.Errorf("Hardware(%q) = %v, want %v", accelerator, got, want)
		}
	}
}

// VALIDATES: a build whose output names an unpopulated module cache is given the
// one command that fills it.
// PREVENTS: "toolchain not available" reading as a broken Go installation. It is
// not one: it is a documented setup step nobody ran, and the message is the only
// place a reader can learn that.
func TestAnUnpopulatedModuleCacheIsNamedInTheFailure(t *testing.T) {
	for _, marker := range modcacheMarkers {
		if hint := buildHint("gok: " + marker + " for linux/amd64"); hint == "" {
			t.Errorf("a build reporting %q was given no hint", marker)
		}
	}
	if hint := buildHint("gok: disk full"); hint != "" {
		t.Errorf("an unrelated failure was given the module-cache hint: %q", hint)
	}
}

// VALIDATES: an appliance build that wrote no image stops the run.
// PREVENTS: an empty answer reaching the boot, where QEMU would be handed a path
// that does not exist and the appliance would be reported as never answering.
func TestABuildThatWroteNoImageStopsTheRun(t *testing.T) {
	empty := t.TempDir()
	if _, err := findImage(empty); err == nil {
		t.Error("a directory holding no image answered one")
	}
}
