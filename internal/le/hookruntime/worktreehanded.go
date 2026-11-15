// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: worktreecopy.go -- worktreeCopyJudge, the walk whose words these read
// Related: shellparse.go -- parseCommandLine and readWord, the parse a handed line goes through
// Related: worktreeplace.go -- placeOf, where a word read here points
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/ze-software/ze/internal/le/heavytool"
)

// wordOf reads one word of a command run from directory: its value when the
// hook reads it exactly (readWord), with a `$PWD` part read as that directory
// (workingDirectory), a home head and `$HOME` as the home directory
// (homeDirectory), and a `$name` the line bound once as its value
// (boundValue); and its typed text otherwise, with a home head written as the
// home directory and a leading bound `$name` as its value (homeToken). A word
// that holds the variable of an outer expansion (worktreeOuterMark), in any
// quoting, is never read exactly; one that expands a variable the line bound
// to a worktree path names that path (namesInside), and one that starts with
// such a variable is placed under it (placingVariable).
func (judge *worktreeCopyJudge) wordOf(word *syntax.Word, directory worktreeDirectory) worktreeWord {
	typed := judge.walk.text(word)
	if strings.Contains(typed, worktreeOuterMark) {
		return worktreeWord{token: typedToken(typed), inside: strings.Contains(typed, worktreeOuterInside)}
	}
	value, err := readWord(judge.rewritten(word, directory, false))
	if err != nil {
		return worktreeWord{token: typedToken(typed), inside: judge.namesInside(word)}
	}
	if value.expands {
		return worktreeWord{
			token:  judge.homeToken(word, typedToken(typed)),
			inside: judge.namesInside(word),
			placed: judge.placingVariable(word),
			handed: judge.handedText(word, directory),
		}
	}
	return worktreeWord{token: value.text, exact: true, handed: value.text}
}

// homeToken answers the typed text of a word the shell expands with its head
// written as the value it names: a leading unquoted `~` before a slash as the
// home directory (homeDirectory), and a leading plain `$name` or `${name}`,
// bare or first in double quotes, as the value boundValue reads for it, the
// home directory for `$HOME` and the value the line bound once for any other
// name. So `~/tmp/$n.log`, and `"$log/run-$i.log"` after `log=~/tmp`, are
// placed where they land. Any other text is answered as typed.
func (judge *worktreeCopyJudge) homeToken(word *syntax.Word, token string) string {
	if worktreeTildeHead(word) {
		home, known := judge.homeDirectory()
		if !known {
			return token
		}
		if rest, found := strings.CutPrefix(token, "~/"); found {
			return home + "/" + rest
		}
		return token
	}
	param := worktreeLeadingParameter(word)
	if param == nil {
		return token
	}
	value, bound := judge.boundValue(param)
	if !bound {
		return token
	}
	if rest, found := strings.CutPrefix(token, judge.walk.text(param)); found {
		return value + rest
	}
	return token
}

// placingVariable answers the name of the variable a word starts with as a
// plain `$name` or `${name}`, bare or first in double quotes, when the line
// assigns it once (a for loop's words count as one assignment) and bound it
// only to paths under a worktree (bind), and empty for any other word. An
// operator (`${f#.claude/worktrees/x/}`) may cut the worktree off its value,
// so it does not count, and a name assigned twice may hold its other value.
func (judge *worktreeCopyJudge) placingVariable(word *syntax.Word) string {
	param := worktreeLeadingParameter(word)
	if param == nil {
		return ""
	}
	name := worktreeParameterName(param)
	if !worktreePlainParameter(judge.walk.text(param), name) {
		return ""
	}
	if !judge.inside[name] {
		return ""
	}
	if judge.mixed[name] {
		return ""
	}
	if judge.assignments.counts[name] != 1 {
		return ""
	}
	return name
}

// worktreeLeadingParameter answers the parameter expansion a word starts
// with, bare or first in double quotes, and nil when it starts with anything
// else.
func worktreeLeadingParameter(word *syntax.Word) *syntax.ParamExp {
	if len(word.Parts) == 0 {
		return nil
	}
	first := word.Parts[0]
	if quoted, isQuoted := first.(*syntax.DblQuoted); isQuoted {
		if len(quoted.Parts) == 0 {
			return nil
		}
		first = quoted.Parts[0]
	}
	param, isParam := first.(*syntax.ParamExp)
	if !isParam {
		return nil
	}
	return param
}

// worktreePlainParameter reports whether a parameter expansion's typed text
// is `$name` or `${name}`, with no operator, index or length.
func worktreePlainParameter(text, name string) bool {
	if name == "" {
		return false
	}
	return oneOf(text, "$"+name, "${"+name+"}")
}

// worktreeTildeHead reports whether a word starts with an unquoted literal
// whose tilde the shell expands to the home directory: `~` alone as the
// whole word, or `~/` first.
func worktreeTildeHead(word *syntax.Word) bool {
	if len(word.Parts) == 0 {
		return false
	}
	literal, isLiteral := word.Parts[0].(*syntax.Lit)
	if !isLiteral {
		return false
	}
	if strings.HasPrefix(literal.Value, "~/") {
		return true
	}
	if literal.Value != "~" {
		return false
	}
	return len(word.Parts) == 1
}

// homeDirectory answers the home directory a `~` and a `$HOME` of the line
// name: the one os.UserHomeDir reads, when the line runs on this host, the
// line assigned no HOME the walk saw, and the path is absolute and holds only
// worktreeWorkingCharacters, so it reads back as the same path in any
// quoting. A home directory the hook cannot read leaves the word one it
// cannot place.
func (judge *worktreeCopyJudge) homeDirectory() (string, bool) {
	if judge.remote {
		return "", false
	}
	if judge.homeAssigned {
		return "", false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(home) {
		return "", false
	}
	home = filepath.Clean(home)
	if strings.IndexFunc(home, worktreeUnsafeCharacter) >= 0 {
		return "", false
	}
	return home, true
}

// handedText answers the line a shell runs when it is handed word: its value
// as the outer shell builds it, with `$PWD` read as directory and every other
// expansion written as a variable the hook cannot read (rewritten). It is
// empty when that value still expands (a glob, a tilde, a brace expansion)
// or cannot be read, and the line is then judged by its text.
func (judge *worktreeCopyJudge) handedText(word *syntax.Word, directory worktreeDirectory) string {
	value, err := readWord(judge.rewritten(word, directory, true))
	if err != nil {
		return ""
	}
	if value.expands {
		return ""
	}
	return value.text
}

// rewritten answers word with each `$PWD` part written as the directory it
// names (workingDirectory), each `$name` the line bound once as its value
// (boundValue), each `$HOME` part and a home head (worktreeTildeHead) as the
// home directory (homeDirectory), and, when marked, each other parameter
// expansion, substitution and arithmetic written as worktreeOuterExpansion,
// or as worktreeOuterInside for a variable the line bound to a worktree path.
// The outer shell substitutes each expansion into the word's text, so the
// rewritten word reads as the line a shell handed it parses. It reads the
// parts of word, never their copies, so their typed text is the line's.
func (judge *worktreeCopyJudge) rewritten(word *syntax.Word, directory worktreeDirectory, marked bool) *syntax.Word {
	parts := judge.rewrittenParts(word.Parts, directory, marked)
	if !worktreeTildeHead(word) {
		return &syntax.Word{Parts: parts}
	}
	home, known := judge.homeDirectory()
	if !known {
		return &syntax.Word{Parts: parts}
	}
	literal, _ := word.Parts[0].(*syntax.Lit)
	parts[0] = &syntax.Lit{Value: home + strings.TrimPrefix(literal.Value, "~")}
	return &syntax.Word{Parts: parts}
}

// rewrittenParts answers each of parts rewritten (rewrittenPart).
func (judge *worktreeCopyJudge) rewrittenParts(parts []syntax.WordPart, directory worktreeDirectory, marked bool) []syntax.WordPart {
	rewritten := make([]syntax.WordPart, 0, len(parts))
	for _, part := range parts {
		rewritten = append(rewritten, judge.rewrittenPart(part, directory, marked))
	}
	return rewritten
}

// rewrittenPart answers one part of a word rewritten (rewritten): a double
// quoted part with its own parts rewritten, an expansion as a literal, and any
// other part as it is.
func (judge *worktreeCopyJudge) rewrittenPart(part syntax.WordPart, directory worktreeDirectory, marked bool) syntax.WordPart {
	switch part := part.(type) {
	case *syntax.DblQuoted:
		return &syntax.DblQuoted{Dollar: part.Dollar, Parts: judge.rewrittenParts(part.Parts, directory, marked)}
	case *syntax.ParamExp:
		if path, named := judge.workingDirectory(part, directory); named {
			return &syntax.Lit{Value: path}
		}
		if value, bound := judge.boundValue(part); bound {
			return &syntax.Lit{Value: value}
		}
		if !marked {
			return part
		}
		if judge.inside[worktreeParameterName(part)] {
			return &syntax.Lit{Value: worktreeOuterInside}
		}
		return &syntax.Lit{Value: worktreeOuterExpansion}
	case *syntax.CmdSubst, *syntax.ArithmExp, *syntax.ProcSubst:
		if !marked {
			return part
		}
		return &syntax.Lit{Value: worktreeOuterExpansion}
	}
	return part
}

// workingDirectory answers the directory a `$PWD` or `${PWD}` part names:
// the one its command runs from, when the hook names it (no `cd` it cannot
// follow came before), the line assigned no PWD, and the path holds only
// worktreeWorkingCharacters, so it reads back as the same path.
func (judge *worktreeCopyJudge) workingDirectory(part *syntax.ParamExp, directory worktreeDirectory) (string, bool) {
	if !oneOf(judge.walk.text(part), "$PWD", "${PWD}") {
		return "", false
	}
	if judge.pwdAssigned {
		return "", false
	}
	if !directory.known() {
		return "", false
	}
	if strings.IndexFunc(directory.path, worktreeUnsafeCharacter) >= 0 {
		return "", false
	}
	return directory.path, true
}

// boundValue answers the value a plain `$name` or `${name}` part names: the
// value the line bound the name to once (bind), or, for HOME, which the line
// did not assign, the home directory (homeDirectory).
func (judge *worktreeCopyJudge) boundValue(part *syntax.ParamExp) (string, bool) {
	name := worktreeParameterName(part)
	if !worktreePlainParameter(judge.walk.text(part), name) {
		return "", false
	}
	if value, bound := judge.values[name]; bound {
		return value, true
	}
	if name != "HOME" {
		return "", false
	}
	return judge.homeDirectory()
}

// worktreeUnsafeCharacter reports whether a character of a directory is not
// one of worktreeWorkingCharacters.
func worktreeUnsafeCharacter(character rune) bool {
	return !strings.ContainsRune(worktreeWorkingCharacters, character)
}

// namesInside reports whether a word expands a variable the line bound to a
// path under a worktree (bind). A substitution prints another value, so a
// variable inside one does not count.
func (judge *worktreeCopyJudge) namesInside(word *syntax.Word) bool {
	names := false
	syntax.Walk(word, func(node syntax.Node) bool {
		switch node := node.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst, *syntax.ArithmExp:
			return false
		case *syntax.ParamExp:
			names = names || judge.inside[worktreeParameterName(node)]
			return false
		}
		return !names
	})
	return names
}

// worktreeParameterName answers the name a parameter expansion reads, empty
// for one with none, which no binding holds.
func worktreeParameterName(part *syntax.ParamExp) string {
	if part.Param == nil {
		return ""
	}
	return part.Param.Value
}

// copyLine judges a line handed to a shell, read from words, and answers
// whether it read a path under a worktree. A line read from words the outer
// shell expands first is parsed from their values as the outer shell builds
// them (worktreeHandedLine), each expansion the hook cannot read standing as
// a word it cannot read either. It is judged by its text when that line
// cannot be built, or when an outer expansion names a program of it, which
// may then be any program (worktreeHandedProgramExpands).
func (judge *worktreeCopyJudge) copyLine(line string, words []worktreeWord, directory worktreeDirectory) bool {
	if worktreeExact(words) {
		return judge.judgeLine(line, parseCommandLine(line), directory)
	}
	handed, built := worktreeHandedLine(line, words)
	if !built {
		return judge.judgeText(line)
	}
	parse := parseCommandLine(handed)
	if worktreeHandedProgramExpands(handed, parse) {
		return judge.judgeText(line)
	}
	return judge.judgeLine(handed, parse, directory)
}

// worktreeExact reports whether the hook read every one of words exactly.
func worktreeExact(words []worktreeWord) bool {
	for _, word := range words {
		if !word.exact {
			return false
		}
	}
	return true
}

// worktreeHandedLine answers the line a shell runs when it is handed words
// joined by spaces (a `-c` operand, eval's words, ssh's), from each word's
// handed text, and false when a word has none or the line is not the words
// joined: env -S splits its string first.
func worktreeHandedLine(line string, words []worktreeWord) (string, bool) {
	if worktreeJoined(words) != line {
		return "", false
	}
	texts := make([]string, 0, len(words))
	for _, word := range words {
		if word.handed == "" {
			return "", false
		}
		texts = append(texts, word.handed)
	}
	return strings.Join(texts, " "), true
}

// worktreeHandedProgramExpands reports whether an outer expansion names the
// program of a command of a handed line (worktreeOuterMark in its program
// word, after its launchers). A line the parser cannot read answers false:
// judgeLine judges it by its text.
func worktreeHandedProgramExpands(line string, parse commandParse) bool {
	if !parse.parsed() {
		return false
	}
	walk := readerWalk{source: line}
	expands := false
	syntax.Walk(parse.file, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return !expands
		}
		tokens := make([]string, 0, len(call.Args))
		for _, argument := range call.Args {
			tokens = append(tokens, typedToken(walk.text(argument)))
		}
		program := heavytool.Program(tokens)
		if len(program) == 0 {
			return !expands
		}
		expands = expands || strings.Contains(program[0], worktreeOuterMark)
		return !expands
	})
	return expands
}
