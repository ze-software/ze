// Design: docs/architecture/cli/root-namespace-grammar.md -- which register a token belongs to
//
// flags.go mechanizes the flag-register rules of ai/rules/cli.md ("CLI Grammar:
// Keywords Before Values"), as pure functions beside the path-level rules in
// checker.go.
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
//	F3 a flag that RENDERS. Rendering is the pipe layer's job on every command,
//	   so the flag is banned whether or not the command's answer reaches that
//	   layer yet: how a command was registered is a fact about its wiring, not
//	   about the command
//	F4 a flag the parser reads that the flag registry never declared, or the
//	   reverse
//
// Each is a pure function over a population the caller collects, so the gate
// (internal/le/cligrammar) and a fixture test read the same rules.

package grammar

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
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

// namedRenderingFlags are the two rendering flag spellings no operator carries,
// so no catalog entry can name them. `--format` SELECTS among the rendering
// operators rather than being one of them, and `--no-header` drops the header
// the table renderer writes. ai/rules/cli.md bans both by name.
var namedRenderingFlags = []string{"format", "no-header"}

// renderingFlags are the flag spellings that do the pipe layer's rendering job.
// `--json` and `| json` are one job under two names and only one of them
// composes, so no command carries these.
//
// The operator spellings are DERIVED from the operator catalog
// (command.PipeOperatorCatalog, filtered by PipeOperator.Renders), which is the
// one statement of the operator language: an operator added to it is a banned
// flag spelling on the same commit, with nothing to hand-copy.
//
// The names are stored bare, as Go's flag package holds them; FlagName
// normalizes a token before the lookup.
var renderingFlags = func() map[string]bool {
	flags := make(map[string]bool, len(namedRenderingFlags)+len(operatorFlags))
	for _, name := range namedRenderingFlags {
		flags[name] = true
	}
	for name := range operatorFlags {
		flags[name] = true
	}
	return flags
}()

// operatorFlags are the rendering flag spellings an operator carries, which is
// what lets a finding name the exact pipe expression that does the flag's job.
var operatorFlags = func() map[string]bool {
	flags := map[string]bool{}
	for _, operator := range command.PipeOperatorCatalog() {
		if operator.Renders() {
			flags[operator.Name] = true
		}
	}
	return flags
}()

// renderingOperatorList spells the rendering operators as an operator types
// them, in the catalog's published order, for the finding that cannot name one
// operator because the flag names none.
var renderingOperatorList = func() string {
	var tb textbuf.Buffer
	for _, operator := range command.PipeOperatorCatalog() {
		if !operator.Renders() {
			continue
		}
		if tb.Len() > 0 {
			tb.Str(", ")
		}
		tb.Str("`| ").Str(operator.Name).Byte('`')
	}
	return tb.String()
}()

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

// ze point: cli/cli-grammar-keywords-before-values/flags-belong-to-the-offline-tooling-only
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

// ze point: cli/cli-grammar-keywords-before-values/flags-belong-to-the-offline-tooling-only
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

// ze point: cli/cli-grammar-keywords-before-values/flags-belong-to-the-offline-tooling-only
// CheckPipeFlags applies F3: no command in the `ze` command surface carries a
// flag that renders its answer, because rendering is the pipe layer's job.
//
// served says whether the path is registered through
// registry.MustRegisterLocalData, which decides the FIX and never the verdict.
// A registered path already reaches the pipe layer (command.ServeLocal renders
// the whole operator set over its answer), so the flag is a second spelling and
// the fix is to delete it. An unregistered path reaches no pipe layer, and that
// is the defect rather than the exemption: whether a command reaches the layer
// is a fact about how it was registered, so the fix is to register the answer
// and then delete the flag.
// pipeLoweringExempt is the one shape ai/rules/cli.md permits: a flag that sets
// a SESSION default and lowers into the pipe operator before dispatch, so the
// pipe layer still does the rendering. `ze cli --format json` is that flag, and
// commandWithFormat (internal/component/cli/client) is the lowering: it appends
// the operator to the command unless the operator is already there.
//
// It is keyed by path and flag rather than by flag alone, because the property
// that makes it legal lives in the caller and not in the token. A second entry
// needs its own lowering to point at.
var pipeLoweringExempt = map[string]bool{"cli --format": true}

func CheckPipeFlags(path string, parsed []string, served bool) []FlagFinding {
	var out []FlagFinding
	for _, name := range sortedUnique(parsed) {
		if !renderingFlags[FlagName(name)] {
			continue
		}
		if pipeLoweringExempt[path+" "+FlagToken(name)] {
			continue
		}
		out = append(out, FlagFinding{
			Command: path,
			Rule:    RuleFlagIsAPipe,
			Flag:    FlagToken(name),
			Message: pipeFlagMessage(path, FlagToken(name), FlagName(name), served),
		})
	}
	return out
}

// pipeFlagMessage states the fix the reader owes: which pipe expression does
// this flag's job, and whether the command's answer already reaches the layer
// that runs it.
func pipeFlagMessage(path, token, bare string, served bool) string {
	expression := msg("`", path, " | ", bare, "`")
	if !operatorFlags[bare] {
		// `--format` and `--no-header` name no operator, so the reader is
		// pointed at the operators that do their job instead.
		expression = msg("the rendering operators (", renderingOperatorList, ")")
	}
	if served {
		return msg("flag ", token, " is a second spelling of ", expression,
			", which this command's answer already reaches. Only the operator composes, so delete the flag")
	}
	return msg("flag ", token, " renders an answer that reaches no pipe layer, which is the defect and not the exemption. ",
		"Register the answer through registry.MustRegisterLocalData, which renders it through ", expression,
		", then delete the flag")
}

// ze point: cli/cli-grammar-keywords-before-values/flags-belong-to-the-offline-tooling-only
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
