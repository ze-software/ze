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

package client

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Answer is what one exec-channel answer turned out to be.
//
// Verdict is one of the rpc.Verdict* values, derived from the terminator rather
// than stated by it. Count and Faults are the records the daemon wrote and the
// rows it rejected. Message is the operational text an aborted or failed
// command carries.
type Answer struct {
	Verdict string
	Count   uint64
	Faults  uint64
	Message string
}

// ErrAnswerTruncated is returned when the answer ended without its terminator.
// The bytes already written to the caller's writer are real; what is missing is
// the rest of them, so a caller MUST NOT treat what it received as complete.
var ErrAnswerTruncated = errors.New("answer ended before its terminator")

// ExecCommandStream runs a command over the SSH exec channel and copies the
// daemon's rendering to body as it arrives.
//
// The daemon frames every exec-channel answer on stderr, so nothing is declared
// and no environment is set up. A peer that writes no frame leaves the answer
// reading as truncated, which is the correct verdict for one that cannot say
// when it has finished.
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
			continue
		}
		kind, tail, err := rpc.ParseAnswerLine(line)
		if err != nil {
			text.Str(strings.TrimSpace(string(line)))
			text.Byte('\n')
			continue
		}
		if kind == rpc.AnswerKindNotUnderstood {
			// The command named no command. It is the whole answer, so there
			// is no terminator to wait for and none to miss.
			answer.Verdict = rpc.VerdictError
			answer.Message = tail.Message
			continue
		}
		if kind != rpc.AnswerKindTerminator {
			continue
		}
		answer.Count = tail.Count
		answer.Faults = tail.Faults
		answer.Message = tail.Message
		answer.Verdict = rpc.Verdict(&tail)
	}
	return answer, strings.TrimSpace(text.String())
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
	hkCb, err := hostKeyCallback(creds.Host)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: creds.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(creds.Auth),
		},
		HostKeyCallback: hkCb,
		Timeout:         dialTimeout,
	}

	var tb textbuf.Buffer
	addr := tb.Str(creds.Host).Byte(':').Str(creds.Port).String()
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	return client, nil
}
