// Related: launcher.go -- launchers, the one declaration of the programs that run another, and its reading
// Related: program.go -- Started, Split and ShellCommand, which read a command's words through it

package heavytool

import (
	"slices"
	"strings"
	"testing"
)

// TestLaunchersAreMatchedByTheirBaseName asks whether a launcher typed by its
// path reads as the bare name does. Its method: one row per launcher, each
// in front of a heavy program, asserting the name Started answers.
//
// VALIDATES: `/usr/bin/env go test`, `/usr/bin/sudo go test`, `/usr/bin/nohup
// go test` and `/bin/bash -c '<line>'` read through the launcher.
// PREVENTS: a heavy program hidden behind the full path of its launcher,
// which the identity guard already matched by base name.
func TestLaunchersAreMatchedByTheirBaseName(t *testing.T) {
	for _, row := range []struct {
		words string
		name  string
	}{
		{"/usr/bin/env go test ./x", "go test"},
		{"/usr/bin/sudo -u u go test ./x", "go test"},
		{"/usr/bin/nohup go test ./x", "go test"},
		{"/usr/bin/timeout 60 go vet ./x", "go vet"},
		{"/usr/bin/nice -n 5 /usr/bin/env go build ./x", "go build"},
	} {
		name, heavy := Started(strings.Fields(row.words))
		if name != row.name || !heavy {
			t.Errorf("Started(%q) = %q, %v, want %q, true", row.words, name, heavy, row.name)
		}
	}
	line, handed := ShellCommand([]string{"/bin/bash", "-c", "go test ./x"})
	if !handed || line != "go test ./x" {
		t.Errorf("ShellCommand(/bin/bash -c ...) = %q, %v, want the line, true", line, handed)
	}
	launcher, _, _, _ := ShellCommandWords([]string{"/bin/sh", "-c", "go test ./x"})
	if launcher != "sh" {
		t.Errorf("ShellCommandWords(/bin/sh -c ...) launcher = %q, want the base name sh", launcher)
	}
}

// TestTracersAndFlockRunTheProgramAfterTheirArguments asks whether strace,
// ltrace and flock are read through to the program they run, each option
// value and flock's lock file skipped, and whether flock's `-c` hands a line.
// Its method: one row per option that takes a value (strace(1) 6.8, ltrace(1)
// 0.7.3, flock(1) of util-linux), in front of a heavy program.
//
// VALIDATES: `strace -o out go test`, `ltrace -l lib go test`, `flock l.lock
// go test` read heavy; `flock l.lock -c 'go test'` hands its line.
// PREVENTS: a heavy program run under a tracer or a lock, which the identity
// guard read through and the heavy reading did not.
func TestTracersAndFlockRunTheProgramAfterTheirArguments(t *testing.T) {
	for _, words := range []string{
		"strace go test ./x",
		"strace -f go test ./x",
		"strace -o out go test ./x",
		"strace -fo out go test ./x",
		"strace -e trace=open go test ./x",
		"strace -s 64 go test ./x",
		"strace -p 1 go test ./x",
		"strace -u nobody go test ./x",
		"strace -E A=1 go test ./x",
		"strace -P /x go test ./x",
		"strace -X raw go test ./x",
		"strace -a 40 -b execve -I 2 -O 5 -S time -U name go test ./x",
		"strace --output out go test ./x",
		"strace --output=out go test ./x",
		"strace --argv0 x go test ./x",
		"strace --env A=1 go test ./x",
		"strace --user nobody go test ./x",
		"strace --trace open go test ./x",
		"ltrace go test ./x",
		"ltrace -o out go test ./x",
		"ltrace -l lib -e malloc go test ./x",
		"ltrace -a 40 -A 5 -D 7 -F conf -n 2 -s 64 -u u -w 3 -x x go test ./x",
		"ltrace --library lib --output out go test ./x",
		"flock l.lock go test ./x",
		"flock -n l.lock go test ./x",
		"flock -w 5 l.lock go test ./x",
		"flock -E 3 l.lock go test ./x",
		"flock --timeout 5 l.lock go test ./x",
		"flock --conflict-exit-code 3 -s l.lock go test ./x",
	} {
		if name, heavy := Started(strings.Fields(words)); name != "go test" || !heavy {
			t.Errorf("Started(%q) = %q, %v, want go test, true", words, name, heavy)
		}
	}
	for _, row := range []struct {
		words []string
		line  string
	}{
		{[]string{"flock", "l.lock", "-c", "go test ./x"}, "go test ./x"},
		{[]string{"flock", "-w", "5", "l.lock", "--command", "go vet ./x"}, "go vet ./x"},
		{[]string{"flock", "-c", "go test ./x", "l.lock"}, "go test ./x"},
	} {
		line, handed := ShellCommand(row.words)
		if !handed || line != row.line {
			t.Errorf("ShellCommand(%q) = %q, %v, want %q, true", row.words, line, handed, row.line)
		}
		if name, heavy := StartedUnread(row.words); name != "flock" || !heavy {
			t.Errorf("StartedUnread(%q) = %q, %v, want flock, true", row.words, name, heavy)
		}
	}
	launcher, first, end, handed := ShellCommandWords([]string{"flock", "l.lock", "-c", "go test", "x"})
	if launcher != "flock" || first != 3 || end != 4 || !handed {
		t.Errorf("ShellCommandWords(flock l.lock -c ...) = %q, %d, %d, %v, want flock, 3, 4, true", launcher, first, end, handed)
	}
}

// TestTasksetReadsItsMaskAsAnOperand asks whether taskset's mask, the first
// word after its options, is skipped whatever it is spelled as, and whether
// `-p`, which sets a running process's mask, runs no program. Its method:
// one row per spelling (taskset(1) of util-linux: a hexadecimal mask with or
// without `0x`, or a list after `-c`, which is a flag that changes how the
// mask is read).
//
// VALIDATES: `taskset ff go test` and `taskset f cp` read go and cp as the
// program; `taskset -p ff 1234` runs none.
// PREVENTS: a mask with no digit read as the program, so the heavy program
// after it is never judged.
func TestTasksetReadsItsMaskAsAnOperand(t *testing.T) {
	for _, words := range []string{
		"taskset ff go test ./x",
		"taskset 0xff go test ./x",
		"taskset 3 go test ./x",
		"taskset -c 0-3 go test ./x",
		"taskset --cpu-list 0,2 go test ./x",
		"taskset -a f go test ./x",
	} {
		if name, heavy := Started(strings.Fields(words)); name != "go test" || !heavy {
			t.Errorf("Started(%q) = %q, %v, want go test, true", words, name, heavy)
		}
	}
	if program := Program(strings.Fields("taskset f cp a b")); !slices.Equal(program, []string{"cp", "a", "b"}) {
		t.Errorf("Program(taskset f cp a b) = %q, want cp a b", program)
	}
	for _, words := range []string{"taskset -p ff 1234", "taskset -p 1234", "taskset --pid 3 go"} {
		prefix, program := Split(strings.Fields(words))
		if len(prefix) != 0 || program[0] != "taskset" {
			t.Errorf("Split(%q) = %q, %q, want taskset as the program", words, prefix, program)
		}
	}
}

// TestTimeoutReadsAnySpellingOfItsDuration asks whether timeout's duration,
// the first word after its options, is skipped however it is spelled:
// timeout(1) reads it as a floating-point number with an optional unit, so
// `inf`, `1e3` and `1.5` are durations as `10s` and `5m` are. Its method:
// one row per spelling, in front of a heavy program.
//
// VALIDATES: `timeout inf go test` reads go test as the program.
// PREVENTS: a duration with no unit letter or with an exponent read as the
// program, so the heavy program after it is never judged.
func TestTimeoutReadsAnySpellingOfItsDuration(t *testing.T) {
	for _, duration := range []string{"inf", "1e3", "1.5", "10s", "5m", "2h", "1d", "0x10", "600"} {
		words := []string{"timeout", duration, "go", "test", "./x"}
		if name, heavy := Started(words); name != "go test" || !heavy {
			t.Errorf("Started(%q) = %q, %v, want go test, true", words, name, heavy)
		}
		words = []string{"timeout", "-k", "5", "-s", "KILL", duration, "go", "vet", "./x"}
		if name, heavy := Started(words); name != "go vet" || !heavy {
			t.Errorf("Started(%q) = %q, %v, want go vet, true", words, name, heavy)
		}
	}
}

// TestLauncherOptionsAreReadAsGetoptReadsThem asks whether a launcher's short
// options are read as a cluster, the first letter that takes a value taking
// the rest of the word or the next word, whether `--` ends the options, and
// whether a shell's `+` options are options. Its method: one row per shape.
//
// VALIDATES: `sudo -iu root go test` skips root; `env -iS '<line>'` hands
// the line; `nice -- go test` reads go test.
// PREVENTS: a value after a cluster read as the program, and a line handed
// through a cluster never judged.
func TestLauncherOptionsAreReadAsGetoptReadsThem(t *testing.T) {
	for _, row := range []struct {
		words string
		name  string
	}{
		{"sudo -iu root go test ./x", "go test"},
		{"sudo -Eu root go test ./x", "go test"},
		{"time -vo t.log go vet ./x", "go vet"},
		{"env -iC dir go build ./x", "go build"},
		{"nice -- go test ./x", "go test"},
		{"timeout -- 5 go test ./x", "go test"},
		{"doas -nu u go test ./x", "go test"},
	} {
		name, heavy := Started(strings.Fields(row.words))
		if name != row.name || !heavy {
			t.Errorf("Started(%q) = %q, %v, want %q, true", row.words, name, heavy, row.name)
		}
	}
	for _, row := range []struct {
		words []string
		line  string
	}{
		{[]string{"env", "-iS", "go test ./x"}, "go test ./x"},
		{[]string{"env", "-iSgo test"}, "go test"},
		{[]string{"bash", "+x", "-c", "go test ./x"}, "go test ./x"},
		{[]string{"bash", "+o", "posix", "-c", "go test ./x"}, "go test ./x"},
	} {
		line, handed := ShellCommand(row.words)
		if !handed || line != row.line {
			t.Errorf("ShellCommand(%q) = %q, %v, want %q, true", row.words, line, handed, row.line)
		}
	}
}

// TestLauncherDeclaresWhatTheIdentityGuardReads asks what the one declaration
// answers the identity guard: which launchers run the program under another
// user or environment whatever follows, which run a program word at all, and
// how many words are a launcher's own. Its method: one row per launcher.
//
// VALIDATES: doas is declared as sudo is; xargs and the tracers run a program
// word; a shell and an interpreter do not; strace's `-E` and `-u` change the
// program's environment or user.
// PREVENTS: a second table of wrappers beside this one that disagrees with
// it (doas missing on one side, strace on the other).
func TestLauncherDeclaresWhatTheIdentityGuardReads(t *testing.T) {
	for _, row := range []struct {
		name     string
		identity bool
		program  bool
	}{
		{"sudo", true, true},
		{"doas", true, true},
		{"/usr/bin/doas", true, true},
		{"env", true, true},
		{"exec", true, true},
		{"su", true, false},
		{"nice", false, true},
		{"timeout", false, true},
		{"xargs", false, true},
		{"strace", false, true},
		{"ltrace", false, true},
		{"flock", false, true},
		{"taskset", false, true},
		{"ionice", false, true},
		{"bash", false, false},
		{"perl", false, false},
	} {
		launcher, found := LauncherNamed(row.name)
		if !found {
			t.Errorf("LauncherNamed(%q) found nothing", row.name)
			continue
		}
		if launcher.ChangesIdentity() != row.identity || launcher.RunsNamedProgram() != row.program {
			t.Errorf("%s: ChangesIdentity %v, RunsNamedProgram %v, want %v, %v",
				row.name, launcher.ChangesIdentity(), launcher.RunsNamedProgram(), row.identity, row.program)
		}
	}
	if _, found := LauncherNamed("cp"); found {
		t.Error("LauncherNamed(cp) found a launcher")
	}
	for _, row := range []struct {
		words   string
		count   int
		changes bool
	}{
		{"nice -n 5 env", 2, false},
		{"nice 10 go", 1, false},
		{"timeout 1.5s go", 1, false},
		{"flock -w 5 l.lock go", 3, false},
		{"strace -o out go", 2, false},
		{"strace -E A=1 go", 2, true},
		{"strace --env=A=1 go", 1, true},
		{"strace -fu nobody go", 2, true},
		{"ltrace -u nobody go", 2, true},
		{"xargs -I x -n 1 go", 4, false},
		{"stdbuf -oL go", 1, false},
		{"time . x", 0, false},
	} {
		words := strings.Fields(row.words)
		launcher, _ := LauncherNamed(words[0])
		count, changes := launcher.Arguments(words[1:])
		if count != row.count || changes != row.changes {
			t.Errorf("Arguments(%q) = %d, %v, want %d, %v", row.words, count, changes, row.count, row.changes)
		}
	}
}

// TestTakesNextWordReadsXargsOptions asks whether TakesNextWord, the reading
// the worktree copy guard skips xargs's options by, answers which option word
// takes the next word as its value (xargs(1) of findutils). Its method: one
// row per shape.
//
// VALIDATES: `-n`, `-0n` and `--max-args` take the next word; `-n1`,
// `--max-args=1`, `-0` and `-t` do not.
// PREVENTS: a second copy of xargs's value options in the worktree copy
// guard, which drifted from the identity guard's.
func TestTakesNextWordReadsXargsOptions(t *testing.T) {
	xargs := MustLauncher("xargs")
	for _, row := range []struct {
		option string
		takes  bool
	}{
		{"-a", true},
		{"-d", true},
		{"-E", true},
		{"-I", true},
		{"-L", true},
		{"-n", true},
		{"-P", true},
		{"-s", true},
		{"-0n", true},
		{"--arg-file", true},
		{"--delimiter", true},
		{"--max-args", true},
		{"--max-chars", true},
		{"--max-procs", true},
		{"--process-slot-var", true},
		{"-n1", false},
		{"--max-args=1", false},
		{"-0", false},
		{"-t", false},
		{"--null", false},
		{"cp", false},
	} {
		if got := xargs.TakesNextWord(row.option); got != row.takes {
			t.Errorf("xargs TakesNextWord(%q) = %v, want %v", row.option, got, row.takes)
		}
	}
}

// TestChangesDirectoryReadsTheLaunchersDirectoryOptions asks whether
// ChangesDirectory, the reading the worktree copy guard asks whether a
// program runs in another directory by, finds a launcher's directory option
// in Split's prefix, in every spelling getopt reads, behind the launcher's
// path too. Its method: one row per shape, the prefix taken from Split.
//
// VALIDATES: `env -C d`, `env -Cd`, `env --chdir=d`, `env -iC d`,
// `/usr/bin/env -C d`, `sudo -D d`, `sudo --chdir d` and `nice env -C d`
// run the program elsewhere; `env -u C`, `sudo -u D` and `nice -n 5` do not.
// PREVENTS: a second copy of the directory options beside the declaration,
// matched by the exact word, which misses `/usr/bin/env -C d`.
func TestChangesDirectoryReadsTheLaunchersDirectoryOptions(t *testing.T) {
	for _, row := range []struct {
		words     string
		elsewhere bool
	}{
		{"env -C d cp a b", true},
		{"env -Cd cp a b", true},
		{"env --chdir=d cp a b", true},
		{"env --chdir d cp a b", true},
		{"env -iC d cp a b", true},
		{"/usr/bin/env -C d cp a b", true},
		{"sudo -D d cp a b", true},
		{"sudo --chdir d cp a b", true},
		{"nice -n 5 env -C d cp a b", true},
		{"FOO=1 env -C d cp a b", true},
		{"env -u C cp a b", false},
		{"sudo -u D cp a b", false},
		{"nice -n 5 cp a b", false},
		{"cp a b", false},
	} {
		prefix, _ := Split(strings.Fields(row.words))
		if got := ChangesDirectory(prefix); got != row.elsewhere {
			t.Errorf("ChangesDirectory(%q) = %v, want %v", prefix, got, row.elsewhere)
		}
	}
}

// TestMustLauncherRefusesAnUndeclaredName asks whether MustLauncher, which a
// package-level variable is initialized with, panics on a name the
// declaration does not hold rather than answering a launcher that reads
// nothing. Its method: one undeclared name, and the panic recovered.
func TestMustLauncherRefusesAnUndeclaredName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustLauncher(cp) did not panic")
		}
	}()
	MustLauncher("cp")
}
