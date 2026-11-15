// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashWorktreeCopy, the guard this judgment answers for
// Related: parsedcommands.go -- parsedCall, the reading of one command's words
// Related: shellparse.go -- parseCommandLine and readWord, the parse this walk reads
package hookruntime

import (
	"path/filepath"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/ze-software/ze/internal/le/heavytool"
)

// worktreesDirectory is the directory of the checkout that holds its worktrees.
const worktreesDirectory = ".claude/worktrees"

// worktreeCopyVerbs are the programs that write copies of their other
// operands to their last one, or to the directory `-t` names.
var worktreeCopyVerbs = map[string]bool{"cp": true, "mv": true, "rsync": true, "install": true}

// worktreeFileRedirects are the redirection operators that write the file their
// word names. `>&` is judged apart (redirectTarget): it writes a file
// only when its word is no descriptor.
var worktreeFileRedirects = map[syntax.RedirOperator]bool{
	syntax.RdrOut: true, syntax.AppOut: true, syntax.RdrInOut: true, syntax.RdrClob: true, syntax.AppClob: true,
	syntax.RdrAll: true, syntax.RdrAllClob: true, syntax.AppAll: true, syntax.AppAllClob: true,
}

// worktreeFindRuns are the find actions that run the command after them, up to
// a `;` or a `+`.
var worktreeFindRuns = map[string]bool{"-exec": true, "-execdir": true, "-ok": true, "-okdir": true}

// worktreeSSHValueOptions are the ssh options that take a value, attached or
// as the next word (ssh(1)). The first word after ssh's options is the
// destination, and the words after it are the line the remote shell runs.
const worktreeSSHValueOptions = "BbcDEeFIiJLlmOoPpQRSWw"

// worktreeXargsValueOptions are the xargs options that take the next word as
// their value. The first word after xargs's options is the program it runs.
const worktreeXargsValueOptions = "aEdILnPs"

// The text judgment of a line the parser cannot read the way bash runs it
// (worktreeCopyText): a copy verb anywhere in its text, or a `>` after a space.
var (
	worktreeCopyTextVerbs    = []string{"cp ", "cp -", "mv ", "mv -", "rsync ", "install "}
	worktreeCopyTextRedirect = regexp.MustCompile(`\s>`)
)

// worktreeDirectory is the directory a command of a line runs in, as far as
// the line itself says: a path relative to the directory the line starts in,
// or an absolute one, while known holds; unknown once a `cd` the hook cannot
// read has run.
type worktreeDirectory struct {
	path  string
	known bool
}

// worktreeStart is the directory a line starts in.
var worktreeStart = worktreeDirectory{known: true}

// worktreePlace answers where a path word points.
type worktreePlace int

const (
	// worktreePlaceUnknown is a word the hook cannot place: one the shell
	// expands, or a relative path after a `cd` it could not read.
	worktreePlaceUnknown worktreePlace = iota
	// worktreePlaceInside is a path under .claude/worktrees.
	worktreePlaceInside
	// worktreePlaceOutside is a path the hook read exactly that is not.
	worktreePlaceOutside
)

// worktreeWord is one word of a command: its token (parsedCommand.tokens), and
// whether the hook read its value exactly (exactToken).
type worktreeWord struct {
	token string
	exact bool
}

// worktreeFlow is what the walk learned of one statement: the directory it
// leaves the next statement in, and whether a command of it reads a path under
// a worktree, whose output a redirection of the statement then writes.
type worktreeFlow struct {
	directory worktreeDirectory
	reads     bool
}

// worktreeCopyJudge walks the statements of one parsed line and records whether
// any of them copies a file out of a worktree. It is not safe for concurrent
// use; one judge reads one line.
type worktreeCopyJudge struct {
	walk   readerWalk
	copies bool
}

// worktreeCopyLine reports whether a command line copies a file out of a
// worktree: a copy verb run as a command (cp, mv, rsync, install, directly,
// behind a launcher, through `find -exec` or `xargs`) whose source is under
// .claude/worktrees and whose destination is not, or a redirection outside a
// worktree that writes what a command read from one. A word inside quotes or
// a here-document is data, so it copies nothing; a line handed to a shell
// (`bash -c`, `env -S`, `eval`, `ssh <host>`) is judged as a line of its own.
//
// A line the parser cannot read the way bash runs it is judged by its text
// (worktreeCopyText), because a hook that cannot read a line never clears it.
//
// The recursion into a handed line is bounded by the line's length: the line
// is made of words of one command, shorter than the line that holds them.
func worktreeCopyLine(command string, parse commandParse, directory worktreeDirectory) bool {
	if !parse.parsed() {
		return worktreeCopyText(command)
	}
	if treeMisreadsLine(command, parse) != "" {
		return worktreeCopyText(command)
	}
	judge := worktreeCopyJudge{walk: readerWalk{source: command}}
	judge.copyStatements(parse.file.Stmts, directory, false)
	return judge.copies
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

// copyStatements judges statements run one after another, each in the
// directory the one before it left.
func (judge *worktreeCopyJudge) copyStatements(statements []*syntax.Stmt, directory worktreeDirectory, upstream bool) worktreeFlow {
	flow := worktreeFlow{directory: directory}
	for _, statement := range statements {
		next := judge.copyStatement(statement, flow.directory, upstream)
		flow.directory = next.directory
		if next.reads {
			flow.reads = true
		}
	}
	return flow
}

// copyStatement judges one statement: its command, then the files its
// redirections write. upstream reports that an earlier stage of the pipeline
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
	}
	if !flow.reads && !upstream {
		return flow
	}
	for _, redirect := range statement.Redirs {
		target, writes := judge.redirectTarget(redirect)
		if !writes {
			continue
		}
		if worktreeLeaves(target, directory) {
			judge.copies = true
		}
	}
	return flow
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
	}
	return judge.copyNested(statement.Cmd, directory, upstream)
}

// copyBinary judges a pipeline, whose later stage reads what the earlier one
// printed, and a `&&` or `||` list, whose right side runs where the left side
// left it.
func (judge *worktreeCopyJudge) copyBinary(command *syntax.BinaryCmd, directory worktreeDirectory, upstream bool) worktreeFlow {
	if command.Op != syntax.Pipe && command.Op != syntax.PipeAll {
		left := judge.copyStatement(command.X, directory, upstream)
		right := judge.copyStatement(command.Y, left.directory, upstream)
		return worktreeFlow{directory: right.directory, reads: left.reads || right.reads}
	}
	left := judge.copyStatement(command.X, directory, upstream)
	right := judge.copyStatement(command.Y, directory, upstream || left.reads)
	return worktreeFlow{directory: directory, reads: left.reads || right.reads}
}

// copyNested judges every statement node holds at any depth (the body of a
// loop, a conditional or a function, a command or process substitution), each
// in directory: a `cd` inside one is not followed out of it.
func (judge *worktreeCopyJudge) copyNested(node syntax.Node, directory worktreeDirectory, upstream bool) worktreeFlow {
	flow := worktreeFlow{directory: directory}
	syntax.Walk(node, func(inner syntax.Node) bool {
		statement, ok := inner.(*syntax.Stmt)
		if !ok {
			return true
		}
		if judge.copyStatement(statement, directory, upstream).reads {
			flow.reads = true
		}
		return false
	})
	return flow
}

// copyCall judges one simple command: the substitutions in its words, a line
// it hands a shell, a `cd`, and a copy verb, and answers whether an argument
// names a path under a worktree. The program word is not read as a path, so a
// binary run from a worktree is no source.
func (judge *worktreeCopyJudge) copyCall(call *syntax.CallExpr, directory worktreeDirectory) worktreeFlow {
	flow := judge.copyNested(call, directory, false)
	if len(call.Args) == 0 {
		// Bare assignments run no command; their substitutions were judged.
		return flow
	}
	parsed := judge.walk.parsedCall(call)
	words := worktreeWords(parsed)
	program := words[len(words)-len(heavytool.Program(parsed.tokens)):]
	judge.copyHanded(parsed, words, program, directory)
	if len(program) == 0 {
		return flow
	}
	verb := filepath.Base(program[0].token)
	arguments := program[1:]
	switch verb {
	case "cd", "pushd":
		flow.directory = worktreeChangedDirectory(arguments, directory)
		return flow
	case "popd":
		flow.directory = worktreeDirectory{}
		return flow
	case "find":
		judge.copyFindExec(arguments, directory)
	case "xargs":
		judge.copyXargs(arguments, directory)
	}
	if worktreeCopyVerbs[verb] {
		judge.copyOperands(verb, arguments, directory, false)
	}
	for _, word := range arguments {
		if worktreePlaceOf(word, directory) == worktreePlaceInside {
			flow.reads = true
		}
	}
	return flow
}

// copyHanded judges the line a command hands another shell to run: a shell's
// `-c` operand or env's `-S` string (heavytool.ShellCommand), the words of
// `eval`, and the remote command of `ssh`, which the remote shell runs.
// words are parsed's tokens (worktreeWords), and program the words from its
// program on.
func (judge *worktreeCopyJudge) copyHanded(parsed parsedCommand, words, program []worktreeWord, directory worktreeDirectory) {
	if _, first, end, handed := heavytool.ShellCommandWords(parsed.tokens); handed {
		line, _ := heavytool.ShellCommand(parsed.tokens)
		judge.copyLine(line, words[first:end], directory)
	}
	if len(program) == 0 {
		return
	}
	switch filepath.Base(program[0].token) {
	case "eval":
		judge.copyLine(worktreeJoined(program[1:]), program[1:], directory)
	case "ssh":
		remote := worktreeRemoteCommand(program[1:])
		if len(remote) != 0 {
			judge.copyLine(worktreeJoined(remote), remote, worktreeStart)
		}
	}
}

// copyLine judges a line handed to a shell, read from words. A line read from
// a word the shell expands first is not the line that runs, so it is judged by
// its text (worktreeCopyText).
func (judge *worktreeCopyJudge) copyLine(line string, words []worktreeWord, directory worktreeDirectory) {
	for _, word := range words {
		if !word.exact {
			if worktreeCopyText(line) {
				judge.copies = true
			}
			return
		}
	}
	if worktreeCopyLine(line, parseCommandLine(line), directory) {
		judge.copies = true
	}
}

// copyFindExec judges each command `find` runs for the files it finds. The
// `{}` it puts in the words is a file the hook cannot place.
func (judge *worktreeCopyJudge) copyFindExec(arguments []worktreeWord, directory worktreeDirectory) {
	for at := 0; at < len(arguments); at++ {
		if !worktreeFindRuns[arguments[at].token] {
			continue
		}
		end := at + 1
		for end < len(arguments) && arguments[end].token != ";" && arguments[end].token != "+" {
			end++
		}
		run := worktreeMarkedHoles(arguments[at+1 : end])
		if len(run) != 0 && worktreeCopyVerbs[filepath.Base(run[0].token)] {
			judge.copyOperands(filepath.Base(run[0].token), run[1:], directory, false)
		}
		at = end
	}
}

// copyXargs judges the program `xargs` runs, after its options, when it is a
// copy verb: the words xargs reads from its input are sources the hook cannot
// place.
func (judge *worktreeCopyJudge) copyXargs(arguments []worktreeWord, directory worktreeDirectory) {
	for at := 0; at < len(arguments); at++ {
		token := arguments[at].token
		if strings.HasPrefix(token, "-") {
			if len(token) == 2 && strings.IndexByte(worktreeXargsValueOptions, token[1]) >= 0 {
				at++
			}
			continue
		}
		verb := filepath.Base(token)
		if worktreeCopyVerbs[verb] {
			judge.copyOperands(verb, worktreeMarkedHoles(arguments[at+1:]), directory, true)
		}
		return
	}
}

// copyOperands judges one copy verb's arguments: it copies out of a worktree
// when its destination (the `-t` directory, or else the last operand) is not
// under a worktree and a source is, or may be. stdin reports that more
// sources arrive on the input (`xargs`), where the hook cannot place them.
func (judge *worktreeCopyJudge) copyOperands(verb string, arguments []worktreeWord, directory worktreeDirectory, stdin bool) {
	var operands []worktreeWord
	var target worktreeWord
	targeted := false
	options := true
	for at := 0; at < len(arguments); at++ {
		word := arguments[at]
		if options && word.token == "--" {
			options = false
			continue
		}
		// rsync's `-t` keeps modification times; only cp, mv and install
		// take a target directory.
		if options && verb != "rsync" {
			if value, consumed, found := worktreeTargetOption(arguments, at); found {
				target, targeted = value, true
				at += consumed
				continue
			}
		}
		if options && strings.HasPrefix(word.token, "-") && word.token != "-" {
			continue
		}
		operands = append(operands, word)
	}
	if !targeted {
		if len(operands) == 0 {
			// xargs supplies every word, the destination included.
			judge.copies = judge.copies || stdin
			return
		}
		target, operands = operands[len(operands)-1], operands[:len(operands)-1]
	}
	if !worktreeLeaves(target, directory) {
		return
	}
	if stdin {
		judge.copies = true
		return
	}
	for _, source := range operands {
		if worktreePlaceOf(source, directory) != worktreePlaceOutside {
			judge.copies = true
			return
		}
	}
}

// redirectReads reports whether a redirection feeds the statement a file
// under a worktree (`<`, `<>`).
func (judge *worktreeCopyJudge) redirectReads(redirect *syntax.Redirect, directory worktreeDirectory) bool {
	if redirect.Op != syntax.RdrIn && redirect.Op != syntax.RdrInOut {
		return false
	}
	if redirect.Word == nil {
		return false
	}
	return worktreePlaceOf(judge.redirectWord(redirect.Word), directory) == worktreePlaceInside
}

// redirectTarget answers the file a redirection writes, and false for one
// that writes no file: an input, a here-document, or a `>&` to a descriptor.
func (judge *worktreeCopyJudge) redirectTarget(redirect *syntax.Redirect) (worktreeWord, bool) {
	if redirect.Word == nil {
		return worktreeWord{}, false
	}
	word := judge.redirectWord(redirect.Word)
	if worktreeFileRedirects[redirect.Op] {
		return word, true
	}
	if redirect.Op != syntax.DplOut {
		return worktreeWord{}, false
	}
	// `>&2` and `>&-` name a descriptor; `>& log` writes the file.
	if word.exact && (word.token == "-" || strings.Trim(word.token, "0123456789") == "") {
		return worktreeWord{}, false
	}
	return word, true
}

// redirectWord reads a redirection's word as the copy judgment reads an
// argument: its value when the hook reads it exactly, its typed text otherwise.
func (judge *worktreeCopyJudge) redirectWord(word *syntax.Word) worktreeWord {
	value, err := readWord(word)
	if err != nil {
		return worktreeWord{token: typedToken(judge.walk.text(word))}
	}
	if value.expands {
		return worktreeWord{token: typedToken(judge.walk.text(word))}
	}
	return worktreeWord{token: value.text, exact: true}
}

// worktreeWords answers each token of parsed with whether the hook read it
// exactly.
func worktreeWords(parsed parsedCommand) []worktreeWord {
	words := make([]worktreeWord, 0, len(parsed.tokens))
	for at, token := range parsed.tokens {
		words = append(words, worktreeWord{token: token, exact: exactToken(parsed, at)})
	}
	return words
}

// worktreePlaceOf answers where a path word points, resolved against the
// directory its command runs in. A word that names .claude/worktrees as a
// path component is inside whatever else the hook cannot read of it; any other
// word the hook cannot read, or cannot resolve, is unknown.
func worktreePlaceOf(word worktreeWord, directory worktreeDirectory) worktreePlace {
	path := word.token
	if !filepath.IsAbs(path) && !directory.known {
		if worktreePath(filepath.Clean(path)) {
			return worktreePlaceInside
		}
		return worktreePlaceUnknown
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(directory.path, path)
	}
	if worktreePath(filepath.Clean(path)) {
		return worktreePlaceInside
	}
	if word.exact {
		return worktreePlaceOutside
	}
	return worktreePlaceUnknown
}

// worktreePath reports whether a cleaned path is .claude/worktrees or under it.
func worktreePath(path string) bool {
	return strings.Contains("/"+path+"/", "/"+worktreesDirectory+"/")
}

// worktreeLeaves reports whether a copy's destination may be outside every
// worktree. A device (`/dev/null`) is no file of a checkout.
func worktreeLeaves(target worktreeWord, directory worktreeDirectory) bool {
	if target.exact && strings.HasPrefix(filepath.Clean(target.token), "/dev/") {
		return false
	}
	return worktreePlaceOf(target, directory) != worktreePlaceInside
}

// worktreeChangedDirectory answers the directory a `cd` or `pushd` with these
// arguments moves to: unknown for one the hook cannot read, for `cd` alone
// (home) and for `cd -`.
func worktreeChangedDirectory(arguments []worktreeWord, directory worktreeDirectory) worktreeDirectory {
	for _, word := range arguments {
		if strings.HasPrefix(word.token, "-") {
			// `-L`, `-P`, and `cd -`, the previous directory.
			if word.token == "-" {
				return worktreeDirectory{}
			}
			continue
		}
		if !word.exact {
			return worktreeDirectory{}
		}
		if filepath.IsAbs(word.token) {
			return worktreeDirectory{path: filepath.Clean(word.token), known: true}
		}
		if !directory.known {
			return worktreeDirectory{}
		}
		return worktreeDirectory{path: filepath.Join(directory.path, word.token), known: true}
	}
	return worktreeDirectory{}
}

// worktreeTargetOption reads a target-directory option of cp, mv or install at
// arguments[at] (`-t DIR`, `-tDIR`, `-rt DIR`, `--target-directory=DIR`,
// `--target-directory DIR`), and answers the directory and how many words
// after at the option consumed.
func worktreeTargetOption(arguments []worktreeWord, at int) (worktreeWord, int, bool) {
	word := arguments[at]
	if value, found := strings.CutPrefix(word.token, "--target-directory="); found {
		return worktreeWord{token: value, exact: word.exact}, 0, true
	}
	if word.token == "--target-directory" {
		return worktreeNextWord(arguments, at)
	}
	if !strings.HasPrefix(word.token, "-") {
		return worktreeWord{}, 0, false
	}
	if strings.HasPrefix(word.token, "--") {
		return worktreeWord{}, 0, false
	}
	_, value, found := strings.Cut(word.token[1:], "t")
	if !found {
		return worktreeWord{}, 0, false
	}
	if value != "" {
		return worktreeWord{token: value, exact: word.exact}, 0, true
	}
	return worktreeNextWord(arguments, at)
}

// worktreeNextWord answers the word after at as an option's value.
func worktreeNextWord(arguments []worktreeWord, at int) (worktreeWord, int, bool) {
	if at+1 < len(arguments) {
		return arguments[at+1], 1, true
	}
	return worktreeWord{}, 0, false
}

// worktreeRemoteCommand answers the words of an ssh command after its options
// and its destination: the line the remote shell runs, empty when it runs none.
func worktreeRemoteCommand(arguments []worktreeWord) []worktreeWord {
	for at := 0; at < len(arguments); at++ {
		token := arguments[at].token
		if token == "--" {
			if at+2 <= len(arguments) {
				return arguments[at+2:]
			}
			return nil
		}
		if !strings.HasPrefix(token, "-") {
			return arguments[at+1:]
		}
		if token == "-" {
			return arguments[at+1:]
		}
		if worktreeSSHTakesNext(token) {
			at++
		}
	}
	return nil
}

// worktreeSSHTakesNext reports whether an ssh option word takes the next word
// as its value: its last option letter takes a value, which is not attached.
func worktreeSSHTakesNext(token string) bool {
	for index := 1; index < len(token); index++ {
		if strings.IndexByte(worktreeSSHValueOptions, token[index]) >= 0 {
			return index == len(token)-1
		}
	}
	return false
}

// worktreeMarkedHoles answers words with each `{}` marked a word the hook
// cannot place: find and xargs put a file name there.
func worktreeMarkedHoles(words []worktreeWord) []worktreeWord {
	marked := make([]worktreeWord, 0, len(words))
	for _, word := range words {
		if strings.Contains(word.token, "{}") {
			word.exact = false
		}
		marked = append(marked, word)
	}
	return marked
}

// worktreeJoined answers words joined into the line a shell runs.
func worktreeJoined(words []worktreeWord) string {
	tokens := make([]string, 0, len(words))
	for _, word := range words {
		tokens = append(tokens, word.token)
	}
	return strings.Join(tokens, " ")
}
