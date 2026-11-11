// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashRawHeavy, the guard whose refusal this writes
// Related: lossypipe.go -- goTest, goVet and the short package form both refusals name
package hookruntime

import (
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

// rawAdvice answers the refusal the raw-job block shows. The guard's verdict
// reads a quote-blind split, which keeps HEAD's text verdict, but that split
// cuts `go test -run 'TestA|TestB' ./x` at the quoted bar and would advise a
// different command. On a line the hook's splitter models, the first heavy
// command it finds is the one the shell runs, quotes intact, with the
// directory changes before it; any other line keeps the text refusal
// (review I1, as advisedSegment does for the piped-command refusal).
func rawAdvice(command, root string, text rawRefusal) rawRefusal {
	if !modeledSyntax(command) {
		return text
	}
	line, parsed := lineCommands(command)
	if !parsed {
		return text
	}
	var moves []string
	for index, segment := range line.segments {
		if oneOf(line.commands[index][0], "cd", "pushd") {
			moves = append(moves, segment)
			continue
		}
		if refusal := rawHeavy(segment, moves, root); refusal.what != "" {
			return refusal
		}
	}
	return text
}

// rawHeavy answers the refusal for a segment that runs a heavy job raw, and
// a zero what for any other segment. moves holds the text of each directory
// change the line makes before the segment, as typed: a relative package then
// names a directory the short action, which runs at the checkout root, would
// not test, and the long form carries them as `env -C`, or names those it
// cannot carry (keptArgv). root is the
// checkout the short action runs in.
func rawHeavy(segment string, moves []string, root string) rawRefusal {
	tokens := shellWords(segment)
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
		if oneOf(head, "bash", "sh", "perl", "ruby", "sudo", "time", "nice", "env", "timeout") {
			prefix = append(prefix, head)
			tokens = tokens[1:]
			for len(tokens) != 0 && (strings.HasPrefix(tokens[0], "-") || regexp.MustCompile(`^\d+(?:\.\d+)?[smhd]?$`).MatchString(tokens[0])) {
				prefix = append(prefix, tokens[0])
				tokens = tokens[1:]
			}
			continue
		}
		break
	}
	if len(tokens) == 0 {
		return rawRefusal{}
	}
	kept, uncarried := keptArgv(moves, prefix, tokens)
	prefixed := len(moves) != 0 || len(prefix) != 0
	if goTest(tokens) {
		refusal := packageRefusal("`go test`", testLabel, prefixed, tokens, kept, root)
		refusal.uncarried = uncarried
		return refusal
	}
	if goVet(tokens) {
		refusal := packageRefusal("`go vet`", vetLabel, prefixed, tokens, kept, root)
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
		return rawRefusal{what: "`golangci-lint`", use: admittedCommand("lint", false, kept), uncarried: uncarried}
	}
	return rawRefusal{}
}

// assignmentWord reports a word the shell reads as an assignment in front of
// a program: a name, then `=`, before any `/`.
func assignmentWord(word string) bool {
	return strings.Contains(strings.Split(word, "/")[0], "=")
}

// keptArgv answers the argv the long admitted form runs for a segment, so it
// runs what the caller typed (review I1), and the directory changes it could
// not carry, as typed, or "" when it carried them all. The directory the
// line's changes reach goes in front of the program as `env -C <dir>`
// (moveDirectory): the form holds no separator, which the hook's quote-blind
// split would cut at, so following the advice is not refused again (review
// ISSUE). An assignment in front of the program goes through `env`, which
// sets it the same way; a launcher stays as typed.
func keptArgv(moves, prefix, program []string) ([]string, string) {
	argv := make([]string, 0, 4+len(prefix)+len(program))
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
	argv = append(argv, prefix...)
	return append(argv, program...), uncarried
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
// whose words it expresses exactly, and otherwise the long admitted form with
// one line naming the short action: a suggestion that dropped a flag, an
// assignment or a launcher would change what the run does
// (spec-agent-economy-2-scoped-test AC-7).
func packageRefusal(what, label string, prefixed bool, tokens, kept []string, root string) rawRefusal {
	vet := goVet(tokens)
	if !prefixed {
		if form, exact := shortPackageAction(tokens, root); exact {
			return rawRefusal{what: what, use: form, vet: vet}
		}
	}
	short := shortPackageForm + " [" + pkgRunKeyword + " <regex>]"
	if vet {
		short = shortPackageForm + " " + pkgVetKeyword
	}
	return rawRefusal{what: what, use: admittedCommand(label, false, kept), short: short, vet: vet}
}

// shortPackageAction writes the `./le test unit pkg` form of the words of a
// go test or go vet, and answers false when the action cannot express them
// exactly. A go test may carry `-race`, which the action always sets, and one
// `-run` value; a go vet carries no flag. Every other flag, a line naming no
// package (go then tests the current directory), the whole tree (which the
// action refuses), a package the action's own judgment does not run
// (actionRuns), and a value the refusal cannot write as one shell word all
// answer false. MUST be called with words goTest or goVet accepts.
func shortPackageAction(tokens []string, root string) (string, bool) {
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
		word, written := shellWord(name)
		if !written {
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
		word, written := shellWord(run)
		if !written {
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
