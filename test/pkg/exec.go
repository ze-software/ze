package functional

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Exec wraps os/exec.Cmd with output capture and graceful termination.
type Exec struct {
	cmd      *exec.Cmd
	stdout   bytes.Buffer
	stderr   bytes.Buffer
	exitCode int
	cmdStr   string
	started  time.Time
	workDir  string
	mu       sync.Mutex
	done     chan struct{}
}

// NewExec creates a new Exec instance.
func NewExec() *Exec {
	return &Exec{
		exitCode: -1,
		done:     make(chan struct{}),
	}
}

// SetDir sets the working directory for the command.
// Must be called before Run().
func (e *Exec) SetDir(dir string) *Exec {
	e.workDir = dir
	return e
}

// Run starts the command with the given arguments and environment.
// If env is nil, inherits the current environment.
func (e *Exec) Run(ctx context.Context, args []string, env map[string]string) error {
	if len(args) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.cmdStr = formatCommand(args)
	e.cmd = exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // Test harness runs known commands

	// Set working directory if specified
	if e.workDir != "" {
		e.cmd.Dir = e.workDir
	}

	// Set up environment
	e.cmd.Env = os.Environ()
	for k, v := range env {
		e.cmd.Env = append(e.cmd.Env, k+"="+v)
	}

	// Capture output
	e.cmd.Stdout = &e.stdout
	e.cmd.Stderr = &e.stderr

	// Create new process group for clean termination
	e.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	e.started = time.Now()

	if err := e.cmd.Start(); err != nil {
		return err
	}

	// Monitor for completion in background
	go func() {
		_ = e.cmd.Wait()
		close(e.done)
	}()

	return nil
}

// Wait blocks until the process exits.
func (e *Exec) Wait() error {
	<-e.done
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cmd != nil && e.cmd.ProcessState != nil {
		e.exitCode = e.cmd.ProcessState.ExitCode()
	}
	return nil
}

// Ready returns true if the process has exited.
func (e *Exec) Ready() bool {
	select {
	case <-e.done:
		e.mu.Lock()
		if e.cmd != nil && e.cmd.ProcessState != nil {
			e.exitCode = e.cmd.ProcessState.ExitCode()
		}
		e.mu.Unlock()
		return true
	default:
		return false
	}
}

// Terminate gracefully stops the process (SIGTERM, then SIGKILL).
func (e *Exec) Terminate() {
	e.mu.Lock()
	cmd := e.cmd
	e.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	// Try SIGTERM first
	_ = cmd.Process.Signal(syscall.SIGTERM)

	// Wait briefly for graceful exit
	select {
	case <-e.done:
		return
	case <-time.After(500 * time.Millisecond):
	}

	// Kill process group with SIGKILL
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	// Wait for process to actually exit
	select {
	case <-e.done:
	case <-time.After(500 * time.Millisecond):
	}
}

// Collect ensures output is captured after process exits.
// Safe to call multiple times.
func (e *Exec) Collect() {
	// Wait for done if not already
	select {
	case <-e.done:
	default:
		// Process still running, don't block
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil && e.cmd.ProcessState != nil {
		e.exitCode = e.cmd.ProcessState.ExitCode()
	}
}

// Stdout returns captured stdout as string.
func (e *Exec) Stdout() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stdout.String()
}

// Stderr returns captured stderr as string.
func (e *Exec) Stderr() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stderr.String()
}

// StdoutBytes returns captured stdout as bytes.
func (e *Exec) StdoutBytes() []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stdout.Bytes()
}

// StderrBytes returns captured stderr as bytes.
func (e *Exec) StderrBytes() []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stderr.Bytes()
}

// ExitCode returns the process exit code (-1 if not exited).
func (e *Exec) ExitCode() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exitCode
}

// Command returns the command string for logging.
func (e *Exec) Command() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cmdStr
}

// Started returns when the process was started.
func (e *Exec) Started() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.started
}

// formatCommand formats command args as a shell-like string.
func formatCommand(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if strings.Contains(arg, " ") {
			parts[i] = "'" + arg + "'"
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}
