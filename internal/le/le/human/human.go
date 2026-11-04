// Design: docs/contributing/running-commands.md -- shared terminal gates
//
// Package human restricts whole-tree runs and issuance of owner authorization
// to processes whose stdin and stdout are both interactive terminals.
//
// A full verification loads the shared machine for its length; only Thomas
// judges when that is safe. Owner authorization records his instruction rather
// than an agent's. Both gates inspect the same process streams before work starts.
//
// The gate stops a tool call, not a determined wrapper: a program that fakes a
// terminal passes it. No keyword, flag or environment variable skips it, so no
// surface can teach a bypass.
package human

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// AgentEvidence is what an agent runs for its own change, and the line every
// refusal gives it: the scoped tests, never a verify. No agent runs any
// ./le verify command (ai/META-DECISIONS.md), and GitHub CI verifies a pushed
// commit.
const AgentEvidence = "the scoped tests of what your change reaches, run in the main checkout; no agent runs any ./le verify command"

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

// RequireAuthorization refuses owner authorization unless stdin and stdout are
// interactive terminals. root, the raw harness session when already known, and
// the action's actual arguments preserve the operation Thomas must run himself.
// Callers MUST check it before creating any session directory or artifact.
// A terminal is an operational restriction, not authenticated authorship.
func RequireAuthorization(root, session, action string, arguments []string) error {
	return check(IsTerminal, os.Stdin, os.Stdout, action, func(condition, action string) error {
		commandLine, err := authorizationCommand(root, session, action, arguments)
		if err != nil {
			return fmt.Errorf("owner authorization needs Thomas's terminal (%s), but its retry context cannot be read: %w; restore access to %s/tmp/session and correct any invalid session or agent identity, then run the original command from Thomas's terminal", condition, err, root)
		}
		return authorizationRefusal(condition, commandLine)
	})
}

// Check is RequireTerminal over an injected question and streams, so a test can
// answer for a terminal it does not have.
func Check(question Question, stdin, stdout *os.File, commandLine string) error {
	return check(question, stdin, stdout, commandLine, refusal)
}

// check shares detection while each issuer supplies the refusal for its purpose.
func check(question Question, stdin, stdout *os.File, commandLine string, refuse func(string, string) error) error {
	stdinTerminal, stdinErr := question(stdin)
	if stdinErr != nil {
		return refuse(unanswered("stdin", stdinErr), commandLine)
	}
	stdoutTerminal, stdoutErr := question(stdout)
	if stdoutErr != nil {
		return refuse(unanswered("stdout", stdoutErr), commandLine)
	}
	if stdinTerminal && stdoutTerminal {
		return nil
	}
	if stdinTerminal {
		return refuse("stdout is not a terminal", commandLine)
	}
	if stdoutTerminal {
		return refuse("stdin is not a terminal", commandLine)
	}
	return refuse("neither stdin nor stdout is a terminal", commandLine)
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
		Str("  for the evidence your own change owes, run:\n").
		Str("    ").Str(AgentEvidence)
	return &Refusal{message: text.String()}
}

// authorizationRefusal names the issuance boundary, never a verification task.
func authorizationRefusal(condition, commandLine string) error {
	var text textbuf.Buffer
	text.Str("owner authorization requires interactive stdin and stdout: ").Str(condition).Byte('\n').
		Str("  this command records the owner's instruction; an agent cannot issue it\n").
		Str("  give Thomas this command to run from his own terminal:\n").
		Str("    ").Str(commandLine)
	return &Refusal{message: text.String()}
}

// Refusal reports that a terminal-only action was not admitted. errors.As finds
// it, so callers can distinguish refusal from an action that ran and failed.
type Refusal struct {
	message string
}

func (r *Refusal) Error() string { return r.message }
