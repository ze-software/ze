// Design: docs/architecture/web-interface.md — the standalone web server's two-step Ctrl+C
// Related: register_web_interrupt.go — the registration of this driver

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"
)

// webInterruptDialTimeout bounds one dial of the web port.
const webInterruptDialTimeout = 500 * time.Millisecond

// webInterrupt plays an operator pressing Ctrl+C at `ze start --web-only`,
// once or twice: `le test fixture web/interrupt <port> <count>`.
//
// The first SIGINT starts a graceful shutdown that waits up to 3 s for open
// connections (runWebOnly, cmd/ze/hub/service_web.go). The driver holds one
// connection open from before that signal until the daemon has exited, so the
// shutdown is still waiting when a second SIGINT lands: the process then has
// to leave through forceExitOnSignal, not through the shutdown deadline.
//
// The second SIGINT is sent only once the web listener refuses a connection,
// which is the daemon's own proof that it consumed the first signal. Two
// SIGINTs sent back to back can merge into one pending signal in the kernel,
// and the test would then prove nothing about the second.
func webInterrupt(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("web/interrupt takes <port> <count>")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("web/interrupt port %q: %w", args[0], err)
	}
	count, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("web/interrupt count %q: %w", args[1], err)
	}
	if count < 1 {
		return fmt.Errorf("web/interrupt count %d: one or two interrupts", count)
	}
	if count > 2 {
		return fmt.Errorf("web/interrupt count %d: one or two interrupts", count)
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	var pid int
	if !Poll(ctx, 300, 100*time.Millisecond, func() bool {
		pid, err = readPID(fileDaemonPID)
		return err == nil
	}) {
		return fmt.Errorf("daemon.pid never appeared: %w", err)
	}
	if pid == os.Getpid() {
		return errors.New("daemon.pid holds the driver's own pid, not the ze daemon")
	}

	held, err := webInterruptHold(ctx, address)
	if err != nil {
		return err
	}
	defer held.Close() //nolint:errcheck // the daemon has exited or the test failed; nothing reads the close

	if err := syscall.Kill(pid, syscall.SIGINT); err != nil {
		return fmt.Errorf("first SIGINT to %d: %w", pid, err)
	}
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool { return webInterruptRefused(ctx, address) }) {
		return fmt.Errorf("the web listener on %s did not close after the first SIGINT (driver context: %v)", address, ctx.Err())
	}
	fmt.Fprintln(os.Stderr, "web-interrupt: listener closed after the first SIGINT")

	if count == 2 {
		if err := syscall.Kill(pid, syscall.SIGINT); err != nil {
			return fmt.Errorf("second SIGINT to %d: %w", pid, err)
		}
		fmt.Fprintln(os.Stderr, "web-interrupt: second SIGINT sent during the shutdown")
	}

	// The held connection keeps the shutdown waiting, so it stays open until
	// the runner ends this driver. The runner does that once the daemon, the
	// test's last command, has exited: its exit code and output are the
	// test's verdict, so the driver waits rather than judging the exit itself.
	// Closing the connection earlier would let the shutdown finish and race
	// the forced exit.
	<-ctx.Done()
	return nil
}

// webInterruptHold dials the web port until it accepts and answers that
// connection, left open and silent: a connection that has sent no request is
// one http.Server.Shutdown waits for.
func webInterruptHold(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: webInterruptDialTimeout}
	var held net.Conn
	var err error
	if !Poll(ctx, 300, 100*time.Millisecond, func() bool {
		held, err = dialer.DialContext(ctx, "tcp", address)
		return err == nil
	}) {
		return nil, fmt.Errorf("the web port %s never accepted: %w", address, err)
	}
	return held, nil
}

// webInterruptRefused answers true once a dial of address is refused, which is
// the listener closed by the shutdown. A dial that connects is closed at once.
// Any other failure (a timeout) is not a refusal and answers false.
func webInterruptRefused(ctx context.Context, address string) bool {
	dialer := net.Dialer{Timeout: webInterruptDialTimeout}
	probe, err := dialer.DialContext(ctx, "tcp", address)
	if err == nil {
		probe.Close() //nolint:errcheck // a probe; the next poll dials again
		return false
	}
	return errors.Is(err, syscall.ECONNREFUSED)
}
