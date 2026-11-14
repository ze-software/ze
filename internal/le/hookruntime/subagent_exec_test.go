// Related: subagent_heavy.go -- heavyTool, which reads an le line through leroot.HeavyCommand
// Related: ../exec/register.go -- carriesHeavy, the judge `le exec` registers for what it carries

package hookruntime

import (
	"strings"
	"testing"
)

// execNamed is the name the hand-back gives a heavy `./le exec` line: the
// command le resolved the line to (leroot.HeavyCommand).
const execNamed = "the `./le` action `./le exec`"

// TestSubagentExecIsJudgedByItsProgram asks whether a subagent's `./le exec
// <program>` is judged by the program it carries, through the judge the
// carrier registers and with no branch in the hook. Its method: each line
// through the pretool-bash action as a subagent, a light program expected to
// pass and a heavy one to be handed back under the name `./le exec`.
//
// VALIDATES: spec-native-action-job-admission decision 3: `./le exec gopls`,
// `./le exec go version` and `./le exec golangci-lint config verify` run, and
// `./le exec go test`, `./le exec golangci-lint run` and a carried heavy le
// line are handed back. The carried words are read the way the hook reads a
// raw command (heavytool.Started): through assignments, launchers and the
// toolchain's `-C dir`.
// PREVENTS: a subagent refused a light program it reaches only through
// `le exec`, a heavy tool that passes the hook because `le exec` carries it,
// and one that passes because a launcher stands in front of it there.
func TestSubagentExecIsJudgedByItsProgram(t *testing.T) {
	for _, row := range []struct {
		command string
		heavy   bool
	}{
		{"./le exec gopls symbols x.go", false},
		{"./le exec gopls", false},
		{"./le exec go version", false},
		{"./le exec golangci-lint config verify", false},
		{"./le exec env FOO=1 go version", false},
		{"./le exec probe word", false},
		{"./le exec go test ./x/", true},
		{"./le exec golangci-lint run", true},
		{"./le exec staticcheck ./...", true},
		{"./le exec ./le test unit all", true},
		{"./le exec env go test ./x/", true},
		{"./le exec nice -n 5 go build ./...", true},
		{"./le exec timeout 600 go vet ./...", true},
		{"./le exec go -C d test ./...", true},
		{"./le exec " + "go run" + " ./cmd/ze", true},
		{"./le exec gofmt -l .", true},
		// A carried shell line is heavy whatever it runs: exec holds no
		// shell parser, and holdHeavy takes a slot by the same answer
		// (heavytool.StartedUnread).
		{"./le exec bash -c 'go test ./x'", true},
		{"./le exec sh -ec 'go " + "build ./...'", true},
		{"./le exec bash -o pipefail -c './le test unit all'", true},
		{"./le exec sh -c 'echo hi'", true},
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+row.command)
		handed := strings.Contains(message, handBackHeader)
		if handed != row.heavy {
			t.Errorf("%q: handed back = %v, want %v: %s", row.command, handed, row.heavy, message)
		}
		if row.heavy && code != 2 {
			t.Errorf("%q: code=%d, want exit 2 for a hand-back: %s", row.command, code, message)
		}
		if row.heavy && !strings.Contains(message, execNamed) {
			t.Errorf("%q: the hand-back does not name `./le exec`: %s", row.command, message)
		}
		if !row.heavy && (code != 0 || output != "") {
			t.Errorf("%q: code=%d output=%q, want exit 0 and no output: %s", row.command, code, output, message)
		}
	}
}

// TestSubagentQuotedLeExecIsData asks whether a quoted argument of another
// program that holds the text `le exec` is read as data, while a real
// `./le exec` line is still handed back wherever the line runs it. Its
// method: each line through the pretool-bash action as a subagent.
//
// VALIDATES: the heavy-work judgment of a line that parses cuts its text only
// outside the tree's quoted parts (unquotedSegments), and reads the line a
// shell is handed with `-c` as a line of its own (heavyWork).
// PREVENTS: the read-only search an independent judge typed being handed
// back: the quote-blind split cut its grep pattern at the quoted bars and
// read `/le exec"` as an le line.
func TestSubagentQuotedLeExecIsData(t *testing.T) {
	for _, row := range []struct {
		command string
		heavy   bool
	}{
		{`grep -rn "exec " internal/le/ai/hooks/*.go | grep -i "le exec\|\"exec\|/le exec" | head -20; grep -rl "exec" internal/le | xargs grep -ln "le exec"`, false},
		{`grep -rl "x" internal/le | xargs grep -ln '/le exec go test ./x/; ./le exec go test'`, false},
		{`grep x y; ./le exec go test ./x/`, true},
		{`true && ./le exec go test ./x/`, true},
		{`echo x | ./le exec go test ./x/`, true},
		{`sh -c "cd x; ./le exec go test ./x/"`, true},
		{`bash -c './le exec go test ./x/'`, true},
		{`bash -o pipefail -c './le exec go test ./x/ | tail -5'`, true},
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+row.command)
		handed := strings.Contains(message, handBackHeader)
		if handed != row.heavy {
			t.Errorf("%q: handed back = %v, want %v: %s", row.command, handed, row.heavy, message)
		}
		if row.heavy && (code != 2 || !strings.Contains(message, execNamed)) {
			t.Errorf("%q: code=%d, want exit 2 with a hand-back naming `./le exec`: %s", row.command, code, message)
		}
		if !row.heavy && (code != 0 || output != "") {
			t.Errorf("%q: code=%d output=%q, want exit 0 and no output: %s", row.command, code, output, message)
		}
	}
}
