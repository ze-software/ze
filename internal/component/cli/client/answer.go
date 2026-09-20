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

// headBytes is how much of the answer is held before its destination is known.
//
// The daemon's pipe layer marks a refusal by writing cmd.PipeErrorPrefix in
// front of it, so the first bytes of the answer say whether the stream is data
// or a diagnostic. Holding exactly that many bytes is what lets a streamed
// refusal reach stderr, the way every other surface sends one (runPipe in
// cmd/ze/ze_core_pipe.go, emitLocalResult in main.go).
const headBytes = len(cmd.PipeErrorPrefix)

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
// An answer the daemon refused is a diagnostic about the operator's own pipe
// chain rather than an answer to their question, so it goes to errw and the
// client exits non-zero. The refusal is only knowable from the first bytes of
// the stream, so the first headBytes of the answer are held until the
// destination is known: before that, a script redirecting stdout collected the
// refusal as data and read exit 0 (plan/journal/silent-fall-through.md,
// 2026-09-05).
//
// A caller MUST call Close exactly once, after the answer ends. Close is what
// writes the final newline and the OK.
type daemonOutput struct {
	w       io.Writer
	errw    io.Writer
	command string
	kept    bool

	// head is the start of the answer, held until sinkKnown. refused is what
	// those bytes turned out to say, and it decides which writer the rest of
	// the answer reaches.
	head      []byte
	sinkKnown bool
	refused   bool

	// pending is the run of whitespace read but not yet written. It is written
	// when a non-whitespace byte follows it and dropped when the stream ends.
	pending []byte

	// transcript is the copy a session recording keeps, and it is nil for
	// every other caller. A transcript is a record of the whole answer, so the
	// copy is what that feature costs rather than an accident of this one.
	transcript *textbuf.Buffer
}

// newDaemonOutput returns the writer for one command's answer. command is the
// operator's text, which decides whether an empty answer prints OK. w takes the
// answer and errw takes a refusal, which is the stdout/stderr split every
// command owes (ai/rules/cli.md).
func newDaemonOutput(w, errw io.Writer, command string, transcript *textbuf.Buffer) *daemonOutput {
	return &daemonOutput{w: w, errw: errw, command: command, transcript: transcript}
}

// Refused reports whether the daemon answered with a pipe refusal rather than
// data, so the caller exits non-zero. It is valid after Close.
func (d *daemonOutput) Refused() bool {
	return d.refused
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
	if err := d.end(); err != nil {
		return err
	}
	// An answer shorter than headBytes is still held at this point, and this
	// is the last chance to write it.
	return d.flushHead()
}

// end writes what ends the answer, before the held head is flushed.
func (d *daemonOutput) end() error {
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
//
// The transcript is written first and never waits for the destination: a
// session recording holds the answer whatever it turns out to be, and a caller
// reads it before Close.
func (d *daemonOutput) emit(piece []byte) error {
	if len(piece) == 0 {
		return nil
	}
	if d.transcript != nil {
		d.transcript.Write(piece) //nolint:errcheck // textbuf.Write never fails
	}
	if !d.sinkKnown {
		d.head = append(d.head, piece...)
		if len(d.head) < headBytes {
			return nil
		}
		return d.flushHead()
	}
	_, err := d.sink().Write(piece)
	return err
}

// flushHead reads what the held bytes say the answer is, then writes them to
// the writer that reading chose. It does nothing once the destination is known.
func (d *daemonOutput) flushHead() error {
	if d.sinkKnown {
		return nil
	}
	d.sinkKnown = true
	// Only the prefix decides, so only the prefix is copied: one Write can
	// deliver a whole table, and the reading must not copy it to answer a
	// question about its first bytes. A head shorter than the prefix cannot be
	// a refusal, and IsPipeError answers false for it.
	lead := d.head
	if len(lead) > headBytes {
		lead = lead[:headBytes]
	}
	d.refused = cmd.IsPipeError(string(lead))
	head := d.head
	d.head = nil
	if len(head) == 0 {
		return nil
	}
	_, err := d.sink().Write(head)
	return err
}

// sink is where the rest of the answer goes: the operator's terminal for data,
// stderr for a refusal.
func (d *daemonOutput) sink() io.Writer {
	if d.refused {
		return d.errw
	}
	return d.w
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
