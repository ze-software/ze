// Overview: run_guestcommand.go -- stopping a guest command so its own teardown runs
package testqemu

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// guestStopFixture is one executePlan over the fake QEMU console, whose ssh
// calls a test answers through its fields.
type guestStopFixture struct {
	run    *Run
	marker string
	report RunReport

	mu     sync.Mutex
	events []string
	// qemuOwnGroup records whether QEMU was started as its own group leader.
	qemuOwnGroup bool
}

func (f *guestStopFixture) record(event string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
}

func (f *guestStopFixture) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.events...)
}

// vmStopped reports whether stopVM has signaled the fake QEMU yet.
func (f *guestStopFixture) vmStopped() bool {
	_, err := os.Stat(f.marker)
	return err == nil
}

// newGuestStopFixture boots the fake QEMU console far enough for executePlan to
// reach runGuestCommand: the console echoes the bootstrap's SSHD_READY, the
// SSH readiness probe answers 0, and no runtime kernel is asserted.
func newGuestStopFixture(t *testing.T) *guestStopFixture {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tmp", "qemu"), 0o750); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, filepath.Join(bin, "qemu-system-x86_64")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	f := &guestStopFixture{marker: filepath.Join(root, "stopped")}
	t.Setenv("MARKER", f.marker)
	t.Setenv(fakeQEMUConsole, "1")

	f.run = NewRun(root, RunOptions{Command: "le perf run dut ze", Boot: 10 * time.Second, Timeout: time.Hour})
	f.run.ops.Sleep = func(context.Context, time.Duration) error { return nil }
	f.run.ops.Look = func(name string) (string, error) { return name, nil }
	f.run.ops.Start = func(command *exec.Cmd) error {
		f.qemuOwnGroup = command.SysProcAttr != nil && command.SysProcAttr.Setpgid
		return command.Start()
	}
	f.report = RunReport{Plan: RunPlan{
		QEMUBinary: "qemu-system-x86_64", QEMUArgv: []string{"qemu-system-x86_64"},
		SSHPort: 2222, SetupCommand: "true",
	}}
	return f
}

// answer installs the fake ssh: the readiness probe answers 0, the guest
// command calls guest, and the forwarded interrupt calls interrupt. Each call
// is recorded with the VM's state at that moment.
func (f *guestStopFixture) answer(t *testing.T,
	guest func(context.Context) (commandResult, error),
	interrupt func() (commandResult, error),
) {
	t.Helper()
	f.run.ops.Run = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		remote := spec.Args[len(spec.Args)-1]
		if remote == "true" {
			// The SSH readiness probe answers 0; one started in the terminal's
			// group is recorded, so every test's call list refuses it.
			if !spec.OwnGroup {
				f.record("probe terminal-group")
			}
			return commandResult{}, nil
		}
		state := "vm-running"
		if f.vmStopped() {
			state = "vm-stopped"
		}
		group := "terminal-group"
		if spec.OwnGroup {
			group = "own-group"
		}
		if strings.Contains(remote, "kill -INT") {
			f.record("interrupt " + state + " " + group)
			return interrupt()
		}
		f.record("guest " + group)
		return guest(ctx)
	}
}

// VALIDATES: G-2 through the guest route (AC-6): a Ctrl-C while the guest
// command runs is forwarded to that command's process group in the guest, the
// run waits for the guest command to end on its own, and only then stops the
// VM; QEMU and both ssh sessions start outside the terminal's process group,
// and the run answers an error naming the completed teardown.
// METHOD: executePlan over the fake QEMU console; the fake guest ssh cancels
// the root once it starts and exits 1 only when the fake interrupt ssh runs,
// which records whether the VM had been stopped by then.
// PREVENTS: the 2026-10-21 real run, where one SIGINT to the host's group
// killed QEMU and the ssh client directly, so the guest le's removals never ran.
func TestGuestCommandInterruptReachesTheGuestBeforeTheVMStops(t *testing.T) {
	f := newGuestStopFixture(t)
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	released := make(chan struct{})
	f.answer(t, func(ctx context.Context) (commandResult, error) {
		cancel()
		select {
		case <-released:
			return commandResult{Code: 1}, nil
		case <-ctx.Done():
			return commandResult{}, ctx.Err()
		}
	}, func() (commandResult, error) {
		close(released)
		return commandResult{Stdout: "4242\n"}, nil
	})

	_, err := f.run.executePlan(root, &f.report)
	if err == nil {
		t.Fatal("an interrupted guest command answered no error")
	}
	if !strings.Contains(err.Error(), "finished its own teardown and exited 1") {
		t.Errorf("the error does not say the guest's teardown completed: %v", err)
	}
	if !f.qemuOwnGroup {
		t.Error("QEMU was started in the terminal's process group, so a Ctrl-C kills it under the guest command")
	}
	want := []string{"guest own-group", "interrupt vm-running own-group"}
	if got := f.recorded(); !equalStrings(got, want) {
		t.Errorf("ssh calls = %q, want %q", got, want)
	}
	if !f.vmStopped() {
		t.Error("the VM was not stopped after the guest command ended")
	}
	if f.report.Verdict != RunVerdictFail || f.report.GuestExitCode != 1 {
		t.Errorf("report = %v exit %d, want FAIL exit 1", f.report.Verdict, f.report.GuestExitCode)
	}
}

// VALIDATES: a guest command that has not ended within the teardown grace
// after the forwarded interrupt is cut off: its ssh session ends, the VM is
// stopped, and the error says the teardown did not finish.
// METHOD: as above, with a fake guest that ends only when its context does and
// a grace of 200ms.
// PREVENTS: a hung guest teardown holding the run, and the VM, forever.
func TestGuestCommandTeardownPastTheGraceStopsTheVM(t *testing.T) {
	f := newGuestStopFixture(t)
	f.run.teardownGrace = 200 * time.Millisecond
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.answer(t, func(ctx context.Context) (commandResult, error) {
		cancel()
		<-ctx.Done()
		return commandResult{}, ctx.Err()
	}, func() (commandResult, error) {
		return commandResult{Stdout: "4242\n"}, nil
	})

	_, err := f.run.executePlan(root, &f.report)
	if err == nil {
		t.Fatal("a guest command past its teardown grace answered no error")
	}
	if !strings.Contains(err.Error(), "did not finish its teardown within 200ms") {
		t.Errorf("the error does not name the expired grace: %v", err)
	}
	want := []string{"guest own-group", "interrupt vm-running own-group"}
	if got := f.recorded(); !equalStrings(got, want) {
		t.Errorf("ssh calls = %q, want %q", got, want)
	}
	if !f.vmStopped() {
		t.Error("the VM was not stopped after the grace expired")
	}
}

// VALIDATES: a second interrupt while the run waits for the guest's teardown
// stops the wait at once and stops the VM, and the error says so.
// METHOD: as above, with a grace of an hour and a fake Interrupts channel the
// fake interrupt ssh fills.
// PREVENTS: a second Ctrl-C swallowed by the signal root for the whole grace.
func TestGuestCommandSecondInterruptStopsTheVM(t *testing.T) {
	f := newGuestStopFixture(t)
	f.run.teardownGrace = time.Hour
	again := make(chan os.Signal, 1)
	released := false
	f.run.ops.Interrupts = func() (<-chan os.Signal, func()) {
		return again, func() { released = true }
	}
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.answer(t, func(ctx context.Context) (commandResult, error) {
		cancel()
		<-ctx.Done()
		return commandResult{}, ctx.Err()
	}, func() (commandResult, error) {
		again <- os.Interrupt
		return commandResult{Stdout: "4242\n"}, nil
	})

	_, err := f.run.executePlan(root, &f.report)
	if err == nil {
		t.Fatal("a second interrupt answered no error")
	}
	if !strings.Contains(err.Error(), "second interrupt") {
		t.Errorf("the error does not name the second interrupt: %v", err)
	}
	if !released {
		t.Error("the second-interrupt subscription was never released")
	}
	if !f.vmStopped() {
		t.Error("the VM was not stopped after the second interrupt")
	}
}

// VALIDATES: an ssh call that cannot deliver the interrupt (no pid file yet,
// or the guest unreachable) ends the guest session, stops the VM, and says the
// teardown never ran.
// PREVENTS: a failed delivery waited out for the whole grace, then reported as
// a teardown that merely ran long.
func TestGuestCommandUndeliveredInterruptSaysSo(t *testing.T) {
	f := newGuestStopFixture(t)
	f.run.teardownGrace = time.Hour
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.answer(t, func(ctx context.Context) (commandResult, error) {
		cancel()
		<-ctx.Done()
		return commandResult{}, ctx.Err()
	}, func() (commandResult, error) {
		return commandResult{Code: 1, Stderr: "cat: can't open '/tmp/ze-guest-command.pgid'"}, nil
	})

	_, err := f.run.executePlan(root, &f.report)
	if err == nil {
		t.Fatal("an undelivered interrupt answered no error")
	}
	if !strings.Contains(err.Error(), "could not forward SIGINT") {
		t.Errorf("the error does not say the interrupt was not delivered: %v", err)
	}
	if !strings.Contains(err.Error(), "can't open") {
		t.Errorf("the error drops the guest's own reason: %v", err)
	}
	if f.report.Verdict != RunVerdictUnspecified {
		t.Errorf("verdict %v for a guest command nobody heard end, want it unassigned", f.report.Verdict)
	}
	if !f.vmStopped() {
		t.Error("the VM was not stopped after the failed delivery")
	}
}

// startGuestCommandLine runs guestCommandBody(groupFile, "true", command)
// under the host's /bin/sh as the leader of its own group, as sshd starts it,
// and answers once the line has written its group file, so a SIGINT sent then
// lands under the line's trap.
func startGuestCommandLine(t *testing.T, groupFile, command string) (*exec.Cmd, *strings.Builder) {
	t.Helper()
	line := exec.CommandContext(t.Context(), "/bin/sh", "-c", guestCommandBody(groupFile, "true", command))
	line.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	output := &strings.Builder{}
	line.Stdout = output
	if err := line.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		// The file exists once the shell ran its redirect. Its content is the
		// group only where /proc exists, which the Mac has not.
		if _, err := os.Stat(groupFile); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the guest command line never wrote its group file")
		}
		time.Sleep(20 * time.Millisecond) // sleep(poll): no shell event reaches the test when the redirect lands
	}
	time.Sleep(200 * time.Millisecond) // sleep(settle): the trapped command has to start before the SIGINT
	return line, output
}

// guestTearsDownOnSIGINT is a guest command that runs until SIGINT, tears
// down, and exits 1, as the guest le does.
const guestTearsDownOnSIGINT = `sh -c 'trap "echo torn down; exit 1" INT; while :; do sleep 0.1; done'`

// VALIDATES: the guest command line keeps the guest command's exit status
// through a SIGINT to its whole process group: the shell survives the signal,
// waits for a command that tears down on SIGINT, exits with that command's
// status, and runs no later step of the line (review I2). A failed setup runs
// no step of a command that is itself a list.
// METHOD: startGuestCommandLine, then signal the group. The guest's busybox
// ash was judged by hand in busybox 1.36 (2026-10-21): `trap 'exit $?' INT`
// stops `a || b; c` after `a`, keeps `a`'s status, and runs an EXIT trap.
// PREVENTS: the shell dying on the forwarded SIGINT and answering the signal
// instead of the guest le's exit 1; a no-op trap that let the shell run the
// line's next step after the interrupted one; an interrupted setup falling
// through to the rest of the command list.
func TestGuestCommandLineKeepsTheCommandsStatus(t *testing.T) {
	command, output := startGuestCommandLine(t, filepath.Join(t.TempDir(), "pgid"),
		guestTearsDownOnSIGINT+"; echo next step")
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	err := command.Wait()
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exit.ExitCode() != 1 {
		t.Errorf("the line answered %v (%s), want exit status 1 from the command", err, strconv.Quote(output.String()))
	}
	if !strings.Contains(output.String(), "torn down") {
		t.Errorf("the command's teardown did not run: %q", output.String())
	}
	if strings.Contains(output.String(), "next step") {
		t.Errorf("the line ran its next step after the interrupted one: %q", output.String())
	}

	failed := guestCommandBody(filepath.Join(t.TempDir(), "pid"), "false", "echo one; echo two")
	ran, err := exec.CommandContext(t.Context(), "/bin/sh", "-c", failed).Output()
	if err == nil {
		t.Error("a failed setup answered success")
	}
	if strings.Contains(string(ran), "two") {
		t.Errorf("a failed setup still ran the rest of the command list: %q", ran)
	}
}

// VALIDATES: a guest command still running when its `timeout` passes is
// stopped the same way as an interrupted one: SIGINT is forwarded, and the
// error names the command timeout (review I4).
// METHOD: executePlan over the fake QEMU console with a 200ms command timeout
// and no cancel; the fake guest ends with exit 1 only once the interrupt ssh ran.
// PREVENTS: a timeout that ended the ssh session under the guest le, so its
// teardown never ran, or an error that blamed an interrupt nobody sent.
func TestGuestCommandTimeoutForwardsTheInterrupt(t *testing.T) {
	f := newGuestStopFixture(t)
	f.run.Options.Timeout = 200 * time.Millisecond
	released := make(chan struct{})
	f.answer(t, func(ctx context.Context) (commandResult, error) {
		select {
		case <-released:
			return commandResult{Code: 1}, nil
		case <-ctx.Done():
			return commandResult{}, ctx.Err()
		}
	}, func() (commandResult, error) {
		close(released)
		return commandResult{Stdout: "4242\n"}, nil
	})

	_, err := f.run.executePlan(t.Context(), &f.report)
	if err == nil {
		t.Fatal("a guest command past its timeout answered no error")
	}
	if !strings.Contains(err.Error(), "200ms command timeout") {
		t.Errorf("the error does not name the command timeout: %v", err)
	}
	want := []string{"guest own-group", "interrupt vm-running own-group"}
	if got := f.recorded(); !equalStrings(got, want) {
		t.Errorf("ssh calls = %q, want %q", got, want)
	}
}

// VALIDATES: an ssh exit 255 after the forward is reported as an ssh failure,
// never as the guest command's exit status, and assigns no verdict (review N11).
// METHOD: as the interrupt test, with a fake guest ssh that answers 255.
// PREVENTS: "the guest command exited 255" for a transport that died.
func TestGuestCommandSSHFailureAfterTheForwardSaysSSH(t *testing.T) {
	f := newGuestStopFixture(t)
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	released := make(chan struct{})
	f.answer(t, func(ctx context.Context) (commandResult, error) {
		cancel()
		select {
		case <-released:
			return commandResult{Code: 255}, nil
		case <-ctx.Done():
			return commandResult{}, ctx.Err()
		}
	}, func() (commandResult, error) {
		close(released)
		return commandResult{Stdout: "4242\n"}, nil
	})

	_, err := f.run.executePlan(root, &f.report)
	if err == nil {
		t.Fatal("an ssh failure answered no error")
	}
	if !strings.Contains(err.Error(), "ssh failed") || strings.Contains(err.Error(), "exited 255") {
		t.Errorf("the error does not say the ssh session failed: %v", err)
	}
	if f.report.Verdict != RunVerdictUnspecified {
		t.Errorf("verdict %v for a guest command nobody heard end, want it unassigned", f.report.Verdict)
	}
}

// deadTerminal is a terminal after a hangup: every write fails.
type deadTerminal struct{}

func (deadTerminal) Write([]byte) (int, error) { return 0, syscall.EIO }

// VALIDATES: a streamed command keeps running, and its output keeps being
// read, after the terminal it streams to has gone (review round 2, NOTE 3): a
// SIGHUP's guest teardown needs its ssh session to survive the dead tty.
// METHOD: runHostCommand streaming to a writer that fails every write, over a
// child that writes before and after a pause.
// PREVENTS: the copy to the dead terminal stopping the drain of the child's
// output, so the guest ssh dies of EPIPE and the guest le of SIGPIPE before
// its teardown.
func TestHostCommandOutlivesADeadTerminal(t *testing.T) {
	result, err := runHostCommand(t.Context(), commandSpec{
		Name: "/bin/sh", Args: []string{"-c", "echo before; sleep 0.2; echo after; echo late >&2"},
		Stdout: deadTerminal{}, Stderr: deadTerminal{},
	})
	if err != nil {
		t.Fatalf("a dead terminal failed the command: %v", err)
	}
	if result.Code != 0 || result.Stdout != "before\nafter\n" || result.Stderr != "late\n" {
		t.Errorf("result = %+v, want exit 0 with both streams drained", result)
	}
	// Round 3, NOTE 1: the lost stream is said once, with its cause, never
	// swallowed (an ENOSPC on a job log reads the same way).
	for _, want := range []string{"stopped accepting writes at ", syscall.EIO.Error(), "captured output is intact"} {
		if !strings.Contains(result.StreamNote, want) {
			t.Errorf("stream note %q does not say %q", result.StreamNote, want)
		}
	}
}

// VALIDATES: OwnGroup starts the host child as the leader of a process group
// of its own, and without it the child stays in le's group.
// METHOD: the child prints its own process group and pid.
// PREVENTS: a guest ssh session that a terminal Ctrl-C reaches directly.
func TestHostCommandOwnGroupLeadsItsOwnProcessGroup(t *testing.T) {
	for _, own := range []bool{true, false} {
		result, err := runHostCommand(t.Context(), commandSpec{
			Name: "/bin/sh", Args: []string{"-c", "ps -o pgid= -p $$; echo $$"}, OwnGroup: own,
		})
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(result.Stdout)
		if len(fields) != 2 {
			t.Fatalf("child printed %q, want its group and its pid", result.Stdout)
		}
		leads := fields[0] == fields[1]
		if leads != own {
			t.Errorf("OwnGroup %v: child group %s, pid %s", own, fields[0], fields[1])
		}
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
