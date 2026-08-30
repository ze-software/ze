// Design: docs/architecture/cli/root-namespace-grammar.md -- which register a token belongs to
//
// flags.go mechanizes the flag-register rules of ai/rules/cli.md ("`--flag` or
// Keyword: Which Register a Token Belongs To"), as pure functions beside the
// path-level rules in checker.go.
//
// R1-R9 govern the YANG command model, where a flag is banned outright. The
// four rules here govern the OFFLINE surface, where a flag is legal and the
// question is which token may wear one:
//
//	F1 a flag that is a COMMAND NAME -- it enters no tree, so completion,
//	   `ze help command` and every grammar feeder are blind to it
//	F2 a flag a client builds into a command string it SENDS TO THE DAEMON --
//	   (*Dispatcher).Dispatch refuses it before any handler runs, so the command
//	   fails on every invocation while both halves read as finished code
//	F3 a flag that is a second spelling of a PIPE OPERATOR on a command whose
//	   answer already reaches the pipe layer
//	F4 a flag the parser reads that the flag registry never declared, or the
//	   reverse
//
// Each is a pure function over a population the caller collects, so the gate
// (internal/le/cligrammar) and a fixture test read the same rules.

package grammar

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The flag-register rule ids. They are deliberately not R10-R13: R1-R9 are one
// ruleset over the YANG command model, and these four judge a different
// surface under a different question.
const (
	// RuleFlagIsACommand is F1: a command name spelled as a flag.
	RuleFlagIsACommand = "F1"
	// RuleFlagToTheDaemon is F2: a client sending a flag to the daemon.
	RuleFlagToTheDaemon = "F2"
	// RuleFlagIsAPipe is F3: a flag doing a pipe operator's job.
	RuleFlagIsAPipe = "F3"
	// RuleFlagUndeclared is F4: a flag the registry and the parser disagree on.
	RuleFlagUndeclared = "F4"
)

// universalFlags are the four spellings ai/rules/cli.md exempts by name,
// because every Unix program answers them and a person meeting `ze` types one
// before any help exists to tell them otherwise. They are ADDITIVE: `ze
// version` and `ze help` stay the canonical commands, and nothing else joins
// this set.
var universalFlags = map[string]bool{
	"--version": true,
	"-V":        true,
	"--help":    true,
	"-h":        true,
}

// pipeFlags are the flag spellings of a pipe operator. `--json` and `| json`
// are one job under two names and only one of them composes, so a command
// whose answer already reaches the pipe layer must not carry these.
//
// The names are stored bare, as Go's flag package holds them; FlagName
// normalizes a token before the lookup.
var pipeFlags = map[string]bool{
	"json":   true,
	"yaml":   true,
	"table":  true,
	"text":   true,
	"format": true,
}

// FlagShaped reports whether a token is a flag rather than a value.
//
// It is the predicate (*Dispatcher).Dispatch refuses a daemon command by
// (internal/component/plugin/server/command.go), and the two MUST stay one
// function: a gate that judged flag shape differently from the daemon would
// pass a command the daemon rejects.
//
// Deliberately NOT flag-shaped: "-" alone, a conventional stdin token; "--",
// the end-of-options marker; and "-5" and friends, so a value that merely looks
// negative is a value.
func FlagShaped(token string) bool {
	rest, dashed := strings.CutPrefix(token, "-")
	if !dashed {
		return false
	}
	if after, dashdash := strings.CutPrefix(rest, "-"); dashdash {
		rest = after
	}
	if rest == "" {
		return false
	}
	letter := rest[0]
	return (letter >= 'a' && letter <= 'z') || (letter >= 'A' && letter <= 'Z')
}

// FlagName answers the bare name of a flag token: "--json" and "json" both
// answer "json". Go's flag package registers a bare name and an operator types
// the dashed token, so every comparison between the two goes through here.
func FlagName(token string) string {
	return strings.TrimLeft(token, "-")
}

// FlagToken answers the token an operator types for a bare flag name: "u"
// answers "-u" and "user" answers "--user", the spelling Go's flag package
// prints and registry.FlagSpec records.
func FlagToken(name string) string {
	bare := FlagName(name)
	var tb textbuf.Buffer
	if len(bare) == 1 {
		return tb.Byte('-').Str(bare).String()
	}
	return tb.Str("--").Str(bare).String()
}

// FlagFinding is one flag-register violation: which command carries it, which
// rule it breaks, and the flag token itself.
//
// It is separate from Finding because the flag token is the thing a reader acts
// on and a debt entry is keyed by, and a path-level Finding has nowhere to hold
// it.
type FlagFinding struct {
	Command string
	Rule    string
	Flag    string
	Message string
}

// ze point: cli/cli-grammar-keywords-before-values/keep-the-flag-form-to-the-tools-that-reach-no-daemon
// ze point: cli/cli-grammar-keywords-before-values/what-a-flag-must-never-be
// CheckRootFlagForm applies F1 to the registered root command names: a root
// whose name is a flag is a verb in disguise.
//
// CheckRootNamespace deliberately skips a leading-hyphen root, because R9 asks
// about namespaces and a flag has no left segment to collide. This is the check
// that governs the shape itself, and the four universal flags are its stated
// exception.
func CheckRootFlagForm(roots []string) []FlagFinding {
	var out []FlagFinding
	for _, name := range roots {
		if !FlagShaped(name) || universalFlags[name] {
			continue
		}
		out = append(out, FlagFinding{
			Command: name,
			Rule:    RuleFlagIsACommand,
			Flag:    name,
			Message: msg("root ", name, " is a command spelled as a flag -- it enters no tree, so completion, ",
				"`ze help command` and the grammar feeders never see it; register the keyword ",
				FlagName(name), " instead. Only --version, -V, --help and -h wear a flag"),
		})
	}
	return out
}

// ze point: cli/cli-grammar-keywords-before-values/never-send-a-flag-from-a-client-to-the-daemon
// DaemonCommandFlag applies F2 to one string literal: it answers the daemon
// command path the literal names and the first flag-shaped token that follows
// that path.
//
// daemonPaths holds every command path the daemon dispatches, in both the
// spelling that carries the verb ("request interface migrate") and the spelling
// a client that supplies the verb itself uses ("interface migrate"). A match
// needs at least two tokens, so a literal opening with a bare verb is not read
// as a command.
//
// Only tokens BEFORE the first flag are read as the path, which is what makes
// this answer the client's own command string rather than any sentence holding
// a flag.
func DaemonCommandFlag(text string, daemonPaths map[string]bool) (path, flag string, found bool) {
	fields := strings.Fields(text)
	first := -1
	for index, token := range fields {
		if FlagShaped(token) {
			first = index
			break
		}
	}
	if first < 2 {
		// No flag at all, or no room for a two-token path in front of it.
		return "", "", false
	}

	var tb textbuf.Buffer
	longest := ""
	for end := 2; end <= first; end++ {
		candidate := tb.Reset().Join(fields[:end], " ").String()
		if daemonPaths[candidate] {
			longest = candidate
		}
	}
	if longest == "" {
		return "", "", false
	}
	return longest, fields[first], true
}

// ClientFlagFinding renders the F2 verdict DaemonCommandFlag answered.
func ClientFlagFinding(path, flag string) FlagFinding {
	return FlagFinding{
		Command: path,
		Rule:    RuleFlagToTheDaemon,
		Flag:    flag,
		Message: msg("the command string carries ", flag,
			" -- (*Dispatcher).Dispatch refuses a flag-shaped token before any handler runs, so this command fails on every invocation; send the keyword form the daemon's grammar declares"),
	}
}

// ze point: cli/cli-grammar-keywords-before-values/what-a-flag-must-never-be
// CheckPipeFlags applies F3: a command whose answer already reaches the pipe
// layer must not carry a flag that renders it.
//
// The caller passes only a path it has established is served with data
// (registry.MustRegisterLocalData, rendered by command.ServeLocal), because
// that is what makes `| json` available and the flag a duplicate.
func CheckPipeFlags(path string, parsed []string) []FlagFinding {
	var out []FlagFinding
	for _, name := range sortedUnique(parsed) {
		if !pipeFlags[FlagName(name)] {
			continue
		}
		out = append(out, FlagFinding{
			Command: path,
			Rule:    RuleFlagIsAPipe,
			Flag:    FlagToken(name),
			Message: msg("flag ", FlagToken(name), " renders an answer this command already serves through the pipe layer -- `",
				path, " | ", FlagName(name), "` is the same job, and only the operator composes"),
		})
	}
	return out
}

// ze point: cli/cli-grammar-keywords-before-values/declare-every-offline-flag-through-the-flag-registry
// CheckFlagDeclarations applies F4 to one command path: every flag its parser
// reads is declared through registry.RegisterCommandFlags, and every flag that
// registry declares is read by a parser.
//
// parsed are the flag names the Go flag package is given; declared are the
// tokens the registry holds. Both are normalized, so "-u" and "u" are one flag.
// Drift runs in both directions and each direction has its own consequence: a
// parsed flag nobody declared is invisible to completion, and a declared flag
// nobody parses is help promising a token the handler drops.
func CheckFlagDeclarations(path string, parsed, declared []string) []FlagFinding {
	declaredNames := make(map[string]bool, len(declared))
	for _, token := range declared {
		declaredNames[FlagName(token)] = true
	}
	parsedNames := make(map[string]bool, len(parsed))
	for _, token := range parsed {
		parsedNames[FlagName(token)] = true
	}

	var out []FlagFinding
	for _, name := range sortedUnique(parsed) {
		if declaredNames[FlagName(name)] {
			continue
		}
		out = append(out, FlagFinding{
			Command: path,
			Rule:    RuleFlagUndeclared,
			Flag:    FlagToken(name),
			Message: msg("flag ", FlagToken(name), " is parsed but not declared -- completion offers what registry.RegisterCommandFlags holds, so this flag is invisible to the operator"),
		})
	}
	for _, name := range sortedUnique(declared) {
		if parsedNames[FlagName(name)] {
			continue
		}
		out = append(out, FlagFinding{
			Command: path,
			Rule:    RuleFlagUndeclared,
			Flag:    FlagToken(name),
			Message: msg("flag ", FlagToken(name), " is declared but no parser reads it -- help offers a token the handler drops"),
		})
	}
	return out
}

// sortedUnique answers the names once each, in order, so a finding set is
// stable whatever order the scan collected them in.
func sortedUnique(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		bare := FlagName(name)
		if seen[bare] {
			continue
		}
		seen[bare] = true
		out = append(out, name)
	}
	sort.Slice(out, func(i, j int) bool { return FlagName(out[i]) < FlagName(out[j]) })
	return out
}
