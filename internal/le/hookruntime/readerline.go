// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: shellparse.go -- the one parse of a call, and readWord, the word values judged here
// Related: readers.go -- readerCommands, the judge of each simple command
// Related: afterline.go -- the after-line walk, which skips the readers this walk admits
// Related: session.go -- commandOutsideReaders and bashAgentIdentity, the predicate's first readers
package hookruntime

import (
	"fmt"
	"os"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// readerDepthMax bounds how deep the walks below descend into nested
// constructs. The parser has already built the tree, so the line's length
// bounds it, but each level is a recursive call here, and a line nested
// deeper than any agent types is refused rather than walked.
const readerDepthMax = 64

// processSubstitutionPath is the value a word that is one whole input
// process substitution (`<(sort a)`) passes on Linux: a path the shell picks
// under /dev/fd. A judge sees it as an operand and never as an option.
const processSubstitutionPath = "/dev/fd/63"

// The constructs the reader-line predicate names in more than one place.
// Each completes "It holds ...", the sentence the identity refusal opens
// with (bashAgentIdentity), so each names what is outside and why.
const (
	outsideControlByte = "a carriage return or a NUL byte, which bash keeps inside a word and the parser drops, so the tree is not the line bash runs"
	outsideBackslash   = "a backslash that ends the line, which joins whatever is typed after it"
	outsideDeep        = "constructs nested deeper than the hook walks"
	outsideArithmetic  = "arithmetic (`$(( ))`, `(( ))` or `let`), whose operands bash evaluates and which can run a command through an array subscript"
	outsideFileWrite   = ", which redirects its output into a file"
	outsideIdentity    = "an identity line the parse does not read as the agent's own exports"
)

// lineCommand is one simple command of a parsed line, at any depth: its
// words as bash passes them (readWord), and where it sits in the tree.
type lineCommand struct {
	// call is the command's node; its Assigns are the assignments in front.
	call *syntax.CallExpr
	// words holds each argument's value with the quoting removed, expands
	// whether the shell changes it (wordValue), and sources each word as
	// typed. The three are parallel, and none is empty.
	words   []string
	expands []bool
	sources []string
	// assigns holds the assignments in front of the command word, each as
	// its name, `=` and its value read by readWord, and assignsExpand
	// whether the shell changes each value. assignsSource is their text as
	// typed, "" when there are none.
	assigns       []string
	assignsExpand []bool
	assignsSource string
	// nested marks a command inside a construct: a group, a subshell, a
	// loop, a conditional, a case, `!`, `time`, or a substitution. A
	// pipeline and a `&&` or `||` list are not constructs.
	nested bool
	// persists marks a command whose change of the shell's state (a cd)
	// holds for the top-level statements after it: it starts a top-level
	// statement, and nothing but `;`, a newline or `&&` follows it.
	persists bool
}

// lineWord is one word the predicate judged: an argument, a loop list item,
// a case word or pattern, an assignment value, a `[[` operand or a
// redirection target. A here-document body and a here-string are data to
// the reader that receives them and are not among them.
type lineWord struct {
	value wordValue
	// source is the word as typed, quotes included.
	source string
}

// readerLine is a line the predicate admitted: every simple command it
// holds and every word it judged, in line order.
type readerLine struct {
	commands []lineCommand
	words    []lineWord
}

// readerPlace is where the walk stands in the tree.
type readerPlace struct {
	nested bool
	// leftmost marks the leftmost statement of a top-level statement, and
	// chained the operator that follows it when one does; andChained marks
	// that operator as `&&`.
	leftmost   bool
	chained    bool
	andChained bool
}

// persists reports whether a command at this place changes the directory
// of the statements after it (lineCommand.persists): a cd that starts the
// line or follows `;` or a newline, and that `;`, `&&` or a newline ends. A
// cd after `&&` or `||` may not run, one before `||` leaves the next command
// where the shell was when it succeeds, and one in a pipeline, a group or a
// subshell may not hold for what follows.
func (place readerPlace) persists() bool {
	if place.nested {
		return false
	}
	if !place.leftmost {
		return false
	}
	if !place.chained {
		return true
	}
	return place.andChained
}

// commandAdmission judges one simple command for a walk, and answers "" when
// it is admitted, else the command in words a refusal carries.
type commandAdmission func(command lineCommand) string

// readerWalk is the state of one walk of a parsed line. It is not safe for
// concurrent use; each walk owns its own.
type readerWalk struct {
	// source is the text that was parsed, which every node's offsets index.
	source string
	admit  commandAdmission
	line   readerLine
	depth  int
	// recordWrites makes a redirection that writes a file set wrote rather
	// than end the walk, for the build guard, which judges a write after the
	// builds (goBuildLine).
	recordWrites bool
	wrote        bool
}

// ctxReaderLine answers whether the call's command is a reader line, and when
// it is not, the first construct or command outside it. The command is read
// from the call's one parse (ctx.commandParse). A subagent's command that
// starts with its own exact identity line (commandIdentityPrefix) is judged
// by what follows that line, so typing the line in front of a reader line
// leaves it a reader line for the guards (AC-14): the two exports are
// compared byte for byte with the line built from the payload, and no reader
// reads either variable.
func ctxReaderLine(ctx context) (readerLine, string) {
	command := stringInput(ctx.input, "command")
	return judgeReaderLine(command, ctx.commandParse(), commandIdentityPrefix(ctx))
}

// judgeReaderLine is the reader-line predicate. A line is a reader line when
// it parses as bash and every node of its tree is one the walk admits
// (readerWalk.statement), every simple command is a reader its judge admits
// (readerAdmit), and no redirection writes a file. It answers the first
// thing outside otherwise, and never answers yes for a line it could not
// read whole: a byte the parser drops, a line that does not parse, a word
// whose value it cannot give, and a node of any kind the walk does not list
// each end on the refusing side.
//
// identity is the caller's own identity line, or "" for the main thread.
//
// A panic inside the walk is answered as outside, with its reason: a line the
// hook failed to read is never a reader line (review I4). readReaderLine is
// the walk itself, which the fuzz target calls without this recover.
func judgeReaderLine(command string, parse commandParse, identity string) (readerLine, string) {
	return judgeReaderWalk(command, parse, identity, readerWalk{source: command, admit: readerAdmit})
}

// judgeReaderWalk is the reader-line predicate run with the walk it is
// given, so the scratch guard can ask the same question with every file
// write recorded rather than refused (readerLineButWrites, readerdata.go).
// walk MUST hold command as its source. A panic inside the walk is answered
// as outside, as judgeReaderLine answers it.
func judgeReaderWalk(command string, parse commandParse, identity string, walk readerWalk) (line readerLine, outside string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			line, outside = readerLine{}, "a line the hook failed to read ("+fmt.Sprint(recovered)+")"
		}
	}()
	return readReaderLine(command, parse, identity, walk)
}

// readReaderLine is judgeReaderWalk without its recover: the predicate's
// checks, then walk over the statements after the identity line. walk MUST
// hold command as its source.
func readReaderLine(command string, parse commandParse, identity string, walk readerWalk) (readerLine, string) {
	if strings.ContainsAny(command, "\r\x00") {
		// The lexer drops a carriage return before a newline and every NUL
		// byte (vendor/mvdan.cc/sh/v3/syntax/lexer.go, rune), and reads a lone
		// carriage return as a blank, while bash keeps each one in a word.
		return readerLine{}, outsideControlByte
	}
	if trailingBackslash(command) {
		return readerLine{}, outsideBackslash
	}
	if !parse.parsed() {
		return readerLine{}, "a line that does not parse as bash (" + parse.failure + "), so the hook cannot tell which commands it runs"
	}
	if misread := treeMisreadsLine(command, parse); misread != "" {
		return readerLine{}, misread
	}
	statements, outside := afterIdentityLine(command, parse.file.Stmts, identity)
	if outside != "" {
		return readerLine{}, outside
	}
	if outside := walk.statements(statements, readerPlace{leftmost: true}); outside != "" {
		return readerLine{}, outside
	}
	return walk.line, ""
}

// trailingBackslash reports whether a line ends in a backslash that escapes
// nothing: an odd run of them. The Bash tool hands the line to eval, which
// keeps that backslash as a character, but a line that ends there is one an
// agent cut short, so the hook keeps it outside.
func trailingBackslash(command string) bool {
	run := len(command) - len(strings.TrimRight(command, `\`))
	return run%2 == 1
}

// afterIdentityLine answers the statements that follow the caller's own
// identity line when the command starts with it, after any leading blank,
// and every statement otherwise. The line is `export A=x; ` once or twice
// (identityLine), so the statements that end inside it must be exactly that
// many exports; anything else is a line the hook does not read as the
// agent's line, and is refused rather than read some other way.
func afterIdentityLine(command string, statements []*syntax.Stmt, identity string) ([]*syntax.Stmt, string) {
	if identity == "" {
		return statements, ""
	}
	trimmed := strings.TrimLeft(command, " \t\n")
	if !strings.HasPrefix(trimmed, identity) {
		return statements, ""
	}
	end := uint(len(command) - len(trimmed) + len(identity))
	exports := strings.Count(identity, "export ")
	count := 0
	for count < len(statements) && statements[count].End().Offset() <= end {
		declaration, ok := statements[count].Cmd.(*syntax.DeclClause)
		if !ok {
			return nil, outsideIdentity
		}
		if declaration.Variant.Value != "export" {
			return nil, outsideIdentity
		}
		count++
	}
	if count != exports {
		return nil, outsideIdentity
	}
	return statements[count:], ""
}

// readerAdmit admits a simple command that is a reader its judge admits
// (readerCommand, readers.go). The messages are those commandOutsideReaders
// gave before the parse, so a refusal an agent knows keeps its words.
func readerAdmit(command lineCommand) string {
	if len(command.call.Assigns) != 0 {
		// GIT_EXTERNAL_DIFF=x git diff and PATH=. cat f each run a program.
		return "`" + command.assignsSource + "` in front of a command, an assignment that changes what the command runs or reads"
	}
	if _, reader := readerCommands[command.words[0]]; !reader {
		return "`" + command.sources[0] + "`, which is outside the reader set"
	}
	if command.expands[0] {
		return "`" + command.sources[0] + "`, which is outside the reader set"
	}
	if !readerCommand(command.words, command.expands) {
		return "`" + strings.Join(command.sources, " ") + "`, which the reader set admits only with the options its judge lists and no word the shell expands"
	}
	return ""
}

// text answers the source text of a node, and "" when its offsets do not
// lie inside the parsed line.
func (walk *readerWalk) text(node syntax.Node) string {
	start, end := node.Pos().Offset(), node.End().Offset()
	if start > end {
		return ""
	}
	if end > uint(len(walk.source)) {
		return ""
	}
	return walk.source[start:end]
}

// statements walks a list of statements at one place.
func (walk *readerWalk) statements(statements []*syntax.Stmt, place readerPlace) string {
	for index, statement := range statements {
		at := place
		if index > 0 {
			at.leftmost = place.leftmost && !place.nested
		}
		if outside := walk.statement(statement, at); outside != "" {
			return outside
		}
	}
	return ""
}

// statement walks one statement. The node table of the spec is this
// function and command: a command kind they do not list is outside, so a
// construct nobody admitted is refused with its name and never passed
// unjudged. The recursion is bounded by readerDepthMax.
func (walk *readerWalk) statement(statement *syntax.Stmt, place readerPlace) string {
	walk.depth++
	defer func() { walk.depth-- }()
	if walk.depth > readerDepthMax {
		return outsideDeep
	}
	if statement.Background {
		return "a background `&`, which runs a command the line does not wait for"
	}
	if statement.Coprocess {
		return "a coprocess"
	}
	if statement.Disown {
		return "a disowned job"
	}
	if statement.Negated {
		place.nested = true
	}
	if outside := walk.command(statement, place); outside != "" {
		return outside
	}
	for _, redirect := range statement.Redirs {
		if outside := walk.redirect(statement, redirect); outside != "" {
			return outside
		}
	}
	return ""
}

// command walks the command of one statement.
func (walk *readerWalk) command(statement *syntax.Stmt, place readerPlace) string {
	inner := place
	inner.nested = true
	switch command := statement.Cmd.(type) {
	case nil:
		// A statement of redirections alone (`< f`) runs no command; its
		// redirections are judged by statement.
		return ""
	case *syntax.CallExpr:
		return walk.call(command, place)
	case *syntax.BinaryCmd:
		// Every list and pipeline operator is admitted: `&&`, `||`, `|` and
		// `|&` join two statements, and each is walked.
		left := place
		left.chained, left.andChained = true, command.Op == syntax.AndStmt
		if outside := walk.statement(command.X, left); outside != "" {
			return outside
		}
		right := place
		right.leftmost = false
		return walk.statement(command.Y, right)
	case *syntax.Block:
		return walk.statements(command.Stmts, inner)
	case *syntax.Subshell:
		return walk.statements(command.Stmts, inner)
	case *syntax.IfClause:
		return walk.ifClause(command, inner)
	case *syntax.WhileClause:
		if outside := walk.statements(command.Cond, inner); outside != "" {
			return outside
		}
		return walk.statements(command.Do, inner)
	case *syntax.ForClause:
		return walk.forClause(command, inner)
	case *syntax.CaseClause:
		return walk.caseClause(command, inner)
	case *syntax.TestClause:
		return walk.test(command.X)
	case *syntax.TimeClause:
		if command.Stmt == nil {
			// `time` alone times nothing and runs nothing.
			return ""
		}
		return walk.statement(command.Stmt, inner)
	case *syntax.DeclClause:
		return "`" + command.Variant.Value + "`, which declares, exports or marks a variable"
	case *syntax.FuncDecl:
		return "a function definition, which can give a reader's name to any command"
	case *syntax.ArithmCmd:
		return outsideArithmetic
	case *syntax.LetClause:
		return outsideArithmetic
	case *syntax.CoprocClause:
		return "a coprocess"
	default:
		return "a construct the hook does not read (" + walk.text(command) + ")"
	}
}

// call walks one simple command: bare assignments alone, or a command word
// and its arguments.
func (walk *readerWalk) call(call *syntax.CallExpr, place readerPlace) string {
	if len(call.Args) == 0 {
		for _, assign := range call.Assigns {
			if outside := walk.bareAssignment(assign); outside != "" {
				return outside
			}
		}
		return ""
	}
	command := lineCommand{call: call, nested: place.nested, persists: place.persists()}
	for _, assign := range call.Assigns {
		// The values are walked for what they run; whether an assignment in
		// front is admitted is the admission's question (readerAdmit refuses
		// every one, goBuildLine admits GOFLAGS before a build).
		if command.assignsSource != "" {
			command.assignsSource += " "
		}
		command.assignsSource += walk.text(assign)
		value := wordValue{}
		if assign.Value != nil {
			read, outside := walk.word(assign.Value)
			if outside != "" {
				return outside
			}
			value = read
		}
		command.assigns = append(command.assigns, assign.Name.Value+"="+value.text)
		command.assignsExpand = append(command.assignsExpand, value.expands || assign.Append || assign.Index != nil || assign.Array != nil)
	}
	for _, argument := range call.Args {
		value, outside := walk.word(argument)
		if outside != "" {
			return outside
		}
		command.words = append(command.words, value.text)
		command.expands = append(command.expands, value.expands)
		command.sources = append(command.sources, walk.text(argument))
	}
	if outside := walk.admit(command); outside != "" {
		return outside
	}
	walk.line.commands = append(walk.line.commands, command)
	return ""
}

// bareAssignment admits `name=value` alone on its statement when the name
// is plain lower case and absent from the hook's environment. Bash gives no
// lower-case name a meaning of its own, and a bare assignment exports
// nothing, so the variable reaches only the shell and its functions; a name
// the environment already holds is exported, so assigning it changes what
// every later program reads (PATH, IFS, CDPATH, GIT_*, and the identity
// variables). The Bash tool's snapshot functions (grep, find, rg, pkill)
// declare every lower-case variable they use local (A-3 probe record), so a
// bare assignment does not reach them. An append (`+=`), an index, an array
// value and a value the hook cannot read are outside.
func (walk *readerWalk) bareAssignment(assign *syntax.Assign) string {
	if assign.Append {
		return "`" + walk.text(assign) + "`, an appending assignment, which the hook does not admit"
	}
	if assign.Index != nil {
		return "`" + walk.text(assign) + "`, an assignment to an array element, whose index bash evaluates as arithmetic"
	}
	if assign.Array != nil {
		return "`" + walk.text(assign) + "`, an array assignment"
	}
	if outside := plainLowerName(assign.Name.Value); outside != "" {
		return "`" + walk.text(assign) + "`, " + outside
	}
	if assign.Value == nil {
		return ""
	}
	_, outside := walk.word(assign.Value)
	return outside
}

// plainLowerName answers "" when a shell variable a reader line may set is
// lower-case letters, digits and `_` and absent from the hook's environment,
// and the reason it may not otherwise. It reads the environment by name with
// os.LookupEnv and registers no variable: the question is whether the agent's
// shell inherited that name, and the Bash tool's shell inherits the
// environment the hook inherits.
func plainLowerName(name string) string {
	if name == "" {
		return "an assignment with no name"
	}
	for index := range len(name) {
		character := name[index]
		if character == '_' {
			continue
		}
		if 'a' <= character && character <= 'z' {
			continue
		}
		if index > 0 && '0' <= character && character <= '9' {
			continue
		}
		return "an assignment to a name that is not lower case, which can be a variable bash or a program reads (PATH, IFS, GIT_*)"
	}
	if _, set := os.LookupEnv(name); set {
		return "an assignment to a variable the environment already holds, which every program the line runs then reads"
	}
	return ""
}

// ifClause walks an if, each elif and the else. Each else is the Else of
// the clause before it, so the chain is bounded by the line.
func (walk *readerWalk) ifClause(clause *syntax.IfClause, place readerPlace) string {
	for ; clause != nil; clause = clause.Else {
		if outside := walk.statements(clause.Cond, place); outside != "" {
			return outside
		}
		if outside := walk.statements(clause.Then, place); outside != "" {
			return outside
		}
	}
	return ""
}

// forClause admits `for name in words; do ...; done`, the name under the
// bare-assignment rule. `select` and the arithmetic `for (( ))` are outside.
func (walk *readerWalk) forClause(clause *syntax.ForClause, place readerPlace) string {
	if clause.Select {
		return "a `select` loop, which reads a choice from its input"
	}
	iteration, ok := clause.Loop.(*syntax.WordIter)
	if !ok {
		return "an arithmetic `for (( ))` loop, whose expressions bash evaluates and which can run a command through an array subscript"
	}
	if outside := plainLowerName(iteration.Name.Value); outside != "" {
		return "the loop variable `" + iteration.Name.Value + "`, " + outside
	}
	for _, item := range iteration.Items {
		if _, outside := walk.word(item); outside != "" {
			return outside
		}
	}
	return walk.statements(clause.Do, place)
}

// caseClause admits a case whose subject and patterns are words and whose
// bodies are admitted.
func (walk *readerWalk) caseClause(clause *syntax.CaseClause, place readerPlace) string {
	if _, outside := walk.word(clause.Word); outside != "" {
		return outside
	}
	for _, item := range clause.Items {
		for _, pattern := range item.Patterns {
			if _, outside := walk.word(pattern); outside != "" {
				return outside
			}
		}
		if outside := walk.statements(item.Stmts, place); outside != "" {
			return outside
		}
	}
	return ""
}

// testUnaryAdmitted holds the unary operators of `[[ ]]` a reader line may
// use: the file tests and the string tests. `-v` and `-R` evaluate an array
// subscript, and `-o` tests a shell option, which is neither a file nor a
// string test, so the three are outside.
var testUnaryAdmitted = map[syntax.UnTestOperator]bool{
	syntax.TsExists: true, syntax.TsRegFile: true, syntax.TsDirect: true, syntax.TsCharSp: true,
	syntax.TsBlckSp: true, syntax.TsNmPipe: true, syntax.TsSocket: true, syntax.TsSmbLink: true,
	syntax.TsSticky: true, syntax.TsGIDSet: true, syntax.TsUIDSet: true, syntax.TsGrpOwn: true,
	syntax.TsUsrOwn: true, syntax.TsModif: true, syntax.TsRead: true, syntax.TsWrite: true,
	syntax.TsExec: true, syntax.TsNoEmpty: true, syntax.TsFdTerm: true, syntax.TsEmpStr: true,
	syntax.TsNempStr: true, syntax.TsNot: true,
}

// testBinaryAdmitted holds the binary operators of `[[ ]]` a reader line may
// use. The arithmetic comparisons (-eq, -ne, -lt, -le, -gt, -ge) are outside:
// inside `[[` bash evaluates their operands as arithmetic expressions, and a
// variable's value can hold a subscript that runs a command. The test and `[`
// builtins compare integers without evaluating them, so their judges
// (testReads, bracketReads) are unchanged.
var testBinaryAdmitted = map[syntax.BinTestOperator]bool{
	syntax.TsReMatch: true, syntax.TsNewer: true, syntax.TsOlder: true, syntax.TsDevIno: true,
	syntax.AndTest: true, syntax.OrTest: true, syntax.TsMatchShort: true, syntax.TsMatch: true,
	syntax.TsNoMatch: true, syntax.TsBefore: true, syntax.TsAfter: true,
}

// test walks the expression of a `[[ ]]`. Its words may expand: inside `[[`
// no expansion becomes an operator, because the parser has already read the
// operators. The recursion is bounded by readerDepthMax.
func (walk *readerWalk) test(expression syntax.TestExpr) string {
	walk.depth++
	defer func() { walk.depth-- }()
	if walk.depth > readerDepthMax {
		return outsideDeep
	}
	switch expression := expression.(type) {
	case *syntax.Word:
		_, outside := walk.word(expression)
		return outside
	case *syntax.UnaryTest:
		if !testUnaryAdmitted[expression.Op] {
			return "the `[[` operator `" + expression.Op.String() + "`, which bash evaluates beyond a file or string test"
		}
		return walk.test(expression.X)
	case *syntax.BinaryTest:
		if !testBinaryAdmitted[expression.Op] {
			return "the `[[` operator `" + expression.Op.String() + "`, whose operands bash evaluates as arithmetic"
		}
		if outside := walk.test(expression.X); outside != "" {
			return outside
		}
		return walk.test(expression.Y)
	case *syntax.ParenTest:
		return walk.test(expression.X)
	default:
		return "a `[[` expression the hook does not read"
	}
}

// redirect judges one redirection of a statement: input from a file, a
// here-string or a here-document, and output onto /dev/null or another
// descriptor, wherever it is written. Any other output writes a file.
func (walk *readerWalk) redirect(statement *syntax.Stmt, redirect *syntax.Redirect) string {
	if redirect.N != nil && strings.Trim(redirect.N.Value, "0123456789") != "" {
		return "`" + walk.text(redirect) + "`, a redirection that assigns a descriptor to a variable"
	}
	switch redirect.Op {
	case syntax.RdrIn:
		_, outside := walk.word(redirect.Word)
		return outside
	case syntax.WordHdoc:
		// A here-string is data to the reader; only what it runs is walked.
		return walk.parts(redirect.Word.Parts)
	case syntax.Hdoc, syntax.DashHdoc:
		// The body is data to the reader that receives it. A body the shell
		// expands is walked for what it runs; a quoted body is one literal.
		if outside := walk.hereDocument(redirect); outside != "" {
			return outside
		}
		if redirect.Hdoc == nil {
			return ""
		}
		return walk.parts(redirect.Hdoc.Parts)
	case syntax.DplIn, syntax.DplOut:
		return walk.duplicate(statement, redirect)
	case syntax.RdrOut, syntax.AppOut, syntax.RdrClob, syntax.RdrAll, syntax.AppAll:
		value, outside := walk.word(redirect.Word)
		if outside != "" {
			return outside
		}
		if value.expands {
			return walk.fileWrite(statement)
		}
		if value.text != "/dev/null" {
			return walk.fileWrite(statement)
		}
		return ""
	case syntax.RdrInOut, syntax.AppClob, syntax.RdrAllClob, syntax.AppAllClob:
		// `<>` opens its target for writing, and the zsh forms are not bash.
		return walk.fileWrite(statement)
	default:
		// The operator set is the vendored parser's; one it adds later is
		// judged a write until it earns a row.
		return walk.fileWrite(statement)
	}
}

// hereDocument admits a here-document whose end bash and the parser find on
// the same line (review I1): its delimiter is plain text, one literal, one
// single-quoted string or one double-quoted literal, so no expansion decides
// where the body ends; a body the shell expands holds no backslash-newline,
// which bash joins before it compares a line with the delimiter; and no body
// line ends with the delimiter after other text (`${x}EOF`), the place a
// reading of the body's expansions could end it.
func (walk *readerWalk) hereDocument(redirect *syntax.Redirect) string {
	delimiter, quoted, plain := hereDelimiter(redirect.Word)
	if !plain {
		return "`" + walk.text(redirect) + "`, a here-document whose delimiter is not plain text, so where its body ends depends on an expansion"
	}
	if redirect.Hdoc == nil || len(redirect.Hdoc.Parts) == 0 {
		return ""
	}
	body := walk.text(redirect.Hdoc)
	if !quoted && strings.Contains(body, "\\\n") {
		return "a here-document body that holds a backslash-newline, which bash joins before it looks for the delimiter `" + delimiter + "`"
	}
	for line := range strings.SplitSeq(body, "\n") {
		if redirect.Op == syntax.DashHdoc {
			line = strings.TrimLeft(line, "\t")
		}
		if line != delimiter && strings.HasSuffix(line, delimiter) {
			return "a here-document body line `" + line + "` that ends with its delimiter `" + delimiter + "` after other text"
		}
	}
	return ""
}

// hereDelimiter reads a here-document's delimiter word: its text, whether it
// is quoted (which makes the body one literal), and whether it is plain, one
// literal with no backslash, one single-quoted string, or one double-quoted
// string of literals with no backslash.
func hereDelimiter(word *syntax.Word) (string, bool, bool) {
	if word == nil || len(word.Parts) != 1 {
		return "", false, false
	}
	switch part := word.Parts[0].(type) {
	case *syntax.Lit:
		return part.Value, false, !strings.ContainsRune(part.Value, '\\')
	case *syntax.SglQuoted:
		return part.Value, true, !part.Dollar
	case *syntax.DblQuoted:
		if part.Dollar {
			return "", false, false
		}
		var text strings.Builder
		for _, inner := range part.Parts {
			literal, ok := inner.(*syntax.Lit)
			if !ok {
				return "", false, false
			}
			if strings.ContainsRune(literal.Value, '\\') {
				return "", false, false
			}
			text.WriteString(literal.Value)
		}
		return text.String(), true, true
	default:
		return "", false, false
	}
}

// fileWrite answers a statement that writes a file: outside, or recorded
// (readerWalk.recordWrites) for the build guard.
func (walk *readerWalk) fileWrite(statement *syntax.Stmt) string {
	if walk.recordWrites {
		walk.wrote = true
		return ""
	}
	return "`" + walk.text(statement) + "`" + outsideFileWrite
}

// duplicate judges `<&N` and `>&N`: onto a descriptor number or `-`, which
// closes it. `>&word` with any other word writes the file it names.
func (walk *readerWalk) duplicate(statement *syntax.Stmt, redirect *syntax.Redirect) string {
	value, outside := walk.word(redirect.Word)
	if outside != "" {
		return outside
	}
	if value.expands {
		return walk.fileWrite(statement)
	}
	if value.text == "-" {
		return ""
	}
	if value.text != "" && strings.Trim(value.text, "0123456789") == "" {
		return ""
	}
	return walk.fileWrite(statement)
}

// word walks one word and reads its value (readWord). A word that is one
// whole input process substitution is an operand whose value is a path the
// shell picks; every other word is walked part by part for what it runs and
// recorded with its value. A word whose value the hook cannot read is
// outside, in readWord's words.
func (walk *readerWalk) word(word *syntax.Word) (wordValue, string) {
	if len(word.Parts) == 1 {
		if substitution, ok := word.Parts[0].(*syntax.ProcSubst); ok && substitution.Op == syntax.CmdIn {
			if outside := walk.nested(substitution.Stmts); outside != "" {
				return wordValue{}, outside
			}
			value := wordValue{text: processSubstitutionPath}
			walk.line.words = append(walk.line.words, lineWord{value: value, source: walk.text(word)})
			return value, ""
		}
	}
	if outside := walk.parts(word.Parts); outside != "" {
		return wordValue{}, outside
	}
	value, err := readWord(word)
	if err != nil {
		return wordValue{}, "the word `" + walk.text(word) + "`: " + err.Error()
	}
	walk.line.words = append(walk.line.words, lineWord{value: value, source: walk.text(word)})
	return value, ""
}

// nested walks the statements of a substitution, which are inside a
// construct wherever the word stands.
func (walk *readerWalk) nested(statements []*syntax.Stmt) string {
	return walk.statements(statements, readerPlace{nested: true})
}

// parts walks the parts of a word for the kinds a reader line admits: text,
// quoting, the parameter expansions paramExpansion lists, and a command
// substitution and an input process substitution whose statements are
// admitted. Arithmetic is outside wherever it is.
func (walk *readerWalk) parts(parts []syntax.WordPart) string {
	for _, part := range parts {
		switch part := part.(type) {
		case *syntax.Lit, *syntax.SglQuoted, *syntax.BraceExp:
			// BraceExp is never produced unless SplitBraces is called
			// (readWord); a Lit's braces are read there instead.
		case *syntax.ExtGlob:
			// The Bash tool runs the line with extglob off (A-3), where bash
			// reads `@(` as a syntax error and the parser reads a pattern, so
			// the tree is not what bash runs.
			return "an extended glob (`@(`, `!(` and the like), which the Bash tool's shell reads with extglob off"
		case *syntax.DblQuoted:
			if outside := walk.parts(part.Parts); outside != "" {
				return outside
			}
		case *syntax.ParamExp:
			if outside := walk.paramExpansion(part); outside != "" {
				return outside
			}
		case *syntax.CmdSubst:
			if part.TempFile || part.ReplyVar {
				return "a `${ ...;}` substitution, which is not bash"
			}
			if outside := walk.nested(part.Stmts); outside != "" {
				return outside
			}
		case *syntax.ProcSubst:
			if part.Op != syntax.CmdIn {
				return "an output process substitution `>( )`, which the hook does not admit"
			}
			if outside := walk.nested(part.Stmts); outside != "" {
				return outside
			}
		case *syntax.ArithmExp:
			return outsideArithmetic
		default:
			return "a word part the hook does not read"
		}
	}
	return ""
}

// expansionAdmitted holds the `${name<op>word}` operators a reader line may
// use: a default, an alternative, a prefix or suffix removal and a case
// change. An assigning default (`=`, `:=`) assigns, an error form (`?`,
// `:?`) is not in the spec's table, and `@` can expand a prompt string,
// which runs the substitutions its value holds.
var expansionAdmitted = map[syntax.ParExpOperator]bool{
	syntax.AlternateUnset: true, syntax.AlternateUnsetOrNull: true,
	syntax.DefaultUnset: true, syntax.DefaultUnsetOrNull: true,
	syntax.RemSmallSuffix: true, syntax.RemLargeSuffix: true,
	syntax.RemSmallPrefix: true, syntax.RemLargePrefix: true,
	syntax.UpperFirst: true, syntax.UpperAll: true, syntax.LowerFirst: true, syntax.LowerAll: true,
}

// paramExpansion admits plain `$name` and `${name}`, the special
// parameters, a length, a replacement, and the operators expansionAdmitted
// holds, and walks every word inside for what it runs. An index, a slice,
// an indirection and every form that is not bash are outside. The recursion
// through a nested word is bounded by readerDepthMax.
func (walk *readerWalk) paramExpansion(expansion *syntax.ParamExp) string {
	walk.depth++
	defer func() { walk.depth-- }()
	if walk.depth > readerDepthMax {
		return outsideDeep
	}
	written := "`" + walk.text(expansion) + "`"
	// Names before Excl: the parser may set Excl on `${!prefix*}` too.
	if expansion.Names != 0 {
		return written + ", an expansion of variable names"
	}
	if expansion.Excl {
		return written + ", an indirect expansion, whose name comes from a variable's value"
	}
	if expansion.Index != nil {
		return written + ", an array index, which bash evaluates as arithmetic"
	}
	if expansion.Slice != nil {
		return written + ", a slice, whose offsets bash evaluates as arithmetic"
	}
	if expansion.Width || expansion.IsSet || expansion.Flags != nil || expansion.NestedParam != nil || len(expansion.Modifiers) != 0 {
		return written + ", an expansion form that is not bash"
	}
	if expansion.Split != syntax.OptUnset || expansion.GlobSubst != syntax.OptUnset || expansion.RcExpand != syntax.OptUnset {
		return written + ", an expansion form that is not bash"
	}
	if expansion.Repl != nil {
		if outside := walk.optionalParts(expansion.Repl.Orig); outside != "" {
			return outside
		}
		if outside := walk.optionalParts(expansion.Repl.With); outside != "" {
			return outside
		}
	}
	if expansion.Exp == nil {
		return ""
	}
	if !expansionAdmitted[expansion.Exp.Op] {
		return written + ", an expansion operator that assigns, stops the shell or expands a prompt string"
	}
	return walk.optionalParts(expansion.Exp.Word)
}

// optionalParts walks the parts of a word an expansion may leave out.
func (walk *readerWalk) optionalParts(word *syntax.Word) string {
	if word == nil {
		return ""
	}
	return walk.parts(word.Parts)
}
