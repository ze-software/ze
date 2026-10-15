// Design: docs/architecture/web-interface.md — the standalone web server's two-step Ctrl+C
// Related: register_web_interrupt.go — the registration of this driver

package fixture

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// webInterruptDialTimeout bounds one dial of the web port.
	webInterruptDialTimeout = 500 * time.Millisecond
	// webInterruptExchangeTimeout bounds the TLS handshake and the wait for
	// "100 Continue" on the held connection.
	webInterruptExchangeTimeout = 5 * time.Second
	// webInterruptBodyOctets is the request body the held connection declares
	// and never sends.
	webInterruptBodyOctets = 64
	// webInterruptExitMin and webInterruptExitMax bound how long after the
	// first SIGINT a graceful stop may take. runWebOnly starts its 3 s
	// Shutdown deadline only once it has received the signal, so a stop that
	// waits for the held connection cannot end sooner than 3 s after the kill:
	// any minimum below 3 s cannot fail a correct daemon, and 2.5 s still
	// refuses a Shutdown that ignores the connection (it ends in milliseconds).
	// The held connection never finishes, so a Shutdown with no deadline
	// never ends: the 10 s maximum leaves 7 s for a loaded machine and still
	// refuses that hang.
	webInterruptExitMin = 2500 * time.Millisecond
	webInterruptExitMax = 10 * time.Second
	// webInterruptExitPoll is the interval between two liveness checks of the
	// daemon, and so the resolution of the measured stop time.
	webInterruptExitPoll = 10 * time.Millisecond
)

// webInterrupt plays an operator pressing Ctrl+C at `ze start --web-only`,
// once or twice: `le test fixture web/interrupt <port> <count>`.
//
// The first SIGINT starts a graceful shutdown that waits up to 3 s for open
// connections (runWebOnly, cmd/ze/hub/service_web.go). The driver holds one
// connection inside a request handler from before that signal until the
// daemon has exited (webInterruptHold says how), so the shutdown has a
// connection to wait for.
//
// With one SIGINT the driver times the stop: the daemon must exit after the
// shutdown deadline, and not long after it (webInterruptExitMin and
// webInterruptExitMax). With two, the shutdown is still waiting when the
// second SIGINT lands, so the process has to leave through forceExitOnSignal,
// not through the shutdown deadline.
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

	sent := time.Now()
	if err := syscall.Kill(pid, syscall.SIGINT); err != nil {
		return fmt.Errorf("first SIGINT to %d: %w", pid, err)
	}
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool { return webInterruptRefused(ctx, address) }) {
		return webInterruptNeverClosed(pid, address, sent, ctx.Err())
	}
	fmt.Fprintln(os.Stderr, "web-interrupt: listener closed after the first SIGINT")

	if count == 1 {
		return webInterruptAwaitExit(ctx, pid, sent)
	}

	// The kernel may hand a dead daemon's pid to another process, so a pid is
	// signaled only while the daemon still holds it.
	gone, err := webInterruptGone(pid)
	if err != nil {
		return err
	}
	if gone {
		return fmt.Errorf("the daemon %d exited after the first SIGINT, before the second: "+
			"the shutdown did not wait for the held connection, so there was nothing left to force", pid)
	}
	if err := syscall.Kill(pid, syscall.SIGINT); err != nil {
		return fmt.Errorf("second SIGINT to %d: %w", pid, err)
	}
	fmt.Fprintln(os.Stderr, "web-interrupt: second SIGINT sent during the shutdown")

	// The held connection keeps the shutdown waiting, so it stays open until
	// the runner ends this driver. The runner does that once the daemon, the
	// test's last command, has exited: its exit code and output are the
	// test's verdict, so the driver waits rather than judging the exit itself.
	// Closing the connection earlier would let the shutdown finish and race
	// the forced exit.
	<-ctx.Done()
	return nil
}

// webInterruptHold opens the connection the shutdown has to wait for, and
// returns only once the web server is serving it inside a request handler.
//
// net/http (server.go, conn.serve and closeIdleConns) decides what a shutdown
// waits for. A plain TCP dial is not enough: it completes in the kernel's
// listen backlog before http.Server has accepted it, and a shutdown closes the
// listener, and the backlog with it, without ever tracking it. A connection
// that is accepted but has sent no request is StateNew, which Shutdown treats
// as idle once it is five seconds old; one that sends a request after the
// shutdown began is closed as soon as its headers are read. Only a connection
// whose handler is already running when the signal lands is waited for until
// the deadline, whatever the timing.
//
// So the driver completes a TLS handshake (proof that the server accepted the
// connection), sends a POST /cli that declares a body with
// "Expect: 100-continue", and waits for "100 Continue". net/http sends that
// line only when the handler reads the body, which the CLI handler does at
// once (r.ParseForm). The body never comes, so the handler stays blocked on it
// and the connection stays active until the daemon exits.
func webInterruptHold(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: webInterruptDialTimeout}
	var raw net.Conn
	var err error
	if !Poll(ctx, 300, 100*time.Millisecond, func() bool {
		raw, err = dialer.DialContext(ctx, "tcp", address)
		return err == nil
	}) {
		return nil, fmt.Errorf("the web port %s never accepted: %w", address, err)
	}

	held := tls.Client(raw, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}) //nolint:gosec // ze's self-signed web certificate; the driver needs a served connection, not the server's identity
	if err := webInterruptRequest(ctx, held, address); err != nil {
		held.Close() //nolint:errcheck // the hold failed; the error says why
		return nil, err
	}
	return held, nil
}

// webInterruptRequest runs the exchange webInterruptHold describes on held,
// within webInterruptExchangeTimeout, and clears the deadline once the
// handler is reading the body.
func webInterruptRequest(ctx context.Context, held *tls.Conn, address string) error {
	if err := held.SetDeadline(time.Now().Add(webInterruptExchangeTimeout)); err != nil {
		return fmt.Errorf("held connection to %s: deadline: %w", address, err)
	}
	if err := held.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("held connection to %s: TLS handshake: %w", address, err)
	}
	request := "POST /cli HTTP/1.1\r\n" +
		"Host: " + address + "\r\n" +
		"Origin: https://" + address + "\r\n" +
		"Content-Type: application/x-www-form-urlencoded\r\n" +
		"Content-Length: " + strconv.Itoa(webInterruptBodyOctets) + "\r\n" +
		"Expect: 100-continue\r\n" +
		"\r\n"
	if _, err := held.Write([]byte(request)); err != nil {
		return fmt.Errorf("held connection to %s: request: %w", address, err)
	}
	status, err := bufio.NewReader(held).ReadString('\n')
	if err != nil {
		return fmt.Errorf("held connection to %s: no answer to POST /cli: %w", address, err)
	}
	status = strings.TrimSpace(status)
	if status != "HTTP/1.1 100 Continue" {
		return fmt.Errorf("held connection to %s: POST /cli answered %q, not \"HTTP/1.1 100 Continue\": "+
			"the CLI handler no longer reads the body first, so this driver cannot hold a connection inside a handler", address, status)
	}
	if err := held.SetDeadline(time.Time{}); err != nil {
		return fmt.Errorf("held connection to %s: clear deadline: %w", address, err)
	}
	return nil
}

// webInterruptAwaitExit waits for the daemon to exit after the first SIGINT,
// sent at sent, and judges the time it took against webInterruptExitMin and
// webInterruptExitMax. It prints the verdict line the test asserts.
func webInterruptAwaitExit(ctx context.Context, pid int, sent time.Time) error {
	var gone bool
	var goneErr error
	attempts := int(webInterruptExitMax / webInterruptExitPoll)
	Poll(ctx, attempts, webInterruptExitPoll, func() bool {
		gone, goneErr = webInterruptGone(pid)
		return gone || goneErr != nil
	})
	if goneErr != nil {
		return goneErr
	}
	// The runner ends this driver the moment it has reaped the daemon, which
	// can fall between two polls, so an ended context gets one last look.
	if !gone {
		gone, goneErr = webInterruptGone(pid)
		if goneErr != nil {
			return goneErr
		}
	}
	elapsed := time.Since(sent).Round(time.Millisecond)
	if !gone {
		return fmt.Errorf("the daemon %d still runs %s after the first SIGINT: "+
			"the 3 s shutdown deadline did not end the wait for the held connection", pid, elapsed)
	}
	if elapsed < webInterruptExitMin {
		return fmt.Errorf("the daemon exited %s after the first SIGINT, before the 3 s shutdown deadline: "+
			"the shutdown stopped waiting for the held connection early", elapsed)
	}
	if elapsed > webInterruptExitMax {
		return fmt.Errorf("the daemon exited %s after the first SIGINT, later than %s: "+
			"the 3 s shutdown deadline did not end the wait for the held connection", elapsed, webInterruptExitMax)
	}
	fmt.Fprintln(os.Stderr, "web-interrupt: the daemon exited after the shutdown deadline ("+
		elapsed.String()+" after the first SIGINT, bounds "+webInterruptExitMin.String()+
		" to "+webInterruptExitMax.String()+")")
	return nil
}

// webInterruptNeverClosed explains why the driver never saw the listener
// refuse after the first SIGINT, sent at sent. The runner ends the driver the
// moment it has reaped the daemon, so a daemon that exits within one poll of
// closing its listener ends the poll too: that is a shutdown that did not wait
// for the held connection, and it is named as one.
func webInterruptNeverClosed(pid int, address string, sent time.Time, ctxErr error) error {
	gone, err := webInterruptGone(pid)
	if err != nil {
		return err
	}
	if gone {
		return fmt.Errorf("the daemon %d exited %s after the first SIGINT, before the driver saw its listener close: "+
			"the shutdown did not wait for the held connection", pid, time.Since(sent).Round(time.Millisecond))
	}
	return fmt.Errorf("the web listener on %s did not close after the first SIGINT (driver context: %w)", address, ctxErr)
}

// webInterruptGone answers whether the daemon has exited: signal 0 to its pid
// fails with ESRCH once the runner has reaped it. Any other failure is an
// error, never an answer.
func webInterruptGone(pid int) (bool, error) {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return true, nil
	}
	return false, fmt.Errorf("liveness check of the daemon %d: %w", pid, err)
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
