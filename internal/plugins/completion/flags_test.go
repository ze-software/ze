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

// TestShellCompletionRejectsNewlineInSummary drives the shell-completion
// emitter with descriptions that hold a newline and a tab. The record format is
// `name`, tab, `description`, newline, so either one makes the shell read one
// candidate as two.
//
// VALIDATES: AC-5 -- no newline and no tab reaches the tab-separated shell
// format, the record count matches the candidate count, and every word of the
// declared text survives.
// PREVENTS: the first-newline cut that stood here until this spec. It
// published the first line as if the author had chosen it as the summary. It
// dropped the rest and logged nothing, so its tell is a MISSING second half.
func TestShellCompletionRejectsNewlineInSummary(t *testing.T) {
	registry.RegisterCommandFlags("ze-newline-cmd sub", []registry.FlagSpec{
		{Name: "--wrapped", Description: "First half.\nSecond half.", ValueHint: registry.FlagValueNone},
		{Name: "--tabbed", Description: "Left\tright", ValueHint: registry.FlagValueNone},
		{Name: "--whole", Description: "Keep every word. The tail must survive.", ValueHint: registry.FlagValueNone},
	})

	var buf bytes.Buffer
	if rc := writeFlags(&buf, []string{"ze-newline-cmd", "sub"}); rc != 0 {
		t.Fatalf("writeFlags rc = %d", rc)
	}

	records := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(records) != 3 {
		t.Fatalf("three flags produced %d records, want 3:\n%q", len(records), buf.String())
	}

	byName := make(map[string]string, len(records))
	for _, record := range records {
		name, description, ok := strings.Cut(record, "\t")
		if !ok {
			t.Fatalf("record %q is not name-tab-description", record)
		}
		byName[name] = description
	}

	for name, want := range map[string]string{
		"--wrapped": "First half. Second half.",
		"--tabbed":  "Left right",
		"--whole":   "Keep every word. The tail must survive.",
	} {
		if byName[name] != want {
			t.Errorf("flag %s came out as %q, want %q", name, byName[name], want)
		}
	}
}
