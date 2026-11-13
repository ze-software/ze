// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
package hookruntime

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path"
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
// parent's session id. The start text prints this line and the identity check
// compares a command with it, so the two cannot disagree. Every caller holds
// an agent id: a payload that names none is the main thread's (fromSubagent),
// which types no line.
func identityLine(agent, session string) string {
	return "export " + agentVariable + "=" + agent + "; export " + sessionVariable + "=" + session + "; "
}

// commandOutsideReaders names what makes a line need the identity line: the
// first construct or command of the call's one parse (shellparse.go) that
// lies outside the reader line (judgeReaderLine, readerline.go), or the
// reason the line did not parse. It answers "" only when the line parses as
// bash and every node of its tree is one the walk admits, every simple
// command is a reader its judge admits (readerCommand, readers.go) and no
// redirection writes a file, so a line the hook cannot read whole is never a
// reader line, and a reader cannot write the git or shell configuration a
// later reader would run. The main thread's caller passes no identity line:
// a subagent's line that carries one is judged by bashAgentIdentity first.
func commandOutsideReaders(command string, parse commandParse) string {
	_, outside := judgeReaderLine(command, parse, "")
	return outside
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

// textReservedWords are the shell's reserved words that run the word after
// them as a command. Only the text route reads them (changesEnvironment): a
// remainder that does not parse has no tree to carry them as structure.
var textReservedWords = map[string]bool{
	"if": true, "then": true, "elif": true, "else": true, "do": true, "while": true, "until": true,
}

// changesEnvironment judges one command of a remainder that does not parse,
// given as its words as typed (raw) and with quotes removed (words, parallel
// to raw): whether it may change the environment the agent's identity lives
// in. An assignment in front of the command word whose name is not a plain
// name (a quote, a backslash, an expansion or a subscript in it) or that
// makes bash run code (startupVariable) is refused. Past the reserved words
// and the wrappers, each wrapper with its declared value options, counts and
// operands (identityWrappers), the command word is judged by its base name
// (toolChangesEnvironment), and `set` is refused with any option, because no
// tree shows what follows it. A plain `FOO=bar ./le x` passes.
func changesEnvironment(raw, words []string) bool {
	var wrapper identityWrapper
	wrapped, operands := false, 0
	for index := 0; index < len(words); index++ {
		word := words[index]
		typed := word
		if index < len(raw) {
			typed = raw[index]
		}
		if strings.Contains(typed, "=") && !strings.HasPrefix(typed, "-") {
			if !plainAssignment(typed) {
				return true
			}
			name, _, _ := strings.Cut(typed, "=")
			if startupVariable(strings.TrimSuffix(name, "+")) {
				return true
			}
			continue
		}
		if !wrapped && textReservedWords[word] {
			continue
		}
		if next, isWrapper := identityWrappers[path.Base(word)]; isWrapper {
			wrapper, wrapped, operands = next, true, next.operands
			continue
		}
		if wrapped && strings.HasPrefix(word, "-") && word != "-" {
			next, changes := wrapperOption(wrapper, word)
			if changes {
				return true
			}
			if next {
				index++
			}
			continue
		}
		if wrapped && wrapper.counts && wrapperNumber(word) {
			continue
		}
		if wrapped && operands > 0 {
			operands--
			continue
		}
		return textToolChangesEnvironment(path.Base(word), words[index+1:])
	}
	return false
}

// textToolChangesEnvironment judges the command word of a remainder that
// does not parse: every `set` is refused, a `test` or `[` as the tree route
// judges it with every word holding `$` or a backtick taken as an expansion
// the shell may split (testEvaluatesSubscript), and every other tool as the
// tree route judges it (toolChangesEnvironment).
func textToolChangesEnvironment(tool string, words []string) bool {
	if tool == setBuiltin {
		return true
	}
	test := oneOf(tool, "test", "[")
	arguments := make([]wordValue, 0, len(words))
	for _, word := range words {
		arguments = append(arguments, wordValue{text: word, expands: test && strings.ContainsAny(word, "$`")})
	}
	if test {
		return testEvaluatesSubscript(arguments, true)
	}
	return toolChangesEnvironment(tool, arguments)
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

// nameStart reports whether a byte can start a shell variable name.
func nameStart(character byte) bool {
	return character == '_' || ('A' <= character && character <= 'Z') || ('a' <= character && character <= 'z')
}

// nameEnd answers the offset after the variable-name bytes that start at
// index.
func nameEnd(line string, index int) int {
	for index < len(line) {
		character := line[index]
		if nameStart(character) || ('0' <= character && character <= '9') {
			index++
			continue
		}
		break
	}
	return index
}

// textChangesEnvironment judges a remainder that does not parse by its text:
// each piece between `;`, `&`, `|`, a newline, a bracket, a brace, `!` or a
// backtick is a command whose whitespace fields, with quotes and backslashes
// removed, changesEnvironment judges. It fails closed: a split the shell does
// not make only adds commands to judge.
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

// fromSubagent reports whether the hook call comes from a subagent, which is
// exactly when its payload names an agent. A fork is such a caller: the
// harness sends a fork's calls with an agent id, like any other subagent's.
// It is the one answer to that question: the identity check
// (bashAgentIdentity), the advice prefix (commandIdentityPrefix) and the
// heavy-work check (bashSubagentHeavy) all read it, so no two of them can
// judge one caller two ways. It reads the payload alone, never the hook's
// environment and never text the agent typed: CLAUDE_CODE_FORK_SUBAGENT is a
// setting of the harness, present in the main thread's hook process too, so
// reading it judged the main thread as a subagent
// (plan/journal/gate-verdict-depends-on-the-machine.md, 2026-11-13).
func fromSubagent(ctx context) bool {
	return ctx.payload.AgentID != ""
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
// A reader line (commandOutsideReaders: every command of the parsed line a
// reader its judge admits, readers.go) passes without the line. A command
// that starts with the agent's exact line passes unless what follows it
// changes the environment or names an identity variable outside a reader
// (takesAnotherIdentity, afterline.go). Every other command of a subagent is refused with
// the line to type. The line is built from the payload's ids and never taken
// from the command, so a line naming another agent or session is refused.
//
// An agent id that cannot name a file refuses every call, because without its
// own identity the command would run as the main thread, which is the sharing
// this exists to end. A payload with an agent id and no usable session id
// refuses the same way. The main thread, the caller whose payload names no
// agent, is the identity le resolves by default and is never judged.
func bashAgentIdentity(ctx context) *verdict {
	if !fromSubagent(ctx) {
		return nil
	}
	agent := ctx.payload.AgentID
	session, _ := payloadSessionID(ctx.payload)
	if session == "" {
		return &verdict{2, red + bold + "BLOCKED: this subagent's hook payload carries no usable session id" + reset +
			"\n  agent " + strconv.Quote(agent) + " arrived with a session_id that is absent or not a file name." +
			"\n  With no identity line to type, the command would act as the main thread and share its commit namespace and spec claim." +
			"\n  Next: stop and report the payload's session_id to the main thread. An id holds only letters, digits, '.', '_' and '-'."}
	}
	if _, err := lepath.AgentFingerprint(session, agent); err != nil {
		return &verdict{2, red + bold + "BLOCKED: this subagent's identity cannot name its own files" + reset +
			"\n  " + err.Error() +
			"\n  With no identity line to type, the command would act as the main thread and share its commit namespace and spec claim." +
			"\n  Next: stop and report the agent id to the main thread. An id holds only letters, digits, '.', '_' and '-'."}
	}
	command := stringInput(ctx.input, "command")
	own := identityLine(agent, session)
	// Leading whitespace runs nothing, so the agent's own exact line after it
	// is the agent's line (review N4); every other check reads it as typed.
	if strings.HasPrefix(strings.TrimLeft(command, " \t\n"), own) {
		if takesAnotherIdentity(command, ctx.commandParse(), own) {
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
// advises must start with: the caller's own line for a subagent, so that
// following the advice is not refused by bashAgentIdentity, and "" for the
// main thread. A payload whose identity cannot be typed answers "" too,
// because bashAgentIdentity refuses that caller's every command with
// stop-and-report before any advice applies.
func commandIdentityPrefix(ctx context) string {
	if !fromSubagent(ctx) {
		return ""
	}
	agent := ctx.payload.AgentID
	session, _ := payloadSessionID(ctx.payload)
	if session == "" {
		return ""
	}
	if _, err := lepath.AgentFingerprint(session, agent); err != nil {
		return ""
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
