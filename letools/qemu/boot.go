// Design: docs/architecture/testing/qemu-integration.md -- booting an image and asking it
// Overview: hugepages.go -- the run that reaches this file
// Related: report.go -- the payload this file decides
//
// boot.go boots one appliance image in QEMU and asks it two questions.
//
// The questions go over SSH to the appliance's own CLI. The ANSWERS are JSON
// because a machine can assert on that format. `show host kernel | json`
// carries the baked kernel command line. `show host memory | json` carries the
// pages that the kernel actually reserved. The first proves that the request
// reached the boot arguments. Only the second proves that the kernel did it.
//
// THE SERIAL CONSOLE IS KEPT because its boot log is the only evidence when the
// appliance does not answer. The log was once discarded, which cost a whole
// debugging session. The daemon was up in ten seconds, and the QUERY was wrong.
// The log is written beside the image, under the checkout's own scratch tree.
// An operator already looks there after a failure.

package qemu

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two CLI commands the proof asks the booted appliance.
const (
	KernelQuery = "show host kernel | json"
	MemoryQuery = "show host memory | json"
)

// The two JSON keys the answers are read out of.
const (
	cmdlineKey = "cmdline"
	totalKey   = "hugepages-total"
)

// errAarch64Firmware is a missing UEFI image. It is a prerequisite this machine
// lacks, like QEMU itself, so it answers a SKIP rather than a failure.
var errAarch64Firmware = errors.New("aarch64 UEFI firmware not found")

// answers is what the booted appliance said, and the last SSH error behind each
// question it did not answer.
type answers struct {
	kernelJSON string
	kernelErr  string
	memoryJSON string
	memoryErr  string
}

// bootAndAssert boots the image, asks the two questions, and answers the
// verdict.
func (h *Hugepages) bootAndAssert(report HugepagesReport, image string) (HugepagesReport, error) {
	port := h.SSHPort
	if port == 0 {
		picked, err := freePort()
		if err != nil {
			return report, err
		}
		port = picked
	}

	// The firmware is a prerequisite this machine either has or lacks, so it is
	// answered here beside the other prerequisites rather than inside the argv.
	if h.Arch == ArchARM64 {
		if !isFile(h.Bios) {
			report.Verdict = VerdictSkip
			report.Reason = firmwareSkipReason(h.Bios)
			return report, nil
		}
	}

	console, err := h.startConsole(image, port)
	if err != nil {
		return report, err
	}

	said, err := h.boot(h.qemuArgs(image, port, report), port, console)
	console.Close() //nolint:errcheck // the VM has stopped writing to it by now
	if err != nil {
		return report, err
	}

	if said.kernelJSON == "" {
		return h.noAnswer(report, console.Name(), said.kernelErr), nil
	}
	return h.assert(report, said), nil
}

// isFile reports whether path names a file this process can see. The reason a
// stat failed is not carried, because there is one answer to every reason: the
// firmware is not usable and the run skips.
func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// firmwareSkipReason names the firmware that an arm64 boot was unable to find.
func firmwareSkipReason(bios string) string {
	var tb textbuf.Buffer
	return tb.Err(errAarch64Firmware).Str(" at ").Str(bios).String()
}

// startConsole opens the file the VM's serial console is written to. It sits
// beside the image, so a reader who has the image has the log.
func (h *Hugepages) startConsole(image string, port int) (*os.File, error) {
	var tb textbuf.Buffer
	name := tb.Str("ze-vpp-hp-console-").Int(int64(port)).Str(".log").String()
	return os.Create(filepath.Join(filepath.Dir(image), name)) //nolint:gosec // a path under the work directory this run made
}

// boot runs the VM for as long as the two questions take. It answers what the
// VM said.
//
// boot asks both questions itself instead of assigning them to the caller. This
// lets boot stop the VM on every path out of one function. It asks the second
// question only after the first question gets an answer. An appliance that
// never came up has nothing more to say. Another question would cost another
// whole deadline.
func (h *Hugepages) boot(argv []string, port int, console *os.File) (answers, error) {
	var said answers

	vm := exec.CommandContext(context.Background(), QemuBinary(h.Arch), argv...) //nolint:gosec // the argv is built by this package, never by an operator
	vm.Stdout = console
	vm.Stderr = console
	if err := vm.Start(); err != nil {
		return said, err
	}
	defer stopVM(vm)

	said.kernelJSON, said.kernelErr = h.ask(port, KernelQuery)
	if said.kernelJSON != "" {
		said.memoryJSON, said.memoryErr = h.ask(port, MemoryQuery)
	}
	return said, nil
}

// qemuArgs answers the VM's arguments. The caller has already established that
// an arm64 boot has firmware to use.
//
// The two architectures differ in more than the binary. arm64 needs a machine
// type, a CPU model and a firmware image. For arm64, highmem is off because the
// appliance is built for a machine with a gigabyte, not for a server.
func (h *Hugepages) qemuArgs(image string, port int, report HugepagesReport) []string {
	var tb textbuf.Buffer
	machine := tb.Str("accel=").Str(report.Accelerator).String()

	argv := make([]string, 0, 20)
	if h.Arch == ArchARM64 {
		tb.Reset()
		machine = tb.Str("virt,highmem=off,accel=").Str(report.Accelerator).String()
		argv = append(argv, "-cpu", "max", "-bios", h.Bios)
	}

	tb.Reset()
	memory := tb.Uint(report.MemoryMiB).String()
	tb.Reset()
	drive := tb.Str("file=").Str(image).Str(",format=raw").String()
	tb.Reset()
	nic := tb.Str("user,model=e1000,hostfwd=tcp::").Int(int64(port)).Str("-:22").String()

	return append(argv,
		"-machine", machine,
		"-smp", "2",
		"-m", memory,
		"-drive", drive,
		"-nographic",
		"-serial", "mon:stdio",
		"-nic", nic,
	)
}

// stopVM ends the VM. It asks first and kills after the grace, because a QEMU
// that has not exited by then is not going to.
func stopVM(vm *exec.Cmd) {
	if vm.Process == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		vm.Wait() //nolint:errcheck // the VM is being stopped; its status says nothing about the proof
	}()

	vm.Process.Signal(os.Interrupt) //nolint:errcheck // a VM that refuses is killed below

	select {
	case <-done:
		return
	case <-time.After(shutdownGrace):
	}

	vm.Process.Kill() //nolint:errcheck // nothing further can be done about a process that survives this
	<-done
}

// ask runs one CLI command over SSH. It retries until the deadline and answers
// the output and the last error.
//
// The LAST ERROR matters. A refused connection means that the appliance is
// still booting. A connected session whose COMMAND was rejected is a different
// failure. Both cases have the same exit status for a caller. That conflation
// made "error: unknown command" look like a boot timeout for months.
func (h *Hugepages) ask(port int, query string) (string, string) {
	end := time.Now().Add(h.Deadline)
	for {
		out, stderr, err := h.ssh(port, query)
		if err == nil {
			return out, ""
		}
		lastError := lastLine(stderr, out, err)
		if !time.Now().Before(end) {
			return "", lastError
		}
		time.Sleep(sshRetryPause)
	}
}

// ssh runs one command on the appliance and answers its two streams.
//
// The appliance is built fresh for this run, and its key is new every time.
// Therefore, host-key checking is off and the known-hosts file is discarded.
// The connection timeout is short. A refused connection becomes a retry, not a
// wait.
func (h *Hugepages) ssh(port int, query string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sshAttemptMax)
	defer cancel()

	var tb textbuf.Buffer
	target := tb.Str(SSHUser).Str("@127.0.0.1").String()
	tb.Reset()
	portArg := tb.Int(int64(port)).String()

	cmd := exec.CommandContext(ctx, "sshpass", "-p", h.SSHPass, "ssh", //nolint:gosec // the argv is built here, never by an operator
		"-p", portArg,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		target, query)

	var out, stderr strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	return out.String(), stderr.String(), err
}

// noAnswer answers the verdict for an appliance that never replied.
//
// Under a HARDWARE accelerator, this result is a failure, not a slow machine.
// It stayed broken because the result was reported as a skip. Only software
// emulation CAN still skip on it.
func (h *Hugepages) noAnswer(report HugepagesReport, consolePath, lastError string) HugepagesReport {
	detail := "no ssh error captured"
	if lastError != "" {
		var tb textbuf.Buffer
		detail = tb.Str("last ssh error: ").Str(lastError).String()
	}

	var tb textbuf.Buffer
	if Hardware(report.Accelerator) {
		report.Verdict = VerdictFail
		report.Reason = tb.Str("appliance did not answer over SSH (accel=").
			Str(report.Accelerator).Str("); ").Str(detail).String()
		report.ConsoleTail = consoleTail(consolePath)
		return report
	}

	report.Verdict = VerdictSkip
	report.Reason = tb.Str("appliance did not answer over SSH within the timeout (accel=").
		Str(report.Accelerator).Str(", software emulation); ").Str(detail).String()
	return report
}

// assert reads the two answers and decides.
//
// Each wanted kernel argument is matched as a WHOLE TOKEN of the command line,
// not as a substring. The Python matched substrings. `hugepagesz=2M` is a
// substring of `default_hugepagesz=2M`. Thus that assertion was unable to come out
// red on its own. A kernel-argument path that emitted the default and not the
// per-size argument would have passed
// (plan/journal/green-that-could-not-have-been-red.md).
func (h *Hugepages) assert(report HugepagesReport, said answers) HugepagesReport {
	var tb textbuf.Buffer

	cmdline, err := cmdlineOf(said.kernelJSON)
	if err != nil {
		report.Verdict = VerdictFail
		report.Reason = tb.Byte('`').Str(KernelQuery).Str("` is not JSON: ").Err(err).String()
		return report
	}
	report.Cmdline = cmdline

	for _, want := range wantedArgs(report) {
		if hasToken(cmdline, want) {
			continue
		}
		report.Verdict = VerdictFail
		report.Reason = tb.Str("kernel cmdline missing ").Quoted(want).String()
		return report
	}

	if said.memoryJSON == "" {
		report.Verdict = VerdictFail
		report.Reason = tb.Str("kernel cmdline is right but `").Str(MemoryQuery).
			Str("` never answered; last ssh error: ").Str(said.memoryErr).String()
		return report
	}

	total, err := pagesTotalOf(said.memoryJSON)
	if err != nil {
		report.Verdict = VerdictFail
		report.Reason = tb.Byte('`').Str(MemoryQuery).Str("` is not JSON: ").Err(err).String()
		return report
	}
	report.PagesTotal = total

	// The command line only proves the REQUEST reached the kernel.
	// hugepages-total is the kernel's own answer, and it is the assertion that
	// can fail on a box with too little contiguous memory.
	if total < 1 {
		report.Verdict = VerdictFail
		report.Reason = tb.Str("hugepages-total=0 (cmdline asked for ").Uint(report.Pages).
			Str("; kernel reserved none)").String()
		return report
	}

	report.Verdict = VerdictPass
	return report
}

// wantedArgs answers the three kernel arguments the reservation must produce.
func wantedArgs(report HugepagesReport) []string {
	var tb textbuf.Buffer
	defaultSize := tb.Str("default_hugepagesz=").Str(report.PageToken).String()
	tb.Reset()
	size := tb.Str("hugepagesz=").Str(report.PageToken).String()
	tb.Reset()
	count := tb.Str("hugepages=").Uint(report.Pages).String()
	return []string{defaultSize, size, count}
}

// hasToken reports whether a kernel command line carries want as one of its
// whitespace-separated arguments.
func hasToken(cmdline, want string) bool {
	for token := range strings.FieldsSeq(cmdline) {
		if token == want {
			return true
		}
	}
	return false
}

// cmdlineOf reads the kernel command line out of `show host kernel | json`.
//
// A document that does not parse is an error. In that case, the appliance
// answered something other than the query's JSON. The reader must know which
// failure occurred. A document with no cmdline key answers the empty string.
// Every later comparison then fails.
func cmdlineOf(document string) (string, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(document), &parsed); err != nil {
		return "", err
	}
	value, _ := parsed[cmdlineKey].(string)
	return value, nil
}

// pagesTotalOf reads the reserved page count out of `show host memory | json`.
//
// If the document does not carry the key or its value is not a number,
// pagesTotalOf answers zero and no error. Zero is the honest reading of "the
// kernel reserved none". The caller's own floor turns it into a verdict.
func pagesTotalOf(document string) (uint64, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(document), &parsed); err != nil {
		return 0, err
	}
	value, ok := parsed[totalKey].(float64)
	if !ok || value < 0 {
		return 0, nil
	}
	return uint64(value), nil
}

// lastLine answers the final line of whichever stream carried something, or the
// process error when neither did.
func lastLine(stderr, out string, err error) string {
	for _, stream := range []string{stderr, out} {
		trimmed := strings.TrimSpace(stream)
		if trimmed == "" {
			continue
		}
		lines := strings.Split(trimmed, "\n")
		return lines[len(lines)-1]
	}
	return err.Error()
}

// consoleTail answers the last lines of the captured serial console.
//
// If this process cannot read a console, consoleTail reports that failure. It
// does not report an empty console. "the appliance said nothing" and "the log
// was not there" are different failures. The operator must tell them apart.
func consoleTail(path string) []string {
	body, err := os.ReadFile(path) //nolint:gosec // a path this run wrote
	if err != nil {
		var tb textbuf.Buffer
		return []string{tb.Str("(no serial console captured: ").Err(err).Byte(')').String()}
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) > consoleTailLines {
		lines = lines[len(lines)-consoleTailLines:]
	}
	return lines
}
