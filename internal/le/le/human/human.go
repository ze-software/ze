// Design: docs/contributing/running-commands.md -- the terminal gate a whole-tree verify verb asks first
//
// Package human answers one question at the entry of a whole-tree run: did a
// person at an interactive terminal start this process?
//
// A full verification loads the whole shared machine for its length, and only
// Thomas can judge when that is safe: several sessions run at once nearly all
// the time, so no check at start can say anything about the hour that follows.
// Who started the run is a fact the process can read at start. An agent's tool
// call reaches le with its stdin and stdout as pipes, and Thomas's shell reaches
// it with a terminal on both, so a whole-tree verb refuses unless both streams
// are terminals.
//
// The gate stops a tool call, not a determined wrapper: a program that fakes a
// terminal passes it. No keyword, flag or environment variable skips it, so no
// surface can teach a bypass.
package human

import (
	"errors"
	"os"

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ScopedCommand is the verification an agent runs for its own change, and the
// line every refusal gives it.
const ScopedCommand = "./le verify changed"

// Question answers whether one stream is an interactive terminal. An error
// means the question had no answer, which the gate refuses on: an unanswered
// question is never read as a terminal.
type Question func(stream *os.File) (bool, error)

// IsTerminal is the Question asked of the real process streams.
func IsTerminal(stream *os.File) (bool, error) {
	if stream == nil {
		return false, errors.New("the stream is not open")
	}
	fd := stream.Fd()
	// os.File answers ^uintptr(0) for a file that has been closed.
	if fd == ^uintptr(0) {
		return false, errors.New("the stream has been closed")
	}
	return term.IsTerminal(int(fd)), nil
}

// RequireTerminal refuses unless this process's stdin and stdout are both
// interactive terminals. commandLine is the exact command line the caller was
// given; the refusal prints it for the agent to hand to Thomas, and never runs
// it. A caller MUST call it before it admits a job, takes a slot, or creates a
// worktree or a container, and MUST exit 2 on a non-nil answer.
func RequireTerminal(commandLine string) error {
	return Check(IsTerminal, os.Stdin, os.Stdout, commandLine)
}

// Check is RequireTerminal over an injected question and streams, so a test can
// answer for a terminal it does not have.
func Check(question Question, stdin, stdout *os.File, commandLine string) error {
	stdinTerminal, stdinErr := question(stdin)
	if stdinErr != nil {
		return refusal(unanswered("stdin", stdinErr), commandLine)
	}
	stdoutTerminal, stdoutErr := question(stdout)
	if stdoutErr != nil {
		return refusal(unanswered("stdout", stdoutErr), commandLine)
	}
	if stdinTerminal && stdoutTerminal {
		return nil
	}
	if stdinTerminal {
		return refusal("stdout is not a terminal", commandLine)
	}
	if stdoutTerminal {
		return refusal("stdin is not a terminal", commandLine)
	}
	return refusal("neither stdin nor stdout is a terminal", commandLine)
}

// unanswered names a stream whose terminal question failed, and why.
func unanswered(stream string, cause error) string {
	var text textbuf.Buffer
	text.Str("whether ").Str(stream).Str(" is a terminal could not be answered (").Str(cause.Error()).Str(")")
	return text.String()
}

// refusal builds the error a whole-tree verb prints: what is wrong, why it
// matters, and the two command lines, each on its own line.
func refusal(condition, commandLine string) error {
	var text textbuf.Buffer
	text.Str("this is a full verification, a release check only Thomas starts, and this process has no interactive terminal: ").
		Str(condition).Byte('\n').
		Str("  a full verification loads the whole shared machine for its length, and only Thomas knows when that is safe\n").
		Str("  give Thomas this command to run from his own terminal:\n").
		Str("    ").Str(commandLine).Byte('\n').
		Str("  for the evidence your own change owes, run the scoped verification:\n").
		Str("    ").Str(ScopedCommand)
	return &Refusal{message: text.String()}
}

// Refusal is the error a whole-tree verb answers when no person at a terminal
// started it. errors.As finds it, so a caller can tell the refusal from a run
// that failed.
type Refusal struct {
	message string
}

func (r *Refusal) Error() string { return r.message }
