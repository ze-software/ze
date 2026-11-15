// Design: ai/rules/context-economy.md -- the figures the agent-economy criteria are measured with
// Related: baseline.go -- the figures that read what this keeps
// Related: tokeneconomy.go -- the scan that calls readShell for each Bash call
//
// shell.go reads the command of a Bash call into the few facts the baseline
// figures need. Of the command text it keeps one word, the file a shell read
// names (Shell.Reader), in memory only, to compare with the file of the
// thread's next call. The report never prints or serializes that word or any
// other part of the command (ToolCall.Shell carries no JSON key), because a
// command can carry anything a session typed.

package aitokens

import (
	"path"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Shell is what the baseline figures need from one Bash call's command. Its
// zero value is a call whose command was not read (Parsed false): every shell
// figure counts such a call apart, never as "not a read" or "not a test".
type Shell struct {
	// Parsed is true when the call carried a command and it parsed as bash.
	Parsed bool
	// Reader is the file the first grep, sed or cat of the command that names
	// one reads, as typed; empty when no such command names a file.
	Reader string
	// Help is true when a command of the line runs `le help`.
	Help bool
	// CommitCreate is true when a command of the line runs `le commit create`.
	CommitCreate bool
	// Job is true when a command of the line runs a test or a build: one of
	// leJobs, or one of goJobVerbs.
	Job bool
	// Background is the call's `run_in_background` input.
	Background bool
}

// shellCommandMax is the longest command, in bytes, the report parses; a
// longer one counts as unparsed and is not handed to the parser. The vendored
// parser (mvdan.cc/sh/v3/syntax) has no nesting limit, and each nesting level
// costs it a recursion, so the input decides its stack depth. Go ends the
// process when a goroutine's stack is exhausted, and recover cannot catch
// that, so the bound must come before the parse. At one level per byte at
// most, 64 KiB bounds the depth at 65,536 levels; an agent's command line is
// far shorter, and the bound gives up only a heredoc that writes more.
const shellCommandMax = 64 << 10

// readerVerb is what the scan of one shell file read needs to find its file.
// least is the number of operands a call needs before its last one is a file
// when no option gave the pattern: grep and sed take a pattern or a script
// first. valueShort and valueLong are the options whose value is a word of
// its own: a short letter whose value is the next word when the letter ends
// its cluster (`-A 3`) and the rest of the cluster otherwise (`-A3`), and a
// long name whose value is the next word unless written `--name=value`.
// patternShort and patternLong are those of them that give the pattern or
// the script, after which one operand is the file.
type readerVerb struct {
	least        int
	valueShort   string
	valueLong    []string
	patternShort string
	patternLong  []string
}

// readerVerbs are the shell file reads the umbrella's baseline names. The
// value-taking options listed are the ones that change which word is the
// file; an option outside them reads as one that takes no value.
// optionFile is the long option that names a file of patterns or of script,
// in grep and in sed.
const optionFile = "file"

var readerVerbs = map[string]readerVerb{
	"grep": {
		least: 2, valueShort: "ABCmef", patternShort: "ef",
		valueLong:   []string{"after-context", "before-context", "context", "max-count", "regexp", optionFile},
		patternLong: []string{"regexp", optionFile},
	},
	"sed": {
		least: 2, valueShort: "ef", patternShort: "ef",
		valueLong:   []string{"expression", optionFile},
		patternLong: []string{"expression", optionFile},
	},
	"cat": {least: 1},
}

// wordTest is the word `test`, which names a test run in three places: the go
// subcommand, the le area (goJobVerbs, leJobs) and the phase of a spawn
// (PhaseRules).
const wordTest = "test"

// goJobVerbs are the go subcommands that test or build.
var goJobVerbs = []string{"vet", wordTest, "build"}

// leJobs are the le commands that test or build, the families the umbrella's
// baseline table of foreground runs names: the first word, and the second
// word it needs, empty for any.
var leJobs = map[string]string{wordTest: "", "go": "", "job": "run", "repo": "compiles", "rfc": "check"}

// The le words the help and commit figures read.
// enumeration: exempt (the report selects help lookups and commit creation for measurement; it does not inventory CLI verbs)
const (
	leHelpWord   = "help"
	leCommitWord = "commit"
	leCreateWord = "create"
)

// readShell reads one Bash call's input. A call with no command, one longer
// than shellCommandMax, or one that does not parse answers Parsed false, and
// carries nothing from the command.
func readShell(input map[string]any) Shell {
	out := Shell{}
	if background, ok := input["run_in_background"].(bool); ok {
		out.Background = background
	}
	command, ok := input["command"].(string)
	if !ok {
		return out
	}
	if len(command) > shellCommandMax {
		return out
	}
	read := Shell{Background: out.Background}
	if !read.readCommand(command) {
		return out
	}
	read.Parsed = true
	return read
}

// readCommand parses a command line as bash and classifies each simple
// command of it into s, answering false when the line does not parse. The
// parse, the walk and the classification all run under one recover: nothing
// here is known to panic, but the input is whatever a session typed, and a
// report over a store of past commands must not die on one of them, so a
// panic is a command that did not parse. readShell then drops what s held.
func (s *Shell) readCommand(command string) (parsed bool) {
	defer func() {
		if recover() != nil {
			parsed = false
		}
	}()
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return false
	}
	syntax.Walk(file, func(node syntax.Node) bool {
		if call, isCall := node.(*syntax.CallExpr); isCall {
			s.classify(literalWords(call.Args))
		}
		return true
	})
	return true
}

// literalWords answers each word's text when it is one literal, single or
// double quoted or bare, and the empty string for a word built by expansion:
// such a word still counts as an argument, and is never a file name.
func literalWords(words []*syntax.Word) []string {
	out := make([]string, 0, len(words))
	for _, word := range words {
		out = append(out, literalWord(word))
	}
	return out
}

func literalWord(word *syntax.Word) string {
	if len(word.Parts) != 1 {
		return ""
	}
	if lit, isLit := word.Parts[0].(*syntax.Lit); isLit {
		return lit.Value
	}
	if quoted, isSingle := word.Parts[0].(*syntax.SglQuoted); isSingle {
		return quoted.Value
	}
	quoted, isDouble := word.Parts[0].(*syntax.DblQuoted)
	// refuse-by-default: guards classify -- an expanded word keeps its argument position but cannot name a command or file
	if !isDouble {
		return ""
	}
	if len(quoted.Parts) != 1 {
		return ""
	}
	if lit, isLit := quoted.Parts[0].(*syntax.Lit); isLit {
		return lit.Value
	}
	return ""
}

// classify records what one simple command of the line does. The command is
// named by its first word, so a wrapper (`timeout`, `env`, `xargs`,
// `bash -c`) hides the command it runs, and `go -C dir test` reads as no job:
// the figures count neither. `time` is a bash keyword whose clause holds the
// command it times, so that command is read.
func (s *Shell) classify(words []string) {
	if len(words) == 0 {
		return
	}
	name := path.Base(words[0])
	args := words[1:]
	if verb, reader := readerVerbs[name]; reader {
		if s.Reader == "" {
			s.Reader = verb.target(args)
		}
		return
	}
	if name == "go" {
		if len(args) > 0 {
			s.Job = s.Job || slices.Contains(goJobVerbs, args[0])
		}
		return
	}
	if name == "le" {
		s.classifyLe(args)
	}
}

// classifyLe records what one `le` command does.
func (s *Shell) classifyLe(args []string) {
	if len(args) == 0 {
		return
	}
	second := ""
	if len(args) > 1 {
		second = args[1]
	}
	if args[0] == leHelpWord {
		s.Help = true
		return
	}
	if args[0] == leCommitWord {
		s.CommitCreate = s.CommitCreate || second == leCreateWord
		return
	}
	if needs, job := leJobs[args[0]]; job {
		s.Job = s.Job || needs == "" || needs == second
	}
}

// target answers the file a grep, sed or cat names: the last operand, when
// the command has at least least of them, or one when an option gave the
// pattern or script. An operand is a word that is not an option and not the
// value of one; after `--` every word is an operand.
func (v readerVerb) target(args []string) string {
	operands := []string{}
	patterned := false
	value := false
	for i, arg := range args {
		if value {
			value = false
			continue
		}
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			operands = append(operands, arg)
			continue
		}
		takes, pattern := v.option(arg)
		value = takes
		patterned = patterned || pattern
	}
	least := v.least
	if patterned {
		least = 1
	}
	if len(operands) < least {
		return ""
	}
	return operands[len(operands)-1]
}

// option reads one option word: whether its value is the next word, and
// whether it gives the pattern or script.
func (v readerVerb) option(arg string) (takesNext, pattern bool) {
	if long, isLong := strings.CutPrefix(arg, "--"); isLong {
		name, _, attached := strings.Cut(long, "=")
		if !slices.Contains(v.valueLong, name) {
			return false, false
		}
		return !attached, slices.Contains(v.patternLong, name)
	}
	cluster := arg[1:]
	for i := range len(cluster) {
		letter := cluster[i]
		if strings.IndexByte(v.valueShort, letter) < 0 {
			continue
		}
		return i == len(cluster)-1, strings.IndexByte(v.patternShort, letter) >= 0
	}
	return false, false
}
