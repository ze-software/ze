// Design: plan/spec-install-0-umbrella.md — fork ze with stdin pipe (ze-chaos pattern)

package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func forkAndServe(config string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot find own binary: %v\n", err)
		return 1
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, self, "-") // #nosec G204 - self is our own binary
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdin, pipeErr := cmd.StdinPipe()
	if pipeErr != nil {
		fmt.Fprintf(os.Stderr, "error: creating stdin pipe: %v\n", pipeErr)
		return 1
	}

	if startErr := cmd.Start(); startErr != nil {
		fmt.Fprintf(os.Stderr, "error: starting ze: %v\n", startErr)
		return 1
	}

	// Write config + NUL sentinel (ze-chaos pattern).
	// NUL marks end of config so ze can start parsing immediately.
	// Pipe stays open: EOF signals shutdown.
	if _, writeErr := stdin.Write([]byte(config)); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: writing config to ze: %v\n", writeErr)
		killAndWait(cmd)
		return 1
	}
	if _, writeErr := stdin.Write([]byte{0}); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: writing config sentinel: %v\n", writeErr)
		killAndWait(cmd)
		return 1
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		// Forward signal to child; closing stdin also triggers ze shutdown.
		if closeErr := stdin.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: closing stdin pipe: %v\n", closeErr)
		}
		if sigErr := cmd.Process.Signal(sig); sigErr != nil {
			fmt.Fprintf(os.Stderr, "warning: forwarding signal: %v\n", sigErr)
		}
	}()

	if waitErr := cmd.Wait(); waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "error: ze exited: %v\n", waitErr)
		return 1
	}

	return 0
}

func killAndWait(cmd *exec.Cmd) {
	if killErr := cmd.Process.Kill(); killErr != nil {
		fmt.Fprintf(os.Stderr, "warning: killing ze: %v\n", killErr)
	}
	if waitErr := cmd.Wait(); waitErr != nil {
		fmt.Fprintf(os.Stderr, "warning: waiting for ze: %v\n", waitErr)
	}
}
