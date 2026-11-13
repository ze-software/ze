// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: session.go -- fromSubagent, the caller predicate this check opens with
// Related: rawheavy.go -- the launcher reading this judgment reuses
// Related: ../heavytool/heavytool.go -- Name, the one definition of a heavy tool this judgment and `le exec` read
// Related: parsedcommands.go -- everyCommand, the parsed commands the tree judgment reads
// Related: ../le/root/heavy.go -- HeavyCommand, which reads an le line from what its area declared and names its command
package hookruntime

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/heavytool"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// handBack is what the heavy-work check refuses for a subagent: what the
// command starts, in the words the refusal names it by, and the command the
// agent lists in its report for the main thread to run, which is the line it
// typed after its identity line (handBackOf). A zero what is a line that
// starts no heavy work.
type handBack struct {
	what    string
	command string
}

// handBackState holds the one heavy-work judgment of a Bash call, made by
// the first check that asks (context.heavyHandBack) and read by every later
// check of the same call. It is not safe for concurrent use: Run calls its
// checks one after another.
type handBackState struct {
	done   bool
	result handBack
}

// handBackHeader opens the refusal, before the name of what was typed.
const handBackHeader = "BLOCKED: a subagent does not run "

// ze point: commands/directives/heavy-work-starts-in-the-main-thread
// bashSubagentHeavy refuses a test, a build, a lint, a formatter or a heavy
// `./le` action a subagent types, and tells the agent to hand the command
// back. One machine carries every session, and the main thread is the one
// caller that schedules heavy work on it, one job at a time; a subagent
// edits, and lists the commands its change owes in its report.
//
// The main thread is never judged here. For a subagent, a reader line
// (ctxReaderLine) starts nothing, so a grep pattern naming `go test` passes.
// On every other line the verdict is the tree judgment of a line that
// parses, which reads every command at any depth, or else the text judgment
// (heavyHandBack). A reason typed in front of the command, the escape the
// raw-job guard reads for the caller who schedules, lifts nothing here.
//
// The refusal carries the only text this line receives for its heavy work.
// The raw-job, piped-command and build guards still refuse a line this check
// refuses, exactly where they refused it before this check existed, and they
// refuse it without text (withoutHandedBackAdvice): each of them advises a
// form this check refuses too. The text names the hand-back alone and ends
// with the command.
func bashSubagentHeavy(ctx context) *verdict {
	if !fromSubagent(ctx) {
		return nil
	}
	if _, outside := ctxReaderLine(ctx); outside == "" {
		return nil
	}
	refusal := ctx.heavyHandBack()
	if refusal.what == "" {
		return nil
	}
	return &verdict{2, red + bold + handBackHeader + refusal.what + reset +
		"\n  The main thread runs tests, builds, lint and formatters, one at a time, on the machine every session shares, so it can schedule each against the machine's load." +
		"\n  Nothing typed in front of the command changes that: it is still the main thread's to start." +
		"\n  Next: do not run it. Put this exact command under `Owed commands` in your report, and the main thread runs it:" +
		"\n  " + refusal.command}
}

// withoutHandedBackAdvice answers the verdict of a guard whose advice names a
// route a subagent is refused (the admitted `./le job run` form, the
// stated-reason escape, a build output path). The verdict stands for every
// caller: a guard that refused a line refuses it still, with the same code,
// so the hook's exit code, the call's refused mark (callState.refused) and
// the recorded verdict table read the guard as they did. For a line the
// heavy-work check hands back (handsBackHeavy) the refusal loses its text,
// which Run then does not print (shownVerdicts): the hand-back already
// carries the one next step. A verdict that does not block is answered as it
// is.
//
// A subagent's line the heavy-work check did not hand back is refused with
// unhanded, the text the guard wrote for that caller, and never with the
// refusal's own: the admitted form and the stated-reason escape are routes a
// subagent does not have on any line, so no path through this function shows
// the main thread's advice to a subagent. The main thread reads the refusal
// as the guard built it, byte for byte. unhanded MUST advise only what a
// subagent can do.
func withoutHandedBackAdvice(ctx context, refusal *verdict, unhanded string) *verdict {
	if refusal == nil {
		return nil
	}
	if refusal.code < 2 {
		return refusal
	}
	if !fromSubagent(ctx) {
		return refusal
	}
	if handsBackHeavy(ctx) {
		return &verdict{refusal.code, ""}
	}
	return &verdict{refusal.code, unhanded}
}

// owedCommandAdvice is the closing advice of a guard's refusal for a
// subagent's line the heavy-work check did not hand back, where the guard has
// no form of the command a subagent may type: the line goes to the main
// thread as typed, after the identity line, which is the agent's own
// (typedCommand).
func owedCommandAdvice(ctx context) string {
	return "  -- Put this exact command under `Owed commands` in your report, and the main thread runs it:\n" +
		"     " + typedCommand(stringInput(ctx.input, "command"), commandIdentityPrefix(ctx))
}

// typedCommand answers the line a caller typed after its identity line
// (identity), trimmed: the command a report hands to the main thread.
func typedCommand(command, identity string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimLeft(command, " \t\n"), identity))
}

// handsBackHeavy reports whether the call is one bashSubagentHeavy refuses: a
// subagent's line that is no reader line and starts heavy work. A guard whose
// advice is a form that check refuses too asks it and refuses without text
// (withoutHandedBackAdvice), so the subagent reads one refusal for the line.
// It answers false for the main thread, whose verdicts are unchanged.
func handsBackHeavy(ctx context) bool {
	if !fromSubagent(ctx) {
		return false
	}
	if _, outside := ctxReaderLine(ctx); outside == "" {
		return false
	}
	return ctx.heavyHandBack().what != ""
}

// heavyHandBack answers the heavy work a subagent's command starts, judging
// the command on the first ask of the hook call and reading that result on
// every later one. It answers a zero what for the main thread. It does not
// ask whether the line is a reader line: its callers do, first. A context
// built without newContext (a unit test's literal) holds no call state, so
// the line is judged for that one ask and the result is the same.
func (ctx context) heavyHandBack() handBack {
	if !fromSubagent(ctx) {
		return handBack{}
	}
	command := stringInput(ctx.input, "command")
	if ctx.call == nil {
		return handBackOf(command, ctx.commandParse(), commandIdentityPrefix(ctx))
	}
	if !ctx.call.handBack.done {
		ctx.call.handBack.result = handBackOf(command, ctx.commandParse(), commandIdentityPrefix(ctx))
		ctx.call.handBack.done = true
	}
	return ctx.call.handBack.result
}

// handBackOf is the heavy-work judgment of one command line: the first
// command of the parsed line, in source order and at any depth
// (everyCommand), that starts heavy work (heavyTool), written from its parsed
// words; and, on a line that does not parse or whose tree holds none, the
// first such segment of the quote-blind split (commandSegments). The text
// judgment is kept so this verdict covers every line the raw-job guard
// refuses: that guard is silent for a line refused here, and a line it would
// refuse and this judgment passed would hand a subagent its admitted-form
// advice.
//
// The command handed back is the whole line as the agent typed it, after its
// identity line (identity), which is the agent's own and not the main
// thread's to type: the directory change in front of a test, the loop around
// it and the filter behind it are part of what the agent asked to have run,
// and a command cut out of the line would run something else.
//
// A panic inside the walk is answered as a refusal that says so: a line the
// hook failed to read is never cleared, as the raw-job guard answers it
// (treeRawHeavy).
func handBackOf(command string, parse commandParse, identity string) (refusal handBack) {
	typed := typedCommand(command, identity)
	defer func() {
		if recovered := recover(); recovered != nil {
			refusal = handBack{
				what:    "a line the hook failed to read (" + fmt.Sprint(recovered) + "), which may start heavy work",
				command: typed,
			}
		}
	}()
	if parse.parsed() {
		for _, parsed := range everyCommand(command, parse.file.Stmts) {
			if what := heavyTool(parsed.tokens); what != "" {
				return handBack{what: what, command: typed}
			}
		}
	}
	for _, segment := range commandSegments(command) {
		if what := heavyTool(shellWords(segment)); what != "" {
			return handBack{what: what, command: typed}
		}
	}
	return handBack{}
}

// heavyTool names the heavy work tokens, one command's words, start, and
// answers "" when they start none. The assignments and launchers in front of
// the program are read through (programTokens, over launcherValueOptions), a
// stated reason to run raw among them. The program is then judged by name.
// An external program, which cannot register, is heavy when the shared
// definition names it (heavytool.Name, which holds the Go toolchain's
// executing verbs, the linters and the formatters, and which `le exec` reads
// too, so this function holds no list of them). le, typed as `le` or as the
// `ze le` personality (leWords), is heavy when the area or the action its
// line names declared so (leroot.HeavyCommand, which also answers the
// command the line resolved to), so this function names no le area.
func heavyTool(tokens []string) string {
	program := programTokens(tokens)
	if len(program) == 0 {
		return ""
	}
	if name, heavy := heavytool.Name(program); heavy {
		return "`" + name + "`"
	}
	words, typed := leWords(program)
	if !typed {
		return ""
	}
	command, heavy := leroot.HeavyCommand(beforeRedirection(words))
	if !heavy {
		return ""
	}
	// The name is the program as typed and the command le resolved the line
	// to, which holds no keyword or value typed after it.
	launcher := strings.Join(program[:len(program)-len(words)], " ")
	return "the `./le` action `" + launcher + " " + command + "`"
}

// leWords answers the words typed after le when program, a command's words
// from its program on, runs le: as `le`, `./le` or a path ending in `/le`,
// or as the `ze le` personality of a binary whose name starts with `ze`. It
// answers false for any other program. MUST be called with at least one
// word.
func leWords(program []string) ([]string, bool) {
	base := filepath.Base(program[0])
	if base == "le" {
		return program[1:], true
	}
	if strings.HasPrefix(base, "ze") && len(program) > 2 && program[1] == "le" {
		return program[2:], true
	}
	return nil, false
}
