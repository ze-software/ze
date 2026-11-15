// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: worktreecopy.go -- worktreeCopyJudge, the walk these program readers serve
// Related: bash.go -- bashWorktreeCopy, the guard this judgment answers for
// Related: gobuild.go -- cdTarget, the directory a cd moves to
package hookruntime

import (
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/heavytool"
)

// worktreeFindRuns are the find actions that run the command after them, up to
// a `;` or a `+`.
var worktreeFindRuns = map[string]bool{"-exec": true, "-execdir": true, "-ok": true, "-okdir": true}

// worktreeFindRunsElsewhere are the find actions that run their command in the
// directory of the file found, which the hook cannot name.
var worktreeFindRunsElsewhere = map[string]bool{"-execdir": true, "-okdir": true}

// worktreeSSHValueOptions are the ssh options that take a value, attached or
// as the next word (ssh(1)). The first word after ssh's options is the
// destination, and the words after it are the line the remote shell runs.
const worktreeSSHValueOptions = "BbcDEeFIiJLlmOoPpQRSWw"

// worktreeXargs is xargs as the launcher declaration holds it, whose options
// that take the next word as their value copyXargs skips. The first word
// after xargs's options is the program it runs.
var worktreeXargs = heavytool.MustLauncher("xargs")

// worktreePatchValueOptions are the patch options that take the next word as
// their value (patch(1)).
var worktreePatchValueOptions = map[string]bool{
	"-i": true, "-o": true, "-r": true, "-d": true, "-D": true, "-p": true, "-F": true, "-z": true,
	"-B": true, "-Y": true, "-V": true, "-g": true, "--input": true, "--output": true,
	"--reject-file": true, "--directory": true, "--ifdef": true, "--strip": true, "--fuzz": true,
	"--suffix": true, "--prefix": true, "--basename-prefix": true, "--version-control": true, "--get": true,
}

// copyFindExec judges each command `find` runs for the files it finds, and
// answers whether find reads under a worktree: its own words name a path
// there, or it runs where it may find one (readsWhereItRuns). The `{}` it
// puts in the words is a file the hook cannot place.
func (judge *worktreeCopyJudge) copyFindExec(arguments []worktreeWord, directory worktreeDirectory) bool {
	reads := judge.readsAny(arguments, directory, nil)
	if judge.readsWhereItRuns(directory) {
		reads = true
	}
	for at := 0; at < len(arguments); at++ {
		action := arguments[at].token
		if !worktreeFindRuns[action] {
			continue
		}
		end := at + 1
		for end < len(arguments) {
			if oneOf(arguments[end].token, ";", "+") {
				break
			}
			end++
		}
		runs := directory
		if worktreeFindRunsElsewhere[action] {
			runs = worktreeDirectory{}
		}
		judge.copyRun(worktreeMarkedHoles(arguments[at+1:end]), runs, false, reads)
		at = end
	}
	return reads
}

// copyXargs judges the program `xargs` runs, after its options: the words xargs
// reads from its input are files the hook cannot place, so they may be under a
// worktree.
func (judge *worktreeCopyJudge) copyXargs(arguments []worktreeWord, directory worktreeDirectory) bool {
	for at := 0; at < len(arguments); at++ {
		token := arguments[at].token
		if !strings.HasPrefix(token, "-") {
			return judge.copyRun(worktreeMarkedHoles(arguments[at:]), directory, true, true)
		}
		if worktreeXargs.TakesNextWord(token) {
			at++
		}
	}
	return false
}

// copyRun judges a command find or xargs runs (copyWords), and answers whether
// its words name a path under a worktree. fed reports that the files it is
// given may be under a worktree, so a file it writes from what it reads is a
// copy.
func (judge *worktreeCopyJudge) copyRun(words []worktreeWord, directory worktreeDirectory, stdin, fed bool) bool {
	if len(words) == 0 {
		return false
	}
	flow := judge.copyWords(words, directory, stdin)
	if !fed {
		if !flow.reads {
			return false
		}
	}
	for _, sink := range flow.sinks {
		judge.copyInto(sink, directory)
	}
	return flow.reads
}

// copyOperands judges one copy verb's arguments: it copies out of a worktree
// when its destination (the `-t` directory, or else the last operand) is in
// the main tree, or may be, and a source is under a worktree, or may be. stdin
// reports that more sources arrive on the input (`xargs`), where the hook
// cannot place them.
func (judge *worktreeCopyJudge) copyOperands(verb string, arguments []worktreeWord, directory worktreeDirectory, stdin bool) {
	var operands []worktreeWord
	var target worktreeWord
	targeted := false
	options := true
	for at := 0; at < len(arguments); at++ {
		word := arguments[at]
		if options {
			if word.token == "--" {
				options = false
				continue
			}
		}
		// rsync's `-t` keeps modification times; cp, mv, install and ln
		// take a target directory.
		if worktreeTakesTarget(verb, options) {
			if value, consumed, found := worktreeTargetOption(arguments, at); found {
				target, targeted = value, true
				at += consumed
				continue
			}
		}
		if worktreeOption(word, options) {
			continue
		}
		operands = append(operands, word)
	}
	if !targeted {
		if len(operands) == 0 {
			// xargs supplies every word, the destination included, which the
			// hook cannot place.
			if stdin {
				judge.refuse("the last file name xargs reads from its input", "the file names before it", true)
			}
			return
		}
		target, operands = operands[len(operands)-1], operands[:len(operands)-1]
	}
	if _, remote := worktreeHostPath(verb, target); remote {
		// rsync writes a `host:path` destination on another host, never into
		// the main tree.
		return
	}
	destination := judge.destinationOf(target, directory)
	if !destination.countsMain() {
		return
	}
	unplaced := destination == worktreeDestinationUnplaced
	if stdin {
		judge.refuse(target.token, "the files xargs reads from its input", unplaced)
		return
	}
	for _, source := range operands {
		if judge.sourceOf(verb, source, directory) == worktreePlaceOutside {
			continue
		}
		judge.refuse(target.token, source.token, unplaced)
		return
	}
}

// sourceOf answers where a copy verb's source points (placeOf). An rsync
// `host:path` source is on another host, outside the checkout, unless its own
// components name a worktree.
func (judge *worktreeCopyJudge) sourceOf(verb string, source worktreeWord, directory worktreeDirectory) worktreePlace {
	path, remote := worktreeHostPath(verb, source)
	if !remote {
		return judge.placeOf(source, directory)
	}
	if worktreePath(filepath.Clean(path)) {
		return worktreePlaceInside
	}
	return worktreePlaceOutside
}

// worktreeHostPath answers the path of an rsync operand read exactly that
// names another host, `host:path` or `host::module/path`, with a colon before
// any slash, and false for any other word. cp, mv, install and ln have no
// remote form, so their `a:b` is a file of that name.
func worktreeHostPath(verb string, word worktreeWord) (string, bool) {
	if verb != worktreeRsync {
		return "", false
	}
	if !word.exact {
		return "", false
	}
	colon := strings.IndexByte(word.token, ':')
	if colon <= 0 {
		return "", false
	}
	slash := strings.IndexByte(word.token, '/')
	if slash < 0 {
		return strings.TrimLeft(word.token[colon+1:], ":"), true
	}
	if slash < colon {
		return "", false
	}
	return strings.TrimLeft(word.token[colon+1:], ":"), true
}

// worktreeTakesTarget reports whether a copy verb's word may be its
// target-directory option: before `--`, and for any verb but rsync.
func worktreeTakesTarget(verb string, options bool) bool {
	if !options {
		return false
	}
	return verb != worktreeRsync
}

// worktreeOption reports whether a word before `--` is an option, which no
// operand is: `-` alone is the standard input.
func worktreeOption(word worktreeWord, options bool) bool {
	if !options {
		return false
	}
	if word.token == "-" {
		return false
	}
	return strings.HasPrefix(word.token, "-")
}

// copyDD judges dd's operands: `if=` is the file it reads, `of=` the file it
// writes.
func (judge *worktreeCopyJudge) copyDD(arguments []worktreeWord, directory worktreeDirectory, flow worktreeFlow) worktreeFlow {
	for _, word := range arguments {
		if source, found := strings.CutPrefix(word.token, "if="); found {
			read := worktreeWord{token: source, exact: word.exact}
			if judge.placeOf(read, directory) == worktreePlaceInside {
				judge.noteRead(source)
				flow.reads = true
			}
			continue
		}
		if target, found := strings.CutPrefix(word.token, "of="); found {
			flow.sinks = append(flow.sinks, worktreeWord{token: target, exact: word.exact})
		}
	}
	return flow
}

// worktreeOperands answers a command's words that are no option: every word
// after `--`, and before it every word that does not start with `-`.
func worktreeOperands(arguments []worktreeWord) []worktreeWord {
	operands := make([]worktreeWord, 0, len(arguments))
	options := true
	for _, word := range arguments {
		if options {
			if word.token == "--" {
				options = false
				continue
			}
			if strings.HasPrefix(word.token, "-") {
				continue
			}
		}
		operands = append(operands, word)
	}
	return operands
}

// worktreeTarSinks answers the directory tar extracts into, when it extracts:
// `-C`, `--directory`, or the directory it runs in.
func worktreeTarSinks(arguments []worktreeWord) []worktreeWord {
	if !worktreeTarExtracts(arguments) {
		return nil
	}
	directory := worktreeWord{token: ".", exact: true}
	for at := 0; at < len(arguments); at++ {
		word := arguments[at]
		if value, found := strings.CutPrefix(word.token, "--directory="); found {
			directory = worktreeWord{token: value, exact: word.exact}
			continue
		}
		if !oneOf(word.token, "-C", "--directory") {
			continue
		}
		if at+1 < len(arguments) {
			directory = arguments[at+1]
			at++
		}
	}
	return []worktreeWord{directory}
}

// worktreeTarExtracts reports whether tar's words ask it to extract: `-x` in a
// cluster of short options, `--extract`, `--get`, or an `x` in the old-style
// first word (`tar xzf a.tar`).
func worktreeTarExtracts(arguments []worktreeWord) bool {
	for index, word := range arguments {
		if oneOf(word.token, "--extract", "--get") {
			return true
		}
		if strings.HasPrefix(word.token, "--") {
			continue
		}
		if strings.HasPrefix(word.token, "-") {
			if strings.ContainsRune(word.token[1:], 'x') {
				return true
			}
			continue
		}
		if index != 0 {
			continue
		}
		if strings.ContainsRune(word.token, 'x') {
			return true
		}
	}
	return false
}

// worktreeGitApplySinks answers the directory `git apply` or `git am` writes
// its input into, the one its `-C` names or the one it runs in, and nothing
// for any other git command.
func worktreeGitApplySinks(arguments []worktreeWord) []worktreeWord {
	directory := worktreeWord{token: ".", exact: true}
	for at := 0; at < len(arguments); at++ {
		token := arguments[at].token
		if token == "-C" {
			if at+1 < len(arguments) {
				directory = arguments[at+1]
			}
			at++
			continue
		}
		if token == "-c" {
			at++
			continue
		}
		if strings.HasPrefix(token, "-") {
			continue
		}
		if !oneOf(token, "apply", "am") {
			return nil
		}
		return []worktreeWord{directory}
	}
	return nil
}

// worktreePatchSinks answers what patch writes: the directory it works in
// (`-d`, or the one it runs in), the file its first operand names, and the
// file `-o` names, each under that directory.
func worktreePatchSinks(arguments []worktreeWord) []worktreeWord {
	directory := worktreeWord{token: ".", exact: true}
	var files []worktreeWord
	operands := 0
	for at := 0; at < len(arguments); at++ {
		word := arguments[at]
		if value, found := strings.CutPrefix(word.token, "--directory="); found {
			directory = worktreeWord{token: value, exact: word.exact}
			continue
		}
		if value, found := strings.CutPrefix(word.token, "--output="); found {
			files = append(files, worktreeWord{token: value, exact: word.exact})
			continue
		}
		if worktreePatchValueOptions[word.token] {
			if at+1 < len(arguments) {
				directory, files = worktreePatchValue(word.token, arguments[at+1], directory, files)
			}
			at++
			continue
		}
		if strings.HasPrefix(word.token, "-") {
			continue
		}
		// patch [originalfile [patchfile]]: the first operand is the file
		// it patches, the second the patch it reads.
		operands++
		if operands == 1 {
			files = append(files, word)
		}
	}
	sinks := make([]worktreeWord, 0, 1+len(files))
	sinks = append(sinks, directory)
	for _, file := range files {
		sinks = append(sinks, worktreeWord{token: filepath.Join(directory.token, file.token), exact: file.exact && directory.exact})
	}
	return sinks
}

// worktreePatchValue reads the value of one patch option that names where it
// writes: `-d` the directory, `-o` the output file.
func worktreePatchValue(option string, value, directory worktreeWord, files []worktreeWord) (worktreeWord, []worktreeWord) {
	if oneOf(option, "-d", "--directory") {
		return value, files
	}
	if oneOf(option, "-o", "--output") {
		return directory, append(files, value)
	}
	return directory, files
}

// worktreeTargetOption reads a target-directory option of cp, mv, install or
// ln at arguments[at] (`-t DIR`, `-tDIR`, `-rt DIR`,
// `--target-directory=DIR`, `--target-directory DIR`), and answers the
// directory and how many words after at the option consumed.
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

// worktreeMarkedHoles answers words with each word holding `{}` marked a
// word the hook cannot place: find and xargs put a file name there. A shell
// handed such a word parses the file name in place of the `{}`, so its
// handed text holds worktreeOuterExpansion there.
func worktreeMarkedHoles(words []worktreeWord) []worktreeWord {
	marked := make([]worktreeWord, 0, len(words))
	for _, word := range words {
		if strings.Contains(word.token, "{}") {
			word.exact = false
			word.handed = strings.ReplaceAll(word.handed, "{}", worktreeOuterExpansion)
		}
		marked = append(marked, word)
	}
	return marked
}
