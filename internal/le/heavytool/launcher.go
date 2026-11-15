// Design: docs/contributing/running-commands.md -- heavy work starts in the main thread, under job admission
// Related: program.go -- Split, Program and ShellCommand, the reading of a command's words through these launchers
// Related: ../hookruntime/afterline.go -- commandWordAt, the identity guard's walk past the same launchers

package heavytool

import (
	"path/filepath"
	"slices"
	"strings"
)

// launchers is the one declaration of the programs that run another program,
// a line or a script named by the words after their own arguments. Its keys
// are the launchers' base names (LauncherNamed). Every reading of a command's
// words through a launcher derives from it: the heavy reading (Split, which
// the Bash hook's heavy-work checks and `le exec` share), the identity guard
// (hookruntime's commandWordAt), and the worktree copy guard. Each option set
// follows the launcher's manual: bash(1), dash(1), zsh(1), ksh(1), perl(1),
// ruby(1), sudo(8), doas(1), su(1), env(1), time(1), nice(1), timeout(1),
// stdbuf(1), nohup(1), setsid(1), ionice(1), taskset(1), flock(1), strace(1)
// 6.8, ltrace(1) 0.7.3 and xargs(1).
var launchers = map[string]Launcher{
	// The shells run their `-c` operand as a line, and otherwise the script
	// their first operand names. `-c` takes no value: the line is the first
	// operand, which then stands where the program stands.
	"bash": {kind: runsShellLine, values: []string{"-o", "+o", "-O", "+O", "--rcfile", "--init-file"}},
	"sh":   {kind: runsShellLine, values: shellSetOptions},
	"zsh":  {kind: runsShellLine, values: shellSetOptions},
	"dash": {kind: runsShellLine, values: shellSetOptions},
	"ksh":  {kind: runsShellLine, values: shellSetOptions},
	// The interpreters run the script their first operand names.
	"perl": {kind: runsScript},
	"ruby": {kind: runsScript, values: []string{"-E", "-I", "-r"}, directories: []string{"-C"}},
	// The launchers that run the program as another user, or under an
	// environment their own words set: env sets and unsets variables, `exec
	// -c` clears them, and a login shell starts from a fresh environment.
	"sudo": {kind: runsProgram, identity: true, directories: []string{"-D", "--chdir"}, values: []string{
		"-a", "-C", "-c", "-g", "-p", "-R", "-r", "-T", "-t", "-U", "-u", "--auth-type",
		"--chroot", "--close-from", "--command-timeout", "--group", "--host", "--login-class",
		"--other-user", "--prompt", "--role", "--type", userOption,
	}},
	"doas": {kind: runsProgram, identity: true, values: []string{"-a", "-C", "-u"}},
	// su runs a login shell whose operands are the user and that shell's
	// arguments: its words name no program, so nothing reads past them, and
	// the identity guard refuses it whatever they hold.
	"su": {kind: runsLoginShell, identity: true},
	"env": {
		kind: runsProgram, identity: true, appends: true,
		values:      []string{"-u", "--unset"},
		directories: []string{"-C", "--chdir"},
		lines:       []string{"-S", "--split-string"},
	},
	"exec": {kind: runsProgram, identity: true, values: []string{"-a"}},
	// The shell builtins that run the program after them: `command` (with
	// `-p`; `-v` and `-V` only describe it) and `builtin`.
	"command": {kind: runsProgram, nothing: []string{"-v", "-V"}},
	"builtin": {kind: runsProgram},
	// `time` is the external program here, which `command time` and
	// `/usr/bin/time` reach: the reserved word is structure in a parsed line.
	"time": {kind: runsProgram, values: []string{"-f", "-o", "--format", outputOption}},
	// nice takes the obsolete count before the program too (`nice 10`).
	"nice": {kind: runsProgram, counts: true, values: []string{"-n", "--adjustment"}},
	// timeout's duration operand is a floating-point number with an optional
	// unit, spelled any way strtod reads one (`5`, `1.5s`, `inf`, `1e3`).
	"timeout": {kind: runsProgram, operands: 1, values: []string{"-k", "-s", "--kill-after", "--signal"}},
	"nohup":   {kind: runsProgram},
	"setsid":  {kind: runsProgram},
	"stdbuf":  {kind: runsProgram, values: []string{"-i", "-o", "-e", "--input", outputOption, "--error"}},
	// ionice's `-p`, `-P` and `-u` name running processes, and then it runs
	// no program.
	"ionice": {
		kind:    runsProgram,
		values:  []string{"-c", "-n", "-p", "-P", "-u", "--class", "--classdata", pidOption, "--pgid", "--uid"},
		nothing: []string{"-p", "-P", "-u", pidOption, "--pgid", "--uid"},
	},
	// taskset's mask is its operand, whatever it is spelled as (`ff`,
	// `0x3`): `-c` is a flag that reads it as a list (`0-3`), and `-p` sets
	// the mask of a running process, which runs no program.
	"taskset": {kind: runsProgram, operands: 1, nothing: []string{"-p", pidOption}},
	// flock takes its lock file before the program, and its `-c` hands a
	// line to the shell: flock(1) reads it after the lock file, and it is
	// read in front of it too, which hands the line all the same.
	"flock": {
		kind: runsProgram, operands: 1,
		values: []string{"-E", "-w", "--conflict-exit-code", "--wait", "--timeout"},
		lines:  []string{"-c", "--command"},
	},
	// strace's `-E` and `-u`, and ltrace's `-u`, run the program under
	// another environment or user, as env and sudo do.
	"strace": {
		kind: runsProgram,
		values: []string{
			"-a", "-b", "-e", "-E", "-I", "-o", "-O", "-p", "-P", "-s", "-S", "-u", "-U", "-X",
			"--abbrev", "--argv0", "--attach", "--columns", "--const-print-style", "--decode-pids",
			"--detach-on", "--env", "--fault", "--inject", "--interruptible", "--kvm", outputOption,
			"--raw", "--read", "--signal", "--stack-trace-frame-limit", "--status", "--string-limit",
			"--summary-columns", "--summary-sort-by", "--summary-syscall-overhead", "--syscall-limit",
			"--trace", "--trace-fds", "--trace-path", userOption, "--verbose", "--write",
		},
		changes: []string{"-E", "-u", "--env", userOption},
	},
	"ltrace": {
		kind: runsProgram,
		values: []string{
			"-a", "-A", "-D", "-e", "-F", "-l", "-n", "-o", "-p", "-s", "-u", "-w", "-x",
			"--align", "--config", "--debug", "--indent", "--library", outputOption, "--where",
		},
		changes: []string{"-u"},
	},
	"xargs": {
		kind: runsProgramWithInput,
		values: []string{
			"-a", "-d", "-E", "-I", "-L", "-n", "-P", "-s",
			"--arg-file", "--delimiter", "--max-args", "--max-chars", "--max-procs", "--process-slot-var",
		},
	},
}

// shellSetOptions are the options of a POSIX shell whose value, the next
// word, is the name of a `set -o` option.
var shellSetOptions = []string{"-o", "+o"}

// The long options several launchers' manuals spell the same way: `--user`
// (sudo, strace), `--output` (time, stdbuf, strace, ltrace) and `--pid`
// (ionice, taskset).
const (
	userOption   = "--user"
	outputOption = "--output"
	pidOption    = "--pid"
)

// Launcher is one program of the declaration (launchers): what it runs, and
// the facts about its own arguments every reading of a command's words
// through it skips. A value is held only by the declaration, so a Launcher
// a caller holds is always one LauncherNamed or MustLauncher answered.
type Launcher struct {
	kind launcherKind
	// values are the options whose value is a word of its own, the rest of
	// a short cluster or the next word (`-o file`, `-ofile`, `--output file`,
	// `--output=file`). A long option is listed only when its manual gives
	// it a required value, which getopt takes from the next word too; one
	// whose value is optional (strace `--quiet[=set]`) takes it only after
	// `=`, and is not listed.
	values []string
	// lines are the options whose value is a command line the launcher runs
	// (env's `-S`, flock's `-c`). Each takes a value as values do.
	lines []string
	// appends reports that the words after the launcher's arguments join the
	// line (env appends them to its `-S` string).
	appends bool
	// changes are the options that run the program under another
	// environment or user (strace `-E`, `-u`).
	changes []string
	// nothing are the options with which the launcher runs no program
	// (`command -v`, `taskset -p`).
	nothing []string
	// directories are the options whose value is the directory the launcher
	// runs its program in (env's `-C`, sudo's `-D`). Each takes a value as
	// values do.
	directories []string
	// operands counts the operands between the options and the program
	// (timeout's duration, flock's lock file, taskset's mask).
	operands int
	// counts reports that a count may stand before the program (`nice 10`).
	counts bool
	// identity reports that the launcher may run the program as another user
	// or under an environment its own words set, whatever they hold (sudo,
	// doas, su, env, exec).
	identity bool
}

// launcherKind is what a launcher runs with the words after its own
// arguments.
type launcherKind uint8

const (
	// kindUnspecified is no launcher: only the zero Launcher holds it.
	kindUnspecified launcherKind = iota
	// runsProgram runs the program the first of those words names (env,
	// nice, sudo, timeout).
	runsProgram
	// runsProgramWithInput runs that program with words its input supplies
	// appended (xargs). Split does not read past it: the words after it do
	// not say what the program is handed, and the worktree copy guard
	// judges it as a program of its own.
	runsProgramWithInput
	// runsShellLine runs the line its `-c` hands it, or the script its first
	// operand names (bash, sh).
	runsShellLine
	// runsScript runs the script its first operand names (perl, ruby).
	runsScript
	// runsLoginShell runs a shell as another user (su).
	runsLoginShell
)

// LauncherNamed answers the launcher a command word names, matched by its
// base name, so `/usr/bin/env` is env. A word the declaration does not hold
// answers false.
func LauncherNamed(word string) (Launcher, bool) {
	launcher, found := launchers[filepath.Base(word)]
	return launcher, found
}

// MustLauncher answers the launcher the declaration holds under name, for a
// package-level variable that reads one launcher's options. It panics when
// the declaration holds none: only a Ze defect, a name typed wrong in the
// caller, produces that, and package initialization surfaces it in every
// test of the caller.
func MustLauncher(name string) Launcher {
	launcher, found := launchers[name]
	if !found {
		panic("BUG: heavytool declares no launcher " + name)
	}
	return launcher
}

// ChangesIdentity reports whether the launcher may run the program as
// another user or under an environment its own words set, whatever they
// hold (sudo, doas, su, env, exec): the identity guard refuses it.
func (launcher Launcher) ChangesIdentity() bool {
	return launcher.identity
}

// RunsNamedProgram reports whether the first word after the launcher's own
// arguments is a program it runs (env, nice, strace, xargs), so a reading of
// what runs reads on from there. A shell, an interpreter and a login shell
// run a line, a script or a user's shell instead.
func (launcher Launcher) RunsNamedProgram() bool {
	switch launcher.kind {
	case runsProgram, runsProgramWithInput:
		return true
	case kindUnspecified, runsShellLine, runsScript, runsLoginShell:
		return false
	}
	return false
}

// readThrough reports whether Split reads past the launcher to what it runs:
// a program, a line or a script the words in front of the program name.
func (launcher Launcher) readThrough() bool {
	switch launcher.kind {
	case runsProgram, runsShellLine, runsScript:
		return true
	case kindUnspecified, runsProgramWithInput, runsLoginShell:
		return false
	}
	return false
}

// Arguments answers how many of words, the words after the launcher, are its
// own (its options, each option's value, its count and its operands), and
// whether one of its options runs the program under another environment or
// user. The count never exceeds len(words).
func (launcher Launcher) Arguments(words []string) (count int, changes bool) {
	read := launcher.read(words)
	return read.count, read.changes
}

// TakesNextWord reports whether option, a word after the launcher, is an
// option whose value is the next word, read as getopt reads it: for xargs,
// `-n`, `-0n` and `--max-args` are, and `-n1` and `--max-args=1` hold their
// value. A word that is no option answers false.
func (launcher Launcher) TakesNextWord(option string) bool {
	if !launcher.isOption(option) {
		return false
	}
	return launcher.option(option).next
}

// ChangesDirectory reports whether the launchers in prefix, Split's words in
// front of the program, run the program in another directory: an option of
// one of them whose value is a directory (env's `-C` and `--chdir`, sudo's
// `-D` and `--chdir`, ruby's `-C`), alone or joined to its value. Each
// launcher is matched by its base name, as Split matched it.
func ChangesDirectory(prefix []string) bool {
	for at := 0; at < len(prefix); {
		if Assignment(prefix[at]) {
			at++
			continue
		}
		launcher, _ := LauncherNamed(prefix[at])
		read := launcher.read(prefix[at+1:])
		if read.directory {
			return true
		}
		at += 1 + read.count
	}
	return false
}

// Option reads one option word after the launcher, as getopt reads it (a
// long option whole, a short cluster letter by letter): whether the next word
// is its value, and whether it runs the program under another environment or
// user. It is Arguments one word at a time, for a caller that walks the words
// itself (the identity guard's text route).
func (launcher Launcher) Option(word string) (next, changes bool) {
	option := launcher.option(word)
	return option.next, option.changes
}

// Count reports whether word is a count the launcher skips in front of its
// program (`nice 10`): only a launcher that takes one skips it, and only
// digits make one, so a lone `.` is the program.
func (launcher Launcher) Count(word string) bool {
	if !launcher.counts {
		return false
	}
	return countWord(word)
}

// Operands answers how many operands stand between the launcher's options
// and its program (timeout's duration, flock's lock file, taskset's mask).
func (launcher Launcher) Operands() int {
	return launcher.operands
}

// launcherWords is what a launcher's own words say (Launcher.read).
type launcherWords struct {
	// count is how many words are the launcher's own.
	count int
	// changes reports an option that runs the program under another
	// environment or user.
	changes bool
	// nothing reports an option with which the launcher runs no program.
	nothing bool
	// directory reports an option that runs the program in another
	// directory.
	directory bool
	// hands reports that the launcher is handed a line: a shell's `-c`, or
	// an option of lines.
	hands bool
	// lineAt indexes the word holding the line an option of lines carries,
	// or is -1: a shell's line is the word after the arguments, and an
	// option of lines that ends the words holds none (`env -S`).
	lineAt int
	// line is the line an option of lines carries.
	line string
}

// read reads the launcher's own words from the front of words, the words
// after the launcher, as getopt reads them: each option, the value of each
// option that takes one, a count when the launcher takes one, then its
// operands; `--` ends the options, and the operands follow it. Each step
// consumes at least one word, so the loop is bounded by the line's length.
func (launcher Launcher) read(words []string) launcherWords {
	read := launcherWords{lineAt: -1}
	operands := launcher.operands
	at := 0
	for at < len(words) {
		word := words[at]
		if word == "--" {
			at += 1 + operands
			break
		}
		if launcher.isOption(word) {
			option := launcher.option(word)
			read.changes = read.changes || option.changes
			read.nothing = read.nothing || option.nothing
			read.directory = read.directory || option.directory
			if option.line {
				read.hands = true
				read.lineAt, read.line = lineWord(words, at, option)
			}
			at++
			if option.next {
				at++
			}
			continue
		}
		if launcher.counts && countWord(word) {
			at++
			continue
		}
		if operands > 0 {
			operands--
			at++
			continue
		}
		break
	}
	read.count = min(at, len(words))
	if launcher.kind == runsShellLine {
		read.hands = carriesShortOption(words[:read.count], 'c')
	}
	return read
}

// lineWord answers the index of the word holding the line the option at
// index at carries, and the line: the text joined to the option, or the next
// word. An option that ends the words carries none, and answers -1.
func lineWord(words []string, at int, option optionWord) (int, string) {
	if !option.next {
		return at, option.value
	}
	if at+1 < len(words) {
		return at + 1, words[at+1]
	}
	return -1, ""
}

// isOption reports whether word is an option of the launcher: a word that
// starts with `-`, a lone `-` included (env's `-` is `-i`), or for a shell
// one that starts with `+`, which unsets the option it names.
func (launcher Launcher) isOption(word string) bool {
	if strings.HasPrefix(word, "-") {
		return true
	}
	if launcher.kind != runsShellLine {
		return false
	}
	return len(word) > 1 && strings.HasPrefix(word, "+")
}

// optionWord is what one option word holds (Launcher.option).
type optionWord struct {
	// next reports that the next word is the value of its last option.
	next bool
	// changes, nothing and directory report an option of changes, of
	// nothing and of directories.
	changes   bool
	nothing   bool
	directory bool
	// line reports that its last option's value is a line (lines), and value
	// is that value when it is joined to the option.
	line  bool
	value string
}

// option reads one option word of the launcher as getopt reads it. A long
// option is named whole, before any `=`, and takes the next word when it
// takes a value and holds no `=`. A word the launcher lists whole (`-o`,
// `+o`) takes the next word. Any other word is a cluster of short options
// (`-fo`, `-0n`, `+eo`) read letter by letter: the first letter that takes a
// value takes the rest of the cluster, or the next word when it ends it.
func (launcher Launcher) option(word string) optionWord {
	if strings.HasPrefix(word, "--") {
		name, value, joined := strings.Cut(word, "=")
		option, valued := launcher.named(name)
		if !valued {
			return option
		}
		option.next = !joined
		option.value = value
		return option
	}
	if option, valued := launcher.named(word); valued {
		option.next = true
		return option
	}
	read := optionWord{}
	for at := 1; at < len(word); at++ {
		option, valued := launcher.named(word[:1] + word[at:at+1])
		read.changes = read.changes || option.changes
		read.nothing = read.nothing || option.nothing
		read.directory = read.directory || option.directory
		if valued {
			read.line = option.line
			read.value = word[at+1:]
			read.next = read.value == ""
			return read
		}
	}
	return read
}

// named answers what the option name, one long option or one short letter
// with its sign, holds, and whether it takes a value.
func (launcher Launcher) named(name string) (optionWord, bool) {
	option := optionWord{
		changes:   slices.Contains(launcher.changes, name),
		nothing:   slices.Contains(launcher.nothing, name),
		directory: slices.Contains(launcher.directories, name),
		line:      slices.Contains(launcher.lines, name),
	}
	return option, option.line || option.directory || slices.Contains(launcher.values, name)
}

// countWord reports a count a launcher that takes one skips (`nice 10`):
// digits alone, so a lone `.`, the source builtin, is the program.
func countWord(word string) bool {
	if word == "" {
		return false
	}
	return strings.Trim(word, "0123456789") == ""
}
