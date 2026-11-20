// VALIDATES: the code-edit gate owes the `## Design decisions` section of every
// page a file's // Design: header names, beside the doors, and a Read that
// returned the whole section clears it
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-1, AC-5).
// PREVENTS: an edit by a context that never met the owner decision the file's
// own design page records, which is how the 2026-11-12 pipe-chain incident
// undid the 2026-08-21 pipe decision.
package hookruntime

import (
	"path/filepath"
	"strings"
	"testing"
)

// Fixture of the design-decisions gate: one code file under the system door
// alone, its design page, and the section that page carries.
const (
	decisionsCodeFile = "internal/component/widget/widget.go"
	decisionsPage     = "docs/architecture/widget.md"
	decisionsHeading  = "## Design decisions"
	decisionsSession  = "sess-a"
)

// decisionsPageText is the design page: prose, then the section, then more
// prose, so a Read that stops inside the section is possible.
const decisionsPageText = "# Widget\n\nHow the widget answers.\n\n" + decisionsHeading + "\n\n" +
	"| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
	"| The widget answers data and the surface renders it | Rendering text in the widget | plan/records/widget.md | 2026-08-21 |\n" +
	"\n## Wire format\n\nThe answer is JSON.\n"

// writeDecisionsFixture lays out the doors, the code file whose header names
// the page, the page, and the record its row cites, and records the whole read
// of the system door, so the only owed reading left is the section.
func writeDecisionsFixture(t *testing.T, root string) {
	t.Helper()
	writeDoors(t, root)
	writeFixtureFile(t, root, decisionsCodeFile, "// Design: "+decisionsPage+" -- the widget\n\npackage widget\n")
	writeFixtureFile(t, root, decisionsPage, decisionsPageText)
	writeFixtureFile(t, root, "plan/records/widget.md", "# Widget record\n")
	readDoor(t, root, decisionsSession, doorSystem, nil)
}

// pageReadPayload is the Read hook payload the harness sends after a Read of
// path that returned lines start to start+lines-1 of a file of total lines.
func pageReadPayload(root, path, content string, start, lines, total int) map[string]any {
	absolute := filepath.Join(root, filepath.FromSlash(path))
	return map[string]any{
		"session_id": decisionsSession, "tool_name": "Read", "tool_use_id": "read-design-page",
		"tool_input": map[string]any{"file_path": absolute, "offset": start, "limit": lines},
		"tool_response": map[string]any{"type": "text", "file": map[string]any{
			"filePath": absolute, "content": content, "startLine": start, "numLines": lines, "totalLines": total,
		}},
	}
}

// requireDecisionsOwed asserts that an Edit of the fixture code file is
// refused, exit 2, with a refusal that names the page and its section.
func requireDecisionsOwed(t *testing.T, root string) {
	t.Helper()
	code, _, message := runHook(t, root, "pretool-writeedit", codeEditPayload(decisionsSession, "", "Edit", decisionsCodeFile))
	if code != 2 {
		t.Fatalf("an edit of %s whose page's decisions are unread exited %d, want 2: %q", decisionsCodeFile, code, message)
	}
	for _, want := range []string{decisionsCodeFile, decisionsPage, decisionsHeading, "Read"} {
		if !strings.Contains(message, want) {
			t.Fatalf("the refusal does not name %q: %q", want, message)
		}
	}
}

// TestDesignPageGateOwesTheDecisionsSection is the wiring row "pre-write hook,
// code edit of a file whose page's decisions are unread". Method: a context
// that read every door it owes edits a file whose // Design: page carries a
// Design decisions section; the edit is refused naming the page and the
// section. A file with no header owes the doors alone, as today.
func TestDesignPageGateOwesTheDecisionsSection(t *testing.T) {
	root := t.TempDir()
	writeDecisionsFixture(t, root)
	requireDecisionsOwed(t, root)

	const headerless = "internal/component/widget/plain.go"
	writeFixtureFile(t, root, headerless, "package widget\n")
	code, _, message := runHook(t, root, "pretool-writeedit", codeEditPayload(decisionsSession, "", "Edit", headerless))
	if strings.Contains(message, decisionsHeading) {
		t.Fatalf("a file with no // Design: header was held to a design page (code %d): %q", code, message)
	}
}

// TestSectionReadRecordedForMarkdown is the wiring row "Read hook, a Read that
// returns the whole section". Method: the edit is refused, the Read hook sees
// a Read of the whole page, and the same edit then passes the gate.
func TestSectionReadRecordedForMarkdown(t *testing.T) {
	root := t.TempDir()
	writeDecisionsFixture(t, root)
	requireDecisionsOwed(t, root)

	lines := strings.Count(decisionsPageText, "\n")
	read := pageReadPayload(root, decisionsPage, decisionsPageText, 1, lines, lines)
	if code, _, message := runHook(t, root, "mark-source-read", read); code != 0 {
		t.Fatalf("mark-source-read of the design page: code=%d message=%q", code, message)
	}
	code, _, message := runHook(t, root, "pretool-writeedit", codeEditPayload(decisionsSession, "", "Edit", decisionsCodeFile))
	if strings.Contains(message, decisionsHeading) {
		t.Fatalf("the section was read whole and is still owed (code %d): %q", code, message)
	}
}
