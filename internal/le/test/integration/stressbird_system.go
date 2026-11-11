// Design: docs/functional-tests.md -- the stress harness, and how its children die with it
// Related: stress.go -- realStressSystem, which embeds this system for the ze scenarios.
// Related: stressbird.go -- stressBirdSystem and stressBirdProcess, the seams this file implements.
//
// stressbird_system.go holds the real system the stress gates fork through:
// how each child is started, stopped and reaped, and why it dies with the
// harness.
package testintegration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
)

// realStressBirdSystem forks the stress harness's real children: ze, BIRD, the
// `le test peer` sink and injector, tcpdump and the profile curl through
// Start, and ip, ethtool, birdc, ss and the DUT build through Run. stress.go's
// realStressSystem embeds it, so both stress gates fork the same way.
//
// Every child forks in a process group of its own, with SIGKILL as its death
// signal (plugin.KillGroupOnCancel, plugin.NewSysProcAttr). The harness runs as
// root and so do its children, so one left behind keeps a port, an address and
// a namespace that redden the next stress or functional run. The death signal
// covers a harness that dies without running Go code (SIGKILL, the OOM killer);
// the group lets a canceled context, Terminate and Kill reach everything the
// child started.
//
// The kernel sends the death signal when the forking THREAD exits, not the
// harness, so every fork goes through plugin.StartChild, whose spawner thread
// lives as long as the harness. A child forked on a goroutine that locked its
// thread and returned would be SIGKILLed when the runtime destroyed that thread.
//
// Most children run as `ip netns exec <namespace> <binary> ...`. Outside batch
// mode iproute2 execs the target in place (ip/ipnetns.c netns_exec calls
// lib/exec.c cmd_exec, which forks only when asked to), so the target IS the
// forked child and keeps its group and its death signal. The fork also runs
// from the host namespace: `ip` moves itself, so no fork needs a namespace
// thread. A credential-changing exec would clear the death signal, and none
// happens here: root execs ordinary binaries.
type realStressBirdSystem struct{}

func (realStressBirdSystem) effectiveUID() int { return os.Geteuid() }

func (realStressBirdSystem) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (realStressBirdSystem) FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (realStressBirdSystem) Executable() (string, error) { return os.Executable() }
func (realStressBirdSystem) PID() int                    { return os.Getpid() }
func (realStressBirdSystem) Environ() []string           { return os.Environ() }
func (realStressBirdSystem) Getenv(key string) string    { return os.Getenv(key) }
func (realStressBirdSystem) Now() time.Time              { return time.Now() }

// Run runs one short command to its end and answers its output and exit code.
// When its timeout or ctx ends it, the command's whole group is SIGKILLed and
// Run answers that context error beside whatever output had arrived.
func (realStressBirdSystem) Run(
	ctx context.Context,
	command stressBirdCommand,
) (stressBirdCommandResult, error) {
	commandCtx := ctx
	cancel := func() {}
	if command.timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, command.timeout)
	}
	defer cancel()
	cmd := exec.CommandContext(commandCtx, command.argv[0], command.argv[1:]...) //nolint:gosec // fixed scenario grammar owns argv
	cmd.Dir = command.dir
	cmd.Env = command.environ
	var stdout, stderr textbuf.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	plugin.KillGroupOnCancel(cmd)
	// The buffers are not files, so exec copies the output through pipes and
	// Wait returns only when every holder of their write ends has closed them.
	// The group SIGKILL closes them for every process in the group, but one
	// that moved to a group of its own would hold Wait open past the timeout
	// that ended the command. WaitDelay bounds that wait once the context is
	// done; it does nothing to a command whose context still runs.
	cmd.WaitDelay = stressBirdStopTimeout
	err := plugin.StartChild(cmd)
	if err == nil {
		err = cmd.Wait()
	}
	result := stressBirdCommandResult{
		stdout: stdout.String(), stderr: stderr.String(), code: gaterun.ExitCode(err),
	}
	if commandCtx.Err() != nil {
		return result, commandCtx.Err()
	}
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return result, err
		}
	}
	return result, nil
}

// Start forks one long-lived child with its output in command.outputPath. A
// canceled ctx SIGKILLs the child's whole group. The caller MUST stop the
// process with Terminate or Kill and then call Wait, as cleanup does.
func (realStressBirdSystem) Start(
	ctx context.Context,
	command stressBirdCommand,
) (stressBirdProcess, error) {
	output, err := os.OpenFile(command.outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open process log %s: %w", command.outputPath, err)
	}
	cmd := exec.CommandContext(ctx, command.argv[0], command.argv[1:]...) //nolint:gosec // fixed scenario grammar owns argv
	cmd.Dir = command.dir
	cmd.Env = command.environ
	cmd.Stdout = output
	cmd.Stderr = output
	plugin.KillGroupOnCancel(cmd)
	// WaitDelay stays unset: the output is a file, so exec copies nothing and
	// Wait returns when the child exits. A canceled context SIGKILLs the whole
	// group, which a process cannot ignore.
	if err := plugin.StartChild(cmd); err != nil {
		return nil, errors.Join(err, output.Close())
	}
	process := &realStressBirdProcess{cmd: cmd, output: output, done: make(chan struct{})}
	go process.collect()
	return process, nil
}

func (realStressBirdSystem) Remove(path string) error { return os.Remove(path) }

func (realStressBirdSystem) Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// realStressBirdProcess owns one wait goroutine from Start until the child exits.
// The caller MUST call Wait after Terminate or Kill. Safe for concurrent use.
type realStressBirdProcess struct {
	cmd    *exec.Cmd
	output io.Closer
	done   chan struct{}
	mu     sync.Mutex
	code   int
	err    error
}

func (p *realStressBirdProcess) collect() {
	err := p.cmd.Wait()
	closeErr := p.output.Close()
	p.mu.Lock()
	p.code = gaterun.ExitCode(err)
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			p.err = err
		}
	}
	if p.err == nil && closeErr != nil {
		p.err = closeErr
	}
	p.mu.Unlock()
	close(p.done)
}

func (p *realStressBirdProcess) Exited() (bool, int, error) {
	select {
	case <-p.done:
		code, err := p.result()
		return true, code, err
	default:
		return false, 0, nil
	}
}

// Wait answers the child's exit once collect has reaped it, or
// errStressBirdWaitTimeout when timeout passes first. It MUST be called after
// Terminate or Kill, so the stop is known to have reached the child.
func (p *realStressBirdProcess) Wait(timeout time.Duration) (int, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return p.result()
	case <-timer.C:
		return 0, errStressBirdWaitTimeout
	}
}

// Terminate sends SIGTERM to the child's whole process group, so a child that
// started children of its own stops with them. The caller MUST call Wait after
// it. A process already reaped answers nil.
func (p *realStressBirdProcess) Terminate() error {
	select {
	case <-p.done:
		return nil
	default:
		return stressGroupSignalResult(syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM))
	}
}

// Kill SIGKILLs the child's whole process group. The caller MUST call Wait
// after it. A process already reaped answers nil.
func (p *realStressBirdProcess) Kill() error {
	select {
	case <-p.done:
		return nil
	default:
		return stressGroupSignalResult(plugin.KillProcessGroup(p.cmd.Process.Pid))
	}
}

// stressGroupSignalResult answers os.ErrProcessDone for a group signal the
// kernel refused with ESRCH, the answer os.Process.Signal gives for a reaped
// process and the one every stop caller already accepts. The group can be gone
// while done is still open: collect reaps the child before it closes done, and
// the child's group then empties with it. ESRCH can mean nothing else here,
// because every child was forked with Setpgid, so its pid heads a group while
// any member lives, and the kernel never hands a pid to a new process while a
// group still carries it.
func stressGroupSignalResult(err error) error {
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func (p *realStressBirdProcess) result() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.code, p.err
}

var _ stressBirdSystem = realStressBirdSystem{}
var _ stressBirdProcess = (*realStressBirdProcess)(nil)
