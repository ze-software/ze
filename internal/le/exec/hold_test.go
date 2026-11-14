// Related: exec.go -- holdHeavy, the decision this file drives
// Related: register.go -- carriesHeavy, the hook's reading of the same program
//
// VALIDATES: spec-native-action-job-admission decision 3: `le exec` asks for a
// job slot only when the program it carries is a heavy tool by the one
// definition the Bash hook applies (heavytool.Name), under `exec` and the
// tool's name; a light program, and a carried le line, ask for none.
// PREVENTS: an editor's gopls that holds a slot for the editor's life, and a
// `go test` typed through `le exec` that runs with no slot.
//
// No case starts a process or reads the registry: the hold is a function the
// case passes, which records what it was asked.

package leexec

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/job"
)

// goRunLine is a carried run verb line, assembled so this file never holds
// the literal the repository contract reads as a test shelling out to a
// compiled tool (TestNoDevelopmentToolTestShellsOutToGoRun).
const goRunLine = "go run" + " ./cmd/ze"

// TestHoldHeavyAsksBySlotOnlyForAHeavyTool asks whether holdHeavy asks the
// hold for a slot exactly when the carried words start a heavy tool, and
// with what. Its method: one row per program, a recording hold, and the
// launch prepare would have built.
func TestHoldHeavyAsksBySlotOnlyForAHeavyTool(t *testing.T) {
	rows := []struct {
		words   string
		command string // empty: the hold is never asked
	}{
		{"go test ./x/", "exec go test"},
		{"/usr/local/go/bin/go vet ./...", "exec go vet"},
		{"golangci-lint run ./...", "exec golangci-lint"},
		{"staticcheck ./...", "exec staticcheck"},
		// The words in front of the program, and the toolchain's directory
		// flag in front of its verb, are read the way the Bash hook reads
		// them (heavytool.Started), so the hook and exec agree.
		{"env go test ./x/", "exec go test"},
		{"nice -n 5 go build ./...", "exec go build"},
		{"timeout 600 go vet ./...", "exec go vet"},
		{"go -C d test ./...", "exec go test"},
		{goRunLine, "exec go run"},
		{"gofmt -l .", "exec gofmt"},
		{"go version", ""},
		{"gopls symbols x.go", ""},
		{"golangci-lint config verify", ""},
		{"env FOO=1 go version", ""},
		{"probe word", ""},
		// The carried le admits its own line, so exec replaces its process.
		{"./le test unit all", ""},
		{"env ./le test unit all", ""},
	}
	for _, row := range rows {
		args := strings.Fields(row.words)
		program := launch{program: "/resolved/" + args[0], argv: args, environ: []string{"HOME=/h"}}
		asked := false
		var command string
		var line []string
		var tool job.Tool
		code, held := holdHeavy(args, program, func(commandName string, typed []string, carried job.Tool) (int, bool) {
			asked = true
			command, line, tool = commandName, typed, carried
			return 9, true
		})
		if row.command == "" {
			if asked || held {
				t.Errorf("le exec %s asked for a slot (%v) or answered held (%v), want neither: the program replaces le", row.words, asked, held)
			}
			continue
		}
		if !asked || !held || code != 9 {
			t.Errorf("le exec %s answered %d, held %v, asked %v; want the hold's 9", row.words, code, held, asked)
			continue
		}
		if command != row.command {
			t.Errorf("le exec %s held its slot as %q, want %q", row.words, command, row.command)
		}
		if len(line) <= len(args) || !slices.Equal(line[len(line)-len(args)-1:], append([]string{"exec"}, args...)) {
			t.Errorf("le exec %s recorded the line %v, want it to end with exec and the carried words", row.words, line)
		}
		if tool.Path != program.program || !slices.Equal(tool.Argv, program.argv) || !slices.Equal(tool.Environ, program.environ) {
			t.Errorf("le exec %s handed the hold %+v, want the prepared launch %+v", row.words, tool, program)
		}
	}
}

// TestCarriesHeavyReadsTheCarriedProgram asks whether the judge exec
// registers names a carried line heavy exactly when its program is a heavy
// tool, or an le line that is heavy itself.
func TestCarriesHeavyReadsTheCarriedProgram(t *testing.T) {
	for _, row := range []struct {
		words string
		heavy bool
	}{
		{"go test ./x/", true},
		{"golangci-lint run", true},
		{"env go test ./x/", true},
		{"nice -n 5 go build ./...", true},
		{"timeout 600 go vet ./...", true},
		{"go -C d test ./...", true},
		{goRunLine, true},
		{"gofmt -l .", true},
		{"go version", false},
		{"gopls", false},
		{"golangci-lint config verify", false},
		{"env FOO=1 go version", false},
		{"probe word", false},
		{"", false},
	} {
		if got := carriesHeavy(strings.Fields(row.words)); got != row.heavy {
			t.Errorf("carriesHeavy(%q) = %v, want %v", row.words, got, row.heavy)
		}
	}
}

// TestCarriedShellLineHoldsASlot asks whether a shell exec carries with a
// `-c` line holds a slot under the shell's name and is heavy for the hook's
// carrier judge, whatever the line runs. Its method: the line as argv holds
// it, one word, through holdHeavy with a recording hold and through
// carriesHeavy.
//
// VALIDATES: `le exec bash -c '<test line>'`, `le exec sh -ec '<build
// line>'` and `le exec bash -o pipefail -c './le test unit all'` take a
// slot, and the hook hands them back: exec holds no shell parser, so both
// sides answer heavy (heavytool.StartedUnread).
// PREVENTS: a heavy tool reaching the machine, unadmitted and unrefused,
// through a carried shell line.
func TestCarriedShellLineHoldsASlot(t *testing.T) {
	for _, row := range []struct {
		args    []string
		command string
	}{
		{[]string{"bash", "-c", "go test ./x"}, "exec bash"},
		{[]string{"sh", "-ec", "go " + "build ./..."}, "exec sh"},
		{[]string{"bash", "-o", "pipefail", "-c", "./le test unit all"}, "exec bash"},
		{[]string{"sh", "-c", "echo hi"}, "exec sh"},
	} {
		program := launch{program: "/resolved/" + row.args[0], argv: row.args, environ: []string{"HOME=/h"}}
		command := ""
		code, held := holdHeavy(row.args, program, func(commandName string, _ []string, _ job.Tool) (int, bool) {
			command = commandName
			return 9, true
		})
		if !held || code != 9 || command != row.command {
			t.Errorf("le exec %q answered %d, held %v, as %q; want the hold's 9 as %q", row.args, code, held, command, row.command)
		}
		if !carriesHeavy(row.args) {
			t.Errorf("carriesHeavy(%q) = false, want true: the hook would pass what holds a slot", row.args)
		}
	}
}
