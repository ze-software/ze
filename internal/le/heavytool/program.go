// Design: docs/contributing/running-commands.md -- heavy work starts in the main thread, under job admission
// Related: heavytool.go -- Name, the judgment of a program's words this reading feeds

package heavytool

import (
	"path/filepath"
	"regexp"
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
// launchers with their own arguments (launcherArguments), and the words from
// the program on. Both are subslices of words. A launcher handed a line
// (handedBy: a shell's `-c`, env's `-S`) ends the words in front: the line
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
		if _, launcher := launcherValueOptions[head]; !launcher {
			break
		}
		count := launcherArguments(head, words[at+1:])
		arguments := words[at+1 : at+1+count]
		at += 1 + count
		if _, hands := handedBy(head, arguments); hands {
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
// word after the shell's options, whatever it holds; or env's `-S` string
// (`env -S '<line>'`, `env --split-string=<line>`) with the words after it,
// which env appends to the string's own. It answers false when no launcher
// in front of the program hands it a line. It reads words only: judging the
// line is the caller's, who can parse it.
func ShellCommand(words []string) (string, bool) {
	handed := shellLine(words)
	return handed.line, handed.ok
}

// ShellCommandWords answers the launcher ShellCommand finds handing a line,
// and the span of words, from first to before end, the line is read from:
// the shell's operand alone, or env's string and every word after it. A
// caller that knows which of its words the shell expands before the launcher
// runs (the Bash hook) asks it, because a line read from an expanded word is
// not the line the launcher runs. It answers false where ShellCommand does.
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
// handed (handedBy). Split ends the words in front at a launcher that hands a
// line, so that launcher is the last one, and it is the one that runs it.
func shellLine(words []string) handedLine {
	prefix, program := Split(words)
	launcher, from, to := "", 0, 0
	for at := 0; at < len(prefix); {
		if Assignment(prefix[at]) {
			at++
			continue
		}
		count := launcherArguments(prefix[at], prefix[at+1:])
		launcher, from, to = prefix[at], at+1, at+1+count
		at = to
	}
	if launcher == "" {
		return handedLine{}
	}
	value, hands := handedBy(launcher, prefix[from:to])
	if !hands {
		return handedLine{}
	}
	if _, shell := shells[launcher]; shell {
		if len(program) == 0 {
			return handedLine{}
		}
		return handedLine{launcher: launcher, line: program[0], first: len(prefix), end: len(prefix) + 1, ok: true}
	}
	if value < 0 {
		return handedLine{}
	}
	at := from + value
	text, _ := splitString(prefix[at], value > 0 && splitStringOptions[prefix[at-1]])
	line := strings.Join(append([]string{text}, program...), " ")
	return handedLine{launcher: launcher, line: line, first: at, end: len(words), ok: true}
}

// handedBy reports whether launcher, with arguments its own, is handed a line
// to run, and for env the index in arguments of the word holding the string,
// -1 when `-S` ends the words with no value. A shell is handed one by `-c`,
// alone or in a cluster of short options (carriesCommandOption); its line is
// the word after the arguments, so the index is -1. env is handed one by
// `-S` or `--split-string`, with the string as the next word, or joined to
// the option (`-S'<line>'`, `--split-string=<line>`).
func handedBy(launcher string, arguments []string) (int, bool) {
	if _, shell := shells[launcher]; shell {
		return -1, carriesCommandOption(arguments)
	}
	if launcher != envLauncher {
		return -1, false
	}
	for at, argument := range arguments {
		if splitStringOptions[argument] {
			if at+1 < len(arguments) {
				return at + 1, true
			}
			return -1, true
		}
		if _, joined := splitString(argument, false); joined {
			return at, true
		}
	}
	return -1, false
}

// splitString answers the string env's `-S` carries in word: the word itself
// when separate reports that it follows the option, and else the text joined
// to the option, with whether word carries one.
func splitString(word string, separate bool) (string, bool) {
	if separate {
		return word, true
	}
	if value, joined := strings.CutPrefix(word, "--split-string="); joined {
		return value, true
	}
	if strings.HasPrefix(word, "--") {
		return "", false
	}
	if value, joined := strings.CutPrefix(word, "-S"); joined && value != "" {
		return value, true
	}
	return "", false
}

// envLauncher is env, which runs its `-S` string as a command line.
const envLauncher = "env"

// splitStringOptions are env's options whose value, the next word, is a
// command line env splits into words and runs.
var splitStringOptions = map[string]bool{"-S": true, "--split-string": true}

// shells holds the launchers that run their `-c` operand as a command line.
var shells = map[string]struct{}{"bash": {}, "sh": {}}

// carriesCommandOption reports whether a shell's options hold `-c`, alone or
// in a cluster of short options (`-ec`).
func carriesCommandOption(options []string) bool {
	for _, option := range options {
		if strings.HasPrefix(option, "--") {
			continue
		}
		if strings.HasPrefix(option, "-") && strings.ContainsRune(option, 'c') {
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

// launcherNumber matches a launcher's numeric argument, a count or a duration
// (`nice 10`, `timeout 60`, `timeout 1.5m`), which launcherArguments skips to
// reach the program. A shell takes none: its first operand is the line or the
// script, whatever it reads. It is compiled once, at package load, because
// the hook reads through launchers on every Bash call. A negative value needs
// no sign here: launcherArguments skips a word that starts with `-` before it
// asks this pattern.
var launcherNumber = regexp.MustCompile(`^\d+(?:\.\d+)?[smhd]?$`)

// launcherValueOptions is the one declaration of the launchers a command's
// words are read through, each with the options whose value is the next
// word. Its keys are the launchers. An option missing from a launcher's set
// is skipped alone, so a value it takes would be read as the program; one
// joined to its value (`--user=u`, `-uroot`) is one word and needs no entry.
// A shell's `-c` is left out on purpose: it takes no value, and the line is
// the shell's first operand, which then stands where the program stands.
// env's `-S` and `--split-string` take the line as their value, so it stays
// among env's own arguments. Both are read as a handed line (ShellCommand),
// never as a program.
var launcherValueOptions = map[string]map[string]bool{
	"bash":    {"-o": true, "+o": true, "-O": true, "+O": true, "--rcfile": true, "--init-file": true},
	"sh":      {"-o": true, "+o": true},
	"perl":    {},
	"ruby":    {"-C": true, "-E": true, "-I": true, "-r": true},
	"sudo":    {"-C": true, "-D": true, "-g": true, "-p": true, "-R": true, "-r": true, "-T": true, "-t": true, "-U": true, "-u": true, "--chdir": true, "--chroot": true, "--close-from": true, "--command-timeout": true, "--group": true, "--host": true, "--other-user": true, "--prompt": true, "--role": true, "--type": true, "--user": true},
	"time":    {"-f": true, "-o": true, "--format": true, "--output": true},
	"nice":    {"-n": true, "--adjustment": true},
	"env":     {"-C": true, "-S": true, "-u": true, "--chdir": true, "--split-string": true, "--unset": true},
	"timeout": {"-k": true, "-s": true, "--kill-after": true, "--signal": true},
}

// launcherArguments answers how many of words, the words after the launcher
// named, are the launcher's own: its options, the value of each option
// launcherValueOptions names for it, and its numeric arguments. The count
// never exceeds len(words), and each step consumes a word, so the loop is
// bounded by the line's length.
func launcherArguments(launcher string, words []string) int {
	values := launcherValueOptions[launcher]
	_, shell := shells[launcher]
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
		if !shell && launcherNumber.MatchString(word) {
			at++
			continue
		}
		break
	}
	return min(at, len(words))
}
