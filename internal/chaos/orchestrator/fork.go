// Design: docs/architecture/chaos-web-dashboard.md -- fork ze as child process with stdin pipe

package orchestrator

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ZeChild represents a forked child process (Ze or external daemon).
type ZeChild struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	done    chan struct{}
	waitErr error
	tmpFile string
}

// ForkZe starts a Ze instance with config piped via stdin.
func ForkZe(ctx context.Context, config, binary string) (*ZeChild, error) {
	if binary == "" {
		var err error
		binary, err = exec.LookPath("ze")
		if err != nil {
			return nil, fmt.Errorf("ze not found in PATH (use --binary to specify): %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, binary, "-") // #nosec G204 - binary from --binary flag or PATH
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ze: %w", err)
	}

	cleanup := func() {
		if err := cmd.Process.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: killing ze: %v\n", err)
		}
		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: waiting for ze: %v\n", err)
		}
	}

	if _, err := stdin.Write([]byte(config)); err != nil {
		cleanup()
		return nil, fmt.Errorf("writing config: %w", err)
	}
	if _, err := stdin.Write([]byte{0}); err != nil {
		cleanup()
		return nil, fmt.Errorf("writing sentinel: %w", err)
	}

	child := &ZeChild{cmd: cmd, stdin: stdin, done: make(chan struct{})}
	go func() {
		child.waitErr = cmd.Wait()
		close(child.done)
	}()

	return child, nil
}

// PID returns the process ID of the child.
func (z *ZeChild) PID() int {
	return z.cmd.Process.Pid
}

// Done returns a channel that closes when the child process exits.
func (z *ZeChild) Done() <-chan struct{} { return z.done }

// WaitErr returns the error from the child process Wait call.
func (z *ZeChild) WaitErr() error { return z.waitErr }

// Signal sends a signal to the child process.
func (z *ZeChild) Signal(sig os.Signal) {
	if err := z.cmd.Process.Signal(sig); err != nil {
		fmt.Fprintf(os.Stderr, "warning: signaling ze: %v\n", err)
	}
}

// Shutdown closes stdin and waits for the child to exit.
func (z *ZeChild) Shutdown() {
	if z.stdin != nil {
		if err := z.stdin.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: closing stdin: %v\n", err)
		}
	}
	select {
	case <-z.done:
		if z.waitErr != nil {
			fmt.Fprintf(os.Stderr, "warning: daemon exited: %v\n", z.waitErr)
		}
	case <-time.After(5 * time.Second):
		fmt.Fprintf(os.Stderr, "warning: daemon did not exit within 5s, killing\n")
		if err := z.cmd.Process.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: killing daemon: %v\n", err)
		}
		<-z.done
	}
	if z.tmpFile != "" {
		if err := os.Remove(z.tmpFile); err != nil {
			fmt.Fprintf(os.Stderr, "warning: removing temp config: %v\n", err)
		}
	}
}

// ForkDaemon launches an external BGP daemon (FRR bgpd or BIRD) with config
// written to a temp file. The daemon runs in the foreground as a child process.
func ForkDaemon(ctx context.Context, config, binary string, target scenario.Target, port int, localAddr string) (*ZeChild, error) {
	if binary == "" {
		var err error
		binary, err = exec.LookPath(target.DefaultBinary())
		if err != nil {
			return nil, fmt.Errorf("%s not found in PATH (use --binary to specify): %w", target.DefaultBinary(), err)
		}
	}

	tmpFile, err := os.CreateTemp("", "ze-chaos-*.conf")
	if err != nil {
		return nil, fmt.Errorf("creating temp config: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(config); err != nil {
		if rmErr := os.Remove(tmpPath); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: removing temp config: %v\n", rmErr)
		}
		return nil, fmt.Errorf("writing config to %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		if rmErr := os.Remove(tmpPath); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: removing temp config: %v\n", rmErr)
		}
		return nil, fmt.Errorf("closing temp config: %w", err)
	}

	var args []string
	switch target {
	case scenario.TargetFRR:
		args = []string{
			"-f", tmpPath,
			"-p", textbuf.StringInt(int64(port)),
			"-l", localAddr,
			"-P", "0",
		}
	case scenario.TargetBIRD:
		var tb textbuf.Buffer
		ctlPath := tb.Str(tmpPath).Str(".ctl").String()
		args = []string{
			"-f",
			"-c", tmpPath,
			"-s", ctlPath,
		}
	default:
		if rmErr := os.Remove(tmpPath); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: removing temp config: %v\n", rmErr)
		}
		return nil, fmt.Errorf("ForkDaemon does not support target %q", target)
	}

	cmd := exec.CommandContext(ctx, binary, args...) // #nosec G204 G702 - binary from --binary flag or PATH
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		if rmErr := os.Remove(tmpPath); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: removing temp config: %v\n", rmErr)
		}
		return nil, fmt.Errorf("starting %s: %w", target, err)
	}

	child := &ZeChild{cmd: cmd, done: make(chan struct{}), tmpFile: tmpPath}
	go func() {
		child.waitErr = cmd.Wait()
		close(child.done)
	}()

	return child, nil
}
