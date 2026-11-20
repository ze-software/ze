// Design: docs/architecture/api/commands.md -- a piped line on the SSH exec route
// Related: register_ssh_exec_line.go -- the registration of this driver
// Related: monitor_remote_as_stream.go -- the dynamic BGP session this driver brings up
// Related: plugin_shell_extra_3.go -- the password SSH client it follows

package fixture

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// sshExecLineUser and sshExecLinePassword are the login the .ci files of
// this driver configure (the TACACS+ mock's admin, or a local user).
const (
	sshExecLineUser     = "admin"
	sshExecLinePassword = "testpass"
	// sshExecLineEventsMax bounds the lines a stream run reads before the
	// wanted one, and the depth of the channel they cross; a stream that
	// prints more is a failure of its own.
	sshExecLineEventsMax = 256
)

// sshExecLineMode is what the driver waits for after it sent the line.
type sshExecLineMode uint8

const (
	sshExecLineModeUnspecified sshExecLineMode = iota
	// sshExecLineModeStream: a monitor. The first output line is the
	// subscription barrier, then the driver brings a dynamic BGP session up
	// and waits for a line holding want.
	sshExecLineModeStream
	// sshExecLineModeOnce: a one-shot line, answered and ended.
	sshExecLineModeOnce
	// sshExecLineModeRefused: a line the daemon refuses with a non-zero exit.
	sshExecLineModeRefused
)

// sshExecLine sends one line on an SSH exec channel, the way any SSH client
// does, with the session environment keys the .ci names, and judges what
// comes back. `ze cli -c` cannot stand in for it: its client splits the chain
// before it sends (StreamMonitor) and sets its own keys.
//
// Arguments: <stream|once|refused> <ssh-port> <bgp-port or -> <line> <want>
// <reject or -> [KEY=VALUE ...]. want is a substring of a stdout line
// (stream, once) or of stderr (refused); reject is a substring no stdout line
// may hold. Every line the daemon printed is echoed to stderr as
// "ssh-exec: <line>" so a failing run shows what arrived.
func sshExecLine(ctx context.Context, args []string) error {
	if len(args) < 6 {
		return errors.New("ssh-exec-line requires <mode> <ssh-port> <bgp-port> <line> <want> <reject> [KEY=VALUE ...]")
	}
	mode, err := sshExecLineModeFrom(args[0])
	if err != nil {
		return err
	}
	line, want, reject := args[3], args[4], args[5]
	if reject == "-" {
		reject = ""
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(addrLoopback, args[1]), &ssh.ClientConfig{
		User:            sshExecLineUser,
		Auth:            []ssh.AuthMethod{ssh.Password(sshExecLinePassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // isolated loopback fixture
		Timeout:         fixture10ProcessTimeout,
	})
	if err != nil {
		return fmt.Errorf("connect SSH on port %s: %w", args[1], err)
	}
	defer client.Close() //nolint:errcheck // fixture teardown follows the observed result
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("open the SSH session: %w", err)
	}
	defer session.Close() //nolint:errcheck // fixture teardown follows the observed result
	for _, pair := range args[6:] {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			return fmt.Errorf("session environment %q is not KEY=VALUE", pair)
		}
		if err := session.Setenv(key, value); err != nil {
			return fmt.Errorf("set session environment %s: %w", key, err)
		}
	}
	switch mode {
	case sshExecLineModeStream:
		return sshExecLineStream(ctx, session, line, args[2], want, reject)
	case sshExecLineModeOnce, sshExecLineModeRefused:
		return sshExecLineOnce(session, mode, line, want, reject)
	case sshExecLineModeUnspecified:
		panic("BUG: ssh-exec-line mode unspecified after sshExecLineModeFrom")
	default:
		panic("BUG: ssh-exec-line mode of an unknown value")
	}
}

func sshExecLineModeFrom(word string) (sshExecLineMode, error) {
	switch word {
	case "stream":
		return sshExecLineModeStream, nil
	case "once":
		return sshExecLineModeOnce, nil
	case "refused":
		return sshExecLineModeRefused, nil
	}
	return sshExecLineModeUnspecified, fmt.Errorf("ssh-exec-line mode %q is none of stream, once, refused", word)
}

// sshExecLineEcho writes one line the daemon printed to stderr, marked.
func sshExecLineEcho(printed string) {
	var tb textbuf.Buffer
	tb.Str("ssh-exec: ").Str(printed).Byte('\n').StdErr() //nolint:errcheck // stderr is the log itself
}

// sshExecLineOK writes the verdict line: what was sent and what it showed.
// It carries no quote of its own, so a .ci reads it with a plain contains=.
func sshExecLineOK(line, verdict string) {
	var tb textbuf.Buffer
	tb.Str("OK: ").Str(line).Byte(' ').Str(verdict).Byte('\n').StdErr() //nolint:errcheck // stderr is the log itself
}

// sshExecLineOnce runs a line that ends by itself and judges its exit, its
// stdout and its stderr.
func sshExecLineOnce(session *ssh.Session, mode sshExecLineMode, line, want, reject string) error {
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	runErr := session.Run(line)
	for printed := range strings.SplitSeq(strings.TrimRight(stdout.String()+stderr.String(), "\n"), "\n") {
		sshExecLineEcho(printed)
	}
	if reject != "" && strings.Contains(stdout.String(), reject) {
		return fmt.Errorf("%q answered a line holding %q: %q", line, reject, stdout.String())
	}
	if mode == sshExecLineModeRefused {
		if runErr == nil {
			return fmt.Errorf("%q was not refused: it exited with status 0", line)
		}
		if _, exited := errors.AsType[*ssh.ExitError](runErr); !exited {
			return fmt.Errorf("%q was not refused with an exit status: %w", line, runErr)
		}
		if !strings.Contains(stderr.String(), want) {
			return fmt.Errorf("%q was refused without %q: %q", line, want, stderr.String())
		}
		sshExecLineOK(line, "refused, naming "+want)
		return nil
	}
	if runErr != nil {
		return fmt.Errorf("%q failed: %w: %q", line, runErr, stderr.String())
	}
	if !strings.Contains(stdout.String(), want) {
		return fmt.Errorf("%q answered without %q: %q", line, want, stdout.String())
	}
	sshExecLineOK(line, "answered "+want)
	return nil
}

// sshExecLineStream starts a monitor line, waits for its first output line,
// brings the dynamic BGP session on bgpPort up, and waits for a line holding
// want. No line before it may hold reject. The session is closed once want
// arrived, which ends the monitor.
func sshExecLineStream(ctx context.Context, session *ssh.Session, line, bgpPort, want, reject string) error {
	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	session.Stderr = &stderr
	if err := session.Start(line); err != nil {
		return fmt.Errorf("start %q: %w", line, err)
	}
	lines := make(chan string, sshExecLineEventsMax)
	go sshExecLineRead(stdout, lines) // goroutine-lifecycle: one reader per session, ended by the session close below
	defer func() {
		session.Close() //nolint:errcheck // ending the monitor is the point
		for range lines {
		}
	}()
	// The first line is the monitor's header, written once it subscribed. It
	// names the filters the line typed, so reject is not judged on it.
	if _, err := sshExecLineNext(ctx, lines, &stderr); err != nil {
		return fmt.Errorf("%q printed nothing to subscribe on: %w", line, err)
	}
	conn, err := sshExecLinePeerUp(ctx, bgpPort)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // the stream assertions carry the result
	for range sshExecLineEventsMax {
		event, err := sshExecLineNext(ctx, lines, &stderr)
		if err != nil {
			return fmt.Errorf("%q never printed a line holding %q: %w", line, want, err)
		}
		if reject != "" && strings.Contains(event, reject) {
			return fmt.Errorf("%q printed %q, which holds %q", line, event, reject)
		}
		if strings.Contains(event, want) {
			sshExecLineOK(line, "printed "+want+" and no line holding "+reject)
			return nil
		}
	}
	return fmt.Errorf("%q printed %d lines and none held %q", line, sshExecLineEventsMax, want)
}

// sshExecLineRead moves each stdout line onto lines and closes lines when
// the session closes its stdout. It is the session's one reader: its caller
// MUST drain lines until it is closed, so the reader never stays parked on a
// send to the bounded channel.
func sshExecLineRead(stdout io.Reader, lines chan<- string) {
	defer close(lines)
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
}

// sshExecLineNext answers the next stdout line, echoing it to stderr.
func sshExecLineNext(ctx context.Context, lines <-chan string, stderr *bytes.Buffer) (string, error) {
	timer := time.NewTimer(fixture10ProcessTimeout)
	defer timer.Stop()
	select {
	case event, open := <-lines:
		if !open {
			return "", fmt.Errorf("the stream ended; stderr %q", stderr.String())
		}
		sshExecLineEcho(event)
		return event, nil
	case <-timer.C:
		return "", fmt.Errorf("no line within %s", fixture10ProcessTimeout)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// sshExecLinePeerUp dials the daemon's dynamic BGP listener and completes the
// handshake as AS65001, so the daemon emits OPEN and state events. The caller
// MUST close the answered connection.
func sshExecLinePeerUp(ctx context.Context, bgpPort string) (net.Conn, error) {
	if _, err := strconv.Atoi(bgpPort); err != nil {
		return nil, fmt.Errorf("stream mode needs the BGP port, got %q", bgpPort)
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(addrLoopback, bgpPort))
	if err != nil {
		return nil, fmt.Errorf("dial the dynamic peer listener: %w", err)
	}
	if err := sshExecLineHandshake(conn); err != nil {
		conn.Close() //nolint:errcheck // the handshake error is the result
		return nil, err
	}
	return conn, nil
}

func sshExecLineHandshake(conn net.Conn) error {
	if err := conn.SetDeadline(time.Now().Add(fixture10ProcessTimeout)); err != nil {
		return err
	}
	if err := delayOpenFixtureRead(conn, 1); err != nil {
		return fmt.Errorf("the daemon's OPEN: %w", err)
	}
	open := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x02020202}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
		return err
	}
	if err := delayOpenFixtureRead(conn, 4); err != nil {
		return fmt.Errorf("the daemon's handshake KEEPALIVE: %w", err)
	}
	_, err := conn.Write(message.PackTo(message.NewKeepalive(), nil))
	return err
}
