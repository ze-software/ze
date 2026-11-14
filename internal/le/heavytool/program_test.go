// Related: program.go -- Started, Split, Program and ShellCommand, the reading this file drives

package heavytool

import (
	"slices"
	"strings"
	"testing"
)

// goRun is the Go toolchain's run verb line, assembled so this file never
// holds the literal the repository contract reads as a test shelling out to
// a compiled tool (TestNoDevelopmentToolTestShellsOutToGoRun).
const goRun = "go run" + " ./cmd/ze"

// TestStartedReadsThroughWhatStandsInFrontOfTheProgram asks whether Started,
// the reading the Bash hook and `le exec` share, names the heavy tool behind
// assignments, launchers and the Go toolchain's directory flag, and names
// nothing for a light program behind the same words. Its method: one row per
// shape, asserting the name and the verdict.
//
// VALIDATES: the hook and `le exec` read `env go test`, `nice -n 5 go
// build`, `timeout 600 go vet` and `go -C dir test` as the same heavy tool.
// PREVENTS: `./le exec env go test ./x/` reading light through exec while
// the hook hands the raw `env go test ./x/` back.
func TestStartedReadsThroughWhatStandsInFrontOfTheProgram(t *testing.T) {
	for _, row := range []struct {
		words string
		name  string
		heavy bool
	}{
		{"go test ./x/", "go test", true},
		{"env go test ./x/", "go test", true},
		{"nice -n 5 go build ./...", "go build", true},
		{"timeout 600 go vet ./...", "go vet", true},
		{"timeout -s KILL 60 go test ./x", "go test", true},
		{"sudo -u u go test ./x", "go test", true},
		{"time -o t.log go vet ./x", "go vet", true},
		{"GOFLAGS=-count=1 go test ./x", "go test", true},
		{"env GOFLAGS=-mod=mod go build ./x", "go build", true},
		{"ZE_ADMIT_RAW=bisect timeout 60 go vet ./x", "go vet", true},
		{goRun, "go run", true},
		{"gofmt -l .", "gofmt", true},
		{"env golangci-lint run", "golangci-lint", true},
		// The Go toolchain's directory flag, in each spelling it reads.
		{"go -C d test ./...", "go test", true},
		{"go --C d test ./...", "go test", true},
		{"go -C=d test ./...", "go test", true},
		{"go --C=d build ./...", "go build", true},
		{"/usr/local/go/bin/go -C d vet ./...", "go vet", true},
		{"env go -C d test ./...", "go test", true},
		// The shell builtins that run the program after them.
		{"command go test ./x", "go test", true},
		{"command -p go vet ./x", "go vet", true},
		{"builtin gofmt -l .", "gofmt", true},
		{"exec -a name go test ./x", "go test", true},
		{"exec -cl go build ./x", "go build", true},
		{"command -v go test", "", false},
		{"command -pV gofmt", "", false},
		// Light programs behind the same words stay light.
		{"go version", "", false},
		{"env FOO=1 go version", "", false},
		{"nice -n 5 go env GOROOT", "", false},
		{"go -C d version", "", false},
		{"go -C d", "", false},
		{"go -C", "", false},
		// A flag the toolchain does not read before its verb is no directory
		// flag: the word after the program stays the verb.
		{"go -x test ./...", "", false},
		{"gopls", "", false},
		{"golangci-lint config verify", "", false},
		{"probe word", "", false},
		{"env", "", false},
		{"", "", false},
	} {
		name, heavy := Started(strings.Fields(row.words))
		if name != row.name || heavy != row.heavy {
			t.Errorf("Started(%q) = %q, %v, want %q, %v", row.words, name, heavy, row.name, row.heavy)
		}
	}
}

// TestStartedUnreadTakesAShellLineAsHeavy asks whether StartedUnread, the
// reading of a caller with no shell parser (`le exec`), answers a shell
// handed a line with `-c` as heavy under the shell's name, whatever the
// line, and otherwise answers as Started. Its method: one row per shape, the
// line held as one word, as argv holds it.
//
// VALIDATES: `le exec bash -c 'go test ./x'` and its kin read heavy on the
// slot side and, through the carrier judge, on the hook side.
// PREVENTS: a heavy tool reaching the machine through a carried shell line.
func TestStartedUnreadTakesAShellLineAsHeavy(t *testing.T) {
	for _, row := range []struct {
		words []string
		name  string
		heavy bool
	}{
		{[]string{"bash", "-c", "go test ./x"}, "bash", true},
		{[]string{"sh", "-ec", "go build ./..."}, "sh", true},
		{[]string{"bash", "-o", "pipefail", "-c", "./le test unit all"}, "bash", true},
		{[]string{"sh", "-c", "echo hi"}, "sh", true},
		{[]string{"env", "go", "test", "./x/"}, "go test", true},
		{[]string{"bash", "script.sh"}, "", false},
		{[]string{"go", "version"}, "", false},
		{[]string{"bash", "-c"}, "", false},
		// A line whose words hold an `=` is still the line, never an
		// assignment in front of a program.
		{[]string{"bash", "-c", "go test -count=1 ./x"}, "bash", true},
		{[]string{"bash", "-c", "FOO=1 go test ./x"}, "bash", true},
		{[]string{"sh", "-c", "600"}, "sh", true},
		// env's `-S` string is a handed line like a shell's `-c` operand.
		{[]string{"env", "-S", "go test ./x"}, "env", true},
		{[]string{"env", "--split-string=go version"}, "env", true},
		{[]string{"env", "-S"}, "", false},
	} {
		name, heavy := StartedUnread(row.words)
		if name != row.name || heavy != row.heavy {
			t.Errorf("StartedUnread(%q) = %q, %v, want %q, %v", row.words, name, heavy, row.name, row.heavy)
		}
	}
}

// TestSplitAnswersTheWordsInFrontOfTheProgram asks whether Split cuts a
// command's words where the program starts, with each launcher's own
// options, option values and numeric arguments in front of it. Its method:
// one row per shape, asserting both halves.
func TestSplitAnswersTheWordsInFrontOfTheProgram(t *testing.T) {
	for _, row := range []struct {
		words   string
		prefix  string
		program string
	}{
		{"go test ./x", "", "go test ./x"},
		{"FOO=1 BAR=2 go test", "FOO=1 BAR=2", "go test"},
		{"timeout -k 5s 60 go test", "timeout -k 5s 60", "go test"},
		{"sudo --user u env -u HOME go vet", "sudo --user u env -u HOME", "go vet"},
		{"bash -o pipefail -c x", "bash -o pipefail -c", "x"},
		{"./bin/x env", "", "./bin/x env"},
		{"nice", "nice", ""},
	} {
		prefix, program := Split(strings.Fields(row.words))
		if !slices.Equal(prefix, strings.Fields(row.prefix)) || !slices.Equal(program, strings.Fields(row.program)) {
			t.Errorf("Split(%q) = %q, %q, want %q, %q", row.words, prefix, program, row.prefix, row.program)
		}
	}
}

// TestShellCommandFindsTheLineHandedToAShell asks whether ShellCommand finds
// the line a shell in front of the program is handed with `-c`, and nothing
// for a shell without it or for another launcher. Its method: one row per
// shape, the line held as one word, as a parsed command holds it.
func TestShellCommandFindsTheLineHandedToAShell(t *testing.T) {
	for _, row := range []struct {
		words  []string
		line   string
		handed bool
	}{
		{[]string{"bash", "-c", "go test ./x"}, "go test ./x", true},
		{[]string{"sh", "-ec", "cd x; go test"}, "cd x; go test", true},
		{[]string{"bash", "-o", "pipefail", "-c", "a | b"}, "a | b", true},
		{[]string{"timeout", "60", "bash", "-c", "go vet"}, "go vet", true},
		{[]string{"FOO=1", "sh", "-c", "x"}, "x", true},
		{[]string{"bash", "script.sh"}, "script.sh", false},
		{[]string{"bash", "--norc", "x"}, "x", false},
		{[]string{"env", "-c", "x"}, "x", false},
		{[]string{"grep", "-c", "x"}, "grep", false},
		{[]string{"bash", "-c"}, "", false},
		// The first operand after the `-c` cluster is the line, whatever it
		// holds: an `=`, a launcher's name, a number.
		{[]string{"bash", "-c", "go test -count=1 ./x"}, "go test -count=1 ./x", true},
		{[]string{"bash", "-c", "FOO=1 go test"}, "FOO=1 go test", true},
		{[]string{"bash", "-c", "env"}, "env", true},
		{[]string{"sh", "-c", "600"}, "600", true},
		// env's `-S` string, separate or joined, with the words env appends.
		{[]string{"env", "-S", "go test", "./x"}, "go test ./x", true},
		{[]string{"env", "-Sgo test"}, "go test", true},
		{[]string{"env", "--split-string", "go vet"}, "go vet", true},
		{[]string{"env", "--split-string=go vet", "./x"}, "go vet ./x", true},
		{[]string{"env", "-u", "HOME", "-S", "go vet"}, "go vet", true},
		{[]string{"env", "-S"}, "", false},
	} {
		line, handed := ShellCommand(row.words)
		if handed != row.handed {
			t.Errorf("ShellCommand(%q) handed = %v, want %v", row.words, handed, row.handed)
			continue
		}
		if handed && line != row.line {
			t.Errorf("ShellCommand(%q) = %q, want %q", row.words, line, row.line)
		}
	}
}

// TestShellCommandWordsSpansTheWordsTheLineIsReadFrom asks whether
// ShellCommandWords names the launcher handing a line and the span of words
// the line comes from: a shell's operand alone, env's string and every word
// after it. Its method: one row per shape, asserting the launcher and the
// span.
//
// VALIDATES: the Bash hook can ask whether the words a handed line is read
// from are words the shell expands first (`bash -c "$X"`).
// PREVENTS: a line read from an expanded word judged as if it were the line
// the launcher runs.
func TestShellCommandWordsSpansTheWordsTheLineIsReadFrom(t *testing.T) {
	for _, row := range []struct {
		words    []string
		launcher string
		first    int
		end      int
		handed   bool
	}{
		{[]string{"bash", "-c", "x", "arg0"}, "bash", 2, 3, true},
		{[]string{"timeout", "60", "sh", "-ec", "x"}, "sh", 4, 5, true},
		{[]string{"env", "-S", "go test", "./x"}, "env", 2, 4, true},
		{[]string{"env", "-Sgo", "./x"}, "env", 1, 3, true},
		{[]string{"bash", "script.sh"}, "", 0, 0, false},
	} {
		launcher, first, end, handed := ShellCommandWords(row.words)
		if launcher != row.launcher || first != row.first || end != row.end || handed != row.handed {
			t.Errorf("ShellCommandWords(%q) = %q, %d, %d, %v, want %q, %d, %d, %v",
				row.words, launcher, first, end, handed, row.launcher, row.first, row.end, row.handed)
		}
	}
}

// TestAssignmentReadsOnlyAShellName asks whether Assignment takes a word as
// an assignment only when a shell name stands before its `=`. Its method: one
// row per shape.
//
// VALIDATES: `FOO=1`, `_x=`, `A1+=b` and `GOFLAGS=-count=1` are assignments;
// a line held as one word and a path holding an `=` are not.
// PREVENTS: `bash -c 'go test -count=1 ./...'` read as a shell handed an
// assignment, with no program, and so as light.
func TestAssignmentReadsOnlyAShellName(t *testing.T) {
	for _, row := range []struct {
		word       string
		assignment bool
	}{
		{"FOO=1", true},
		{"_x=", true},
		{"A1+=b", true},
		{"GOFLAGS=-count=1", true},
		{"go test -count=1 ./x", false},
		{"./bin/x=y", false},
		{"1A=b", false},
		{"=x", false},
		{"go", false},
	} {
		if got := Assignment(row.word); got != row.assignment {
			t.Errorf("Assignment(%q) = %v, want %v", row.word, got, row.assignment)
		}
	}
}
