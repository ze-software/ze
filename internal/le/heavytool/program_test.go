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
