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

// readerCommands is the declared set of commands a subagent runs without its
// identity line: a command here reads neither identity variable and, with the
// arguments its judge accepts, starts no le, test binary or script. A nil
// judge accepts every argument. A command absent from the table needs the
// line, so the table is wrong only in the safe direction: a refusal that
// prints its fix. Two members stay on trust: a GNU sed script can run a
// command through its `e` command, and gopls starts the go tool, which reads
// neither variable.
var readerCommands = map[string]func(arguments []string) bool{
	// `[` is the test builtin. A `!` ends a command for the splitter, so a
	// negated `[ ! -f x ]` leaves `-f` as a command word and needs the line.
	"[":         nil,
	"basename":  nil,
	catCommand:  nil,
	"cd":        nil,
	"cut":       nil,
	"diff":      nil,
	"dirname":   nil,
	"echo":      nil,
	"egrep":     nil,
	"fgrep":     nil,
	"find":      findStartsNoProgram,
	"git":       gitReads,
	"gopls":     nil,
	"grep":      nil,
	"head":      nil,
	"ls":        nil,
	"printf":    nil,
	"pwd":       nil,
	"rg":        ripgrepStartsNoProgram,
	"sed":       nil,
	"sort":      sortStartsNoProgram,
	"stat":      nil,
	"tail":      nil,
	testCommand: nil,
	"tr":        nil,
	"true":      nil,
	"uniq":      nil,
	"wc":        nil,
}

// findStartsNoProgram reports whether a find runs no command of its own.
func findStartsNoProgram(arguments []string) bool {
	for _, argument := range arguments {
		if oneOf(argument, "-exec", "-execdir", "-ok", "-okdir") {
			return false
		}
	}
	return true
}

// gitReads reports whether a git command is a read verb directly after `git`,
// with no option that hands the output to a program. An option before the
// verb, such as `-C` or `-c`, is not judged, so that form needs the line.
func gitReads(arguments []string) bool {
	if len(arguments) == 0 {
		return false
	}
	if !oneOf(arguments[0], "blame", "cat-file", "describe", "diff", "grep", "log", "ls-files",
		"ls-tree", "merge-base", "rev-list", "rev-parse", "show", "status") {
		return false
	}
	for _, argument := range arguments[1:] {
		if anyPrefix(argument, "--ext-diff", "--open-files-in-pager", "-O") {
			return false
		}
	}
	return true
}

// ripgrepStartsNoProgram reports whether an rg names no preprocessor command.
func ripgrepStartsNoProgram(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "--pre" {
			return false
		}
		if strings.HasPrefix(argument, "--pre=") {
			return false
		}
	}
	return true
}

// sortStartsNoProgram reports whether a sort names no compression program.
func sortStartsNoProgram(arguments []string) bool {
	for _, argument := range arguments {
		if strings.HasPrefix(argument, "--compress-program") {
			return false
		}
	}
	return true
}

// commandOutsideReaders names what makes a line need the identity line: the
// first command that is not a reader, a command substitution, or a line the
// splitter cannot parse. It answers "" only when every command of the line is
// a reader, so a line the hook cannot see whole is never a reader line.
func commandOutsideReaders(command string) string {
	line, parsed := lineCommands(command)
	if !parsed {
		return "a line that ends inside a quote or after a backslash, which the hook cannot split into commands"
	}
	if line.substitutes {
		return "a command substitution (a backtick or `$(`), whose inner command the hook does not judge"
	}
	for _, words := range line.commands {
		judge, reader := readerCommands[words[0]]
		if !reader {
			return "`" + words[0] + "`, which is outside the reader set"
		}
		if judge == nil {
			continue
		}
		if !judge(words[1:]) {
			return "`" + strings.Join(words, " ") + "`, which the reader set admits only as a plain read"
		}
	}
	return ""
}

// namesIdentityVariable reports whether a word is one of the identity
// variables or an assignment to one.
func namesIdentityVariable(word string) bool {
	name, _, _ := strings.Cut(word, "=")
	return oneOf(name, agentVariable, sessionVariable)
}

// setsIdentity reports whether one command sets or unsets an identity
// variable. It sees exactly three forms:
//
//   - an assignment to one in front of the command word;
//   - one named as an argument of `declare`, `env`, `export`, `local`,
//     `readonly`, `typeset` or `unset`, which set and unset variables;
//   - text handed to `eval`, `bash`, `sh`, `zsh`, `source` or `.`, which run
//     it as shell: an argument holding an assignment to one, or `unset` with
//     one of the names (evaluatedTextSetsIdentity).
//
// A variable named as an argument of any other command, a grep pattern for
// one, changes nothing. A script file that sets a variable is not seen: the
// hook judges the command, not the files it runs.
func setsIdentity(words []string) bool {
	index := 0
	for index < len(words) && strings.Contains(words[index], "=") {
		if namesIdentityVariable(words[index]) {
			return true
		}
		index++
	}
	if index == len(words) {
		return false
	}
	arguments := words[index+1:]
	if oneOf(words[index], "declare", "env", "export", "local", "readonly", "typeset", "unset") {
		return slices.ContainsFunc(arguments, namesIdentityVariable)
	}
	if oneOf(words[index], "eval", "bash", "sh", "zsh", "source", ".") {
		return evaluatedTextSetsIdentity(strings.Join(arguments, " "))
	}
	return false
}

// evaluatedTextSetsIdentity reports whether text a shell will evaluate sets or
// unsets an identity variable. The text is one quoted word the splitter does
// not open, so it is judged as text: an assignment to either name anywhere in
// it, or the word `unset` together with either name.
func evaluatedTextSetsIdentity(text string) bool {
	if anyContains(text, agentVariable+"=", sessionVariable+"=") {
		return true
	}
	if !strings.Contains(text, "unset") {
		return false
	}
	return anyContains(text, agentVariable, sessionVariable)
}

// takesAnotherIdentity reports whether what follows the agent's identity line
// sets or unsets an identity variable again, in one of the forms setsIdentity
// sees. A remainder the splitter cannot parse is judged by its text: one that
// names either variable is refused, because the hook cannot see what it does
// with it.
func takesAnotherIdentity(rest string) bool {
	line, parsed := lineCommands(rest)
	if !parsed {
		return strings.Contains(rest, agentVariable) || strings.Contains(rest, sessionVariable)
	}
	return slices.ContainsFunc(line.commands, setsIdentity)
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
// A command made only of readers (readerCommands) passes without the line. A
// command that starts with the agent's exact line passes unless it sets an
// identity variable again. Every other command of a subagent is refused with
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
	if rest, carries := strings.CutPrefix(command, own); carries {
		if takesAnotherIdentity(rest) {
			return anotherIdentityVerdict(own, "After its identity line it sets or unsets "+agentVariable+" or "+sessionVariable+
				", or names one on a line the hook cannot split into commands.")
		}
		return nil
	}
	if anyPrefix(strings.TrimSpace(command), "export "+agentVariable+"=", "export "+sessionVariable+"=") {
		return anotherIdentityVerdict(own, "It starts with an identity line that is not this agent's: it names another agent or session, or leaves the agent out.")
	}
	outsider := commandOutsideReaders(command)
	if outsider == "" {
		return nil
	}
	return &verdict{2, red + bold + "BLOCKED: this subagent command needs the agent's identity line in front of it" + reset +
		"\n  It holds " + outsider + "." +
		"\n  Such a command can start ./le, and without the line ./le acts as the main thread: it shares the main thread's commit namespace and spec claim." +
		"\n  Next: type the command again with this line in front of it, exactly as printed. The hook checks the line and never adds it:" +
		"\n  " + own +
		"\n  A command made only of these readers needs no line (git only as `git <read verb>`): " +
		strings.Join(slices.Sorted(maps.Keys(readerCommands)), ", ")}
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
