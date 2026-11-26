// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashTestDeletion, the guard that judges these words
// Related: parsedcommands.go -- pipelineMembership, the pipeline stages this reads
package hookruntime

import (
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/ze-software/ze/internal/le/heavytool"
)

// handedDepthMax bounds how deep the deletion reading follows a line handed to
// a shell inside a line handed to a shell (`bash -c "eval 'rm x'"`). Each
// handed line is a word or a body of the line holding it, so it is shorter
// and the recursion ends anyway; past the bound the line is judged by its
// text, which only adds words to judge.
const handedDepthMax = 8

// deletionReading is what the deletion guard reads of one command line.
type deletionReading struct {
	// words holds every word a deleting command of the line can reach
	// (lineDeletes.reached): the guard refuses the line when one of them is
	// a test path.
	words []string
	// unread is the first line the parse could not read, the command itself
	// or a line it hands to a shell, and "" when every line was read. An
	// unread line that runs rm is refused whatever it names.
	unread string
}

// deletionReach answers the words a line's deleting commands can reach, read
// per simple command from the call's one parse (parse), so a test path that
// is only an argument of another command on the line is never judged.
//
// A deleting command is a simple command one of whose words runs a delete
// (runsDeletion): `rm` or a path ending in `/rm` as its command word or
// handed to a launcher (`sudo /bin/rm`, `xargs rm`, `find -exec rm`, `eval
// rm`), `git rm`, and find's `-delete`. A command that names the word `rm`
// in another place (`grep -n rm x_test.go`) counts too, because the words
// alone do not say which launcher runs the next one. A line handed to a
// shell (`bash -c '<line>'`, `eval <words>`, a here-document or here-string
// a shell reads as its script) is read the same way, as a line of its own.
//
// A line the parse cannot read is judged by its text: every word of it when
// it runs a delete anywhere. A panic inside the walk answers that text
// reading (review I4); readDeletionReach is the walk without the recover.
func deletionReach(command string, parse commandParse) (reading deletionReading) {
	defer func() {
		if recover() != nil {
			reading = deletionReading{words: textDeletedWords(command), unread: command}
		}
	}()
	return readDeletionReach(command, parse, 0)
}

// readDeletionReach is deletionReach without its recover, at depth handed
// lines below the command.
func readDeletionReach(command string, parse commandParse, depth int) deletionReading {
	if !deletionLineReadable(command, parse) {
		return deletionReading{words: textDeletedWords(command), unread: command}
	}
	deletes := newLineDeletes(command, parse.file)
	return deletes.read(depth)
}

// textDeletedWords answers every word of a line the parse cannot read when
// the line runs a delete anywhere, and none otherwise: the text does not say
// which command a word belongs to, so a delete reaches all of them.
func textDeletedWords(command string) []string {
	words := textDeletionWords(command)
	if runsDeletion(words) {
		return words
	}
	return nil
}

// lineDeletes is one parsed line as the deletion reading walks it.
type lineDeletes struct {
	walk readerWalk
	// statements holds every statement of the line whose command is a
	// simple command with an argument, at any depth, in source order.
	statements []*syntax.Stmt
	// stages holds the pipeline stages each simple command runs in.
	stages map[*syntax.CallExpr][]pipelineStage
	// assigned holds, per shell name, every node that gives it a value on
	// the line: an assignment, and each word a `for` loop iterates over.
	assigned map[string][]syntax.Node
	// directories holds each operand of a `cd` or `pushd` on the line, which
	// changes what a relative operand of a delete names.
	directories []string
}

// newLineDeletes collects what the reading needs from one parsed line.
func newLineDeletes(source string, file *syntax.File) lineDeletes {
	deletes := lineDeletes{
		walk:     readerWalk{source: source},
		stages:   pipelineMembership(file.Stmts),
		assigned: make(map[string][]syntax.Node),
	}
	syntax.Walk(file, func(node syntax.Node) bool {
		switch node := node.(type) {
		case *syntax.Stmt:
			if _, simple := simpleCall(node); simple {
				deletes.statements = append(deletes.statements, node)
			}
		case *syntax.Assign:
			if node.Name != nil {
				deletes.assigned[node.Name.Value] = append(deletes.assigned[node.Name.Value], node)
			}
		case *syntax.ForClause:
			deletes.noteLoop(node)
		}
		return true
	})
	for _, statement := range deletes.statements {
		deletes.noteDirectory(statement)
	}
	return deletes
}

// noteLoop records each word a `for` or `select` loop gives its name.
func (deletes *lineDeletes) noteLoop(loop *syntax.ForClause) {
	iteration, words := loop.Loop.(*syntax.WordIter)
	if !words {
		return
	}
	if iteration.Name == nil {
		return
	}
	for _, item := range iteration.Items {
		deletes.assigned[iteration.Name.Value] = append(deletes.assigned[iteration.Name.Value], item)
	}
}

// noteDirectory records the operands of a `cd` or `pushd` statement.
func (deletes *lineDeletes) noteDirectory(statement *syntax.Stmt) {
	call, _ := simpleCall(statement)
	program := heavytool.Program(deletes.argumentValues(call))
	if len(program) == 0 {
		return
	}
	verb := filepath.Base(program[0])
	if verb != "cd" && verb != "pushd" {
		return
	}
	deletes.directories = append(deletes.directories, operands(program)...)
}

// read answers the words every deleting command of the line reaches, and the
// words of every line it hands to a shell, read depth lines below the
// command.
func (deletes *lineDeletes) read(depth int) deletionReading {
	var reading deletionReading
	for _, statement := range deletes.statements {
		for _, line := range deletes.handedLines(statement) {
			inner := readHandedLine(line, depth)
			reading.words = append(reading.words, inner.words...)
			if reading.unread == "" {
				reading.unread = inner.unread
			}
		}
		call, _ := simpleCall(statement)
		if !runsDeletion(deletes.argumentWords(call)) {
			continue
		}
		reading.words = append(reading.words, deletes.reached(statement)...)
	}
	return reading
}

// readHandedLine reads a line a command hands to a shell, depth lines below
// the command, as a line of its own.
func readHandedLine(line string, depth int) deletionReading {
	if depth >= handedDepthMax {
		return deletionReading{words: textDeletedWords(line), unread: line}
	}
	return readDeletionReach(line, parseCommandLine(line), depth+1)
}

// handedLines answers each line a statement hands to a shell to run: a
// launcher's line (`bash -c '<line>'`, heavytool.ShellCommand), eval's words
// joined, and for a shell that reads its script from its input
// (heavytool.StdinShell) each here-document body and here-string it is
// given. A shell given a script file, and `source`, run a file the line does
// not hold, so neither hands a line.
func (deletes *lineDeletes) handedLines(statement *syntax.Stmt) []string {
	call, _ := simpleCall(statement)
	values := deletes.argumentValues(call)
	var lines []string
	if line, handed := heavytool.ShellCommand(values); handed {
		lines = append(lines, line)
	}
	program := heavytool.Program(values)
	if len(program) != 0 && program[0] == evalCommand {
		lines = append(lines, strings.Join(program[1:], " "))
	}
	if !heavytool.StdinShell(values) {
		return lines
	}
	for _, redirect := range statement.Redirs {
		if redirect.Hdoc != nil {
			lines = append(lines, deletes.walk.text(redirect.Hdoc))
			continue
		}
		if redirect.Op == syntax.WordHdoc {
			lines = append(lines, deletes.wordText(redirect.Word))
		}
	}
	return lines
}

// reached answers every word a deleting statement can delete:
//   - every word of the statement at any depth, the words of a substitution
//     in it included, and each here-document body or here-string it reads,
//     split as shell text, because xargs reads its operands there;
//   - every word that gives a value to a name the statement expands, through
//     the names those words expand in turn (resolve);
//   - every word of a command in an earlier stage of a pipeline the
//     statement runs in, which prints what xargs reads;
//   - each relative operand joined to each `cd` or `pushd` operand of the
//     line, because the delete may run in that directory.
func (deletes *lineDeletes) reached(statement *syntax.Stmt) []string {
	words := deletes.nodeWords(statement)
	resolved := map[string]bool{}
	for _, name := range expandedNames(statement) {
		words = append(words, deletes.resolve(name, resolved)...)
	}
	call, _ := simpleCall(statement)
	for _, other := range deletes.statements {
		feeder, _ := simpleCall(other)
		if stagesFeed(deletes.stages[feeder], deletes.stages[call]) {
			words = append(words, deletes.nodeWords(other)...)
		}
	}
	for _, directory := range deletes.directories {
		for _, operand := range operands(deletes.argumentValues(call)) {
			if !filepath.IsAbs(operand) {
				words = append(words, filepath.Join(directory, operand))
			}
		}
	}
	return words
}

// resolve answers the words of every node that gives name a value on the
// line, and of the nodes that give a value to each name those expand. seen
// holds the names already resolved, so the walk ends after each name of the
// line at most once.
func (deletes *lineDeletes) resolve(name string, seen map[string]bool) []string {
	if seen[name] {
		return nil
	}
	seen[name] = true
	var words []string
	for _, node := range deletes.assigned[name] {
		words = append(words, deletes.nodeWords(node)...)
		for _, inner := range expandedNames(node) {
			words = append(words, deletes.resolve(inner, seen)...)
		}
	}
	return words
}

// nodeWords answers every word under node at any depth: its value with the
// quoting removed (readWord), and for a word the shell expands or the hook
// cannot read also its source with the quotes removed, so `$D/x_test.go` is
// judged as typed; and each here-document body and here-string split as
// shell text.
func (deletes *lineDeletes) nodeWords(node syntax.Node) []string {
	words := make([]string, 0, 8)
	syntax.Walk(node, func(inner syntax.Node) bool {
		switch inner := inner.(type) {
		case *syntax.Redirect:
			if inner.Hdoc != nil {
				words = append(words, textDeletionWords(deletes.walk.text(inner.Hdoc))...)
			}
			if inner.Op == syntax.WordHdoc {
				words = append(words, textDeletionWords(deletes.wordText(inner.Word))...)
			}
		case *syntax.Word:
			words = append(words, deletes.wordReadings(inner)...)
		}
		return true
	})
	return words
}

// argumentWords answers the words of a simple command's own arguments, read
// as nodeWords reads a word, without the commands of a substitution in them,
// which are statements of their own.
func (deletes *lineDeletes) argumentWords(call *syntax.CallExpr) []string {
	words := make([]string, 0, len(call.Args))
	for _, argument := range call.Args {
		words = append(words, deletes.wordReadings(argument)...)
	}
	return words
}

// wordReadings answers a word's value, and its source with the quotes
// removed when the shell expands it or the hook cannot read it.
func (deletes *lineDeletes) wordReadings(word *syntax.Word) []string {
	value, err := readWord(word)
	if err != nil {
		return []string{withoutQuotes(deletes.walk.text(word))}
	}
	if value.expands {
		return []string{value.text, withoutQuotes(deletes.walk.text(word))}
	}
	return []string{value.text}
}

// wordText answers a word's value, or its source with the quotes removed when
// the shell expands it or the hook cannot read it.
func (deletes *lineDeletes) wordText(word *syntax.Word) string {
	readings := deletes.wordReadings(word)
	return readings[len(readings)-1]
}

// simpleCall answers the simple command a statement runs, when it is one
// with at least one argument.
func simpleCall(statement *syntax.Stmt) (*syntax.CallExpr, bool) {
	call, simple := statement.Cmd.(*syntax.CallExpr)
	if !simple {
		return nil, false
	}
	if len(call.Args) == 0 {
		return nil, false
	}
	return call, true
}

// argumentValues answers each argument's value with the quoting removed, its
// literal parts alone for a word the shell expands, and its source with the
// quotes removed for a word the hook cannot read: the words the launcher
// reading (heavytool) takes.
func (deletes *lineDeletes) argumentValues(call *syntax.CallExpr) []string {
	values := make([]string, 0, len(call.Args))
	for _, argument := range call.Args {
		value, err := readWord(argument)
		if err != nil {
			values = append(values, withoutQuotes(deletes.walk.text(argument)))
			continue
		}
		values = append(values, value.text)
	}
	return values
}

// operands answers a command's words after its command word that are no
// option.
func operands(words []string) []string {
	if len(words) == 0 {
		return nil
	}
	found := make([]string, 0, len(words)-1)
	for _, word := range words[1:] {
		if strings.HasPrefix(word, "-") {
			continue
		}
		found = append(found, word)
	}
	return found
}

// expandedNames answers every shell name a parameter expansion under node
// expands.
func expandedNames(node syntax.Node) []string {
	var names []string
	syntax.Walk(node, func(inner syntax.Node) bool {
		expansion, parameter := inner.(*syntax.ParamExp)
		if !parameter {
			return true
		}
		if expansion.Param != nil {
			names = append(names, expansion.Param.Value)
		}
		return true
	})
	return names
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

// runsDeletion reports whether words run a delete: a word that is `rm` or
// ends in `/rm`, which also covers `git rm`, or find's `-delete`. It does not
// ask WHICH word of a command runs it, so an rm a launcher, eval, xargs or
// find runs still counts.
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
	// A bare `test` is judged on a deleting command, where it is far more
	// often the test builtin or a word of `go test` than the directory, so
	// the element counts only inside a path.
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
