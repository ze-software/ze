// Design: docs/architecture/chaos-web-dashboard.md — fork ze as child process with stdin pipe
// Overview: main.go — CLI entry, mode selection (fork/pipe/config-only)

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

type zeChild struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	done    chan struct{}
	waitErr error
}

func forkZe(ctx context.Context, config, binary string) (*zeChild, error) {
	if binary == "" {
		var err error
		binary, err = exec.LookPath("ze")
		if err != nil {
			return nil, fmt.Errorf("ze not found in PATH (use --ze to specify): %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, binary, "-") // #nosec G204 - binary from --ze flag or PATH
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

	child := &zeChild{cmd: cmd, stdin: stdin, done: make(chan struct{})}
	go func() {
		child.waitErr = cmd.Wait()
		close(child.done)
	}()

	return child, nil
}

func (z *zeChild) PID() int {
	return z.cmd.Process.Pid
}

func (z *zeChild) Signal(sig os.Signal) {
	if err := z.cmd.Process.Signal(sig); err != nil {
		fmt.Fprintf(os.Stderr, "warning: signaling ze: %v\n", err)
	}
}

func (z *zeChild) Shutdown() {
	if err := z.stdin.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: closing ze stdin: %v\n", err)
	}
	select {
	case <-z.done:
		if z.waitErr != nil {
			fmt.Fprintf(os.Stderr, "warning: ze exited: %v\n", z.waitErr)
		}
	case <-time.After(5 * time.Second):
		fmt.Fprintf(os.Stderr, "warning: ze did not exit within 5s, killing\n")
		if err := z.cmd.Process.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: killing ze: %v\n", err)
		}
		<-z.done
	}
}
