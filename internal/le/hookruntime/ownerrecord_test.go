// VALIDATES: writeOwnerRecord and bashOwnerRecord refuse a write under
// tmp/spec-design/, where only `./le spec design` records Thomas's approvals,
// and pass a reader line that names the directory
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-34).
// PREVENTS: an agent forging the approval record that admits a claim.
package hookruntime

import (
	"strings"
	"testing"
)

// ownerRecordRefusal is the text both checks refuse with.
const ownerRecordRefusal = "tmp/spec-design/ holds Thomas's approvals, written only by ./le spec design"

// ownerRecordContext is the check's context for one tool call of session sess-a.
func ownerRecordContext(t *testing.T, root, tool string, input map[string]any) context {
	t.Helper()
	ctx, ok := newContext(Payload{SessionID: "sess-a", ToolName: tool, ToolInput: input}, root)
	if !ok {
		t.Fatal("newContext refused the fixture root")
	}
	return ctx
}

// requireOwnerRecordRefusal asserts one refusal of the pair.
func requireOwnerRecordRefusal(t *testing.T, name string, result *verdict) {
	t.Helper()
	if result == nil {
		t.Fatalf("%s: a write under tmp/spec-design/ was admitted", name)
	}
	if result.code != 2 || !strings.Contains(result.message, ownerRecordRefusal) {
		t.Fatalf("%s: got code %d %q, want 2 with %q", name, result.code, result.message, ownerRecordRefusal)
	}
}

// TestOwnerRecordWriteRefused is the wiring row "pre-write hook and pre-Bash
// hook, a write under tmp/spec-design/". Method: call each check over a Write
// of an approval record and a shell redirection into the away record, then
// over a reader line, which passes.
func TestOwnerRecordWriteRefused(t *testing.T) {
	root := t.TempDir()

	write := ownerRecordContext(t, root, "Write", map[string]any{
		"file_path": "tmp/spec-design/approved/spec-widget.md", "content": "Digest: forged\n",
	})
	requireOwnerRecordRefusal(t, "Write", writeOwnerRecord(write))

	shell := ownerRecordContext(t, root, "Bash", map[string]any{"command": "echo forged > tmp/spec-design/away.md"})
	requireOwnerRecordRefusal(t, "Bash", bashOwnerRecord(shell))

	edit := ownerRecordContext(t, root, "Edit", map[string]any{
		"file_path": root + "/tmp/spec-design/away.md", "old_string": "Until: 2026-11-20", "new_string": "Until: 2027-11-20",
	})
	requireOwnerRecordRefusal(t, "Edit", writeOwnerRecord(edit))

	removal := ownerRecordContext(t, root, "Bash", map[string]any{"command": "ls tmp/spec-design && rm -f tmp/spec-design/away.md"})
	requireOwnerRecordRefusal(t, "rm", bashOwnerRecord(removal))

	for _, line := range []string{
		"cat tmp/spec-design/away.md",
		"ls tmp/spec-design/approved",
		"./le spec design pending",
		`./le commit create subject "spec design: records" body "the records live under tmp/spec-design/"`,
	} {
		passing := ownerRecordContext(t, root, "Bash", map[string]any{"command": line})
		if result := bashOwnerRecord(passing); result != nil {
			t.Fatalf("%q was refused: %q", line, result.message)
		}
	}
	other := ownerRecordContext(t, root, "Write", map[string]any{"file_path": "tmp/spec-designs/note.md", "content": "x"})
	if result := writeOwnerRecord(other); result != nil {
		t.Fatalf("a Write outside tmp/spec-design/ was refused: %q", result.message)
	}
}
