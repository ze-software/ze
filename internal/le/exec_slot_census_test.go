// Related: heavy_census_test.go -- the census of every heavy line and its slot
// Related: exec/register.go -- carriesHeavy, the judge `le exec` registers

package le

import (
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestCensusExecTakesASlotByItsProgram asks whether `le exec` reads as heavy,
// and takes a slot, by the program it carries, over the composition root le
// links. Its method: one line per kind of program, with its slot reading and
// its heavy verdict.
//
// VALIDATES: spec-native-action-job-admission decision 3 and C-1: a light
// program takes no slot and is light for a subagent; a heavy tool, and a
// carried le line that is heavy, admit themselves under the command exec, so
// the dispatcher never takes a slot for exec, whose body replaces le.
// PREVENTS: an editor's gopls that holds a slot for its whole life, and a
// heavy tool through exec that the dispatcher admits and can never release.
func TestCensusExecTakesASlotByItsProgram(t *testing.T) {
	for _, row := range []struct {
		line string
		kind leroot.SlotKind
	}{
		{"exec go version", leroot.SlotLight},
		{"exec gopls symbols x.go", leroot.SlotLight},
		{"exec golangci-lint config verify", leroot.SlotLight},
		{"exec probe word", leroot.SlotLight},
		{"exec env FOO=1 go version", leroot.SlotLight},
		{"exec go test ./x/", leroot.SlotSelf},
		{"exec golangci-lint run", leroot.SlotSelf},
		{"exec staticcheck ./...", leroot.SlotSelf},
		{"exec ./le test unit all", leroot.SlotSelf},
		// The words in front of the program and the toolchain's directory
		// flag are read the way the Bash hook reads a raw command
		// (heavytool.Started), so the hook and exec agree.
		{"exec env go test ./x/", leroot.SlotSelf},
		{"exec nice -n 5 go build ./...", leroot.SlotSelf},
		{"exec timeout 600 go vet ./...", leroot.SlotSelf},
		{"exec go -C d test ./...", leroot.SlotSelf},
		{"exec " + "go run" + " ./cmd/ze", leroot.SlotSelf},
		{"exec gofmt -l .", leroot.SlotSelf},
	} {
		words := strings.Fields(row.line)
		slot := leroot.SlotOfLine(words)
		if slot.Kind != row.kind {
			t.Errorf("le %s has the slot reading %+v, want the kind %v", row.line, slot, row.kind)
		}
		if row.kind == leroot.SlotSelf && slot.Command != "exec" {
			t.Errorf("le %s admits itself as %q, want exec", row.line, slot.Command)
		}
		want := row.kind != leroot.SlotLight
		if got := leroot.HeavyLine(words); got != want {
			t.Errorf("HeavyLine(%q) = %v, want %v", row.line, got, want)
		}
		if !leroot.CarrierLine(words) {
			t.Errorf("le %s is not a carrier line, and the piped-command guard would read it as the expensive command", row.line)
		}
	}
}

// TestCensusExecShellLineTakesASlot asks whether `le exec` carrying a shell
// with a `-c` line reads heavy and admits itself under exec, whatever the
// line runs. Its method: the line as argv holds it, one word per argument.
//
// VALIDATES: the carrier judge answers a carried shell line heavy
// (heavytool.StartedUnread), the same answer holdHeavy takes a slot by.
// PREVENTS: `le exec bash -c '<test line>'` reading light.
func TestCensusExecShellLineTakesASlot(t *testing.T) {
	for _, words := range [][]string{
		{"exec", "bash", "-c", "go test ./x"},
		{"exec", "sh", "-ec", "go " + "build ./..."},
		{"exec", "bash", "-o", "pipefail", "-c", "./le test unit all"},
	} {
		slot := leroot.SlotOfLine(words)
		if slot.Kind != leroot.SlotSelf || slot.Command != "exec" {
			t.Errorf("le %q has the slot reading %+v, want SlotSelf under exec", words, slot)
		}
		if !leroot.HeavyLine(words) {
			t.Errorf("HeavyLine(%q) = false, want true", words)
		}
	}
}
