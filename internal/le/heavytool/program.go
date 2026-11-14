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
// the program on. Both are subslices of words. Each step consumes at least one
// word, so the loop is bounded by the line's length.
func Split(words []string) (prefix, program []string) {
	at := 0
	for at < len(words) {
		head := words[at]
		if Assignment(head) {
			at++
			continue
		}
		if _, launcher := launcherValueOptions[head]; launcher {
			at += 1 + launcherArguments(head, words[at+1:])
			continue
		}
		break
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

// ShellCommand answers the command string a shell in front of the program is
// handed with `-c` (`bash -c '<line>'`, `sh -ec '<line>'`, `bash -o pipefail
// -c '<line>'`): the word where the program stands, which the shell runs as a
// line of its own. It answers false when no shell launcher in front of the
// program carries `-c`. It reads words only: judging the string is the
// caller's, who can parse it.
func ShellCommand(words []string) (string, bool) {
	_, line, handed := shellLine(words)
	return line, handed
}

// StartedUnread is Started for a caller that holds a command's words and no
// shell parser, so it cannot read the line a shell is handed with `-c`:
// `le exec`, whose carried words arrive as argv. Such a line is answered
// heavy, under the shell's name (`bash`), whatever it runs, because it may
// run anything Started names and the words alone cannot say it does not.
// The Bash hook, which parses, reads that line itself (ShellCommand); it
// reaches `le exec`'s answer through the carrier judge, so both sides of a
// `./le exec bash -c '<line>'` agree: heavy, and a slot.
func StartedUnread(words []string) (string, bool) {
	if name, heavy := Started(words); heavy {
		return name, true
	}
	shell, _, handed := shellLine(words)
	if !handed {
		return "", false
	}
	return shell, true
}

// shellLine answers the shell in front of the program that is handed the
// program's word as a line with `-c`, that line, and whether there is one.
// The last shell in front of the program decides, since it is the one that
// runs the word.
func shellLine(words []string) (string, string, bool) {
	prefix, program := Split(words)
	if len(program) == 0 {
		return "", "", false
	}
	shell, command := "", false
	for at := 0; at < len(prefix); {
		head := prefix[at]
		if Assignment(head) {
			at++
			continue
		}
		count := launcherArguments(head, prefix[at+1:])
		if _, isShell := shells[head]; isShell {
			shell, command = head, carriesCommandOption(prefix[at+1:at+1+count])
		}
		at += 1 + count
	}
	return shell, program[0], command
}

// shells holds the launchers that run their `-c` value as a command line.
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
// program: a name, then `=`, before any `/`.
func Assignment(word string) bool {
	return strings.Contains(strings.Split(word, "/")[0], "=")
}

// launcherNumber matches a launcher's numeric argument, a count or a duration
// (`nice 10`, `timeout 60`, `timeout 1.5m`), which launcherArguments skips to
// reach the program. It is compiled once, at package load, because the hook
// reads through launchers on every Bash call. A negative value needs no sign
// here: launcherArguments skips a word that starts with `-` before it asks
// this pattern.
var launcherNumber = regexp.MustCompile(`^\d+(?:\.\d+)?[smhd]?$`)

// launcherValueOptions is the one declaration of the launchers a command's
// words are read through, each with the options whose value is the next
// word. Its keys are the launchers. An option missing from a launcher's set
// is skipped alone, so a value it takes would be read as the program; one
// joined to its value (`--user=u`, `-uroot`) is one word and needs no entry.
// A shell's `-c` and env's `-S` are left out on purpose: their value is the
// command string, which then stands where the program stands (ShellCommand).
var launcherValueOptions = map[string]map[string]bool{
	"bash":    {"-o": true, "+o": true, "-O": true, "+O": true, "--rcfile": true, "--init-file": true},
	"sh":      {"-o": true, "+o": true},
	"perl":    {},
	"ruby":    {"-C": true, "-E": true, "-I": true, "-r": true},
	"sudo":    {"-C": true, "-D": true, "-g": true, "-p": true, "-R": true, "-r": true, "-T": true, "-t": true, "-U": true, "-u": true, "--chdir": true, "--chroot": true, "--close-from": true, "--command-timeout": true, "--group": true, "--host": true, "--other-user": true, "--prompt": true, "--role": true, "--type": true, "--user": true},
	"time":    {"-f": true, "-o": true, "--format": true, "--output": true},
	"nice":    {"-n": true, "--adjustment": true},
	"env":     {"-C": true, "-u": true, "--chdir": true, "--unset": true},
	"timeout": {"-k": true, "-s": true, "--kill-after": true, "--signal": true},
}

// launcherArguments answers how many of words, the words after the launcher
// named, are the launcher's own: its options, the value of each option
// launcherValueOptions names for it, and its numeric arguments. The count
// never exceeds len(words), and each step consumes a word, so the loop is
// bounded by the line's length.
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
