// Design: docs/contributing/running-commands.md -- heavy work starts in the main thread, under job admission
// Related: heavytool.go -- Name, the judgment of a program's words this reading feeds
// Related: launcher.go -- launchers, the one declaration of the launchers this reading reads through

package heavytool

import (
	"path/filepath"
	"strings"
)

// Started answers the heavy tool one command's words start, read the way the
// shell runs them: through the assignments and launchers in front of the
// program (Program), then through the Go toolchain's own flag that precedes
// its verb (goVerbWords), and judged by Name. The Bash hook asks it of a
// command's words and `le exec` of the words it carries, so `env go test`,
// `nice -n 5 go build` and `go -C dir test` read heavy on both sides.
func Started(words []string) (string, bool) {
	return Name(goVerbWords(Program(words)))
}

// Split answers the words in front of the program, the assignments and the
// launchers with their own arguments (Launcher.read), and the words from the
// program on. Both are subslices of words. A launcher is matched by its base
// name (LauncherNamed). One that runs no program with the options it is given
// (`command -v go`, `taskset -p 3 1234`) is the program, and so is one Split
// does not read past (readThrough: xargs, su). A launcher handed a line (a
// shell's `-c`, env's `-S`, flock's `-c`) ends the words in front: the line
// stands where the program stands, so no word from it on is read as an
// assignment or a launcher, whatever it holds (`bash -c 'go test -count=1
// ./x'`, `bash -c env`). Each step consumes at least one word, so the loop is
// bounded by the line's length.
func Split(words []string) (prefix, program []string) {
	at := 0
	for at < len(words) {
		head := words[at]
		if Assignment(head) {
			at++
			continue
		}
		launcher, found := LauncherNamed(head)
		if !found {
			break
		}
		if !launcher.readThrough() {
			break
		}
		read := launcher.read(words[at+1:])
		if read.nothing {
			break
		}
		at += 1 + read.count
		if read.hands {
			break
		}
	}
	return words[:at], words[at:]
}

// Program answers words from their program on: Split's program, with the
// options a launcher left in front of it removed. It returns a subslice of
// words.
func Program(words []string) []string {
	_, program := Split(words)
	for len(program) != 0 && strings.HasPrefix(program[0], "-") {
		program = program[1:]
	}
	return program
}

// ShellCommand answers the command line a launcher in front of the program
// is handed to run as a line of its own: a shell's `-c` operand (`bash -c
// '<line>'`, `sh -ec '<line>'`, `bash -o pipefail -c '<line>'`), the first
// word after the shell's options, whatever it holds; flock's `-c` value; or
// env's `-S` string (`env -S '<line>'`, `env --split-string=<line>`) with the
// words after it, which env appends to the string's own. It answers false
// when no launcher in front of the program hands it a line. It reads words
// only: judging the line is the caller's, who can parse it.
func ShellCommand(words []string) (string, bool) {
	handed := shellLine(words)
	return handed.line, handed.ok
}

// ShellCommandWords answers the launcher ShellCommand finds handing a line,
// by its base name, and the span of words, from first to before end, the
// line is read from: the shell's operand or flock's value alone, or env's
// string and every word after it. A caller that knows which of its words the
// shell expands before the launcher runs (the Bash hook) asks it, because a
// line read from an expanded word is not the line the launcher runs. It
// answers false where ShellCommand does.
func ShellCommandWords(words []string) (launcher string, first, end int, handed bool) {
	line := shellLine(words)
	return line.launcher, line.first, line.end, line.ok
}

// StartedUnread is Started for a caller that holds a command's words and no
// shell parser, so it cannot read the line a launcher is handed
// (ShellCommand): `le exec`, whose carried words arrive as argv. Such a line
// is answered heavy, under the launcher's name (`bash`, `env`), whatever it
// runs, because it may run anything Started names and the words alone cannot
// say it does not. The Bash hook, which parses, reads that line itself
// (ShellCommand); it reaches `le exec`'s answer through the carrier judge, so
// both sides of a `./le exec bash -c '<line>'` agree: heavy, and a slot.
func StartedUnread(words []string) (string, bool) {
	if name, heavy := Started(words); heavy {
		return name, true
	}
	handed := shellLine(words)
	if !handed.ok {
		return "", false
	}
	return handed.launcher, true
}

// handedLine is the line a launcher in front of a program is handed: the
// launcher, the line, and the span of the words it is read from, first to
// before end. A false ok is a command no launcher hands a line.
type handedLine struct {
	launcher string
	line     string
	first    int
	end      int
	ok       bool
}

// shellLine answers the line the last launcher in front of the program is
// handed (launcherWords.hands). Split ends the words in front at a launcher
// that hands a line, so that launcher is the last one, and it is the one that
// runs it.
func shellLine(words []string) handedLine {
	prefix, program := Split(words)
	name, launcher, from, to := lastLauncher(prefix)
	if name == "" {
		return handedLine{}
	}
	read := launcher.read(prefix[from:to])
	if !read.hands {
		return handedLine{}
	}
	if launcher.kind == runsShellLine {
		if len(program) == 0 {
			return handedLine{}
		}
		return handedLine{launcher: name, line: program[0], first: len(prefix), end: len(prefix) + 1, ok: true}
	}
	if read.lineAt < 0 {
		return handedLine{}
	}
	first := from + read.lineAt
	if !launcher.appends {
		return handedLine{launcher: name, line: read.line, first: first, end: first + 1, ok: true}
	}
	line := strings.Join(append([]string{read.line}, program...), " ")
	return handedLine{launcher: name, line: line, first: first, end: len(words), ok: true}
}

// lastLauncher answers the last launcher of prefix (Split's words in front of
// the program), by its base name and as declared, and the span of its own
// arguments, from to before to, or "" when prefix holds assignments alone.
// Each step consumes at least one word, so the loop is bounded by prefix's
// length.
func lastLauncher(prefix []string) (name string, launcher Launcher, from, to int) {
	for at := 0; at < len(prefix); {
		if Assignment(prefix[at]) {
			at++
			continue
		}
		launcher, _ = LauncherNamed(prefix[at])
		name, from = filepath.Base(prefix[at]), at+1
		to = from + launcher.read(prefix[from:]).count
		at = to
	}
	return name, launcher, from, to
}

// StdinShell reports whether words run a shell that reads its script from its
// input: a shell launcher handed no `-c` line and no script file (`bash`,
// `sh -e`), or one whose `-s` reads the script there and takes the words after
// it as arguments (`bash -s x`). A here-document or a pipe into it is a line
// the shell runs, never data.
func StdinShell(words []string) bool {
	prefix, _ := Split(words)
	name, launcher, from, to := lastLauncher(prefix)
	if name == "" {
		return false
	}
	if launcher.kind != runsShellLine {
		return false
	}
	arguments := prefix[from:to]
	if launcher.read(arguments).hands {
		return false
	}
	if carriesShortOption(arguments, 's') {
		return true
	}
	return len(Program(words)) == 0
}

// carriesShortOption reports whether a shell's options hold the short option
// letter, alone or in a cluster.
func carriesShortOption(options []string, letter rune) bool {
	for _, option := range options {
		if strings.HasPrefix(option, "--") {
			continue
		}
		if !strings.HasPrefix(option, "-") {
			continue
		}
		if strings.ContainsRune(option, letter) {
			return true
		}
	}
	return false
}

// goChdirFlag is the Go toolchain's one flag that may precede its verb.
const goChdirFlag = "C"

// goVerbWords answers program with the Go toolchain's directory flag in front
// of its verb removed, so the verb follows the program as Name reads it. The
// toolchain reads that flag in one of four spellings (`-C dir`, `--C dir`,
// `-C=dir`, `--C=dir`), and only once, as the first word after the program
// (cmd/go/main.go, handleChdirFlag): every other flag there is refused by the
// toolchain, so the word after it is the verb. Any other program is answered
// as it is.
func goVerbWords(program []string) []string {
	if len(program) < 2 {
		return program
	}
	if filepath.Base(program[0]) != goProgram {
		return program
	}
	if !strings.HasPrefix(program[1], "-") {
		return program
	}
	flag := strings.TrimPrefix(strings.TrimPrefix(program[1], "-"), "-")
	skipped := 0
	if flag == goChdirFlag {
		skipped = 2
	}
	if strings.HasPrefix(flag, goChdirFlag+"=") {
		skipped = 1
	}
	if skipped == 0 {
		return program
	}
	if 1+skipped > len(program) {
		return program[:1]
	}
	verb := make([]string, 0, len(program)-skipped)
	verb = append(verb, program[0])
	return append(verb, program[1+skipped:]...)
}

// Assignment reports a word the shell reads as an assignment in front of a
// program: a shell name (a letter or an underscore, then letters, digits and
// underscores), then `=` or `+=`. A word whose text before its first `=` is
// no name is no assignment: a line held as one word (`go test -count=1 ./x`)
// is a command, and `./bin/x=y` is a program.
func Assignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found {
		return false
	}
	return shellName(strings.TrimSuffix(name, "+"))
}

// shellName reports whether name is one the shell assigns to.
func shellName(name string) bool {
	if name == "" {
		return false
	}
	for at := range len(name) {
		c := name[at]
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			continue
		}
		if at > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}
