// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashLossyPipe, the guard whose refusal this writes
// Related: parsedcommands.go -- everyCommand and the pipeline stages the tree judgment reads
package hookruntime

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/heavytool"
)

// lintCommand is the command word of the Go linter, which the cost judgment,
// the raw-job refusal and the piped-command refusal all name. The name is
// declared once, in the shared heavy-tool definition.
const lintCommand = heavytool.Linter

// goTestVerb is the go subcommand that runs tests, and goVetVerb the one
// that vets.
const (
	goTestVerb = "test"
	goVetVerb  = "vet"
)

// shortPackageForm is the short package action both refusals name for a
// package test: `./le test unit pkg`, which adds the feature tags and the
// race detector and admits the run.
const shortPackageForm = pkgActionCommand + " " + pkgPathKeyword + " <package>"

// verifyArea is the le area whose runs no agent starts. The piped-command
// refusal reads it only to choose its advice (verifyCommand): which of the
// area's lines are expensive is the area's own declaration (expensiveWords).
const verifyArea = "verify"

// lossyPipeRefusal is the piped-command refusal for the expensive segment.
// It shows the caller's own command in the quiet admitted form, which keeps
// the whole output in this session's scratch log and answers the exit code,
// the log's path and the failure lines. It never advises a redirect or a
// tee into a scratch path, which ai/rules/commands.md forbids. A
// verification command gets the scoped package run instead, because no agent
// runs one (ai/META-DECISIONS.md), so the refusal never names it. identity is
// the caller's identity line (commandIdentityPrefix), which the advised
// command starts with so that following it is not refused again. words are
// the refused command's words from its program on, with no redirection:
// read from the tree on a line that parses (treeLossyPipe), and from the
// text segment otherwise. written holds each of words as the refusal writes
// it back, one shell word each (parsedCommand.written, quotedWords), so a
// word the shell expands is advised as typed. MUST be called with written
// as long as words.
func lossyPipeRefusal(identity, filter string, words, written []string) *verdict {
	head := lossyPipeHead(filter)
	if len(words) == 0 {
		return &verdict{2, head +
			"  -- Run the command with no filter, redirect or tee, in the quiet admitted form (ai/rules/commands.md):\n" +
			"     ./le job run label <label> quiet command <argv...>\n" +
			"  -- Then read the log it names with the Read tool, offset and limit."}
	}
	if verifyCommand(words) {
		return &verdict{2, head +
			"  -- No agent runs a verification command: GitHub CI verifies a pushed commit (ai/META-DECISIONS.md).\n" +
			"  -- The evidence you owe is the scoped run of each package your change reaches, which runs quiet:\n" +
			"     " + shortPackageForm}
	}
	advice := head +
		"  -- Run it with no filter, redirect or tee, in the quiet admitted form (ai/rules/commands.md). Its output goes to this session's scratch log, and the answer carries the exit code, the log's path and the failure lines:\n" +
		"     " + identity + admittedCommand(lossyPipeLabel(words), written) + "\n"
	if goTest(words) {
		advice += "  -- For one package with its feature tags, the short form: " + shortPackageForm + "\n"
	}
	return &verdict{2, advice + "  -- Then read the log it names with the Read tool, offset and limit."}
}

// lossyPipeHead opens the piped-command refusal for every caller: what was
// refused, with the filter's command word, and why.
func lossyPipeHead(filter string) string {
	return "❌ Blocked: piping an expensive command's output through a lossy filter (" + filter + ")\n" +
		"  -- The filter drops the lines that say why a run failed, and what it keeps is what you would judge the run by.\n"
}

// lossyPipeSubagentAdvice is the piped-command refusal a subagent reads on a
// line the heavy-work check did not hand back (withoutHandedBackAdvice). The
// piped command starts no heavy work for this caller, or the line would have
// been handed back: it is a command this guard classes as expensive and that
// check does not, today the linter's subcommands that run no analysis. The
// admitted `./le job run` form lossyPipeRefusal advises is refused to a
// subagent, so this text never names it: the route is the command itself with
// no filter, written from the same words (written, with the caller's identity
// line in front so that typing it back is not refused), or the main thread's
// run when its output is too long to read. With no words, a line the hook
// failed to read, it names the route and no command.
func lossyPipeSubagentAdvice(identity, filter string, written []string) string {
	head := lossyPipeHead(filter)
	owed := "  -- If its output is too long to read, do not run it: put the command under `Owed commands` in your report, and the main thread runs it."
	if len(written) == 0 {
		return head + "  -- Run the command with no filter, redirect or tee, and read its whole output.\n" + owed
	}
	return head +
		"  -- Run it with no filter, redirect or tee, and read its whole output:\n" +
		"     " + identity + strings.Join(written, " ") + "\n" + owed
}

// treeLossyPipe is the piped-command guard's tree judgment (AC-10): on a line
// that parses (parse, the call's one parse), every pipeline of the tree at
// any depth, inside a loop, a conditional, a group or a substitution
// included, where an expensive command (expensiveWords over its program
// words) runs in an earlier stage than a lossy filter (parsedCommand.feeds).
// The text split cuts `for p in a b; do ./le test unit all | tail -3; done`
// into a piece that starts with `do`, and cuts `go test -run 'TestA|TestB'
// ./x | tail` at the quoted bar; the tree reads the command the shell runs.
// It reads command words only, so an expensive command that is an argument
// of another command is not one.
//
// It answers the expensive command's program words, from its parsed tokens,
// so a redirection is never one of them and an expensive command of another
// statement is never advised in its place (AC-15, review I2), with each of
// them as the refusal writes it back (parsedCommand.written), and the
// filter's command word; found is false for a line that does not parse or
// holds no such pipeline.
//
// A panic inside the walk is answered as found, with no words and a filter
// that names the failure, so the refusal advises the generic admitted form: a
// line the hook failed to read is never cleared, as the reader-line
// predicate answers it (review I4). readTreeLossyPipe is the walk itself,
// which the fuzz target calls without this recover.
func treeLossyPipe(command string, parse commandParse) (words, written []string, filter string, found bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			words, written, filter, found = nil, nil, "unknown: a line the hook failed to read ("+fmt.Sprint(recovered)+")", true
		}
	}()
	return readTreeLossyPipe(command, parse)
}

// readTreeLossyPipe is treeLossyPipe without its recover.
func readTreeLossyPipe(command string, parse commandParse) (words, written []string, filter string, found bool) {
	if !parse.parsed() {
		return nil, nil, "", false
	}
	commands := everyCommand(command, parse.file.Stmts)
	for index, candidate := range commands {
		program := heavytool.Program(candidate.tokens)
		if !expensiveWords(program) {
			continue
		}
		for _, later := range commands[index+1:] {
			if !candidate.feeds(later) {
				continue
			}
			if lossyFilter(later.commandWord()) {
				// heavytool.Program answers a suffix of the tokens, so the
				// program's written words are the same suffix of written.
				return program, candidate.written[len(candidate.tokens)-len(program):], later.commandWord(), true
			}
		}
	}
	return nil, nil, "", false
}

// lossyFilter reports a program that drops lines of what it reads: the
// filters the piped-command refusal judges.
func lossyFilter(program string) bool {
	return oneOf(program, "head", "tail", "grep", "egrep", "fgrep", "awk", "sed", catCommand, "less", "more")
}

// lossyPipeLabel names the job the piped-command refusal advises. A go test
// and a lint take the labels the raw-job refusal gives them (rawHeavy), and
// any other program takes its name and its first argument. MUST be called
// with at least one word.
func lossyPipeLabel(words []string) string {
	base := filepath.Base(words[0])
	if goTest(words) {
		return testLabel
	}
	if base == lintCommand {
		return "lint"
	}
	if len(words) > 1 {
		return base + "-" + filepath.Base(words[1])
	}
	return base
}

// goTest reports whether program words run `go test`.
func goTest(words []string) bool {
	if len(words) < 2 {
		return false
	}
	return filepath.Base(words[0]) == "go" && words[1] == goTestVerb
}

// goVet reports whether program words run `go vet`.
func goVet(words []string) bool {
	if len(words) < 2 {
		return false
	}
	return filepath.Base(words[0]) == "go" && words[1] == goVetVerb
}

// verifyCommand reports whether program words run an le verification area,
// `le verify ...` or `ze le verify ...`. expensiveWords already judged the
// words heavy, so the lines that run nothing (`verify status`, `verify
// summary`, `verify reds`, `verify list`) never reach it. MUST be called with
// at least one word.
func verifyCommand(words []string) bool {
	base := filepath.Base(words[0])
	if base == "le" {
		return len(words) > 1 && words[1] == verifyArea
	}
	if strings.HasPrefix(base, "ze") {
		return len(words) > 2 && words[1] == "le" && words[2] == verifyArea
	}
	return false
}
