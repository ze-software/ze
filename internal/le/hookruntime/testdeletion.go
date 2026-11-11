// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashTestDeletion, the guard that judges these arguments
// Related: readerline.go -- readerWalk, the walk of the parsed line this reads
package hookruntime

import (
	"path/filepath"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// plainRmArguments answers the arguments of every rm on a line whose deletes
// the hook can see whole, and false for any other line, which keeps the
// line-wide judgment of bashTestDeletion.
//
// Such a line parses as bash (parse, the call's one parse), every node of its
// tree is one the reader-line walk admits (readerWalk), no redirection writes
// a file, and it holds two kinds of command only, each at the top level of
// the line, outside any group, subshell, loop, conditional, `!` or
// substitution, and with no assignment in front:
//   - an `rm` written as the command word the shell does not expand, with no
//     argument the shell expands, so its arguments are the paths it deletes;
//   - a reader its judge admits (readerAdmit) that is not `cd`, which changes
//     what a relative rm argument names, and names no delete word, so
//     `grep -n rm x_test.go` stays line-wide.
//
// Every other way to reach a delete (`git rm`, `find -delete`, xargs, an rm
// after eval, sudo, a loop or an assignment, a variable, a glob, a subshell
// or a group) fails one of these and keeps the line-wide judgment.
//
// A panic inside the walk answers false, so the line keeps the line-wide
// judgment (review I4); readPlainRmArguments is the walk without the recover.
func plainRmArguments(command string, parse commandParse) (arguments []string, plain bool) {
	defer func() {
		if recover() != nil {
			arguments, plain = nil, false
		}
	}()
	return readPlainRmArguments(command, parse)
}

// readPlainRmArguments is plainRmArguments without its recover.
func readPlainRmArguments(command string, parse commandParse) ([]string, bool) {
	if !deletionLineReadable(command, parse) {
		return nil, false
	}
	arguments := make([]string, 0, 8)
	admit := func(candidate lineCommand) string {
		if candidate.nested {
			return "a command inside a construct"
		}
		if len(candidate.call.Assigns) != 0 {
			return "an assignment in front of a command"
		}
		if candidate.words[0] == "rm" && !candidate.expands[0] {
			if slices.Contains(candidate.expands, true) {
				return "an rm argument the shell expands"
			}
			arguments = append(arguments, candidate.words[1:]...)
			return ""
		}
		if candidate.words[0] == "cd" {
			return "a cd beside an rm"
		}
		if outside := readerAdmit(candidate); outside != "" {
			return outside
		}
		if runsDeletion(candidate.words) {
			return "a reader that names a delete word"
		}
		return ""
	}
	walk := readerWalk{source: command, admit: admit}
	if outside := walk.statements(parse.file.Stmts, readerPlace{leftmost: true}); outside != "" {
		return nil, false
	}
	return arguments, true
}

// deletionLineReadable reports whether the deletion guard may read a line's
// words from its parse: it parses, and it holds no byte the parser drops and
// no trailing backslash, which a reader-line judgment refuses too
// (judgeReaderLine), and its tree names every command bash runs
// (treeMisreadsLine).
func deletionLineReadable(command string, parse commandParse) bool {
	if strings.ContainsAny(command, "\r\x00") {
		return false
	}
	if trailingBackslash(command) {
		return false
	}
	if !parse.parsed() {
		return false
	}
	return treeMisreadsLine(command, parse) == ""
}

// deletionWords answers every word of a line for the deletion guard's
// line-wide judgment, and whether the line was read from its parse. On a line
// that parses it is every word of the tree at any depth: its value with the
// quoting removed (readWord), and for a word the shell expands also its
// source with the quotes removed, so `$D/x_test.go` is judged as typed; and
// every here-document body split as shell text (textDeletionWords), because a
// body handed to a shell (`bash <<EOF`) runs the rm it holds, whatever quote,
// `;` or bracket stands beside it (review B5). On any other line it is the
// line's text split at blanks and shell operators with the quotes removed,
// which only adds words to judge. A panic inside the walk answers that text
// reading (review I4); readDeletionWords is the walk without the recover.
func deletionWords(command string, parse commandParse) (words []string, parsed bool) {
	defer func() {
		if recover() != nil {
			words, parsed = textDeletionWords(command), false
		}
	}()
	return readDeletionWords(command, parse)
}

// readDeletionWords is deletionWords without its recover.
func readDeletionWords(command string, parse commandParse) ([]string, bool) {
	if !deletionLineReadable(command, parse) {
		return textDeletionWords(command), false
	}
	reader := readerWalk{source: command}
	words := make([]string, 0, 16)
	syntax.Walk(parse.file, func(node syntax.Node) bool {
		switch node := node.(type) {
		case *syntax.Redirect:
			if node.Hdoc != nil && len(node.Hdoc.Parts) != 0 {
				words = append(words, textDeletionWords(reader.text(node.Hdoc))...)
			}
		case *syntax.Word:
			value, err := readWord(node)
			if err == nil {
				words = append(words, value.text)
			}
			if err != nil || value.expands {
				words = append(words, withoutQuotes(reader.text(node)))
			}
		}
		return true
	})
	return words, true
}

// textDeletionWords splits a line the parse cannot read at blanks and at the
// shell operators, with the quotes and backslashes removed.
func textDeletionWords(command string) []string {
	fields := strings.FieldsFunc(command, func(character rune) bool {
		return strings.ContainsRune(" \t\n;&|(){}<>!`", character)
	})
	for index, field := range fields {
		fields[index] = withoutQuotes(field)
	}
	return fields
}

// withoutQuotes removes every quote and backslash from a word's text.
func withoutQuotes(text string) string {
	return strings.NewReplacer(`'`, "", `"`, "", `\`, "").Replace(text)
}

// runsDeletion reports whether a line runs a delete: a word that is `rm` or
// ends in `/rm`, which also covers `git rm`, or find's `-delete`. It does not
// ask WHICH command runs it, so an rm reached through eval, a loop, a brace
// group or xargs still counts.
func runsDeletion(words []string) bool {
	for _, word := range words {
		if word == "rm" {
			return true
		}
		if strings.HasSuffix(word, "/rm") {
			return true
		}
		if word == "-delete" {
			return true
		}
	}
	return false
}

// testPath reports whether a word names content the testing rule guards: a
// `*_test.go`, `.ci` or `.et` file, a `test` or `testdata` directory and
// anything under one, or, inside internal/ or test/, any path element whose
// name contains "test", such as internal/component/cli/testing or a glob like
// internal/foo/*test*. Outside those trees a name that only contains "test",
// such as docs/architecture/testing or bin/le-latest, is not a test path.
// test/draft holds unfinished tests, which carry no coverage yet, so it is not
// guarded.
func testPath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "test/draft" {
		return false
	}
	if strings.HasPrefix(clean, "test/draft/") {
		return false
	}
	for _, suffix := range []string{"_test.go", ".ci", ".et"} {
		if strings.HasSuffix(clean, suffix) {
			return true
		}
	}
	if clean == testdataDir {
		return true
	}
	// A bare `test` is judged on the whole line, where it is far more often
	// `go test` or the test builtin than the directory, so the element counts
	// only inside a path.
	if !strings.Contains(path, "/") {
		return false
	}
	anchored := "/" + clean + "/"
	testTree := strings.Contains(anchored, "/internal/") || strings.Contains(anchored, "/test/")
	for element := range strings.SplitSeq(clean, "/") {
		if element == testElement {
			return true
		}
		if element == testdataDir {
			return true
		}
		if testTree && strings.Contains(element, testElement) {
			return true
		}
	}
	return false
}
