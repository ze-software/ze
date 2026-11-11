// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashRawHeavy, the guard whose refusal this writes
// Related: lossypipe.go -- goTest, goVet and the short package form both refusals name
// Related: parsedcommands.go -- everyCommand, the parsed commands the tree judgment reads
package hookruntime

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// rawRefusal is what the raw-job refusal advises for one segment: what ran
// raw, the command to use instead, and, when that command is the long
// admitted form of a go test or go vet, the short package action the caller
// can reach by dropping what the line carries beyond it.
type rawRefusal struct {
	what  string
	use   string
	short string
	// vet marks a go vet, whose short line says the action vets.
	vet bool
	// uncarried holds the directory changes the line makes before the run,
	// as typed, when the long form cannot carry them (keptArgv): the refusal
	// then says to run that form from the directory they reach.
	uncarried string
}

// textRawHeavy is the raw-job guard's text judgment, its verdict at HEAD for
// a line that is not a reader line: the first segment of the quote-blind
// split (commandSegments) that runs a heavy job, judged with the directory
// changes before it, and a zero what when none does.
func textRawHeavy(command, root string) rawRefusal {
	var moves []string
	for _, segment := range commandSegments(command) {
		if words := shellWords(segment); len(words) != 0 && oneOf(words[0], "cd", "pushd") {
			moves = append(moves, segment)
			continue
		}
		if refusal := rawHeavy(segment, moves, root); refusal.what != "" {
			return refusal
		}
	}
	return rawRefusal{}
}

// treeRawHeavy is the raw-job guard's tree judgment (AC-9): every simple
// command of a line that parses (parse, the call's one parse), at any depth
// (everyCommand), judged by the word judgment the text judgment applies to a
// segment (rawHeavyWords), over the command's own assignment nodes and words
// (parsedCommand.tokens). The text split reads the first word of each piece,
// so `for p in a b; do go test ./$p; done` and `( go test ./x )` read there as
// the commands `do` and `(`; the tree reads `go test`. It reads command words
// only, so a heavy job that is an argument of another command
// (`./le job run label unit-pkg quiet command go test ./x`) passes, and a
// string handed to a shell is never opened.
//
// It answers the first heavy command in source order, with the directory
// changes before it, written from its parsed words (parsedCommand.written,
// a word the shell expands as typed), so a redirection is never read as one
// of its words, a quoted bar never cuts it, and the advice runs what the
// caller ran (AC-15), and
// a zero what for a line that does not parse or runs no heavy job.
//
// A panic inside the walk is answered as a refusal that says so
// (unreadRawRefusal): a line the hook failed to read is never cleared, as
// the reader-line predicate answers it (review I4). readTreeRawHeavy is the
// walk itself, which the fuzz target calls without this recover.
func treeRawHeavy(command string, parse commandParse, root string) (refusal rawRefusal) {
	defer func() {
		if recovered := recover(); recovered != nil {
			refusal = unreadRawRefusal(recovered)
		}
	}()
	return readTreeRawHeavy(command, parse, root)
}

// unreadRawRefusal is the raw-job refusal for a line whose tree walk
// panicked: it names the failure, and advises the generic admitted form,
// there being no parsed command to write back.
func unreadRawRefusal(recovered any) rawRefusal {
	return rawRefusal{
		what: "a line the hook failed to read (" + fmt.Sprint(recovered) + "), which may hold a heavy job",
		use:  "./le job run label <label> quiet command <argv...>",
	}
}

// readTreeRawHeavy is treeRawHeavy without its recover.
func readTreeRawHeavy(command string, parse commandParse, root string) rawRefusal {
	if !parse.parsed() {
		return rawRefusal{}
	}
	var moves []string
	for _, parsed := range everyCommand(command, parse.file.Stmts) {
		if oneOf(parsed.commandWord(), "cd", "pushd") {
			moves = append(moves, parsed.segment())
			continue
		}
		if refusal := rawHeavyWords(parsed.tokens, parsed.written, moves, root); refusal.what != "" {
			return refusal
		}
	}
	return rawRefusal{}
}

// rawHeavy answers the refusal for a segment that runs a heavy job raw, and
// a zero what for any other segment: rawHeavyWords over the segment's words
// as the text judgment reads them (shellWords), each written back through
// quotedWord, there being no parse to say which of them the shell expands.
func rawHeavy(segment string, moves []string, root string) rawRefusal {
	words := shellWords(segment)
	return rawHeavyWords(words, quotedWords(words), moves, root)
}

// rawHeavyWords is the word judgment of the raw-job guard, the one the text
// judgment applies to a segment's words and the tree judgment to a parsed
// command's tokens. It answers the refusal for words that run a heavy job
// raw, and a zero what for any other words. written holds each token as the
// refusal writes it back, one shell word each (parsedCommand.written), so
// the advice runs the words the agent typed. moves holds the text of each
// directory change the line makes before them, as typed: a relative package
// then names a directory the short action, which runs at the checkout root,
// would not test, and the long form carries them as `env -C`, or names those
// it cannot carry (keptArgv). root is the checkout the short action runs in.
// MUST be called with written as long as tokens.
func rawHeavyWords(tokens, written, moves []string, root string) rawRefusal {
	if len(tokens) == 0 {
		return rawRefusal{}
	}
	// prefix holds each assignment and launcher word in front of the program:
	// either one changes how the run behaves, so the short action, which
	// carries neither, is not advised, and the long form keeps them.
	var prefix []string
	for len(tokens) != 0 {
		head := tokens[0]
		if strings.HasPrefix(head, "ZE_ADMIT_RAW=") && strings.Trim(strings.TrimPrefix(head, "ZE_ADMIT_RAW="), "'\"") != "" {
			return rawRefusal{}
		}
		if assignmentWord(head) {
			prefix = append(prefix, head)
			tokens = tokens[1:]
			continue
		}
		if _, launcher := launcherValueOptions[head]; launcher {
			skipped := 1 + launcherArguments(head, tokens[1:])
			prefix = append(prefix, tokens[:skipped]...)
			tokens = tokens[skipped:]
			continue
		}
		break
	}
	if len(tokens) == 0 {
		return rawRefusal{}
	}
	kept, uncarried := keptArgv(moves, prefix, written)
	prefixed := len(moves) != 0 || len(prefix) != 0
	// The prefix words are the first ones, so the program's own written
	// words are the rest.
	program := written[len(prefix):]
	if goTest(tokens) {
		refusal := packageRefusal("`go test`", testLabel, prefixed, tokens, program, kept, root)
		refusal.uncarried = uncarried
		return refusal
	}
	if goVet(tokens) {
		refusal := packageRefusal("`go vet`", vetLabel, prefixed, tokens, program, kept, root)
		refusal.uncarried = uncarried
		return refusal
	}
	if filepath.Base(tokens[0]) == lintCommand {
		sub := ""
		for _, token := range tokens[1:] {
			if !strings.HasPrefix(token, "-") {
				sub = token
				break
			}
		}
		if oneOf(sub, "config", "version", "help", "linters", "cache", "completion") {
			return rawRefusal{}
		}
		return rawRefusal{what: "`golangci-lint`", use: admittedCommand("lint", kept), uncarried: uncarried}
	}
	return rawRefusal{}
}

// assignmentWord reports a word the shell reads as an assignment in front of
// a program: a name, then `=`, before any `/`.
func assignmentWord(word string) bool {
	return strings.Contains(strings.Split(word, "/")[0], "=")
}

// keptArgv answers the argv the long admitted form runs for a segment, as
// shell words, so it runs what the caller typed (review I1), and the
// directory changes it could not carry, as typed, or "" when it carried them
// all. prefix holds the tokens in front of the program (rawHeavyWords), and
// written the shell words of the prefix and the program, as the refusal
// writes them back. The directory the line's changes reach goes in front of
// the program as `env -C <dir>` (moveDirectory, which admits only a literal
// word): the form holds no separator, which the hook's quote-blind split
// would cut at, so following the advice is not refused again (review
// ISSUE). An assignment in front of the program goes through `env`, which
// sets it the same way; a launcher stays as typed.
func keptArgv(moves, prefix, written []string) ([]string, string) {
	argv := make([]string, 0, 4+len(written))
	uncarried := ""
	if len(moves) != 0 {
		directory, carried := moveDirectory(moves)
		if carried {
			argv = append(argv, "env", "-C", directory)
		}
		if !carried {
			uncarried = strings.Join(moves, " && ")
		}
	}
	if len(argv) == 0 && len(prefix) != 0 && assignmentWord(prefix[0]) {
		argv = append(argv, "env")
	}
	return append(argv, written...), uncarried
}

// moveDirectory answers the directory a line's directory changes reach,
// relative to where the line starts, when `env -C` can carry them: each one a
// literal `cd <dir>`, whose word the shell reads as it is written. A move to
// the previous directory (`cd -`), home (a bare `cd`), a `pushd`, a flag, and
// a word the shell would expand or that holds a quoted character answer
// false, and the refusal then keeps the moves as a sentence (keptArgv). A
// relative change joins the directory so far, cleaned the way the shell's
// logical `cd` resolves `..`; an absolute one replaces it.
func moveDirectory(moves []string) (string, bool) {
	directory := ""
	for _, move := range moves {
		words := shellWords(move)
		if len(words) != 2 {
			return "", false
		}
		if words[0] != "cd" {
			return "", false
		}
		target := words[1]
		if strings.HasPrefix(target, "-") {
			return "", false
		}
		if !literalWord.MatchString(target) {
			return "", false
		}
		if directory == "" || filepath.IsAbs(target) {
			directory = target
			continue
		}
		directory = filepath.Join(directory, target)
	}
	return directory, directory != ""
}

// packageRuns is the short package action's own judgment of a package value:
// testunit.PackageRuns, which testunit's register.go installs through
// RegisterPackageRuns. The hook cannot import testunit, which reaches this
// package through repo/changed.
var packageRuns func(root, pkg string) (bool, error)

// RegisterPackageRuns installs the short package action's judgment of a
// package value, the one its `path` refusal reads, so the raw-job refusal
// never advises a short form the action refuses (review N7: a tree spanning
// groups, AC-12, and a package behind a group's own tags, AC-11). MUST be
// called from testunit's init.
func RegisterPackageRuns(runs func(root, pkg string) (bool, error)) {
	packageRuns = runs
}

// actionRuns reports whether the short action runs pkg in the checkout at
// root. With no judgment installed, or one that cannot read the checkout, it
// answers no: the refusal then advises the long form, which runs what was
// typed, rather than a short form the action may refuse.
func actionRuns(root, pkg string) bool {
	// refuse-by-default: guards shortPackageAction -- no judgment keeps the long form, which runs what was typed
	if packageRuns == nil {
		return false
	}
	runs, err := packageRuns(root, pkg)
	// refuse-by-default: guards shortPackageAction -- an unread checkout keeps the long form, which runs what was typed
	if err != nil {
		return false
	}
	return runs
}

// The job labels of a package test and a package vet, the labels the short
// package action admits them under (`./le test unit pkg`).
const (
	testLabel = "unit-pkg"
	vetLabel  = "vet-pkg"
)

// The words of the short package action a refusal writes, as its action table
// (internal/le/test/unit/pkg.go) declares them.
const (
	pkgPathKeyword = "path"
	pkgRunKeyword  = "run"
	pkgVetKeyword  = "vet"
)

// pkgActionCommand is the command line of the short package action, before
// its keywords.
const pkgActionCommand = "./le test unit pkg"

// PackageActionWords answers the labels, the command and the keywords of the
// short package action that the raw-job refusal writes. They are copies,
// because the hook cannot import testunit (packageRuns), so testunit's own
// test compares each with the constant its action table reads (review N6).
// The keys are "test-label", "vet-label", "command", "path", "run" and "vet".
func PackageActionWords() map[string]string {
	return map[string]string{
		"test-label": testLabel,
		"vet-label":  vetLabel,
		"command":    pkgActionCommand,
		"path":       pkgPathKeyword,
		"run":        pkgRunKeyword,
		"vet":        pkgVetKeyword,
	}
}

// packageRefusal advises the short package action for a go test or go vet
// whose words it expresses exactly, and otherwise the quiet long admitted form
// with one line naming the short action: a suggestion that dropped a flag, an
// assignment or a launcher would change what the run does
// (spec-agent-economy-2-scoped-test AC-7). The long form is quiet, as the
// piped-command refusal's is, so the run's output goes to the job log and not
// into the caller's context (spec-agent-economy-6-shell-parser AC-20).
// written holds each token of the program as the refusal writes it back,
// and kept the long form's argv as shell words (keptArgv).
func packageRefusal(what, label string, prefixed bool, tokens, written, kept []string, root string) rawRefusal {
	vet := goVet(tokens)
	if !prefixed {
		if form, exact := shortPackageAction(tokens, written, root); exact {
			return rawRefusal{what: what, use: form, vet: vet}
		}
	}
	short := shortPackageForm + " [" + pkgRunKeyword + " <regex>]"
	if vet {
		short = shortPackageForm + " " + pkgVetKeyword
	}
	return rawRefusal{what: what, use: admittedCommand(label, kept), short: short, vet: vet}
}

// shortPackageAction writes the `./le test unit pkg` form of the words of a
// go test or go vet, and answers false when the action cannot express them
// exactly. A go test may carry `-race`, which the action always sets, and one
// `-run` value; a go vet carries no flag. Every other flag, a line naming no
// package (go then tests the current directory), the whole tree (which the
// action refuses), a package the action's own judgment does not run
// (actionRuns), and a value the refusal cannot write as one shell word all
// answer false. So does a word the refusal writes back by its source text
// (writtenByValue), one the shell expands: the action's judgment and the
// words this form writes read the value the hook holds, the typed text,
// while the shell passes the expanded one, so only the long form, which
// writes that word as typed, runs what the caller ran. written holds each
// token as the refusal writes it back (parsedCommand.written). MUST be
// called with words goTest or goVet accepts, and written as long.
func shortPackageAction(tokens, written []string, root string) (string, bool) {
	for index, token := range tokens {
		if !writtenByValue(token, written[index]) {
			return "", false
		}
	}
	vet := goVet(tokens)
	var packages []string
	run, runGiven := "", false
	for index := 2; index < len(tokens); index++ {
		token := tokens[index]
		if !strings.HasPrefix(token, "-") {
			if oneOf(token, "./...", "...") {
				return "", false
			}
			packages = append(packages, token)
			continue
		}
		if vet {
			return "", false
		}
		if token == "-race" {
			continue
		}
		value, isRun := strings.CutPrefix(token, "-run=")
		if token == "-run" && index+1 < len(tokens) {
			index++
			value, isRun = tokens[index], true
		}
		if !isRun {
			return "", false
		}
		if runGiven {
			return "", false
		}
		run, runGiven = value, true
	}
	if len(packages) == 0 {
		return "", false
	}
	var form strings.Builder
	form.WriteString(pkgActionCommand)
	for _, name := range packages {
		if !actionRuns(root, name) {
			return "", false
		}
		word, quoted := shellWord(name)
		if !quoted {
			return "", false
		}
		form.WriteString(" " + pkgPathKeyword + " " + word)
	}
	if runGiven {
		// The action's parser refuses a dash-leading value (keywordValue), so
		// a `-run` value that opens with a dash keeps the long form.
		if run == "" {
			return "", false
		}
		if strings.HasPrefix(run, "-") {
			return "", false
		}
		word, quoted := shellWord(run)
		if !quoted {
			return "", false
		}
		form.WriteString(" " + pkgRunKeyword + " " + word)
	}
	if vet {
		form.WriteString(" " + pkgVetKeyword)
	}
	return form.String(), true
}

// shellWord writes value as one shell word: as it is when every character is
// one the shell reads literally, and in single quotes otherwise. A value that
// holds a single quote cannot be written that way and answers false.
func shellWord(value string) (string, bool) {
	if literalWord.MatchString(value) {
		return value, true
	}
	if strings.Contains(value, "'") {
		return "", false
	}
	return "'" + value + "'", true
}

// literalWord matches a word the shell reads as it is written.
var literalWord = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]+$`)

// launcherNumber matches a launcher's numeric argument, a count or a duration
// (`nice 10`, `timeout 60`, `timeout 1.5m`), which launcherArguments skips to
// reach the program. It is compiled once, at package load, because both
// launcher loops run on every Bash call. A negative value needs no sign here:
// launcherArguments skips a word that starts with `-` before it asks this
// pattern.
var launcherNumber = regexp.MustCompile(`^\d+(?:\.\d+)?[smhd]?$`)

// launcherValueOptions is the one declaration of the launchers the text
// judgment reads through, rawHeavy's and programWords', each with the options
// whose value is the next word. Its keys are the launchers. An option missing
// from a launcher's set is skipped alone, so a value it takes would be read
// as the program; one joined to its value (`--user=u`, `-uroot`) is one word
// and needs no entry. A shell's `-c` and env's `-S` are left out on purpose:
// their value is the command string, which the text judgment reads where the
// program stands, so skipping it would read the word after it instead.
var launcherValueOptions = map[string]map[string]bool{
	"bash":         {"-o": true, "+o": true, "-O": true, "+O": true, "--rcfile": true, "--init-file": true},
	"sh":           {"-o": true, "+o": true},
	"perl":         {},
	"ruby":         {"-C": true, "-E": true, "-I": true, "-r": true},
	sudoCommand:    {"-C": true, "-D": true, "-g": true, "-p": true, "-R": true, "-r": true, "-T": true, "-t": true, "-U": true, "-u": true, "--chdir": true, "--chroot": true, "--close-from": true, "--command-timeout": true, "--group": true, "--host": true, "--other-user": true, "--prompt": true, "--role": true, "--type": true, "--user": true},
	timeCommand:    {"-f": true, "-o": true, "--format": true, "--output": true},
	niceCommand:    {"-n": true, "--adjustment": true},
	"env":          {"-C": true, "-u": true, "--chdir": true, "--unset": true},
	timeoutCommand: {"-k": true, "-s": true, "--kill-after": true, "--signal": true},
}

// The launchers launcherValueOptions names that other tables of the package
// name too (environmentTools, identityWrappers).
const (
	sudoCommand = "sudo"
	timeCommand = "time"
)

// launcherArguments answers how many of words, the words after the launcher
// named, are the launcher's own: its options, the value of each option
// launcherValueOptions names for it, and its numeric arguments. Both launcher
// loops skip that many to reach the program, so they read one rule. The
// count never exceeds len(words), and each step consumes a word, so the loop
// is bounded by the line's length.
func launcherArguments(launcher string, words []string) int {
	values := launcherValueOptions[launcher]
	at := 0
	for at < len(words) {
		word := words[at]
		if values[word] {
			at += 2
			continue
		}
		if strings.HasPrefix(word, "-") {
			at++
			continue
		}
		if launcherNumber.MatchString(word) {
			at++
			continue
		}
		break
	}
	return min(at, len(words))
}
