// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: readerline.go -- the reader-line walk this one skips admitted readers with
// Related: session.go -- bashAgentIdentity, the caller, and the text route for a line that does not parse
package hookruntime

import (
	"path"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// identityWrapper declares one wrapper that runs the command after its
// options: the options that take one value as the next word, whether a count
// or a duration may stand before the command, how many operands stand
// between the options and the command, and the options that change the
// command's environment or user, which make the wrapper itself refused.
type identityWrapper struct {
	values   []string
	counts   bool
	operands int
	changes  []string
}

// identityWrappers are the commands that run the word after their options
// as a command, each with the options its manual gives a separate value.
// `time` is here as the external program, which `command time` and
// `/usr/bin/time` reach: the reserved word is structure in the tree, but a
// word after a wrapper is not. setsid, flock, strace and ltrace read their
// options from util-linux setsid(1) and flock(1), strace(1) 6.8 and ltrace(1)
// 0.7.3 (spec-agent-economy-6-shell-parser, phase 3). flock takes its lock
// file before the command, and strace's `-E` and `-u` and ltrace's `-u` set
// the command's environment or user, as env and sudo do. A long option is
// listed only when its manual gives it a required value, which getopt takes
// from the next word too (`--output out`); one whose value is optional
// (xargs `--max-lines[=N]`, strace `--quiet[=set]`) takes it only after `=`
// and is not listed. timeout takes one duration operand before the command,
// spelled any way (`5`, `1.5s`, `inf`, `1e3`), so it is an operand rather
// than a count (review I3).
//
// textChangesEnvironment, the judgment of a remainder that does not parse,
// reads this table too, so the two paths skip the same option values.
var identityWrappers = map[string]identityWrapper{
	"builtin": {},
	"command": {},
	"flock":   {values: []string{"-c", "--command", "-E", "--conflict-exit-code", "-w", "--wait", "--timeout"}, operands: 1},
	"ltrace": {values: []string{
		"-a", "-A", "-D", "-e", "-F", "-l", "-n", "-o", "-p", "-s", "-u", "-w", "-x",
		"--align", "--config", "--debug", "--indent", "--library", outputOption, "--where",
	}, changes: []string{"-u"}},
	niceCommand: {values: []string{"-n", "--adjustment"}, counts: true},
	"nohup":     {},
	"setsid":    {},
	"stdbuf":    {values: []string{"-i", "-o", "-e", "--input", outputOption, "--error"}},
	"strace": {values: []string{
		"-a", "-b", "-e", "-E", "-I", "-o", "-O", "-p", "-P", "-s", "-S", "-u", "-U", "-X",
		"--abbrev", "--attach", "--columns", "--const-print-style", "--detach-on", "--fault", "--inject",
		"--interruptible", "--kvm", outputOption, "--raw", "--read", signalOption, "--stack-trace-frame-limit",
		"--status", "--string-limit", "--summary-columns", "--summary-sort-by", "--summary-syscall-overhead",
		"--syscall-limit", "--trace", "--trace-fds", "--trace-path", "--verbose", "--write",
	}, changes: []string{"-E", "--env", "-u", "--user"}},
	timeCommand:    {values: []string{"-f", "-o", "--format", outputOption}},
	timeoutCommand: {values: []string{"-s", "-k", signalOption, "--kill-after"}, operands: 1},
	"xargs": {values: []string{
		"-a", "-d", "-E", "-I", "-L", "-n", "-P", "-s",
		"--arg-file", "--delimiter", "--max-args", "--max-procs", "--max-chars", "--process-slot-var",
	}},
}

// The words more than one wrapper or judge of this walk names.
const (
	outputOption  = "--output"
	signalOption  = "--signal"
	setBuiltin    = "set"
	printfBuiltin = "printf"
)

// identitySetOptions are the long `set -o` names that change no variable.
// With the letters e, u and x in their `-` and `+` forms they are every
// `set` that passes. `-a` exports later assignments, `-k` moves assignments
// into the environment, and an operand replaces the positional parameters.
var identitySetOptions = map[string]bool{"pipefail": true, "errexit": true, "nounset": true, "xtrace": true}

// startupVariable reports whether a variable makes bash, or a program bash
// starts, run code the line does not show: BASH_ENV and ENV name a script a
// non-interactive shell sources, SHELLOPTS and BASHOPTS set options every
// child bash starts with, PROMPT_COMMAND runs before each prompt, and a
// BASH_FUNC_ name exports a function. `BASH_ENV=./x.sh ./le x` runs the
// script inside the launcher before le starts.
func startupVariable(name string) bool {
	if oneOf(name, "BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "PROMPT_COMMAND") {
		return true
	}
	return strings.HasPrefix(name, "BASH_FUNC_")
}

// takesAnotherIdentity reports whether what follows the agent's identity
// line, own, may set, unset or drop an identity variable. It catches an agent
// that forgets or mistypes its identity, and it fails closed. On a line that
// parses, the statements after the agent's exports are judged from the tree
// (treeTakesAnotherIdentity). On any other line the remainder is judged by
// its text (textChangesEnvironment), and any mention of either variable is
// refused, because no command of it can be shown to be a reader. It is not a
// sandbox: it judges the command, not the files the command runs. A line
// whose tree names fewer commands than bash runs (treeMisreadsLine) is judged
// by its text, and a panic inside either judgment answers yes (review I4);
// readAnotherIdentity is the judgment without the recover.
func takesAnotherIdentity(command string, parse commandParse, own string) (takes bool) {
	defer func() {
		if recover() != nil {
			takes = true
		}
	}()
	return readAnotherIdentity(command, parse, own)
}

// readAnotherIdentity is takesAnotherIdentity without its recover.
func readAnotherIdentity(command string, parse commandParse, own string) bool {
	rest := strings.TrimPrefix(strings.TrimLeft(command, " \t\n"), own)
	if parse.parsed() && !strings.ContainsAny(command, "\r\x00") && treeMisreadsLine(command, parse) == "" {
		statements, outside := afterIdentityLine(command, parse.file.Stmts, own)
		if outside != "" {
			return true
		}
		return treeTakesAnotherIdentity(command, rest, statements)
	}
	if textChangesEnvironment(rest) {
		return true
	}
	return identityMentions(rest) != 0
}

// identityWalk judges what follows the agent's own identity line, command by
// command at every depth. It is not safe for concurrent use.
type identityWalk struct {
	source string
	// read counts the identity variables' names that lie inside admitted
	// readers, which read them as text.
	read  int
	depth int
}

// treeTakesAnotherIdentity reports whether the statements after the agent's
// identity line may set, unset or drop an identity variable: a command that
// changes the environment by its form, or a name of either variable outside
// an admitted reader. source is the parsed command, which the nodes' offsets
// index, and rest the text after the line, whose mentions the rule counts.
// It fails closed: a node of a kind the walk does not list, a word it cannot
// read and a command word the shell expands each answer yes.
func treeTakesAnotherIdentity(source, rest string, statements []*syntax.Stmt) bool {
	walk := identityWalk{source: source}
	if walk.statements(statements) {
		return true
	}
	return walk.read < identityMentions(rest)
}

func (walk *identityWalk) statements(statements []*syntax.Stmt) bool {
	return slices.ContainsFunc(statements, walk.statement)
}

// statement judges one statement: skipped whole when it is an admitted
// reader line of its own, else judged by its kind and by every substitution
// and expansion its words hold. The recursion is bounded by readerDepthMax.
func (walk *identityWalk) statement(statement *syntax.Stmt) bool {
	walk.depth++
	defer func() { walk.depth-- }()
	if walk.depth > readerDepthMax {
		return true
	}
	reader := readerWalk{source: walk.source, admit: readerAdmit}
	if reader.statement(statement, readerPlace{}) == "" {
		walk.read += readerMentions(&reader, statement)
		return false
	}
	if walk.wordsChange(statement) {
		return true
	}
	return walk.command(statement)
}

// wordsChange judges what the words of a statement run and assign: the
// statements of each command or process substitution, which are judged as
// commands of their own, an arithmetic expansion, which can assign a name a
// value builds (`$(( $n = 1 ))`), an assigning default (`${n:=x}`), an
// index or a slice (`${a[$n=7]}`, `${x:$n=7}`), whose subscript or offsets
// bash evaluates as arithmetic, a `[[` operator outside the reader set
// (`-eq`, `-v`), which evaluates its operand the same way, and a `{name}>`
// redirection, which assigns the variable it names (review I2).
func (walk *identityWalk) wordsChange(statement *syntax.Stmt) bool {
	changes := false
	syntax.Walk(statement, func(node syntax.Node) bool {
		if changes {
			return false
		}
		switch node := node.(type) {
		case *syntax.Stmt:
			// A statement inside a construct is judged when command reaches
			// it, so its readers are counted once.
			return node == statement
		case *syntax.CmdSubst:
			changes = walk.statements(node.Stmts)
			return false
		case *syntax.ProcSubst:
			changes = walk.statements(node.Stmts)
			return false
		case *syntax.ArithmExp:
			changes = true
		case *syntax.ParamExp:
			changes = paramExpansionAssigns(node)
		case *syntax.UnaryTest:
			changes = !testUnaryAdmitted[node.Op]
		case *syntax.BinaryTest:
			changes = !testBinaryAdmitted[node.Op]
		case *syntax.Redirect:
			if node.N != nil && strings.Trim(node.N.Value, "0123456789") != "" {
				changes = true
			}
		}
		return !changes
	})
	return changes
}

// paramExpansionAssigns reports whether a parameter expansion can assign a
// variable: an index or a slice, evaluated as arithmetic, or an assigning
// default.
func paramExpansionAssigns(expansion *syntax.ParamExp) bool {
	if expansion.Index != nil {
		return true
	}
	if expansion.Slice != nil {
		return true
	}
	if expansion.Exp == nil {
		return false
	}
	if expansion.Exp.Op == syntax.AssignUnset {
		return true
	}
	return expansion.Exp.Op == syntax.AssignUnsetOrNull
}

// readerMentions counts the identity variables' names in what an admitted
// reader receives: its words, outermost only so a word inside a substitution
// is not counted twice, and its here-document bodies and here-strings, which
// are words of its redirections (review B4). The statement's own span is not
// counted: with a here-document it runs to the body's end, past every command
// typed after the operator that follows the reader on its line.
func readerMentions(reader *readerWalk, statement *syntax.Stmt) int {
	mentions := 0
	syntax.Walk(statement, func(node syntax.Node) bool {
		word, ok := node.(*syntax.Word)
		if !ok {
			return true
		}
		if len(word.Parts) != 0 {
			mentions += identityMentions(reader.text(word))
		}
		return false
	})
	return mentions
}

// command judges the command of a statement that is not an admitted reader.
func (walk *identityWalk) command(statement *syntax.Stmt) bool {
	switch command := statement.Cmd.(type) {
	case nil:
		return false
	case *syntax.CallExpr:
		return walk.call(command)
	case *syntax.BinaryCmd:
		if walk.statement(command.X) {
			return true
		}
		return walk.statement(command.Y)
	case *syntax.Block:
		return walk.statements(command.Stmts)
	case *syntax.Subshell:
		return walk.statements(command.Stmts)
	case *syntax.IfClause:
		for clause := command; clause != nil; clause = clause.Else {
			if walk.statements(clause.Cond) {
				return true
			}
			if walk.statements(clause.Then) {
				return true
			}
		}
		return false
	case *syntax.WhileClause:
		if walk.statements(command.Cond) {
			return true
		}
		return walk.statements(command.Do)
	case *syntax.ForClause:
		// The loop variable is judged by the mention rule; an arithmetic
		// `for (( ))` can assign any name its expressions build.
		if _, words := command.Loop.(*syntax.WordIter); !words {
			return true
		}
		return walk.statements(command.Do)
	case *syntax.CaseClause:
		for _, item := range command.Items {
			if walk.statements(item.Stmts) {
				return true
			}
		}
		return false
	case *syntax.TimeClause:
		if command.Stmt == nil {
			return false
		}
		return walk.statement(command.Stmt)
	case *syntax.TestClause:
		return false
	default:
		// A declaration (export, declare, typeset, readonly, local,
		// nameref) is refused whatever it names. A function definition, an
		// arithmetic command, `let` and a coprocess can each assign a
		// variable whose name a value builds (`(( $n = 1 ))`), so they are
		// refused too, and so is every kind this walk does not list.
		return true
	}
}

// call judges one simple command that is not an admitted reader.
func (walk *identityWalk) call(call *syntax.CallExpr) bool {
	for _, assign := range call.Assigns {
		if assign.Index != nil {
			return true
		}
		if assign.Array != nil {
			return true
		}
		if assign.Naked {
			return true
		}
		if assign.Name == nil {
			return true
		}
		if startupVariable(assign.Name.Value) {
			return true
		}
		// An assignment to either identity variable in front of a command
		// gives that command another identity (review B4). The agent's own
		// exports are not here: they end inside its identity line.
		if oneOf(assign.Name.Value, agentVariable, sessionVariable) {
			return true
		}
	}
	if len(call.Args) == 0 {
		return false
	}
	reader := readerWalk{source: walk.source}
	if strings.Contains(reader.text(call.Args[0]), "=") {
		// A command word that holds `=` but is no assignment (`"A"=1`,
		// `A\=1`) is refused, as changesEnvironment refuses it by its text.
		return true
	}
	words := make([]wordValue, 0, len(call.Args))
	for _, argument := range call.Args {
		value, err := readWord(argument)
		if err != nil {
			return true
		}
		words = append(words, value)
	}
	at, changes := commandWordAt(words)
	if changes {
		return true
	}
	if at >= len(words) {
		// The words end before a command word: a wrapper with nothing to
		// run, or one whose last option wanted a value (`xargs -n`).
		return false
	}
	if words[at].expands {
		// A command word the shell expands can be any tool (`c=env; $c -i`),
		// so it is refused whatever it would expand to.
		return true
	}
	tool := path.Base(words[at].text)
	if oneOf(tool, "test", "[") {
		return testEvaluatesSubscript(words[at+1:], unquotedExpansion(call.Args[at+1:]))
	}
	return toolChangesEnvironment(tool, words[at+1:])
}

// testEvaluatesSubscript reports whether a `test` or `[` after the identity
// line may evaluate an array subscript as arithmetic, which can assign a
// variable whose name a value builds (`test -v "a[$n=7]"`, review I2): an
// operand `-v` or `-R`, or an expanded word that may stand for one. One
// expanded word cannot be both the operator and the subscript it evaluates,
// so a test with one expanded word passes unless the shell may split it
// (splits) or another word holds a `[`.
func testEvaluatesSubscript(arguments []wordValue, splits bool) bool {
	expanded, bracket := 0, false
	for _, argument := range arguments {
		if oneOf(argument.text, "-v", "-R") {
			return true
		}
		if argument.expands {
			expanded++
		}
		if strings.ContainsRune(argument.text, '[') {
			bracket = true
		}
	}
	if expanded == 0 {
		return false
	}
	if splits {
		return true
	}
	if bracket {
		return true
	}
	return expanded > 1
}

// unquotedExpansion reports whether a word holds an expansion outside
// quotes, whose result the shell splits into words.
func unquotedExpansion(words []*syntax.Word) bool {
	for _, word := range words {
		for _, part := range word.Parts {
			switch part.(type) {
			case *syntax.ParamExp, *syntax.CmdSubst, *syntax.ArithmExp, *syntax.ProcSubst:
				return true
			}
		}
	}
	return false
}

// commandWordAt answers the index of the command word past every wrapper:
// after a wrapper the walk skips an option, the one value of a value-taking
// option (wrapperOption), a count for a wrapper that takes one
// (wrapperNumber, which requires a digit, so a lone `.` is a command word),
// and the wrapper's operands. A wrapper is matched by its base name. It
// answers len(words) when the words end before a command word, never more,
// and true when a wrapper option changes the command's environment or user.
func commandWordAt(words []wordValue) (int, bool) {
	index := 0
	for index < len(words) {
		if words[index].expands {
			return index, false
		}
		wrapper, isWrapper := identityWrappers[path.Base(words[index].text)]
		if !isWrapper {
			return index, false
		}
		end, changes := wrapperArgumentsEnd(wrapper, words, index+1)
		if changes {
			return end, true
		}
		index = end
	}
	return index, false
}

// wrapperArgumentsEnd answers the index of the first word after a wrapper's
// options, values, counts and operands, starting at index, and whether one
// of its options changes the command's environment or user. An option whose
// value the words do not hold ends them: the answer is len(words) at most
// (review B1).
func wrapperArgumentsEnd(wrapper identityWrapper, words []wordValue, index int) (int, bool) {
	operands := wrapper.operands
	for index < len(words) {
		word := words[index].text
		if strings.HasPrefix(word, "-") && word != "-" {
			next, changes := wrapperOption(wrapper, word)
			if changes {
				return index, true
			}
			index++
			if next {
				index++
			}
			continue
		}
		if wrapper.counts && wrapperNumber(word) {
			index++
			continue
		}
		if operands > 0 {
			operands--
			index++
			continue
		}
		return index, false
	}
	return min(index, len(words)), false
}

// wrapperOption reads one option word of a wrapper, as getopt reads it:
// whether the next word is its value, and whether it changes the command's
// environment or user. A long option is named whole and takes the next word
// unless its value follows `=`. A short word is a cluster (`-fo`, `-0n`) read
// letter by letter: a letter that takes a value takes the rest of the
// cluster, or the next word when it ends the cluster (review I3).
func wrapperOption(wrapper identityWrapper, word string) (next, changes bool) {
	if strings.HasPrefix(word, "--") {
		name, _, valued := strings.Cut(word, "=")
		if oneOf(name, wrapper.changes...) {
			return false, true
		}
		return !valued && oneOf(name, wrapper.values...), false
	}
	for index := 1; index < len(word); index++ {
		letter := "-" + word[index:index+1]
		if oneOf(letter, wrapper.changes...) {
			return false, true
		}
		if oneOf(letter, wrapper.values...) {
			return index == len(word)-1, false
		}
	}
	return false, false
}

// toolChangesEnvironment judges the command word past the wrappers, by its
// base name: an environment tool is refused, `set` passes with only the
// options that change no variable (AC-31), and `printf` is refused with
// `-v`, which assigns the variable it names. read, mapfile, readarray,
// getopts, trap and enable are refused too: each assigns a variable whose
// name a value can build, runs a string later in this shell, or loads code
// into it, and an admitted `read` (readReads) was skipped as a reader before
// it reached this walk. Those six and `let` are refused by their command
// word alone, whatever their arguments name.
func toolChangesEnvironment(tool string, arguments []wordValue) bool {
	if tool == setBuiltin {
		return !setChangesNothing(arguments)
	}
	if environmentTools[tool] {
		return true
	}
	if tool == printfBuiltin {
		return printfAssigns(arguments)
	}
	return assigningTool(tool)
}

// assigningTool reports a builtin that assigns a variable whose name a value
// can build, runs a string later in this shell, or loads code into it.
func assigningTool(tool string) bool {
	return oneOf(tool, "read", "mapfile", "readarray", "getopts", "trap", "enable", "let")
}

// printfAssigns reports whether a printf's options hold `-v`, or a word the
// shell expands where an option can stand.
func printfAssigns(arguments []wordValue) bool {
	for _, argument := range arguments {
		if argument.expands {
			return true
		}
		if strings.HasPrefix(argument.text, "-v") {
			return true
		}
		if !strings.HasPrefix(argument.text, "-") {
			return false
		}
	}
	return false
}

// setChangesNothing reports whether a `set` holds at least one option and
// only the letters e, u and x and the names identitySetOptions holds, in
// their `-` and `+` forms. A cluster may end in `o`, whose value is the next
// word, so `set -euo pipefail` passes.
func setChangesNothing(arguments []wordValue) bool {
	if len(arguments) == 0 {
		return false
	}
	for index := 0; index < len(arguments); index++ {
		word := arguments[index].text
		if arguments[index].expands {
			return false
		}
		if len(word) < 2 {
			return false
		}
		if word[0] != '-' && word[0] != '+' {
			return false
		}
		letters, named := strings.CutSuffix(word[1:], "o")
		if strings.Trim(letters, "eux") != "" {
			return false
		}
		if !named {
			continue
		}
		index++
		if index == len(arguments) {
			return false
		}
		if arguments[index].expands {
			return false
		}
		if !identitySetOptions[arguments[index].text] {
			return false
		}
	}
	return true
}
