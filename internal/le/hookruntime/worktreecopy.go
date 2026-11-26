// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashWorktreeCopy, the guard this judgment answers for
// Related: shellparse.go -- parseCommandLine and readWord, the parse this walk reads
// Related: gobuild.go -- cdTarget, the directory a cd moves to
// Related: readgate.go -- insideRoot and resolveExisting, the checkout a path lies in
// Related: worktreeplace.go -- worktreeLineConcerns, placeOf and destinationOf, which lines and paths concern a worktree
// Related: worktreebind.go -- bind and worktreeLineAssignments, the variables a line binds
// Related: worktreehanded.go -- wordOf and copyLine, a word's value and the line a shell is handed
// Related: worktreeprograms.go -- copyOperands and the readers of find, xargs, dd, tar, git apply and patch
package hookruntime

import (
	"path/filepath"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/ze-software/ze/internal/le/heavytool"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// worktreeClaudeDirectory and worktreeWorktreesComponent are the two
// components of lepath.WorktreesDir, which a line may name apart.
const (
	worktreeClaudeDirectory    = ".claude"
	worktreeWorktreesComponent = "worktrees"
)

// worktreeScratchDirectory is the checkout's gitignored scratch tree. It holds
// no tracked work, so a write there is no copy into the main tree.
const worktreeScratchDirectory = "tmp"

// worktreeCopyVerbs are the programs that write copies, or links, of their
// other operands to their last one, or to the directory `-t` names.
var worktreeCopyVerbs = map[string]bool{"cp": true, "mv": true, worktreeRsync: true, "install": true, "ln": true}

// worktreeRsync is the copy verb with a remote form (`host:path`) and no
// target-directory option, which worktreeHostPath and worktreeTakesTarget
// read apart from the others.
const worktreeRsync = "rsync"

// worktreeFileRedirects are the redirection operators that write the file their
// word names. `>&` is judged apart (redirectTarget): it writes a file
// only when its word is no descriptor.
var worktreeFileRedirects = map[syntax.RedirOperator]bool{
	syntax.RdrOut: true, syntax.AppOut: true, syntax.RdrInOut: true, syntax.RdrClob: true, syntax.AppClob: true,
	syntax.RdrAll: true, syntax.RdrAllClob: true, syntax.AppAll: true, syntax.AppAllClob: true,
}

// The text judgment of a line the parser cannot read the way bash runs it
// (worktreeCopyText): a copy verb anywhere in its text, or a `>` after a space.
var (
	worktreeCopyTextVerbs    = []string{"cp ", "cp -", "mv ", "mv -", "rsync ", "install "}
	worktreeCopyTextRedirect = regexp.MustCompile(`\s>`)
)

// worktreeDirectory is the directory a command of a line runs in: an
// absolute, cleaned path, or unknown, its zero value, once the hook cannot
// name it (a `cd` it cannot read, a program word the shell expands, a
// launcher's own directory option, a line another host runs).
type worktreeDirectory struct {
	path string
}

// known reports whether the hook names the directory.
func (directory worktreeDirectory) known() bool {
	return directory.path != ""
}

// worktreePlace answers where a path word points.
type worktreePlace int

const (
	// worktreePlaceUnknown is a word the hook cannot place: one the shell
	// expands, or a relative path from a directory it cannot name.
	worktreePlaceUnknown worktreePlace = iota
	// worktreePlaceInside is a path under a worktree.
	worktreePlaceInside
	// worktreePlaceOutside is a path the hook read exactly that is not.
	worktreePlaceOutside
)

// worktreeDestination answers where a destination word writes.
type worktreeDestination int

const (
	// worktreeDestinationUnspecified is no answer; destinationOf never gives it.
	worktreeDestinationUnspecified worktreeDestination = iota
	// worktreeDestinationElsewhere is outside the main tree: a worktree, the
	// checkout's tmp/, a device, a path outside the checkout, another host.
	worktreeDestinationElsewhere
	// worktreeDestinationMain is a path the hook placed in the main tree.
	worktreeDestinationMain
	// worktreeDestinationUnplaced is a destination the hook cannot place,
	// which counts as the main tree.
	worktreeDestinationUnplaced
)

// countsMain reports whether a destination counts as the main tree.
func (destination worktreeDestination) countsMain() bool {
	switch destination {
	case worktreeDestinationMain, worktreeDestinationUnplaced:
		return true
	case worktreeDestinationElsewhere:
		return false
	case worktreeDestinationUnspecified:
	}
	panic("BUG: destinationOf answered no destination")
}

// worktreeOuterMark starts the name of the variable a handed line holds in
// place of an expansion of the line that handed it (worktreeRewritten), which
// the hook cannot read: worktreeOuterExpansion for any expansion, and
// worktreeOuterInside for a variable the line bound to a worktree path. As a
// `$name` the inner parse reads it as an expansion, and a word that holds it
// in any quoting is never read exactly (wordOf).
const (
	worktreeOuterMark      = "$__ze_outer_"
	worktreeOuterExpansion = worktreeOuterMark + "expansion_"
	worktreeOuterInside    = worktreeOuterMark + "inside_"
)

// worktreeWorkingCharacters are the characters of a directory the hook
// writes in place of `$PWD` (worktreeRewritten): none of them is a quote, an
// expansion, a glob, a separator or a space, so the value reads back as the
// same path in any quoting and in a handed line.
const worktreeWorkingCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._+-@%,:"

// worktreeWord is one word of a command: its value when the hook read it
// exactly, its typed text otherwise (typedToken), and which of the two.
type worktreeWord struct {
	token string
	exact bool
	// inside reports that the word expands a variable the line bound to a
	// path under a worktree (bind), so it names that path.
	inside bool
	// placed is the name of the variable a word starts with as a plain
	// `$name`, when the line assigns it once and bound it only to paths under
	// a worktree (placingVariable), and empty otherwise. With no `..` after
	// it, the word writes under that path from where the name was bound
	// (placedUnder).
	placed string
	// handed is the line a shell runs when it is handed this word: its value
	// with each expansion the hook cannot read written as
	// worktreeOuterExpansion, or empty when the hook cannot build it.
	handed string
}

// worktreeFlow is what the walk learned of one statement.
type worktreeFlow struct {
	// directory is the directory the statement leaves the next one in.
	directory worktreeDirectory
	// reads reports that a command of the statement read a path under a
	// worktree, whose output a redirection of the statement then writes.
	reads bool
	// sinks are the files the command writes from what it reads: tee's
	// files, sponge's file, dd's `of=`, the directory tar -x, git apply,
	// git am or patch writes into. They are a copy when the command read a
	// worktree file, or its input did.
	sinks []worktreeWord
	// script reports that the command is a shell that reads its script from
	// its input (heavytool.StdinShell).
	script bool
}

// worktreeCopyFinding is the judgment of one line, one variant per case:
// worktreeCopyNone, worktreeCopyFound or worktreeCopyUnread.
type worktreeCopyFinding interface {
	worktreeCopyFinding()
}

// worktreeCopyNone is a line that copies nothing out of a worktree into the
// main tree.
type worktreeCopyNone struct{}

// worktreeCopyFound is the first copy out of a worktree into the main tree a
// line makes: its destination and its source as typed, and whether the
// destination counts as the main tree only because the hook could not place
// it.
type worktreeCopyFound struct {
	target   string
	source   string
	unplaced bool
}

// worktreeCopyUnread is a line the hook could not read, whose text may copy a
// file out of a worktree (worktreeCopyText).
type worktreeCopyUnread struct{}

func (worktreeCopyNone) worktreeCopyFinding()   {}
func (worktreeCopyFound) worktreeCopyFinding()  {}
func (worktreeCopyUnread) worktreeCopyFinding() {}

// worktreeCopyJudge walks the statements of one line and records the first
// copy out of a worktree into the main tree. It is not safe for concurrent
// use; one judge reads one line.
type worktreeCopyJudge struct {
	walk readerWalk
	// root is the checkout root, absolute and cleaned.
	root string
	// remote reports that the line runs on another host (ssh, a remote
	// runner such as rex), whose checkout root the hook cannot know, so a path there is
	// judged by its own components alone.
	remote  bool
	finding worktreeCopyFinding
	// read is the first path under a worktree a command read, the source a
	// redirection's copy names.
	read string
	// inside holds the variables the line bound to a path under a worktree
	// (bind). A name is never unbound: the walk does not follow the order a
	// loop or a conditional runs in, and a stale binding only refuses more,
	// since a destination is placed by it only for a name the line assigns
	// once and never to another place (placingVariable).
	inside map[string]bool
	// boundFrom holds the directory each variable in inside was bound from,
	// for a name bound to a relative word, which names a path from that
	// directory alone. A name bound only to absolute words has no entry: it
	// names its path from anywhere.
	boundFrom map[string]worktreeDirectory
	// mixed holds the variables the line also bound to a word that is not
	// under a worktree, so a `$name` destination is not placed by inside.
	mixed map[string]bool
	// values holds the exact value of each variable the line in hand binds
	// once, by a bare assignment outside a loop or a function body
	// (worktreeLineAssignments), so a later `$name` reads as that value.
	values map[string]string
	// assignments is what the walk learned of the line in hand's
	// assignments before it judged its commands.
	assignments worktreeAssignments
	// pwdAssigned reports that the line assigns PWD, so `$PWD` no longer
	// names the directory a command runs in.
	pwdAssigned bool
	// homeAssigned reports that the line assigns HOME, so `~` and `$HOME`
	// no longer name the home directory the hook reads (homeDirectory).
	homeAssigned bool
}

// worktreeLineStart answers the directory a line starts in: the session's
// cwd, or the session's root when the envelope carries none.
func worktreeLineStart(root, cwd string) string {
	if filepath.IsAbs(cwd) {
		return filepath.Clean(cwd)
	}
	return root
}

// worktreeCopyLine judges whether a command line that starts in cwd, inside
// the main checkout at root (both absolute and cleaned), copies a file out of
// a worktree into the main tree: a copy verb run as a command (cp, mv, rsync,
// install, ln, directly, behind a launcher, through `find -exec` or `xargs`,
// or as the program an le carrier runs on this host: worktreeCarried)
// whose source is under a worktree, or may be, and whose destination is in
// the main tree, or may be; or a redirection, or a program that writes what
// it reads (tee, sponge, dd, tar -x, git apply, git am, patch), whose command
// or input read a worktree file and whose destination is in the main tree,
// or may be. The main tree is the checkout outside .claude/worktrees and
// outside its gitignored tmp/, and a destination the hook cannot place counts
// as the main tree.
//
// A command reads a worktree file when a word of it names one, a variable
// the line bound to one included, or when it runs inside a worktree or in a
// directory the hook cannot name, which may be one (readsWhereItRuns).
//
// A quoted word is a word, never a command; a line handed to a shell (`bash
// -c`, `env -S`, `eval`, a here-document or a pipe into a shell, a shell a
// later word runs) is judged as a line of its own, parsed from the word's
// value when the outer shell expands part of it (copyLine), and a line
// another host runs (`ssh`, or a shell a later word of a remote runner such
// as `rex` is handed: copyHandedSuffixes) is judged by its paths' components
// alone. A line the parser cannot read the way bash runs it is
// judged by its text (worktreeCopyText), because a hook that cannot read a
// line never clears it.
func worktreeCopyLine(command string, parse commandParse, root, cwd string) worktreeCopyFinding {
	judge := worktreeCopyJudge{
		root:      root,
		finding:   worktreeCopyNone{},
		inside:    map[string]bool{},
		boundFrom: map[string]worktreeDirectory{},
		mixed:     map[string]bool{},
	}
	judge.judgeLine(command, parse, worktreeDirectory{path: cwd})
	return judge.finding
}

// worktreeCopyText is the judgment of a line the hook cannot read: a copy verb
// or a redirection anywhere in its text. It is the refusing side, kept for the
// line whose commands the hook cannot name.
func worktreeCopyText(command string) bool {
	for _, token := range worktreeCopyTextVerbs {
		if strings.Contains(command, token) {
			return true
		}
	}
	return worktreeCopyTextRedirect.MatchString(command)
}

// judgeLine judges one line from directory and answers whether a command of
// it read a path under a worktree. The recursion into a handed line is
// bounded by the line's quoting: a handed line is the value of one word of
// the line that holds it, one level of quoting in, and the text rewritten
// writes into it holds no quote. A word that holds the variable of an outer
// expansion is handed no further (wordOf gives it no handed text). The exact
// values a line binds are its own: a handed line starts with none, since the
// outer shell already wrote each one it expands into the handed text.
func (judge *worktreeCopyJudge) judgeLine(line string, parse commandParse, directory worktreeDirectory) bool {
	if !parse.parsed() {
		return judge.judgeText(line)
	}
	if treeMisreadsLine(line, parse) != "" {
		return judge.judgeText(line)
	}
	outer, values, assignments := judge.walk, judge.values, judge.assignments
	judge.walk = readerWalk{source: line}
	judge.values = map[string]string{}
	judge.assignments = worktreeLineAssignments(parse.file)
	flow := judge.copyStatements(parse.file.Stmts, directory, false)
	judge.walk, judge.values, judge.assignments = outer, values, assignments
	return flow.reads
}

// judgeText judges a line by its text (worktreeCopyText), and answers whether
// the text may name a worktree path, which the line may read.
func (judge *worktreeCopyJudge) judgeText(line string) bool {
	if worktreeCopyText(line) {
		judge.refuseUnread()
	}
	return worktreeTextNamesClaude(line)
}

// refuse records a copy of source to target, keeping the first one found.
// unplaced reports that target counts as the main tree only because the hook
// could not place it.
func (judge *worktreeCopyJudge) refuse(target, source string, unplaced bool) {
	if _, none := judge.finding.(worktreeCopyNone); !none {
		return
	}
	judge.finding = worktreeCopyFound{target: target, source: source, unplaced: unplaced}
}

// refuseUnread records a copy in a line the hook could not read, keeping the
// first copy found.
func (judge *worktreeCopyJudge) refuseUnread() {
	if _, none := judge.finding.(worktreeCopyNone); !none {
		return
	}
	judge.finding = worktreeCopyUnread{}
}

// readsWhereItRuns reports whether a command run from directory may read a
// worktree's files without naming them: it runs inside a worktree, or in a
// directory the hook cannot name, which may be one. It notes the directory as
// what the command read. A line another host runs starts in a directory the
// hook never names, and is judged by its paths' components alone, so there
// only a `cd` whose target names a worktree (worktreeRemoteDirectory) makes a
// command read where it runs.
func (judge *worktreeCopyJudge) readsWhereItRuns(directory worktreeDirectory) bool {
	if !directory.known() {
		if judge.remote {
			return false
		}
		judge.noteRead("a directory the hook cannot name")
		return true
	}
	if !judge.inWorktree(directory.path) {
		return false
	}
	judge.noteRead(directory.path)
	return true
}

// noteRead records a path under a worktree a command read, keeping the first.
func (judge *worktreeCopyJudge) noteRead(token string) {
	if judge.read == "" {
		judge.read = token
	}
}

// readSource names what a redirection or a sink copies: what a command read
// from the first worktree path the walk met.
func (judge *worktreeCopyJudge) readSource() string {
	if judge.read == "" {
		return "what a command read under " + lepath.WorktreesDir
	}
	return "what a command read from " + judge.read
}

// copyStatements judges statements run one after another, each in the
// directory the one before it left. A statement run in the background runs in
// a subshell, so its `cd` leaves the next statement where it was.
func (judge *worktreeCopyJudge) copyStatements(statements []*syntax.Stmt, directory worktreeDirectory, upstream bool) worktreeFlow {
	flow := worktreeFlow{directory: directory}
	for _, statement := range statements {
		next := judge.copyStatement(statement, flow.directory, upstream)
		if next.reads {
			flow.reads = true
		}
		if statement.Background {
			continue
		}
		flow.directory = next.directory
	}
	return flow
}

// copyStatement judges one statement: its command, the script a shell reads
// from a here-document, then the files its redirections and its command write
// from what it read. upstream reports that an earlier stage of the pipeline
// the statement runs in read a path under a worktree, so what the statement
// writes may be that file.
func (judge *worktreeCopyJudge) copyStatement(statement *syntax.Stmt, directory worktreeDirectory, upstream bool) worktreeFlow {
	flow := judge.copyCommand(statement, directory, upstream)
	for _, redirect := range statement.Redirs {
		// A substitution in a redirection's word, or in a here-document,
		// runs commands whose output the statement reads.
		if judge.copyNested(redirect, directory, false).reads {
			flow.reads = true
		}
		if judge.redirectReads(redirect, directory) {
			flow.reads = true
		}
		if !flow.script {
			continue
		}
		if judge.copyScript(redirect, directory) {
			flow.reads = true
		}
	}
	if !flow.reads {
		if !upstream {
			return flow
		}
	}
	for _, redirect := range statement.Redirs {
		target, writes := judge.redirectTarget(redirect, directory)
		if !writes {
			continue
		}
		judge.copyInto(target, directory)
	}
	for _, sink := range flow.sinks {
		judge.copyInto(sink, directory)
	}
	return flow
}

// copyInto records a copy when a destination that receives what a command
// read from a worktree counts as the main tree (destinationOf).
func (judge *worktreeCopyJudge) copyInto(target worktreeWord, directory worktreeDirectory) {
	destination := judge.destinationOf(target, directory)
	if !destination.countsMain() {
		return
	}
	judge.refuse(target.token, judge.readSource(), destination == worktreeDestinationUnplaced)
}

// copyScript judges the here-document or here-string a shell reads its
// script from as a line the shell runs, and answers whether it read a path
// under a worktree.
func (judge *worktreeCopyJudge) copyScript(redirect *syntax.Redirect, directory worktreeDirectory) bool {
	body := worktreeScriptBody(redirect)
	if body == nil {
		return false
	}
	word := judge.wordOf(body, directory)
	return judge.copyLine(word.token, []worktreeWord{word}, directory)
}

// worktreeScriptBody answers the word a here-document (`<<`, `<<-`) or a
// here-string (`<<<`) feeds its command, and nil for any other redirection.
func worktreeScriptBody(redirect *syntax.Redirect) *syntax.Word {
	if redirect.Op == syntax.WordHdoc {
		return redirect.Word
	}
	if redirect.Op == syntax.Hdoc {
		return redirect.Hdoc
	}
	if redirect.Op == syntax.DashHdoc {
		return redirect.Hdoc
	}
	return nil
}

// copyCommand judges the command of one statement. A subshell, and every stage
// of a pipeline, runs in a directory of its own, so a `cd` inside one leaves
// the next statement where it was.
func (judge *worktreeCopyJudge) copyCommand(statement *syntax.Stmt, directory worktreeDirectory, upstream bool) worktreeFlow {
	if statement.Cmd == nil {
		// A statement of redirections alone runs no command and reads nothing.
		return worktreeFlow{directory: directory}
	}
	switch command := statement.Cmd.(type) {
	case *syntax.CallExpr:
		return judge.copyCall(command, directory)
	case *syntax.BinaryCmd:
		return judge.copyBinary(command, directory, upstream)
	case *syntax.Block:
		return judge.copyStatements(command.Stmts, directory, upstream)
	case *syntax.Subshell:
		inner := judge.copyStatements(command.Stmts, directory, upstream)
		return worktreeFlow{directory: directory, reads: inner.reads}
	case *syntax.DeclClause:
		// `export f=x`, `declare f=x`, `local f=x` bind f as a bare
		// assignment does.
		judge.bindAssigns(command.Args, directory, false)
	}
	return judge.copyCompound(statement.Cmd, directory, upstream)
}

// copyCompound judges a loop, a conditional, a case, a function or a
// declaration. Its body runs in an order the hook does not follow, so a body
// that may change the directory is judged from an unknown one, and leaves the
// next statement in an unknown one too: never in a directory that is known
// and wrong. A for loop binds its variable from the words it runs over, in
// the directory the loop starts in (bindLoop).
func (judge *worktreeCopyJudge) copyCompound(node syntax.Command, directory worktreeDirectory, upstream bool) worktreeFlow {
	if loop, isFor := node.(*syntax.ForClause); isFor {
		judge.bindLoop(loop, directory)
	}
	if worktreeMovesDirectory(node) {
		directory = worktreeDirectory{}
	}
	flow := judge.copyNested(node, directory, upstream)
	flow.directory = directory
	return flow
}

// worktreeMovesDirectory reports whether a command of node may change the
// directory (worktreeCallMoves) in any simple command of it.
func worktreeMovesDirectory(node syntax.Node) bool {
	moves := false
	syntax.Walk(node, func(inner syntax.Node) bool {
		call, ok := inner.(*syntax.CallExpr)
		if !ok {
			return !moves
		}
		moves = moves || worktreeCallMoves(call)
		return !moves
	})
	return moves
}

// worktreeCallMoves reports whether a simple command may change the
// directory: a word whose value is `cd`, `pushd` or `popd` (`"cd"` and `\cd`
// included), or a program word the shell expands or the hook cannot read
// (`$CD x`), which may be any of them. A word that only prints the name
// counts too, which is the refusing side.
func worktreeCallMoves(call *syntax.CallExpr) bool {
	tokens := make([]string, 0, len(call.Args))
	exact := make([]bool, 0, len(call.Args))
	for _, argument := range call.Args {
		value, err := readWord(argument)
		if err != nil {
			tokens, exact = append(tokens, ""), append(exact, false)
			continue
		}
		if oneOf(value.text, "cd", "pushd", "popd") {
			return true
		}
		tokens, exact = append(tokens, value.text), append(exact, !value.expands)
	}
	start := len(tokens) - len(heavytool.Program(tokens))
	if start == len(tokens) {
		return false
	}
	return !exact[start]
}

// copyBinary judges a pipeline, whose later stage reads what the earlier one
// printed, and a `&&` or `||` list, whose right side runs where the left side
// left it. `a || b` runs b only where a failed, so the directory after it is
// the one both sides leave, or unknown.
func (judge *worktreeCopyJudge) copyBinary(command *syntax.BinaryCmd, directory worktreeDirectory, upstream bool) worktreeFlow {
	if command.Op == syntax.Pipe {
		return judge.copyPipe(command, directory, upstream)
	}
	if command.Op == syntax.PipeAll {
		return judge.copyPipe(command, directory, upstream)
	}
	left := judge.copyStatement(command.X, directory, upstream)
	right := judge.copyStatement(command.Y, left.directory, upstream)
	flow := worktreeFlow{directory: right.directory, reads: left.reads || right.reads}
	if command.Op != syntax.OrStmt {
		return flow
	}
	if right.directory == left.directory {
		return flow
	}
	flow.directory = worktreeDirectory{}
	return flow
}

// copyPipe judges a pipeline. A shell that reads its script from the pipe runs
// what the stage before it printed, which the hook cannot read, so that
// stage's text is judged (judgeText).
func (judge *worktreeCopyJudge) copyPipe(command *syntax.BinaryCmd, directory worktreeDirectory, upstream bool) worktreeFlow {
	left := judge.copyStatement(command.X, directory, upstream)
	right := judge.copyStatement(command.Y, directory, upstream || left.reads)
	if right.script {
		judge.judgeText(judge.walk.text(command.X))
	}
	return worktreeFlow{directory: directory, reads: left.reads || right.reads}
}

// copyNested judges every statement node holds at any depth (the body of a
// loop, a conditional or a function), each in directory, and the statements
// of a command or process substitution in order, in a subshell of their own.
func (judge *worktreeCopyJudge) copyNested(node syntax.Node, directory worktreeDirectory, upstream bool) worktreeFlow {
	flow := worktreeFlow{directory: directory}
	syntax.Walk(node, func(inner syntax.Node) bool {
		switch inner := inner.(type) {
		case *syntax.CmdSubst:
			if judge.copyStatements(inner.Stmts, directory, false).reads {
				flow.reads = true
			}
			return false
		case *syntax.ProcSubst:
			if judge.copyStatements(inner.Stmts, directory, false).reads {
				flow.reads = true
			}
			return false
		case *syntax.Stmt:
			if judge.copyStatement(inner, directory, upstream).reads {
				flow.reads = true
			}
			return false
		}
		return true
	})
	return flow
}

// copyCall judges one simple command: the substitutions in its words, its
// assignments (bindAssigns), then its words (copyWords). An assignment in
// front of a command binds its name for the words after it too, which is the
// refusing side.
func (judge *worktreeCopyJudge) copyCall(call *syntax.CallExpr, directory worktreeDirectory) worktreeFlow {
	nested := judge.copyNested(call, directory, false)
	judge.bindAssigns(call.Assigns, directory, len(call.Args) == 0)
	if len(call.Args) == 0 {
		// Bare assignments run no command; their substitutions were judged.
		return nested
	}
	flow := judge.copyWords(judge.callWords(call, directory), directory, false)
	if nested.reads {
		flow.reads = true
	}
	return flow
}

// callWords answers the words of one simple command run from directory, its
// assignments first.
func (judge *worktreeCopyJudge) callWords(call *syntax.CallExpr, directory worktreeDirectory) []worktreeWord {
	words := make([]worktreeWord, 0, len(call.Assigns)+len(call.Args))
	for _, assign := range call.Assigns {
		token, exact := judge.walk.assignToken(assign)
		words = append(words, worktreeWord{token: token, exact: exact})
	}
	for _, argument := range call.Args {
		words = append(words, judge.wordOf(argument, directory))
	}
	return words
}

// copyWords judges the words of one command run from directory: a line it
// hands a shell, a `cd`, a copy verb, a program that writes what it reads
// (tee, sponge, dd, tar -x, git apply, git am, patch), the commands find and
// xargs run, the program an le carrier runs on this host (worktreeCarried), and a
// shell a later word of any other program runs (copyHandedSuffixes). stdin
// reports that more words arrive on the input (`xargs`), where the hook
// cannot place them. The program word is not read as a path, so a binary run
// from a worktree is no source, but a program run inside a worktree, or from
// a directory the hook cannot name, may read what is there
// (readsWhereItRuns). A program word the shell expands may be cd, pushd or
// popd, so the next statement runs in a directory the hook cannot name. The
// recursion into a carried program is bounded by the words: the carried words
// leave out at least le's program word.
func (judge *worktreeCopyJudge) copyWords(words []worktreeWord, directory worktreeDirectory, stdin bool) worktreeFlow {
	flow := worktreeFlow{directory: directory}
	tokens := worktreeTokens(words)
	prefix, _ := heavytool.Split(tokens)
	runs := directory
	if heavytool.ChangesDirectory(prefix) {
		runs = worktreeDirectory{}
	}
	flow.script = heavytool.StdinShell(tokens)
	if _, first, end, handed := heavytool.ShellCommandWords(tokens); handed {
		// The handed line stands where the program stands: its words are a
		// line the shell runs, never a path or a program.
		line, _ := heavytool.ShellCommand(tokens)
		flow.reads = judge.copyLine(line, words[first:end], runs)
		return flow
	}
	start := len(words) - len(heavytool.Program(tokens))
	if start == len(words) {
		return flow
	}
	verb := filepath.Base(words[start].token)
	arguments := words[start+1:]
	if !words[start].exact {
		flow.directory = worktreeDirectory{}
	}
	if carried, found := worktreeCarried(words[start:]); found {
		// le runs the carried program as its child on this host: a copy, a
		// shell or a program that writes what it reads among its words is
		// judged as that command, from the directory the line runs in, and a
		// `cd` among them moves the child alone. `le job run` runs the child
		// at the checkout root, so this reading is right when the line runs
		// from a checkout root, which is how agents run ./le.
		inner := judge.copyWords(carried, runs, stdin)
		inner.directory = flow.directory
		return inner
	}
	switch verb {
	case "cd", "pushd":
		if judge.remote {
			flow.directory = worktreeRemoteDirectory(arguments, runs)
			return flow
		}
		flow.directory = worktreeChangedDirectory(arguments, runs)
		return flow
	case "popd":
		flow.directory = worktreeDirectory{}
		return flow
	case evalCommand:
		flow.reads = judge.copyLine(worktreeJoined(arguments), arguments, runs)
		return flow
	case "ssh":
		flow.reads = judge.copyRemote(worktreeRemoteCommand(arguments))
		return flow
	case "find":
		flow.reads = judge.copyFindExec(arguments, runs)
		return flow
	case "xargs":
		flow.reads = judge.copyXargs(arguments, runs)
		return flow
	case "tee", "sponge":
		// tee writes its input to every operand, sponge to its one, and
		// neither reads a file by name.
		flow.sinks = worktreeOperands(arguments)
		flow.reads = judge.readsWhereItRuns(runs)
		return flow
	case "dd":
		flow = judge.copyDD(arguments, runs, flow)
		if judge.readsWhereItRuns(runs) {
			flow.reads = true
		}
		return flow
	case "tar":
		flow.sinks = worktreeTarSinks(arguments)
	case gitCommand:
		flow.sinks = worktreeGitApplySinks(arguments)
	case "patch":
		flow.sinks = worktreePatchSinks(arguments)
	}
	if worktreeCopyVerbs[verb] {
		judge.copyOperands(verb, arguments, runs, stdin)
	}
	handed := judge.copyHandedSuffixes(arguments)
	// The word it names is noted before the directory it runs in, so the
	// refusal names the file.
	flow.reads = judge.readsAny(arguments, runs, handed)
	if judge.readsWhereItRuns(runs) {
		flow.reads = true
	}
	return flow
}

// readsAny reports whether a word of arguments, other than the ones skip
// marks, names a path under a worktree, and notes the first one.
func (judge *worktreeCopyJudge) readsAny(arguments []worktreeWord, directory worktreeDirectory, skip map[int]bool) bool {
	for index, word := range arguments {
		if skip[index] {
			continue
		}
		if judge.placeOf(word, directory) != worktreePlaceInside {
			continue
		}
		judge.noteRead(word.token)
		return true
	}
	return false
}

// worktreeCarried answers the words of the program an le line carries, when
// program, one command's words from its program on, runs le (heavytool.LeWords)
// and the registry reads its line as a carrier of a program
// (leroot.CarriedArgv): `./le job run label t command cp a b` carries
// `cp a b`, and `./le exec sh -c x` carries `sh -c x`. le runs that program
// on this host, so it is a command of this host: `le exec` in the directory
// the line runs in, `le job run` at the checkout root. The guard judges it
// from the line's directory, which is the checkout root when the line runs
// from one, as agents run ./le.
func worktreeCarried(program []worktreeWord) ([]worktreeWord, bool) {
	tokens := worktreeTokens(program)
	typed, runsLe := heavytool.LeWords(tokens)
	if !runsLe {
		return nil, false
	}
	start, end, carried := leroot.CarriedArgv(typed)
	if !carried {
		return nil, false
	}
	offset := len(tokens) - len(typed)
	return program[offset+start : offset+end], true
}

// copyHandedSuffixes judges each line a shell named by a later word of a
// command is handed, where the command is no launcher (heavytool.Split) and
// no le carrier (worktreeCarried), whose words run on this host: a remote runner
// (`rex -S host run -- sh -c '<line>'`) may run it on another host, so it is
// judged as a line another host runs. It answers the indexes of arguments
// those lines were read from, which are lines, never paths. Each step moves
// past at least one word, so the loop is bounded by the number of arguments.
func (judge *worktreeCopyJudge) copyHandedSuffixes(arguments []worktreeWord) map[int]bool {
	handed := map[int]bool{}
	tokens := worktreeTokens(arguments)
	for at := 0; at < len(tokens); at++ {
		_, first, end, found := heavytool.ShellCommandWords(tokens[at:])
		if !found {
			continue
		}
		line, _ := heavytool.ShellCommand(tokens[at:])
		judge.copyRemoteLine(line, arguments[at+first:at+end])
		for index := at + first; index < at+end; index++ {
			handed[index] = true
		}
		at += end - 1
	}
	return handed
}

// copyRemote judges the line ssh hands the remote shell, empty when it runs
// none.
func (judge *worktreeCopyJudge) copyRemote(remote []worktreeWord) bool {
	if len(remote) == 0 {
		return false
	}
	return judge.copyRemoteLine(worktreeJoined(remote), remote)
}

// copyRemoteLine judges a line another host runs, from a directory the hook
// cannot name, by its paths' components alone.
func (judge *worktreeCopyJudge) copyRemoteLine(line string, words []worktreeWord) bool {
	outer := judge.remote
	judge.remote = true
	reads := judge.copyLine(line, words, worktreeDirectory{})
	judge.remote = outer
	return reads
}

// redirectReads reports whether a redirection feeds the statement a file
// under a worktree (`<`, `<>`), and notes it.
func (judge *worktreeCopyJudge) redirectReads(redirect *syntax.Redirect, directory worktreeDirectory) bool {
	if redirect.Op != syntax.RdrIn {
		if redirect.Op != syntax.RdrInOut {
			return false
		}
	}
	if redirect.Word == nil {
		return false
	}
	word := judge.wordOf(redirect.Word, directory)
	if judge.placeOf(word, directory) != worktreePlaceInside {
		return false
	}
	judge.noteRead(word.token)
	return true
}

// redirectTarget answers the file a redirection of a statement run from
// directory writes, and false for one that writes no file: an input, a
// here-document, or a `>&` to a descriptor.
func (judge *worktreeCopyJudge) redirectTarget(redirect *syntax.Redirect, directory worktreeDirectory) (worktreeWord, bool) {
	if redirect.Word == nil {
		return worktreeWord{}, false
	}
	word := judge.wordOf(redirect.Word, directory)
	if worktreeFileRedirects[redirect.Op] {
		return word, true
	}
	if redirect.Op != syntax.DplOut {
		return worktreeWord{}, false
	}
	// `>&2` and `>&-` name a descriptor; `>& log` writes the file.
	if worktreeDescriptor(word) {
		return worktreeWord{}, false
	}
	return word, true
}

// worktreeDescriptor reports whether a `>&` word, read exactly, names a
// descriptor (`2`) or closes one (`-`).
func worktreeDescriptor(word worktreeWord) bool {
	if !word.exact {
		return false
	}
	if word.token == "-" {
		return true
	}
	return strings.Trim(word.token, "0123456789") == ""
}

// worktreeChangedDirectory answers the directory a `cd` or `pushd` with these
// arguments moves to (cdTarget): unknown wherever the hook cannot tell, a
// target that does not exist included, where the cd fails.
func worktreeChangedDirectory(arguments []worktreeWord, directory worktreeDirectory) worktreeDirectory {
	tokens := worktreeTokens(arguments)
	expands := make([]bool, 0, len(arguments))
	for _, word := range arguments {
		expands = append(expands, !word.exact)
	}
	logical, _ := cdTarget(directory.path, tokens, expands)
	return worktreeDirectory{path: logical}
}

// worktreeTokens answers the tokens of words, which heavytool reads.
func worktreeTokens(words []worktreeWord) []string {
	tokens := make([]string, 0, len(words))
	for _, word := range words {
		tokens = append(tokens, word.token)
	}
	return tokens
}

// worktreeJoined answers words joined into the line a shell runs.
func worktreeJoined(words []worktreeWord) string {
	return strings.Join(worktreeTokens(words), " ")
}
