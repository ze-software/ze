// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: session.go -- fromSubagent, the caller predicate this check opens with
// Related: ../heavytool/heavytool.go -- Name, the one definition of a heavy tool this judgment and `le exec` read
// Related: ../heavytool/program.go -- Started, Program and ShellCommand, the reading of a command's words this judgment and `le exec` share
// Related: parsedcommands.go -- everyCommand, the parsed commands the tree judgment reads
// Related: ../le/root/heavy.go -- HeavyCommand, which reads an le line from what its area declared and names its command
package hookruntime

import (
	"fmt"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"

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

// handBackOf is the heavy-work judgment of one command line (heavyWork).
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
	if what := heavyWork(command, parse, ""); what != "" {
		return handBack{what: what, command: typed}
	}
	return handBack{}
}

// heavyWork names the first heavy work a command line starts, and answers ""
// when it starts none. within names the launcher that was handed the line,
// and is empty for the line the agent typed. On a line that parses it is the
// first command of the tree, in source order and at any depth (everyCommand),
// that starts heavy work (heavyTool), or whose launcher is handed a line (a
// shell's `-c` operand, env's `-S` string) that does (heavytool.ShellCommand,
// judged here as a line of its own), or that the hook cannot read before it
// runs (unreadHandedLine); and else the
// first such segment of the split that cuts only outside the tree's quoted
// parts (unquotedSegments), which reads a here-document's lines. On a line
// that does not parse it is the first such segment of the quote-blind split
// (commandSegments), there being nothing else to read.
//
// A quoted argument of another program is data, never a command: the
// quote-blind split cut `grep "le exec\|/le exec"` at its quoted bars and
// read `/le exec"` as an le line, so a reader's pattern was handed back.
// The raw-job guard's own text judgment still splits quote-blind; a line it
// refuses that is not handed back here reads that guard's text for a
// subagent (withoutHandedBackAdvice), which names no route a subagent lacks.
//
// The recursion into a shell's line is bounded by the line's length: the
// line handed to the shell is one word of the command, shorter than it.
func heavyWork(command string, parse commandParse, within string) string {
	if !parse.parsed() {
		for _, segment := range commandSegments(command) {
			if what := heavyTool(shellWords(segment)); what != "" {
				return what
			}
		}
		return ""
	}
	for _, parsed := range everyCommand(command, parse.file.Stmts) {
		if what := heavyTool(parsed.tokens); what != "" {
			return what
		}
		if what := unreadProgram(parsed, within); what != "" {
			return what
		}
		launcher, first, end, handed := heavytool.ShellCommandWords(parsed.tokens)
		if !handed {
			continue
		}
		if what := unreadHandedLine(parsed, launcher, first, end); what != "" {
			return what
		}
		line, _ := heavytool.ShellCommand(parsed.tokens)
		if what := heavyWork(line, parseCommandLine(line), launcher); what != "" {
			return what
		}
	}
	for _, segment := range unquotedSegments(command, parse.file) {
		if what := heavyTool(shellWords(segment)); what != "" {
			return what
		}
	}
	return ""
}

// unreadHandedLine names the line parsed hands its launcher when a word the
// line is read from, first to before end, is one the shell expands or the
// hook cannot read (`bash -c "$X"`, `bash -c "echo $HOME"`), and answers ""
// for a line read only from words the hook read exactly. The launcher runs
// the expanded value, which the hook never sees, so the line it judged is not
// the line that runs: a subagent's such line is handed back, as a line the
// hook fails to read is (handBackOf). A single-quoted line is read exactly
// and judged as its own line.
func unreadHandedLine(parsed parsedCommand, launcher string, first, end int) string {
	for at := first; at < end; at++ {
		if exactToken(parsed, at) {
			continue
		}
		return "a line handed to `" + launcher + "` that the shell expands before it runs, so the hook cannot read it, and it may start heavy work"
	}
	return ""
}

// unreadProgram names a command of a line handed to the launcher within whose
// program word the hook cannot read before it runs (`sh -c '"$@"' _ go test`,
// `bash -c '$X'`), or whose program runs text as shell (shellTextTools: `bash
// -c 'eval "go test ./..."'`, `sh -c '. ./run.sh'`), and answers "" for one
// whose program it read exactly and runs no such text, or
// for the line the agent typed (an empty within), where the identity guard
// already refuses a program word the shell expands.
func unreadProgram(parsed parsedCommand, within string) string {
	if within == "" {
		return ""
	}
	// refuse-by-default: guards heavyWork -- a statement of assignments alone runs no program, so it has no program word to read
	if len(parsed.assigns) >= len(parsed.tokens) {
		return ""
	}
	if !exactToken(parsed, len(parsed.assigns)) {
		return "a command in a line handed to `" + within + "` whose program the hook cannot read before it runs, which may start heavy work"
	}
	// A program that runs text as shell (eval, source, `.`) runs what the hook
	// never reads: a quoted string, or a file. The identity guard refuses one
	// on the line as typed, never inside a handed line, so it is judged here.
	program := heavytool.Program(parsed.tokens)
	if len(program) == 0 {
		return ""
	}
	// refuse-by-default: guards handsBackHeavy -- a program that runs no text as shell is judged by heavyTool, and the empty name is the caller's "not handed back"
	if !shellTextTools[filepath.Base(program[0])] {
		return ""
	}
	return "`" + program[0] + "` in a line handed to `" + within + "`, which runs text the hook cannot read before it runs, which may start heavy work"
}

// exactToken reports whether the hook read parsed's token at, its value
// exactly, with nothing in it the shell expands (writtenByValue). A token
// past the words the command holds is not read.
func exactToken(parsed parsedCommand, at int) bool {
	if at >= len(parsed.tokens) {
		return false
	}
	if at >= len(parsed.written) {
		return false
	}
	return writtenByValue(parsed.tokens[at], parsed.written[at])
}

// unquotedSegments is commandSegments for a line that parsed into file: it
// cuts at a separator only where the separator stands outside every quoted
// part of the tree, a single-quoted or a double-quoted one, so a quoted
// argument stays inside the segment of the command it belongs to. The
// offsets of file are offsets into command, its source.
func unquotedSegments(command string, file *syntax.File) []string {
	var quoted [][2]uint
	syntax.Walk(file, func(node syntax.Node) bool {
		_, single := node.(*syntax.SglQuoted)
		_, double := node.(*syntax.DblQuoted)
		if !single && !double {
			return true
		}
		// A quoted part's children are inside it, so the walk stops there.
		quoted = append(quoted, [2]uint{node.Pos().Offset(), node.End().Offset()})
		return false
	})
	var segments []string
	start := 0
	for _, cut := range segmentSeparator.FindAllStringIndex(command, -1) {
		if insideQuoted(quoted, uint(cut[0])) {
			continue
		}
		segments = appendSegment(segments, command[start:cut[0]])
		start = cut[1]
	}
	return appendSegment(segments, command[start:])
}

// insideQuoted reports whether offset lies within one of the quoted spans,
// each a start offset and the end offset past it.
func insideQuoted(quoted [][2]uint, offset uint) bool {
	for _, span := range quoted {
		if offset >= span[0] && offset < span[1] {
			return true
		}
	}
	return false
}

// appendSegment appends part, trimmed, to segments unless it is blank, as
// commandSegments keeps its pieces.
func appendSegment(segments []string, part string) []string {
	trimmed := strings.TrimSpace(part)
	if trimmed == "" {
		return segments
	}
	return append(segments, trimmed)
}

// heavyTool names the heavy work tokens, one command's words, start, and
// answers "" when they start none. The assignments and launchers in front of
// the program, a stated reason to run raw among them, and the Go toolchain's
// flag in front of its verb are read through by the reading `le exec` shares
// (heavytool.Started, heavytool.Program). The program is then judged by name.
// An external program, which cannot register, is heavy when the shared
// definition names it (heavytool.Name, which holds the Go toolchain's
// executing verbs, the linters and the formatters, so this function holds no
// list of them). le, typed as `le` or as the
// `ze le` personality (heavytool.LeWords), is heavy when the area or the
// action its line names declared so (leroot.HeavyCommand, which also answers
// the command the line resolved to), so this function names no le area. A
// carrier area's line is heavy by the program it carries, as the carrier's
// own judge reads it (leroot.RegisterCarrierJudge): `./le exec go version`
// runs and `./le exec go test ./x/` is handed back, with no branch here.
func heavyTool(tokens []string) string {
	program := heavytool.Program(tokens)
	if len(program) == 0 {
		return ""
	}
	if name, heavy := heavytool.Started(tokens); heavy {
		return "`" + name + "`"
	}
	words, typed := heavytool.LeWords(program)
	// refuse-by-default: guards handsBackHeavy -- a program that is neither a heavy tool nor an le line starts no heavy work, and the empty name is what the caller reads as "not handed back"
	if !typed {
		return ""
	}
	command, heavy := leroot.HeavyCommand(beforeRedirection(words))
	// refuse-by-default: guards handsBackHeavy -- an le line that resolves to no heavy command (a listing, help, a light action) is the caller's "not handed back"
	if !heavy {
		return ""
	}
	// The name is the program as typed and the command le resolved the line
	// to, which holds no keyword or value typed after it.
	launcher := strings.Join(program[:len(program)-len(words)], " ")
	return "the `./le` action `" + launcher + " " + command + "`"
}
