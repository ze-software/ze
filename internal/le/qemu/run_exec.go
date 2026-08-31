// Design: docs/architecture/testing/qemu-integration.md -- the bounded VM lifecycle
// Overview: run.go -- the plan this lifecycle executes
package qemu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Execute boots the plan, configures the guest, and runs its command. Operating
// failures return an error. A guest or proof failure is report data.
func (r *Run) Execute(ctx context.Context) (RunReport, error) {
	plan, err := r.Plan(ctx)
	if err != nil {
		return RunReport{Plan: plan}, err
	}
	report := RunReport{Plan: plan}
	return r.executePlan(ctx, &report)
}

func (r *Run) executePlan(ctx context.Context, report *RunReport) (RunReport, error) {
	errorFile, err := r.ops.FS.CreateTemp(filepath.Join(r.Tree, "tmp", "qemu"), ".qemu-errors-*")
	if err != nil {
		return *report, fmt.Errorf("create the QEMU diagnostics file: %w", err)
	}
	errorName := errorFile.Name()
	defer func() {
		errorFile.Close()          //nolint:errcheck // the diagnostics have already been read
		r.ops.FS.Remove(errorName) //nolint:errcheck // this per-run resource must not persist
	}()
	fmt.Fprintf(os.Stderr, "Booting Alpine VM (ssh forwarded on host port %d)...\n", report.Plan.SSHPort) //nolint:errcheck // progress output

	// #nosec G204 -- Run.Plan fixes the executable and option sequence; settings and resolved fixture paths each occupy one argv entry and never enter a shell.
	command := exec.CommandContext(ctx, report.Plan.QEMUArgv[0], report.Plan.QEMUArgv[1:]...)
	// The existing stopVM owner preserves the interrupt-then-kill shutdown
	// contract for QEMU; CommandContext's default would kill it immediately.
	command.Cancel = func() error { return nil }
	command.Env = r.ops.Environ()
	stdin, err := command.StdinPipe()
	if err != nil {
		return *report, fmt.Errorf("open QEMU standard input: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return *report, fmt.Errorf("open QEMU serial output: %w", err)
	}
	command.Stderr = errorFile
	if err := r.ops.Start(command); err != nil {
		return *report, fmt.Errorf("start %s: %w", report.Plan.QEMUBinary, err)
	}
	vmWaitOwned := false
	defer func() {
		if !vmWaitOwned {
			stopVM(command)
		}
	}()

	serial := newSerialStream(ctx, stdout)
	login, err := serial.expect(ctx, "login:", r.Options.Boot)
	if err != nil {
		return *report, err
	}
	if !login {
		detail := readQEMUErrors(errorFile)
		if detail != "" {
			return *report, fmt.Errorf("VM did not reach a login prompt; QEMU said: %s", detail)
		}
		return *report, fmt.Errorf("VM did not reach a login prompt after %s", r.Options.Boot)
	}
	if err := r.ops.Sleep(ctx, time.Second); err != nil {
		return *report, err
	}
	if err := sendSerial(stdin, "root"); err != nil {
		return *report, err
	}
	if err := r.ops.Sleep(ctx, 3*time.Second); err != nil {
		return *report, err
	}
	if err := r.bootstrap(ctx, stdin, serial, report.Plan.SSHPort); err != nil {
		return *report, err
	}
	fmt.Fprintln(os.Stderr, "  VM ready.") //nolint:errcheck // progress output
	if report.Plan.Kernel != "" {
		failure, err := r.assertRuntimeKernel(ctx, &report.Plan)
		if err != nil {
			return *report, err
		}
		if failure != "" {
			report.Verdict = RunVerdictFail
			report.GuestExitCode = 1
			report.ProofFailure = failure
			return *report, nil
		}
	}
	if r.Options.KeepAlive {
		vmWaitOwned = true
		return r.keepAlive(ctx, report, command)
	}
	return r.runGuestCommand(ctx, report)
}

func readQEMUErrors(file *os.File) string {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func sendSerial(stdin io.Writer, command string) error {
	var b textbuf.Buffer
	if _, err := io.WriteString(stdin, b.Str(command).Byte('\n').String()); err != nil {
		return fmt.Errorf("write QEMU serial command: %w", err)
	}
	return nil
}

type serialStream struct {
	chunks   <-chan []byte
	failures <-chan error
	seen     strings.Builder
}

func newSerialStream(ctx context.Context, stdout io.Reader) *serialStream {
	chunks := make(chan []byte, 1)
	failures := make(chan error, 1)
	go func() {
		buffer := make([]byte, serialReadSize)
		for {
			count, err := stdout.Read(buffer)
			if count != 0 {
				copied := append([]byte(nil), buffer[:count]...)
				select {
				case chunks <- copied:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				failures <- err
				return
			}
		}
	}()
	return &serialStream{chunks: chunks, failures: failures}
}

func (s *serialStream) expect(ctx context.Context, pattern string, timeout time.Duration) (bool, error) {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		if strings.Contains(s.seen.String(), pattern) {
			s.seen.Reset()
			return true, nil
		}
		select {
		case <-deadline.Done():
			if errors.Is(deadline.Err(), context.DeadlineExceeded) {
				return false, nil
			}
			return false, deadline.Err()
		case err := <-s.failures:
			if errors.Is(err, io.EOF) {
				return false, nil
			}
			return false, fmt.Errorf("read QEMU serial output: %w", err)
		case chunk := <-s.chunks:
			s.seen.Write(chunk)
			if s.seen.Len() > serialBufferMax {
				text := s.seen.String()
				s.seen.Reset()
				s.seen.WriteString(text[len(text)-serialBufferKeep:])
			}
		}
	}
}

func (r *Run) bootstrap(ctx context.Context, stdin io.Writer, serial *serialStream, port int) error {
	for attempt := 1; attempt <= bootstrapAttempts; attempt++ {
		fmt.Fprintf(os.Stderr, "  bootstrapping VM (network + sshd), attempt %d...\n", attempt) //nolint:errcheck // progress output
		if err := sendSerial(stdin, runBootstrapCommand); err != nil {
			return err
		}
		ready, err := serial.expect(ctx, "SSHD_READY", bootstrapTimeout)
		if err != nil {
			return err
		}
		if !ready {
			continue
		}
		fmt.Fprintln(os.Stderr, "  waiting for SSH...") //nolint:errcheck // progress output
		if err := r.waitForSSH(ctx, port, sshReadyTimeout); err == nil {
			return nil
		}
		fmt.Fprintln(os.Stderr, "  SSH not up yet; re-running bootstrap...") //nolint:errcheck // progress output
		if err := r.ops.Sleep(ctx, sshPollInterval); err != nil {
			return err
		}
	}
	return errors.New("VM bootstrap failed: SSH was not reachable after three attempts")
}

func (r *Run) sshOptions(port int) []string {
	return []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "PreferredAuthentications=none",
		"-o", "LogLevel=ERROR",
		"-p", strconv.Itoa(port),
	}
}

func (r *Run) waitForSSH(ctx context.Context, port int, timeout time.Duration) error {
	deadline := r.ops.Now().Add(timeout)
	for r.ops.Now().Before(deadline) {
		args := append(r.sshOptions(port), "-o", "ConnectTimeout=2", "root@localhost", "true")
		result, err := r.runCommand(ctx, commandSpec{
			Name: "ssh", Args: args, Env: r.ops.Environ(),
			Stdin: strings.NewReader(""),
		})
		if err != nil {
			return err
		}
		if result.Code == 0 {
			return nil
		}
		if err := r.ops.Sleep(ctx, sshPollInterval); err != nil {
			return err
		}
	}
	return fmt.Errorf("SSH was not reachable after %s", timeout)
}

func (r *Run) sshRun(ctx context.Context, plan *RunPlan, remote string, timeout time.Duration, stream bool) (int, error) {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := append(r.sshOptions(plan.SSHPort), "-o", "ServerAliveInterval=30", "root@localhost", remote)
	spec := commandSpec{Name: "ssh", Args: args, Env: r.ops.Environ()}
	if stream {
		spec.Stdout = os.Stdout
		spec.Stderr = os.Stderr
	}
	result, err := r.runCommand(deadline, spec)
	if err != nil {
		return 0, err
	}
	return result.Code, nil
}

func (r *Run) assertRuntimeKernel(ctx context.Context, plan *RunPlan) (string, error) {
	versionRaw, err := r.ops.FS.ReadFile(filepath.Join(r.Tree, "internal", "appliance", "kernel.version"))
	if err != nil {
		return "", fmt.Errorf("read expected runtime kernel: %w", err)
	}
	want := strings.TrimSpace(string(versionRaw))
	var b textbuf.Buffer
	probe := b.Str("actual=$(uname -r); case \"$actual\" in ").Str(want).Byte('|').Str(want).
		Str(".*) exit 0 ;; esac; echo \"the VM booted $actual, not the ").Str(want).
		Str(" kernel ze ships -- --kernel was passed but QEMU is running another kernel, so every verdict from this run would be about a kernel no operator gets\" >&2; exit 1").String()
	code, err := r.sshRun(ctx, plan, probe, time.Minute, true)
	if err != nil {
		return "", err
	}
	if code == 0 {
		var note textbuf.Buffer
		note.Str("Runtime kernel confirmed in the guest: ").Str(want).Byte('\n').StdErr() //nolint:errcheck // progress output
		return "", nil
	}
	b.Reset()
	return b.Str("guest is not running ze's ").Str(want).Str(" runtime kernel").String(), nil
}

func (r *Run) runGuestCommand(ctx context.Context, report *RunReport) (RunReport, error) {
	var b textbuf.Buffer
	body := b.Str(report.Plan.SetupCommand).Str(" && ").Str(r.Options.Command).String()
	fmt.Fprintf(os.Stderr, "  running: %s\n", r.Options.Command) //nolint:errcheck // progress output
	b.Reset()
	remote := b.Str("sh -c ").Str(shellQuote(body)).String()
	code, err := r.sshRun(ctx, &report.Plan, remote, r.Options.Timeout, true)
	if err != nil {
		return *report, err
	}
	report.GuestExitCode = code
	if code == 0 {
		report.Verdict = RunVerdictPass
		return *report, nil
	}
	report.Verdict = RunVerdictFail
	return *report, nil
}

func (r *Run) keepAlive(ctx context.Context, report *RunReport, vm *exec.Cmd) (RunReport, error) {
	done := waitVM(vm)
	defer stopVMWaiting(vm, done)
	profile := []string{
		"export PATH=/usr/local/go/bin:$PATH", "export GOROOT=/usr/local/go",
		"export GOCACHE=/workspace/tmp/qemu/go-cache",
		"export GOMODCACHE=/workspace/tmp/qemu/gomodcache",
		"export GOFLAGS=-buildvcs=false", "export CGO_ENABLED=0",
		"export HOME=/root", "export TMPDIR=/tmp", "cd /workspace",
	}
	quoted := make([]string, 0, len(profile))
	for _, line := range profile {
		quoted = append(quoted, shellQuote(line))
	}
	var b textbuf.Buffer
	writeProfile := b.Str("printf '%s\\n' ").Join(quoted, " ").Str(" > /etc/profile.d/ze.sh").String()
	b.Reset()
	body := b.Str(report.Plan.SetupCommand).Str(" && ").Str(writeProfile).String()
	b.Reset()
	remote := b.Str("sh -c ").Str(shellQuote(body)).String()
	code, err := r.sshRun(ctx, &report.Plan, remote, r.Options.Timeout, true)
	if err != nil {
		return *report, err
	}
	if code != 0 {
		report.GuestExitCode = code
		report.Verdict = RunVerdictFail
		return *report, nil
	}
	fmt.Fprintln(os.Stderr, "ZE_QEMU_READY")                                            //nolint:errcheck // progress output
	fmt.Fprintln(os.Stderr, "VM remains available until the timeout or a stop signal.") //nolint:errcheck // progress output
	var hint textbuf.Buffer
	ssh := hint.Str("ssh ").Join(r.sshOptions(report.Plan.SSHPort), " ").Str(" root@localhost").String()
	hint.Reset()
	fmt.Fprintln(os.Stderr, hint.Str("Run a guest test: ").Str(ssh).Str(" 'cd /workspace && ZE_TEST_NO_BUILD=1 ZE_BIN=").
		Str(settingOr(runBinaryEntry.Key, runBinaryEntry.Default)).Byte(' ').
		Str(settingOr(runTestBinEntry.Key, runTestBinEntry.Default)).Str(" bgp parse 264 -v'").String()) //nolint:errcheck // progress output
	timer := time.NewTimer(r.Options.Timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return *report, ctx.Err()
	case <-done:
		report.Verdict = RunVerdictPass
		return *report, nil
	case <-timer.C:
		report.Verdict = RunVerdictPass
		return *report, nil
	}
}
