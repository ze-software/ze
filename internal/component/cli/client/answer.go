// Design: docs/architecture/api/ipc_protocol.md — the answer grammar
// Overview: main.go — cliClient.Execute, which writes through this
//
// answer.go holds what `ze cli -c` does with the daemon's answer while it is
// still arriving.
//
// The daemon renders and streams. This client therefore prints what it reads,
// as it reads it, and holds no copy of the answer: that is the last hop of the
// memory the protocol exists to bound, and collecting here would spend it again
// on the operator's machine.
//
// What the operator sees is unchanged. A rendering has its surrounding
// whitespace trimmed and ends in exactly one newline, and a command that
// reported nothing prints OK unless the chain named a format. daemonOutput does
// both while streaming, which is what the collected form did in one pass.

package client

import (
	"io"

	cmd "github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// okAnswerLine is what a command reporting no data prints, so silence never
// reads as a failure.
const okAnswerLine = "OK\n"

// answerNewline ends a rendering. The daemon's formatters may or may not end
// with one, so the answer carries exactly the one written here.
const answerNewline = "\n"

// daemonOutput writes the daemon's rendering to an operator's terminal, and
// gives that rendering the shape it has always had.
//
// Leading whitespace is dropped, a run of trailing whitespace is held until
// something follows it, and the answer ends in exactly one newline. Held
// whitespace at the end of the stream is therefore never written, which is what
// makes this equal to trimming the collected answer.
//
// The trim is over ASCII whitespace. A rendering is produced by ze's own
// formatters, which indent with spaces and end lines with a newline, so no
// other space character is ever at an edge of one.
//
// A caller MUST call Close exactly once, after the answer ends. Close is what
// writes the final newline and the OK.
type daemonOutput struct {
	w       io.Writer
	command string
	kept    bool

	// pending is the run of whitespace read but not yet written. It is written
	// when a non-whitespace byte follows it and dropped when the stream ends.
	pending []byte

	// transcript is the copy a session recording keeps, and it is nil for
	// every other caller. A transcript is a record of the whole answer, so the
	// copy is what that feature costs rather than an accident of this one.
	transcript *textbuf.Buffer
}

// newDaemonOutput returns the writer for one command's answer. command is the
// operator's text, which decides whether an empty answer prints OK.
func newDaemonOutput(w io.Writer, command string, transcript *textbuf.Buffer) *daemonOutput {
	return &daemonOutput{w: w, command: command, transcript: transcript}
}

// Write writes the part of the answer that is not whitespace at an edge of it.
func (d *daemonOutput) Write(p []byte) (int, error) {
	cut := len(p)
	for cut > 0 && isASCIISpace(p[cut-1]) {
		cut--
	}

	body := p[:cut]
	if !d.kept {
		body = trimLeadingSpace(body)
	}
	if len(body) > 0 {
		if err := d.emit(d.pending); err != nil {
			return 0, err
		}
		d.pending = d.pending[:0]
		if err := d.emit(body); err != nil {
			return 0, err
		}
		d.kept = true
	}
	d.pending = append(d.pending, p[cut:]...)
	return len(p), nil
}

// Close ends the answer: one newline after a rendering, or OK when the command
// reported nothing and named no format operator.
//
// A command that names a format gets nothing at all, because OK is not valid
// JSON and a caller that asked for JSON is parsing what it receives.
func (d *daemonOutput) Close() error {
	if d.kept {
		return d.emit([]byte(answerNewline))
	}
	if cmd.HasFormatPipe(d.command) {
		return nil
	}
	return d.emit([]byte(okAnswerLine))
}

// Transcript is the answer as the operator saw it, for a session recording.
// It is empty for a client that keeps none.
func (d *daemonOutput) Transcript() string {
	if d.transcript == nil {
		return ""
	}
	return d.transcript.String()
}

// emit writes one piece to the terminal, and to the transcript when one is
// kept.
func (d *daemonOutput) emit(piece []byte) error {
	if len(piece) == 0 {
		return nil
	}
	if d.transcript != nil {
		d.transcript.Write(piece) //nolint:errcheck // textbuf.Write never fails
	}
	_, err := d.w.Write(piece)
	return err
}

// trimLeadingSpace drops the whitespace in front of the first byte of the
// answer.
func trimLeadingSpace(p []byte) []byte {
	start := 0
	for start < len(p) && isASCIISpace(p[start]) {
		start++
	}
	return p[start:]
}

// isASCIISpace reports the six characters strings.TrimSpace removes from an
// ASCII rendering.
func isASCIISpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}
