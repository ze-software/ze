// Design: docs/architecture/api/ipc_protocol.md — the answer grammar
// Overview: client.go — ExecCommand, the whole-answer sibling of this file
//
// answer.go reads an exec-channel answer as it arrives.
//
// ExecCommand collects the whole answer into one string, which is right for a
// caller that unmarshals it and wrong for the answer this protocol exists for:
// a command over a large table materializes in the client at the last hop, and
// every byte the daemon streamed is held again. This file reads the two streams
// instead. The rendering is copied to the caller's writer as the daemon
// produces it, and the frame on stderr says what the answer turned out to be.
//
// The frame is what makes a short answer detectable. A connection that dies
// part way delivers records and no terminator, and rpc.Verdict reads a missing
// terminator as truncated. Without it a client cannot tell a complete answer
// from the beginning of one.

package sshclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Answer is what one exec-channel answer turned out to be.
//
// Verdict is one of the rpc.Verdict* values, derived from the terminator rather
// than stated by it. Count and Faults are the records the daemon wrote and the
// rows it rejected. Message is the operational text an aborted or failed
// command carries. Warnings is the warning text the command wrote before its
// frame, including when faults or a failure outrank the warning in Verdict.
// The caller prints it on stderr as the local route does. NotUnderstood is
// true when the daemon answered that the command text
// names no command it holds (the nay answer); Verdict is then
// rpc.VerdictError, and Message is the daemon's text.
type Answer struct {
	Verdict       string
	Count         uint64
	Faults        uint64
	Message       string
	Warnings      string
	NotUnderstood bool
}

// ErrAnswerTruncated is returned when the answer ended without its terminator.
// The bytes already written to the caller's writer are real; what is missing is
// the rest of them, so a caller MUST NOT treat what it received as complete.
var ErrAnswerTruncated = errors.New("answer ended before its terminator")

// ExecCommandStream runs a command over the SSH exec channel and copies the
// daemon's rendering to body as it arrives.
//
// The client opts into stderr answer framing by setting EnvAnswerFrame to
// AnswerFrameEnabled before starting the command, so plain SSH sessions remain
// unframed while ze clients detect truncation and read structured completion
// counts. A peer that writes no frame leaves the answer reading as truncated,
// which is the correct verdict for one that cannot say when it has finished.
//
// The returned error is the command's failure, the connection's, or
// ErrAnswerTruncated. Bytes may already have reached body when it is non-nil:
// this is a stream, and what arrived arrived.
func ExecCommandStream(creds Credentials, command string, body io.Writer) (Answer, error) {
	client, err := dialDaemon(creds)
	if err != nil {
		return Answer{}, err
	}
	defer client.Close() //nolint:errcheck // best-effort cleanup

	session, err := client.NewSession()
	if err != nil {
		return Answer{}, fmt.Errorf("create session: %w", err)
	}
	defer session.Close() //nolint:errcheck // best-effort cleanup

	if err := session.Setenv(EnvAnswerFrame, AnswerFrameEnabled); err != nil {
		return Answer{}, fmt.Errorf("request answer framing: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return Answer{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return Answer{}, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := session.Start(command); err != nil {
		return Answer{}, fmt.Errorf("start command: %w", err)
	}

	// One goroutine for the life of one session, which is the shape
	// ai/rules/goroutine-lifecycle.md permits. It ends when stdout reaches EOF,
	// and stdout reaches EOF when the session ends, so it cannot outlive the
	// call below.
	copied := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(body, stdout)
		copied <- copyErr
	}()

	answer, text := readAnswerFrame(stderr)
	copyErr := <-copied
	waitErr := session.Wait()

	return answer, answerError(answer, text, copyErr, waitErr)
}

// readAnswerFrame reads the daemon's stderr to the end and returns what the
// frame said, along with any text that was not part of it.
//
// A line that does not parse as an answer line is the daemon talking to an
// operator rather than to this client: the exec channel writes `error: ...` for
// a failure it meets before the answer opens, and for one it meets while
// writing it. Keeping that text is what lets this function report the same
// message ExecCommand reports.
func readAnswerFrame(stderr io.Reader) (Answer, string) {
	answer := Answer{Verdict: rpc.VerdictTruncated}
	var text textbuf.Buffer
	var warning bool

	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, answerScanInitial), answerScanMax)
	// The frame states its own widths, so a line is taken by them and never by
	// searching for a newline. A counted value may hold a raw newline or a
	// carriage return, and bufio.ScanLines would split on the first and strip
	// the second (rpc.ScanAnswerLines).
	scanner.Split(rpc.ScanAnswerLines)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			text.Byte('\n')
			continue
		}
		kind, tail, err := rpc.ParseAnswerLine(line)
		if err != nil {
			text.Str(string(line))
			text.Byte('\n')
			continue
		}
		if kind == rpc.AnswerKindNotUnderstood {
			// The command named no command. It is the whole answer, so there
			// is no terminator to wait for and none to miss.
			answer.Verdict = rpc.VerdictError
			answer.Message = tail.Message
			answer.NotUnderstood = true
			continue
		}
		// The parsed kind, never the token: a warning terminator is spelled
		// wrn on the wire and still ends the answer (rpc.AnswerKindWarning).
		if tail.Kind != rpc.AnswerKindTerminator {
			continue
		}
		answer.Count = tail.Count
		answer.Faults = tail.Faults
		answer.Message = tail.Message
		answer.Verdict = rpc.Verdict(&tail)
		warning = tail.Warning
	}
	// Remove only the writer's final line ending before matching the framed
	// message. Its trailing spaces or newlines belong to the failure itself.
	return answerText(answer, strings.TrimSuffix(text.String(), "\n"), warning)
}

// answerText classifies only text whose role the completion proves. A warning
// terminator remains a warning even when rejected rows derive a partial verdict.
// With a failure, the server's exact final error line separates prior warnings;
// an unrelated diagnostic must remain failure evidence rather than gain a label.
func answerText(answer Answer, text string, warning bool) (Answer, string) {
	if warning {
		answer.Warnings = strings.TrimSpace(text)
		return answer, ""
	}
	if answer.Message == "" {
		return answer, strings.TrimSpace(text)
	}
	failure := errorPrefix + answer.Message
	if text == failure {
		return answer, text
	}
	suffix := "\n" + failure
	if prefix, found := strings.CutSuffix(text, suffix); found {
		answer.Warnings = strings.TrimSpace(prefix)
		return answer, failure
	}
	return answer, strings.TrimSpace(text)
}

// answerScanInitial and answerScanMax bound one frame line. A head naming every
// column of a wide table is the longest line the frame writes, and a line past
// the maximum is a peer this client will not read: bufio.Scanner stops, the
// terminator never arrives, and the answer reads as truncated rather than as
// complete.
const (
	answerScanInitial = 4 * 1024
	answerScanMax     = 1024 * 1024
)

// answerError turns what the streams reported into the one error a caller acts
// on, in the order a caller must learn things.
//
// A stated failure comes first, because the daemon knows why. A short answer
// comes next, and it outranks whatever the transport reports: a connection that
// dies mid-answer reports itself as a session that ended without an exit
// status, which says how the answer ended and not that it was incomplete. The
// text the daemon wrote for an operator wins over both, because it names the
// cause; ErrAnswerTruncated is what is left when nothing else was said.
func answerError(answer Answer, text string, copyErr, waitErr error) error {
	switch answer.Verdict {
	case rpc.VerdictError, rpc.VerdictAborted:
		return errors.New(answerMessage(answer, text))
	}
	if copyErr != nil {
		return fmt.Errorf("read answer: %w", copyErr)
	}
	if text != "" {
		return errors.New(trimErrorPrefix(text))
	}
	if answer.Verdict == rpc.VerdictTruncated {
		return ErrAnswerTruncated
	}
	return waitErr
}

// answerMessage is the text a failed answer reports: what the daemon stated on
// the frame, or what it wrote for an operator when it stated nothing.
func answerMessage(answer Answer, text string) string {
	if answer.Message != "" {
		return answer.Message
	}
	if text != "" {
		return trimErrorPrefix(text)
	}
	return "command failed"
}

// dialDaemon opens the SSH connection every helper in this package works over.
// The caller MUST close the returned client.
func dialDaemon(creds Credentials) (*ssh.Client, error) {
	return dialDaemonContext(context.Background(), creds)
}

// dialDaemonContext opens the SSH connection to the daemon bounded by ctx
// across TCP connect, SSH key exchange, and authentication. The caller MUST
// close the returned client.
//
// A failure after the client accepted the server's host key is a
// *LoginRefused: key exchange, and so the host key check, comes before the
// login, so a daemon that presented an accepted key answered, and only the
// login can have failed after it. A context cancellation or deadline timeout
// is checked before keyAccepted so a slow authentication exchange never reads
// as a refused login. Any earlier failure (nothing listening, a timeout, a
// refused host key) is the wrapped dial error, as before. The SSH library
// words a refused login as text alone, so the accepted key is the typed fact
// the two are told apart by.
func dialDaemonContext(ctx context.Context, creds Credentials) (*ssh.Client, error) {
	addr := net.JoinHostPort(creds.Host, creds.Port)
	hkCb, err := hostKeyCallback(addr)
	if err != nil {
		return nil, err
	}

	handshakeDeadline := time.Now().Add(dialTimeout)
	ctxBoundedHandshake := false
	if ctxDeadline, ok := ctx.Deadline(); ok {
		if !ctxDeadline.After(handshakeDeadline) {
			handshakeDeadline = ctxDeadline
			ctxBoundedHandshake = true
		}
	}

	dialCtx, cancelDial := context.WithDeadline(ctx, handshakeDeadline)
	defer cancelDial()

	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("connect to %s: %w", addr, ctxErr)
		}
		if ctxBoundedHandshake {
			if isTimeoutError(err) {
				return nil, fmt.Errorf("connect to %s: %w", addr, context.DeadlineExceeded)
			}
		}
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}

	if err := conn.SetDeadline(handshakeDeadline); err != nil {
		conn.Close() //nolint:errcheck // best-effort cleanup
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}

	// The library runs the host key check on its key-exchange goroutine, so
	// the flag it sets is read here only through the atomic.
	var keyAccepted atomic.Bool
	config := &ssh.ClientConfig{
		User: creds.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(creds.Auth),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if err := hkCb(hostname, remote, key); err != nil {
				return err
			}
			keyAccepted.Store(true)
			return nil
		},
		Timeout: dialTimeout,
	}

	stopClose := context.AfterFunc(ctx, func() {
		conn.Close() //nolint:errcheck,gosec // context ended; unblock handshake or auth
	})
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	stopped := stopClose()
	if err != nil {
		conn.Close() //nolint:errcheck // best-effort cleanup
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("connect to %s: %w", addr, ctxErr)
		}
		if isTimeoutError(err) {
			if ctxBoundedHandshake {
				return nil, fmt.Errorf("connect to %s: %w", addr, context.DeadlineExceeded)
			}
			return nil, fmt.Errorf("connect to %s: %w", addr, err)
		}
		if keyAccepted.Load() {
			return nil, &LoginRefused{Address: addr, User: creds.Username, Cause: err}
		}
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	if !stopped {
		c.Close() //nolint:errcheck // context ended right as handshake completed
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("connect to %s: %w", addr, ctxErr)
		}
		return nil, fmt.Errorf("connect to %s: %w", addr, context.Canceled)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		c.Close() //nolint:errcheck // best-effort cleanup
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// isTimeoutError reports whether err is a deadline or network timeout error.
func isTimeoutError(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	if netErr, ok := errors.AsType[net.Error](err); ok {
		return netErr.Timeout()
	}
	return false
}
