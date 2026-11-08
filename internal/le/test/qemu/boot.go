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

package testqemu

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/runner"
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
func (h *Hugepages) bootAndAssert(ctx context.Context, report HugepagesReport, image string) (HugepagesReport, error) {
	if err := ctx.Err(); err != nil {
		return report, err
	}
	port := h.SSHPort
	if port == 0 {
		picked, err := reserveForwardPort()
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

	said, err := h.boot(ctx, h.qemuArgs(image, port, report), port, console)
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
func (h *Hugepages) boot(ctx context.Context, argv []string, port int, console *os.File) (said answers, err error) {
	vm := exec.CommandContext(ctx, qemuBinary(h.Arch), argv...) //nolint:gosec // the argv is built by this package, never by an operator
	vm.Stdout = console
	vm.Stderr = console
	owner, err := runner.StartServerProcess(vm)
	if err != nil {
		return said, err
	}
	defer func() {
		err = errors.Join(err, ctx.Err())
		if err != nil {
			err = errors.Join(err, owner.Close())
			return
		}
		err = owner.Stop(syscall.SIGINT, shutdownGrace, 0)
	}()

	said.kernelJSON, said.kernelErr = h.ask(ctx, port, KernelQuery)
	if err := ctx.Err(); err != nil {
		return said, err
	}
	if said.kernelJSON != "" {
		said.memoryJSON, said.memoryErr = h.ask(ctx, port, MemoryQuery)
	}
	return said, ctx.Err()
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

	// The arm64 runtime kernel builds the e1000 driver as a module, and gokrazy
	// loads no modules, so an arm64 guest given an e1000 has no NIC and never
	// answers SSH. virtio-net is the NIC gokrazy/kernel/runtime.require makes
	// built in.
	model := "e1000"
	argv := make([]string, 0, 20)
	if h.Arch == ArchARM64 {
		tb.Reset()
		machine = tb.Str("virt,highmem=off,accel=").Str(report.Accelerator).String()
		argv = append(argv, "-cpu", "max", "-bios", h.Bios)
		model = "virtio-net-pci"
	}

	tb.Reset()
	memory := tb.Uint(report.MemoryMiB).String()
	tb.Reset()
	drive := tb.Str("file=").Str(image).Str(",format=raw").String()
	tb.Reset()
	nic := tb.Str("user,model=").Str(model).Str(",hostfwd=tcp:").Str(forwardHost).Byte(':').Int(int64(port)).Str("-:22").String()

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
	stopVMWaiting(vm, waitVM(vm))
}

// waitVM owns the single Wait call. The caller MUST call stopVMWaiting.
func waitVM(vm *exec.Cmd) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		vm.Wait() //nolint:errcheck // the VM is being stopped; its status says nothing about the proof
	}()
	return done
}

// stopVMWaiting MUST be called with the channel returned by waitVM.
func stopVMWaiting(vm *exec.Cmd, done <-chan struct{}) {
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
func (h *Hugepages) ask(ctx context.Context, port int, query string) (string, string) {
	end := time.Now().Add(h.Deadline)
	for {
		if err := ctx.Err(); err != nil {
			return "", err.Error()
		}
		out, stderr, err := h.ssh(ctx, port, query)
		if err == nil {
			return out, ""
		}
		lastError := sshFailure(stderr, out, err)
		if !time.Now().Before(end) {
			return "", lastError
		}
		timer := time.NewTimer(sshRetryPause)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err().Error()
		case <-timer.C:
		}
	}
}

// ssh runs one command on the appliance and answers its two streams.
func (h *Hugepages) ssh(ctx context.Context, port int, query string) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, sshAttemptMax)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sshpass", sshpassArgv(h.SSHPass, port, query)...) //nolint:gosec // the argv is built here, never by an operator

	var out, diagnostic textbuf.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	owner, err := runner.StartServerProcess(cmd)
	if err != nil {
		return "", "", err
	}
	defer func() { err = errors.Join(owner.Close(), ctx.Err()) }()
	<-owner.Done()
	return out.String(), diagnostic.String(), nil
}

// The two ssh options every ssh this package starts carries: a VM built for
// one run has a new host key each time, so neither checking nor remembering it
// means anything.
const (
	sshNoHostKeyCheck = "StrictHostKeyChecking=no"
	sshNoKnownHosts   = "UserKnownHostsFile=/dev/null"
)

// sshpassArgv answers the arguments of the one sshpass call that asks the
// appliance a question: the password, then ssh with its options, the target and
// the query.
//
// The appliance is built fresh for this run, and its key is new every time.
// Therefore, host-key checking is off and the known-hosts file is discarded.
// The connection timeout is short. A refused connection becomes a retry, not a
// wait.
//
// PUBLIC-KEY AUTHENTICATION IS OFF, and the ssh-agent is never asked. The
// appliance accepts only the seeded password. An agent holding many keys made
// OpenSSH offer each of them first, and Ze's server (x/crypto, six attempts)
// ended the session with "Too many authentication failures" before sshpass
// ever typed the password. The proof then read as an appliance that dropped
// the session.
func sshpassArgv(password string, port int, query string) []string {
	var tb textbuf.Buffer
	target := tb.Str(SSHUser).Byte('@').Str(forwardHost).String()
	tb.Reset()
	portArg := tb.Int(int64(port)).String()

	return []string{
		"-p", password, "ssh",
		"-p", portArg,
		"-o", sshNoHostKeyCheck,
		"-o", sshNoKnownHosts,
		"-o", "ConnectTimeout=5",
		"-o", "PubkeyAuthentication=no",
		"-o", "IdentityAgent=none",
		"-o", "IdentitiesOnly=yes",
		"-o", "PreferredAuthentications=password",
		target, query,
	}
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

// sshFailure answers why an ssh attempt failed, in one line for the report.
//
// It is the final line of whichever stream carried something, or the process
// error when neither did. When that stream is stderr, the earlier lines that
// name why the server ended the session come first, joined with "; ", the last
// sshCauseLinesMax of them. OpenSSH prints the cause ("Received disconnect from
// ... Too many authentication failures", "Permission denied") BEFORE its
// closing "Disconnected from 127.0.0.1 port 2223". A report that kept only the
// last line hid the cause and read as an appliance that dropped the session.
// Standard output is never scanned: an answer that is not JSON may contain
// these words without being an ssh diagnostic.
func sshFailure(stderr, out string, err error) string {
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		return withSessionEndCause(strings.Split(trimmed, "\n"))
	}
	if trimmed := strings.TrimSpace(out); trimmed != "" {
		lines := strings.Split(trimmed, "\n")
		return lines[len(lines)-1]
	}
	return err.Error()
}

// sshCauseLinesMax bounds how many cause lines sshFailure keeps before the final
// line, so a stream of repeated refusals cannot grow the report without limit.
const sshCauseLinesMax = 3

// withSessionEndCause answers the final stderr line, preceded by the last
// sshCauseLinesMax earlier lines that name a session ending. lines is never
// empty: sshFailure splits a non-empty trimmed stream.
func withSessionEndCause(lines []string) string {
	final := strings.TrimSpace(lines[len(lines)-1])
	var causes []string
	for _, line := range lines[:len(lines)-1] {
		line = strings.TrimSpace(line)
		if !namesSessionEnd(line) {
			continue
		}
		causes = append(causes, line)
	}
	if len(causes) > sshCauseLinesMax {
		causes = causes[len(causes)-sshCauseLinesMax:]
	}
	var tb textbuf.Buffer
	for _, cause := range causes {
		tb.Str(cause).Str("; ")
	}
	return tb.Str(final).String()
}

// namesSessionEnd reports whether an ssh stderr line says why the server ended
// or refused the session.
func namesSessionEnd(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range []string{"disconnect", "denied", "authentication fail"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
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
