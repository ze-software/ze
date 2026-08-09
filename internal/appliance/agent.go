// Design: docs/architecture/appliance/builder.md -- passphrase agent (key-on-socket)

package appliance

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const DefaultAgentDuration = 30 * time.Minute

func RunAgent(key []byte, duration time.Duration) error {
	sockPath := AgentSocketPath()

	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old socket: %w", err)
	}

	// Set umask to restrict socket permissions before creation (avoids TOCTOU
	// window between Listen and Chmod where another user could connect).
	oldMask := syscall.Umask(0o177)
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "unix", sockPath)
	syscall.Umask(oldMask)
	if err != nil {
		return fmt.Errorf("listen %s: %w", sockPath, err)
	}
	defer listener.Close() //nolint:errcheck // cleanup

	fmt.Fprintf(os.Stderr, "agent: listening on %s (expires in %s, PID %d)\n", sockPath, duration, os.Getpid())

	done := make(chan struct{})

	go func() {
		timer := time.NewTimer(duration)
		defer timer.Stop()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		defer signal.Stop(sigCh)

		select {
		case <-timer.C:
			fmt.Fprintf(os.Stderr, "agent: expired after %s\n", duration)
		case sig := <-sigCh:
			fmt.Fprintf(os.Stderr, "agent: received %s\n", sig)
		case <-done:
			return
		}
		listener.Close() //nolint:errcheck // trigger Accept to return
	}()

	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			break
		}
		conn.Write(key) //nolint:errcheck // best-effort
		conn.Close()    //nolint:errcheck // best-effort
	}

	ZeroBytes(key)
	os.Remove(sockPath) //nolint:errcheck // best-effort
	close(done)
	fmt.Fprintf(os.Stderr, "agent: key zeroed, socket removed\n")
	return nil
}

// StopAgent removes the agent socket, causing the agent process to exit on its
// next Accept call. We cannot signal the agent by PID because the PID is not
// stored; removing the socket is sufficient since the agent's Accept loop will
// return an error and the agent will zero its key and exit.
func StopAgent() error {
	sockPath := AgentSocketPath()
	if _, err := os.Stat(sockPath); err != nil {
		return fmt.Errorf("agent not running (no socket at %s)", sockPath)
	}
	if err := os.Remove(sockPath); err != nil {
		return fmt.Errorf("remove socket: %w", err)
	}
	fmt.Fprintf(os.Stderr, "agent: socket removed (agent will exit on next operation)\n")
	return nil
}
