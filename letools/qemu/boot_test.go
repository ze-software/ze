package qemu

// The hugepage proof's boot and assert halves.
//
// Goal: pin the virtual machine's arguments and the two decisions that the proof
// makes from the answers. The proof determines which kernel arguments count as
// present. It also determines whether an appliance that never replied is a
// failure or a slow machine.
// Method: call each function with the answers that a booted appliance would
// give. No VM is ever started.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: an amd64 VM is given the accelerator, two processors, the planned
// memory, the image as a raw drive, a serial console and the forwarded SSH port.
// PREVENTS: an argument dropped in a rewrite. Each one is load-bearing: without
// the forward the proof cannot log in, and without the raw format QEMU probes
// the image and can guess wrong.
func TestTheAmd64MachineCarriesEveryArgumentTheProofNeeds(t *testing.T) {
	run := fixtureHugepages(t)
	report, err := run.plan()
	if err != nil {
		t.Fatalf("plan the run: %v", err)
	}

	line := strings.Join(run.qemuArgs("/work/ze.img", 34122, report), " ")
	for _, want := range []string{
		"-machine accel=" + report.Accelerator,
		"-smp 2",
		"-m 1024",
		"-drive file=/work/ze.img,format=raw",
		"-nographic",
		"-serial mon:stdio",
		"-nic user,model=e1000,hostfwd=tcp::34122-:22",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the VM argv does not carry %q:\n%s", want, line)
		}
	}
	if strings.Contains(line, "-bios") {
		t.Errorf("an amd64 VM was given firmware:\n%s", line)
	}
}

// VALIDATES: an arm64 VM is given the virt machine, a CPU model and the UEFI
// firmware, with highmem off.
// PREVENTS: an arm64 boot that hangs with no console output, which is what an
// aarch64 QEMU does when it is given no firmware.
func TestTheArm64MachineCarriesItsFirmwareAndMachineType(t *testing.T) {
	run := fixtureHugepages(t)
	run.Arch = ArchARM64
	run.Bios = filepath.Join(t.TempDir(), "edk2.fd")
	report, err := run.plan()
	if err != nil {
		t.Fatalf("plan the run: %v", err)
	}

	line := strings.Join(run.qemuArgs("/work/ze.img", 34122, report), " ")
	for _, want := range []string{"-cpu max", "-bios " + run.Bios, "-machine virt,highmem=off,accel="} {
		if !strings.Contains(line, want) {
			t.Errorf("the arm64 VM argv does not carry %q:\n%s", want, line)
		}
	}
}

// VALIDATES: firmware that is absent, or that is a directory rather than a file,
// is not usable.
// PREVENTS: an arm64 run started against a path that cannot be read as firmware,
// which QEMU reports in a way nobody reading the proof's output would recognize.
func TestFirmwareIsUsableOnlyWhenItIsAFile(t *testing.T) {
	dir := t.TempDir()
	if isFile(dir) {
		t.Error("a directory was accepted as firmware")
	}
	if isFile(filepath.Join(dir, "absent.fd")) {
		t.Error("an absent path was accepted as firmware")
	}

	present := filepath.Join(dir, "edk2.fd")
	if err := os.WriteFile(present, []byte("firmware"), 0o600); err != nil {
		t.Fatalf("write the fixture firmware: %v", err)
	}
	if !isFile(present) {
		t.Error("a real file was refused as firmware")
	}
}

// VALIDATES: a kernel argument is present only as a WHOLE argument of the
// command line.
// PREVENTS: the assertion that was unable to come out red. `hugepagesz=2M` is a
// substring of `default_hugepagesz=2M`. A substring match therefore made the
// second of the three wanted arguments vacuous. A kernel-argument path that
// emitted the default and dropped the per-size argument would have passed.
func TestAKernelArgumentIsMatchedAsAWholeArgument(t *testing.T) {
	const withBoth = "console=ttyS0 default_hugepagesz=2M hugepagesz=2M hugepages=64"
	const withDefaultOnly = "console=ttyS0 default_hugepagesz=2M hugepages=64"

	if !hasToken(withBoth, "hugepagesz=2M") {
		t.Error("a command line carrying the argument was read as not carrying it")
	}
	if hasToken(withDefaultOnly, "hugepagesz=2M") {
		t.Error("default_hugepagesz=2M was read as hugepagesz=2M")
	}
	if hasToken("", "hugepages=64") {
		t.Error("an empty command line carried an argument")
	}
	if hasToken("hugepages=640", "hugepages=64") {
		t.Error("hugepages=640 was read as hugepages=64")
	}
}

// VALIDATES: the three arguments a reservation must produce are derived from the
// plan rather than written down.
// PREVENTS: a page count in the assertion that disagrees with the page count in
// the appliance configuration, which would make the proof judge a request it
// never made.
func TestTheWantedArgumentsComeFromThePlan(t *testing.T) {
	run := fixtureHugepages(t)
	report, err := run.plan()
	if err != nil {
		t.Fatalf("plan the run: %v", err)
	}

	want := []string{"default_hugepagesz=2M", "hugepagesz=2M", "hugepages=64"}
	got := wantedArgs(report)
	if len(got) != len(want) {
		t.Fatalf("the proof wants %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("argument %d is %q, want %q", i, got[i], want[i])
		}
	}
}

// VALIDATES: an appliance that never answered is a FAILURE under a hypervisor
// and a SKIP under software emulation.
// PREVENTS: the way this stayed broken. Reporting a hardware-accelerated boot
// that never answered as a skip made the gate green for months over an appliance
// that was not answering at all.
func TestANoAnswerIsAFailureOnlyUnderAHypervisor(t *testing.T) {
	run := fixtureHugepages(t)
	console := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(console, []byte("boot line one\nboot line two\n"), 0o600); err != nil {
		t.Fatalf("write the fixture console: %v", err)
	}

	hard := run.noAnswer(HugepagesReport{Accelerator: "kvm"}, console, "connection refused")
	if hard.Verdict != VerdictFail {
		t.Errorf("a hardware-accelerated boot that never answered is %v, want fail", hard.Verdict)
	}
	if len(hard.ConsoleTail) == 0 {
		t.Error("the failure carries no serial console, which is the only evidence of why")
	}
	if !strings.Contains(hard.Reason, "connection refused") {
		t.Errorf("the failure does not name the last ssh error: %q", hard.Reason)
	}

	soft := run.noAnswer(HugepagesReport{Accelerator: "tcg"}, console, "connection refused")
	if soft.Verdict != VerdictSkip {
		t.Errorf("a software-emulated boot that never answered is %v, want skip", soft.Verdict)
	}
}

// VALIDATES: the two answers are read out of JSON, and a document that does not
// parse is a failure rather than a silent zero.
// PREVENTS: an appliance whose CLI answered something else being reported as a
// kernel that reserved nothing, which would send a reader looking at memory
// rather than at the query.
func TestTheAnswersAreReadOutOfJSON(t *testing.T) {
	cmdline, err := cmdlineOf(`{"cmdline":"console=ttyS0 hugepages=64"}`)
	if err != nil || cmdline != "console=ttyS0 hugepages=64" {
		t.Errorf("the command line read as %q (%v)", cmdline, err)
	}
	if _, err := cmdlineOf("error: unknown command"); err == nil {
		t.Error("a CLI refusal parsed as JSON")
	}
	if got, _ := cmdlineOf(`{"other":"value"}`); got != "" {
		t.Errorf("a document with no cmdline answered %q", got)
	}

	total, err := pagesTotalOf(`{"hugepages-total":64}`)
	if err != nil || total != 64 {
		t.Errorf("the total read as %d (%v)", total, err)
	}
	if got, _ := pagesTotalOf(`{"hugepages-total":"lots"}`); got != 0 {
		t.Errorf("a non-numeric total answered %d, want 0", got)
	}
	if got, _ := pagesTotalOf(`{"hugepages-total":-4}`); got != 0 {
		t.Errorf("a negative total answered %d, want 0", got)
	}
}

// VALIDATES: the verdict is PASS only when every wanted argument is on the
// command line AND the kernel reserved at least one page.
// PREVENTS: use of the command line alone as the kernel's own answer. The
// command line says what was asked for. Only the reserved page count says what
// happened. The reserved-page check fails on a box with too little contiguous
// memory.
func TestTheVerdictNeedsBothAnswers(t *testing.T) {
	run := fixtureHugepages(t)
	report, err := run.plan()
	if err != nil {
		t.Fatalf("plan the run: %v", err)
	}

	const good = `{"cmdline":"console=ttyS0 default_hugepagesz=2M hugepagesz=2M hugepages=64"}`

	cases := []struct {
		name    string
		said    answers
		verdict Verdict
	}{
		{"both answers agree", answers{kernelJSON: good, memoryJSON: `{"hugepages-total":64}`}, VerdictPass},
		{"the kernel reserved none", answers{kernelJSON: good, memoryJSON: `{"hugepages-total":0}`}, VerdictFail},
		{"the memory query never answered", answers{kernelJSON: good, memoryErr: "timed out"}, VerdictFail},
		{"an argument is missing", answers{
			kernelJSON: `{"cmdline":"console=ttyS0 hugepages=64"}`,
			memoryJSON: `{"hugepages-total":64}`,
		}, VerdictFail},
		{"the kernel answer is not JSON", answers{
			kernelJSON: "error: unknown command", memoryJSON: `{"hugepages-total":64}`,
		}, VerdictFail},
	}
	for _, one := range cases {
		got := run.assert(report, one.said)
		if got.Verdict != one.verdict {
			t.Errorf("%s: verdict %v, want %v (%s)", one.name, got.Verdict, one.verdict, got.Reason)
		}
	}
}

// VALIDATES: the serial console is bounded, and a console that could not be read
// says so rather than answering nothing.
// PREVENTS: "the appliance said nothing" and "the log was not there" being the
// same answer, which are two different failures with two different next steps.
func TestTheConsoleTailIsBoundedAndNeverSilentlyEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.log")
	var builder strings.Builder
	for i := range consoleTailLines * 3 {
		builder.WriteString("boot ")
		builder.WriteByte(byte('0' + i%10))
		builder.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write the fixture console: %v", err)
	}

	if got := consoleTail(path); len(got) != consoleTailLines {
		t.Errorf("a long console produced %d lines, want %d", len(got), consoleTailLines)
	}

	missing := consoleTail(filepath.Join(t.TempDir(), "absent.log"))
	if len(missing) != 1 || !strings.Contains(missing[0], "no serial console captured") {
		t.Errorf("an unreadable console answered %v", missing)
	}
}
