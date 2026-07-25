// Design: (none -- shell completion flag/value inventory tests)

package completion

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// TestFlagInventoryCompletion verifies `ze completion flags <path>` emits the
// registered flags as word\tdescription TSV.
//
// VALIDATES: AC-8 -- flag names complete from the registry inventory.
// PREVENTS: regression to hardcoded per-subcommand flag lists in the shell
// generators.
func TestFlagInventoryCompletion(t *testing.T) {
	registry.RegisterCommandFlags("ze-test-cmd sub", []registry.FlagSpec{
		{Name: "--family", Description: "address family", ValueHint: registry.FlagValueFamily},
		{Name: "--add-path", Description: "advertise add-path", ValueHint: registry.FlagValueNone},
	})

	var buf bytes.Buffer
	if rc := writeFlags(&buf, []string{"ze-test-cmd", "sub"}); rc != 0 {
		t.Fatalf("writeFlags rc = %d", rc)
	}
	out := buf.String()
	if !strings.Contains(out, "--family\t") {
		t.Fatalf("missing --family flag:\n%s", out)
	}
	if !strings.Contains(out, "--add-path\t") {
		t.Fatalf("missing --add-path flag:\n%s", out)
	}
	// Each line must be a word\tdescription pair.
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if !strings.Contains(line, "\t") {
			t.Fatalf("line without tab separator: %q", line)
		}
	}
}

// TestFlagInventoryUnknownPath verifies an unregistered path yields no output.
func TestFlagInventoryUnknownPath(t *testing.T) {
	var buf bytes.Buffer
	if rc := writeFlags(&buf, []string{"no", "such", "command"}); rc != 0 {
		t.Fatalf("writeFlags rc = %d", rc)
	}
	if buf.Len() != 0 {
		t.Fatalf("unknown path produced output: %q", buf.String())
	}
}

// TestFamilyFlagValues verifies `ze completion families` emits address families
// (used to complete `--family <TAB>`) from the shared registry.
//
// VALIDATES: AC-8 -- --family completes from registry.AllFamilies().
// PREVENTS: a second family source drifting from the plugin registry.
func TestFamilyFlagValues(t *testing.T) {
	var buf bytes.Buffer
	if rc := writeFamilies(&buf); rc != 0 {
		t.Fatalf("writeFamilies rc = %d", rc)
	}
	out := buf.String()
	if !strings.Contains(out, "ipv4/unicast") {
		t.Fatalf("families output missing ipv4/unicast:\n%s", out)
	}
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(line, "\t") {
			t.Fatalf("family line without tab separator: %q", line)
		}
	}
}
