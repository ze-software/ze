// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
package hookruntime

import (
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func payloadSessionID(payload Payload) (string, bool) {
	if payload.SessionID == nil {
		return "", false
	}
	id, ok := payload.SessionID.(string)
	if !ok || !lepath.ValidSessionID(id) {
		return "", true
	}
	return id, true
}

func resolvedSessionID(ctx context) string {
	if id, present := payloadSessionID(ctx.payload); present {
		return id
	}
	paths, err := lepath.ResolveSession(ctx.root, false)
	if err != nil {
		return ""
	}
	return paths.ID
}

func runSessionID(ctx context, out, _ io.Writer) int {
	if id, present := payloadSessionID(ctx.payload); present {
		if id == "" {
			return 2
		}
		fmt.Fprintln(out, id) //nolint:errcheck // hook protocol
		return 0
	}
	return 1
}

const (
	// agentVariable carries a subagent's own id to le, which keys on it
	// everything one agent must own alone.
	agentVariable = "ZE_AGENT_ID"
	// sessionVariable carries the parent's session id to le, which keeps the
	// session's directory and scratch shared.
	sessionVariable = "CLAUDE_CODE_SESSION_ID"
	// testCommand is the shell builtin `test`. It is the command word, not
	// the path element testElement names, which only spells the same.
	testCommand = "test"
)

// identityLine is the line a subagent types in front of a command so that le
// resolves the identity the harness gave the agent: the agent's own id and the
// parent's session id. A fork subagent has no agent id, and its line is the
// session part alone. The start text prints this line and the identity check
// compares a command with it, so the two cannot disagree.
func identityLine(agent, session string) string {
	sessionPart := "export " + sessionVariable + "=" + session + "; "
	if agent == "" {
		return sessionPart
	}
	return "export " + agentVariable + "=" + agent + "; " + sessionPart
}

// commandOutsideReaders names what makes a line need the identity line: the
// shell syntax the hook does not model (unmodeledSyntax, shellsyntax.go), on
// which the splitter's commands need not be the ones bash runs, the first
// command that is not a reader or redirects its output into a file, a
// command substitution, or a line the splitter cannot parse. It answers ""
// only when the line lies inside the modeled syntax and every command of the
// line is a reader its judge admits
// (readerCommand, readers.go) and writes no file (shellLine.writesFile), so
// a line the hook cannot see whole is never a reader line, and a reader
// cannot write the git or shell configuration a later reader would run.
//
// A line bash's grammar does not accept (parse, the call's one parse,
// shellparse.go) is never a reader line either: the splitter reads
// `cat a |` as the reader cat, while bash refuses the line, so what the
// splitter answered is not what bash runs. That reason is asked last, so a
// line that is outside the readers for a reason named above keeps the
// reason it had before the parse existed.
func commandOutsideReaders(command string, parse commandParse) string {
	if construct := unmodeledSyntax(command); construct != "" {
		return construct + ", shell syntax the hook does not model, so it cannot tell which commands the line runs"
	}
	line, parsed := lineCommands(command)
	if !parsed {
		return "a line that ends inside a quote or after a backslash, which the hook cannot split into commands"
	}
	if line.substitutes {
		return "a command substitution (a backtick or `$(`), whose inner command the hook does not judge"
	}
	for index, words := range line.commands {
		if _, reader := readerCommands[words[0]]; !reader {
			return "`" + words[0] + "`, which is outside the reader set"
		}
		if !readerCommand(words, line.expands[index]) {
			return "`" + strings.Join(words, " ") + "`, which the reader set admits only with the options its judge lists and no word the shell expands"
		}
		if line.writesFile(index) {
			return "`" + line.segments[index] + "`, which redirects its output into a file"
		}
	}
	if !parse.parsed() {
		return "a line that does not parse as bash (" + parse.failure + "), so the hook cannot tell which commands it runs"
	}
	return ""
}

// identityMentions counts the identity variables' names in a text.
func identityMentions(text string) int {
	return strings.Count(text, agentVariable) + strings.Count(text, sessionVariable)
}

// environmentTools are the commands that set, unset, export or clear
// variables, run text as shell, or run a command under another environment
// or user. After the agent's identity line any of them is refused, whatever
// names it carries, because a name built from pieces (`ZE_AGENT_""ID`,
// `"${n}_ID"`) hides from a literal match.
var environmentTools = map[string]bool{
	"export": true, "declare": true, "typeset": true, "readonly": true, "local": true, "unset": true,
	"env": true, "exec": true, "sudo": true, "eval": true, "source": true, ".": true, "set": true,
}

// The wrappers that run the command after their options and arguments.
const (
	niceCommand    = "nice"
	timeoutCommand = "timeout"
)

// transparentWords run the word after them as the command: the shell's
// reserved words and the wrappers that take a command, whose options and
// numeric arguments are skipped too.
var transparentWords = map[string]bool{
	"if": true, "then": true, "elif": true, "else": true, "do": true, "while": true, "until": true,
	"time": true, "command": true, "builtin": true, niceCommand: true, "nohup": true, timeoutCommand: true,
	"xargs": true, "stdbuf": true,
}

// changesEnvironment reports whether one command, given as its words as
// typed (raw) and with quotes removed (words, parallel to raw), may change
// the environment the agent's identity lives in: an assignment in front of
// the command word whose name is not a plain name (a quote, a backslash, an
// expansion or a subscript in it), or a command word, past any reserved word
// or wrapper, among environmentTools. A plain `FOO=bar ./le x` passes.
func changesEnvironment(raw, words []string) bool {
	afterWrapper := false
	for index, word := range words {
		typed := word
		if index < len(raw) {
			typed = raw[index]
		}
		if strings.Contains(typed, "=") && !strings.HasPrefix(typed, "-") {
			if !plainAssignment(typed) {
				return true
			}
			continue
		}
		if transparentWords[word] {
			afterWrapper = true
			continue
		}
		if afterWrapper && (strings.HasPrefix(word, "-") || wrapperNumber(word)) {
			continue
		}
		return environmentTools[word]
	}
	return false
}

// wrapperNumber reports whether a word after a wrapper is a count or a
// duration the walk skips (`nice 10`, `timeout 1.5s`): made only of digits,
// `.` and the unit letters `smhd`, and holding a digit. A word with no digit
// is the command word the wrapper runs, so a lone `.` is the source builtin
// and `time . ./env.sh` is judged as `.` (spec-agent-economy-6-shell-parser
// AC-26).
func wrapperNumber(word string) bool {
	if strings.Trim(word, "0123456789.smhd") != "" {
		return false
	}
	return strings.ContainsAny(word, "0123456789")
}

// plainAssignment reports whether a word typed in front of a command assigns
// a plain name: letters, digits and `_`, not starting with a digit, then `=`
// or `+=`.
func plainAssignment(typed string) bool {
	name, _, _ := strings.Cut(typed, "=")
	name = strings.TrimSuffix(name, "+")
	if name == "" || !nameStart(name[0]) {
		return false
	}
	return nameEnd(name, 0) == len(name)
}

// rawWords splits one command's text at whitespace outside quotes, keeping
// the quotes and backslashes as typed.
func rawWords(text string) []string {
	words := make([]string, 0, 8)
	start, quote := -1, byte(0)
	for index := range len(text) {
		character := text[index]
		switch {
		case quote != 0:
			if character == quote {
				quote = 0
			}
		case character == ' ' || character == '\t':
			if start >= 0 {
				words = append(words, text[start:index])
				start = -1
			}
			continue
		case character == '\'' || character == '"':
			quote = character
		}
		if start < 0 {
			start = index
		}
	}
	if start >= 0 {
		words = append(words, text[start:])
	}
	return words
}

// textChangesEnvironment judges a remainder the splitter cannot be trusted
// on by its text: each piece between `;`, `&`, `|`, a newline, a bracket, a
// brace, `!` or a backtick is a command whose whitespace fields, with quotes
// and backslashes removed, changesEnvironment judges. It fails closed: a
// split the shell does not make only adds commands to judge.
func textChangesEnvironment(rest string) bool {
	pieces := strings.FieldsFunc(rest, func(character rune) bool {
		return strings.ContainsRune(";&|\n(){}!`", character)
	})
	for _, piece := range pieces {
		raw := strings.Fields(piece)
		words := make([]string, len(raw))
		for index, field := range raw {
			words[index] = strings.NewReplacer(`'`, "", `"`, "", `\`, "").Replace(field)
		}
		if changesEnvironment(raw, words) {
			return true
		}
	}
	return false
}

// remainderChangesEnvironment applies changesEnvironment to every command of
// what follows the agent's identity line that is not a reader its judge
// admits writing no file: by the splitter's words on a line inside the
// modeled syntax, and by its text (textChangesEnvironment) on any other.
func remainderChangesEnvironment(rest string) bool {
	if !modeledSyntax(rest) {
		return textChangesEnvironment(rest)
	}
	line, parsed := lineCommands(rest)
	if !parsed {
		return textChangesEnvironment(rest)
	}
	if line.substitutes {
		return textChangesEnvironment(rest)
	}
	for index, words := range line.commands {
		if readerCommand(words, line.expands[index]) && !line.writesFile(index) {
			continue
		}
		if changesEnvironment(rawWords(line.segments[index]), words) {
			return true
		}
	}
	return false
}

// takesAnotherIdentity reports whether what follows the agent's identity line
// may set, unset or drop an identity variable. It catches an agent that
// forgets or mistypes its identity, and it fails closed in two ways. First,
// any command that is not an admitted reader and that changes the
// environment by its form (changesEnvironment: export, env, exec, sudo, set
// and the rest, or an assignment to a name that is not plain) is refused
// whatever names it carries, by its words on a line inside the modeled
// syntax and by its text on any other. Second, bash sets a variable in more
// forms than a list holds (`+=`, `read`, `printf -v`, `for`), so any command
// whose text names either variable is refused, unless it is a reader its
// judge admits (readerCommand) that writes no file, which reads the name as
// text (`grep ZE_AGENT_ID f`, `echo $ZE_AGENT_ID`). Every mention must lie
// inside such a reader, so a remainder outside the shell syntax the hook
// models (modeledSyntax), one the splitter cannot parse, or one with a
// command substitution, is refused whenever it names either variable. It is
// not a sandbox: it judges the command, not the files the command runs.
func takesAnotherIdentity(rest string) bool {
	if remainderChangesEnvironment(rest) {
		return true
	}
	named := identityMentions(rest)
	if named == 0 {
		return false
	}
	if !modeledSyntax(rest) {
		return true
	}
	line, parsed := lineCommands(rest)
	if !parsed {
		return true
	}
	if line.substitutes {
		return true
	}
	read := 0
	for index, words := range line.commands {
		if !readerCommand(words, line.expands[index]) {
			continue
		}
		if line.writesFile(index) {
			continue
		}
		read += identityMentions(line.segments[index])
	}
	return read < named
}

// ze point: planning/directives/brief-every-subagent-with-its-spec-phase-and-rules
// bashAgentIdentity keeps a subagent's command under the identity the harness
// gave the agent: the parent's session id, which keeps the session's directory
// and scratch shared, and the agent's own id, which keys everything one agent
// must own alone (its commit namespace, its debt file and its spec claim, in
// internal/le/le/path). Without the agent id every sibling resolved one
// identity and shared all three
// (plan/journal/concurrent-session-corruption.md).
//
// The check never changes the command: a changed command is one auto mode
// holds no verdict for, so the harness voids it and the calls sent with it.
// The agent types its identity line (identityLine) and the check compares.
// A command made only of readers its judges admit (readerCommands,
// readers.go) passes without the line. A
// command that starts with the agent's exact line passes unless a command
// after it other than a reader names an identity variable
// (takesAnotherIdentity). Every other command of a subagent is refused with
// the line to type. The line is built from the payload's ids and never taken
// from the command, so a line naming another agent or session is refused.
//
// An agent id that cannot name a file refuses every call, because without its
// own identity the command would run as the main thread, which is the sharing
// this exists to end. A payload with an agent id and no usable session id
// refuses the same way. A fork subagent carries no agent id, so it has no
// identity of its own to lose: with no usable session id it runs unjudged.
// The main thread is the identity le resolves by default and is never judged.
func bashAgentIdentity(ctx context) *verdict {
	agent := ctx.payload.AgentID
	if agent == "" && os.Getenv("CLAUDE_CODE_FORK_SUBAGENT") == "" {
		return nil
	}
	session, _ := payloadSessionID(ctx.payload)
	if session == "" {
		if agent == "" {
			return nil
		}
		return &verdict{2, red + bold + "BLOCKED: this subagent's hook payload carries no usable session id" + reset +
			"\n  agent " + strconv.Quote(agent) + " arrived with a session_id that is absent or not a file name." +
			"\n  With no identity line to type, the command would act as the main thread and share its commit namespace and spec claim." +
			"\n  Next: stop and report the payload's session_id to the main thread. An id holds only letters, digits, '.', '_' and '-'."}
	}
	if agent != "" {
		if _, err := lepath.AgentFingerprint(session, agent); err != nil {
			return &verdict{2, red + bold + "BLOCKED: this subagent's identity cannot name its own files" + reset +
				"\n  " + err.Error() +
				"\n  With no identity line to type, the command would act as the main thread and share its commit namespace and spec claim." +
				"\n  Next: stop and report the agent id to the main thread. An id holds only letters, digits, '.', '_' and '-'."}
		}
	}
	command := stringInput(ctx.input, "command")
	own := identityLine(agent, session)
	// Leading whitespace runs nothing, so the agent's own exact line after it
	// is the agent's line (review N4); every other check reads it as typed.
	if rest, carries := strings.CutPrefix(strings.TrimLeft(command, " \t\n"), own); carries {
		if takesAnotherIdentity(rest) {
			return anotherIdentityVerdict(own, "After its identity line a command changes the environment (export, env, exec, sudo, set and the like, or an assignment to a name that is not plain),"+
				" or a command other than a reader names "+agentVariable+" or "+sessionVariable+", which can set, unset or drop the identity.")
		}
		return nil
	}
	if anyPrefix(strings.TrimSpace(command), "export "+agentVariable+"=", "export "+sessionVariable+"=") {
		return anotherIdentityVerdict(own, "It starts with an identity line that is not this agent's: it names another agent or session, or leaves the agent out.")
	}
	outsider := commandOutsideReaders(command, ctx.commandParse())
	if outsider == "" {
		return nil
	}
	return &verdict{2, red + bold + "BLOCKED: this subagent command needs the agent's identity line in front of it" + reset +
		"\n  It holds " + outsider + "." +
		"\n  Such a command can start ./le, and without the line ./le acts as the main thread: it shares the main thread's commit namespace and spec claim." +
		"\n  A command made only of these readers needs no line (git only as `git <read verb>`): " +
		strings.Join(slices.Sorted(maps.Keys(readerCommands)), ", ") +
		"\n  Next: type the command again with this line in front of it, exactly as printed. The hook checks the line and never adds it:" +
		"\n  " + own}
}

// commandIdentityPrefix answers the identity line a command another refusal
// advises must start with: the caller's own line for a subagent (an agent id
// or a fork), so that following the advice is not refused by
// bashAgentIdentity, and "" for the main thread. A payload whose identity
// cannot be typed answers "" too, because bashAgentIdentity refuses that
// caller's every command with stop-and-report before any advice applies.
func commandIdentityPrefix(ctx context) string {
	agent := ctx.payload.AgentID
	if agent == "" && os.Getenv("CLAUDE_CODE_FORK_SUBAGENT") == "" {
		return ""
	}
	session, _ := payloadSessionID(ctx.payload)
	if session == "" {
		return ""
	}
	if agent != "" {
		if _, err := lepath.AgentFingerprint(session, agent); err != nil {
			return ""
		}
	}
	return identityLine(agent, session)
}

// anotherIdentityVerdict refuses a subagent command that would run under an
// identity other than the agent's own, and prints the line the agent owns.
func anotherIdentityVerdict(own, what string) *verdict {
	return &verdict{2, red + bold + "BLOCKED: this subagent command takes another identity than the agent's own" + reset +
		"\n  " + what +
		"\n  ./le would then act for another agent or for the main thread, and write into its commit namespace and spec claim." +
		"\n  Next: start the command with this identity line, exactly as printed, and set neither variable after it:" +
		"\n  " + own}
}

// specClaim answers the spec the hook's identity has claimed: the agent's own
// claim when the payload names an agent that made one, else the session's,
// which is the claim the agent was spawned under. "" is no claim.
func specClaim(ctx context, id string) string {
	if agent := ctx.payload.AgentID; agent != "" {
		if marker, err := lepath.SpecClaimMarker(ctx.root, id, agent); err == nil {
			if claim := readFirstLine(marker); claim != "" {
				return claim
			}
		}
	}
	marker, err := lepath.SpecClaimMarker(ctx.root, id, "")
	if err != nil {
		return ""
	}
	return readFirstLine(marker)
}

func sessionMarker(ctx context, prefix string) string {
	id := resolvedSessionID(ctx)
	if id == "" {
		return ""
	}
	return strings.Join([]string{ctx.root, "tmp", "session", prefix + id}, string(os.PathSeparator))
}
