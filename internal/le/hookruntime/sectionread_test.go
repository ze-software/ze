// VALIDATES: the code-edit gate owes the `## Design decisions` section of every
// page a file's // Design: header names, beside the doors, and a Read that
// returned the whole section clears it
// (spec-design-needs-owner-approval, AC-1, AC-2,
// AC-4 to AC-7).
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
// prose, so a Read that stops inside the section is possible. The section is
// lines 5 to 9, and the heading that ends it is line 11 of 13.
const decisionsPageText = "# Widget\n\nHow the widget answers.\n\n" + decisionsHeading + "\n\n" +
	"| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
	"| The widget answers data and the surface renders it | Rendering text in the widget | plan/records/widget.md | 2026-08-21 |\n" +
	"\n## Wire format\n\nThe answer is JSON.\n"

// danglingRefusal is the fix line of the refusal of a header naming no page.
const danglingRefusal = "correct the header to name the page that documents this file"

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

// decisionsEditPayload proposes an Edit of file that leaves its header as it
// is, from agent ("" for the session itself).
func decisionsEditPayload(agent, file string) map[string]any {
	payload := map[string]any{
		"session_id": decisionsSession, "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": file, "old_string": "package widget", "new_string": "package widget // edited"},
	}
	if agent != "" {
		payload["agent_id"] = agent
	}
	return payload
}

// readPageRange runs the Read hook over a Read by agent of lines first to last
// of text, the page at path, a file of total lines.
func readPageRange(t *testing.T, root, agent, path, text string, first, last int) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	content := strings.Join(lines[first-1:last], "\n")
	payload := pageReadPayload(root, path, content, first, last-first+1, len(lines))
	if agent != "" {
		payload["agent_id"] = agent
	}
	if code, _, message := runHook(t, root, "mark-source-read", payload); code != 0 {
		t.Fatalf("mark-source-read of %s: code=%d message=%q", path, code, message)
	}
}

// readWholePage runs the Read hook over a Read by agent of the whole page.
func readWholePage(t *testing.T, root, agent, path, text string) {
	t.Helper()
	readPageRange(t, root, agent, path, text, 1, strings.Count(text, "\n"))
}

// editMessage runs the pre-write hook over payload and answers its exit code
// and message.
func editMessage(t *testing.T, root string, payload map[string]any) (int, string) {
	t.Helper()
	code, _, message := runHook(t, root, "pretool-writeedit", payload)
	return code, message
}

// requireDecisionsOwed asserts that an Edit of the fixture code file by agent
// is refused, exit 2, with a refusal that names the page and its section.
func requireDecisionsOwed(t *testing.T, root string) {
	t.Helper()
	requireDecisionsOwedFrom(t, root, "")
}

// requireDecisionsOwedFrom is requireDecisionsOwed for an edit by agent.
func requireDecisionsOwedFrom(t *testing.T, root, agent string) {
	t.Helper()
	code, message := editMessage(t, root, decisionsEditPayload(agent, decisionsCodeFile))
	if code != 2 {
		t.Fatalf("an edit of %s whose page's decisions are unread exited %d, want 2: %q", decisionsCodeFile, code, message)
	}
	for _, want := range []string{decisionsCodeFile, decisionsPage, decisionsHeading, "Read"} {
		if !strings.Contains(message, want) {
			t.Fatalf("the refusal does not name %q: %q", want, message)
		}
	}
}

// requireDecisionsCleared asserts that an Edit of the fixture code file by
// agent no longer owes the section.
func requireDecisionsCleared(t *testing.T, root, agent string) {
	t.Helper()
	code, message := editMessage(t, root, decisionsEditPayload(agent, decisionsCodeFile))
	if strings.Contains(message, decisionsHeading) {
		t.Fatalf("the section was read and is still owed (code %d): %q", code, message)
	}
}

// TestDesignPageGateOwesTheDecisionsSection is the wiring row "pre-write hook,
// code edit of a file whose page's decisions are unread". Method: a context
// that read every door it owes edits a file whose // Design: page carries a
// Design decisions section; the edit is refused naming the page, the section's
// lines and the Read that clears it. A file with no header, and a file whose
// page carries no section, owe the doors alone, as today.
func TestDesignPageGateOwesTheDecisionsSection(t *testing.T) {
	root := t.TempDir()
	writeDecisionsFixture(t, root)
	requireDecisionsOwed(t, root)
	_, message := editMessage(t, root, decisionsEditPayload("", decisionsCodeFile))
	if !strings.Contains(message, "(lines 5 to 9): Read it with offset 5 and limit 7") {
		t.Fatalf("the refusal does not give the section's lines and the Read through the heading that ends it: %q", message)
	}

	const headerless = "internal/component/widget/plain.go"
	writeFixtureFile(t, root, headerless, "package widget\n")
	code, _, message := runHook(t, root, "pretool-writeedit", codeEditPayload(decisionsSession, "", "Edit", headerless))
	if strings.Contains(message, decisionsHeading) {
		t.Fatalf("a file with no // Design: header was held to a design page (code %d): %q", code, message)
	}

	const sectionless = "internal/component/widget/quiet.go"
	writeFixtureFile(t, root, "docs/architecture/quiet.md", "# Quiet\n\nNo decision is recorded here.\n")
	writeFixtureFile(t, root, sectionless, "// Design: docs/architecture/quiet.md -- quiet\n\npackage widget\n")
	code, message = editMessage(t, root, decisionsEditPayload("", sectionless))
	if strings.Contains(message, decisionsHeading) || strings.Contains(message, "quiet.md") {
		t.Fatalf("a page with no Design decisions section was owed (code %d): %q", code, message)
	}
}

// TestDesignPageGateReadsADeepHeaderLine pins that the gate reads the whole
// header, as `./le doc consistency` does, and not a fixed count of lines.
// Method: the code file's `// Design:` line is line 27, after a 26-line
// package comment, the shape of internal/component/command/registry/registry.go;
// an edit of it owes the page's section, and the Read clears it.
func TestDesignPageGateReadsADeepHeaderLine(t *testing.T) {
	root := t.TempDir()
	writeDecisionsFixture(t, root)
	deep := strings.Repeat("// Package widget is described at length.\n", 25) + "//\n" +
		"// Design: " + decisionsPage + " -- the widget\npackage widget\n"
	writeFixtureFile(t, root, decisionsCodeFile, deep)
	requireDecisionsOwed(t, root)
	readWholePage(t, root, "", decisionsPage, decisionsPageText)
	requireDecisionsCleared(t, root, "")
}

// TestDesignPageGateReadsEveryHeaderLine pins AC-2. Method: a Write creating a
// file whose proposed header carries two // Design: lines owes both pages; an
// Edit that moves the header from one page to another owes the page on disk
// and the page the edit proposes.
func TestDesignPageGateReadsEveryHeaderLine(t *testing.T) {
	root := t.TempDir()
	writeDecisionsFixture(t, root)
	const gadget = "docs/architecture/gadget.md"
	writeFixtureFile(t, root, gadget, decisionsPageText)

	created := map[string]any{
		"session_id": decisionsSession, "tool_name": "Write",
		"tool_input": map[string]any{
			"file_path": "internal/component/widget/both.go",
			"content":   "// Design: " + decisionsPage + " -- the widget\n// Design: " + gadget + " -- the gadget\n\npackage widget\n",
		},
	}
	code, message := editMessage(t, root, created)
	if code != 2 || !strings.Contains(message, decisionsPage) || !strings.Contains(message, gadget) {
		t.Fatalf("a new file whose header names two pages did not owe both (code %d): %q", code, message)
	}

	moved := map[string]any{
		"session_id": decisionsSession, "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": decisionsCodeFile, "old_string": decisionsPage, "new_string": gadget},
	}
	code, message = editMessage(t, root, moved)
	if code != 2 || !strings.Contains(message, decisionsPage) || !strings.Contains(message, gadget) {
		t.Fatalf("an edit moving the header did not owe the page on disk and the page proposed (code %d): %q", code, message)
	}
}

// TestSectionRecordGoesStaleWithTheSection pins AC-4 both ways. Method: the
// section is read and the edit passes; a row added to the section makes it
// owed again; a fresh read clears it; an edit of the page outside the section
// leaves the record current.
func TestSectionRecordGoesStaleWithTheSection(t *testing.T) {
	root := t.TempDir()
	writeDecisionsFixture(t, root)
	readWholePage(t, root, "", decisionsPage, decisionsPageText)
	requireDecisionsCleared(t, root, "")

	grown := strings.Replace(decisionsPageText, "| 2026-08-21 |\n",
		"| 2026-08-21 |\n| The widget answers in one line | A multi-line answer | plan/records/widget.md | 2026-11-20 |\n", 1)
	writeFixtureFile(t, root, decisionsPage, grown)
	requireDecisionsOwed(t, root)

	readWholePage(t, root, "", decisionsPage, grown)
	requireDecisionsCleared(t, root, "")

	elsewhere := strings.Replace(grown, "The answer is JSON.", "The answer is JSON, always.", 1)
	writeFixtureFile(t, root, decisionsPage, elsewhere)
	requireDecisionsCleared(t, root, "")
}

// TestSectionReadRecordedForMarkdown is the wiring row "Read hook, a Read that
// returns the whole section", and AC-5. Method: the edit is refused, the Read
// hook sees a Read of the whole page, and the same edit then passes the gate.
// A Read that stops inside the section, or that ends on its last line without
// reaching the heading that ends it, records nothing; a Read of a copy of the
// page kept elsewhere in the checkout records nothing for the page; a Read of
// the section through its closing heading records it.
func TestSectionReadRecordedForMarkdown(t *testing.T) {
	root := t.TempDir()
	writeDecisionsFixture(t, root)
	requireDecisionsOwed(t, root)

	lines := strings.Count(decisionsPageText, "\n")
	read := pageReadPayload(root, decisionsPage, decisionsPageText, 1, lines, lines)
	if code, _, message := runHook(t, root, "mark-source-read", read); code != 0 {
		t.Fatalf("mark-source-read of the design page: code=%d message=%q", code, message)
	}
	code, _, message := runHook(t, root, "pretool-writeedit", decisionsEditPayload("", decisionsCodeFile))
	if strings.Contains(message, decisionsHeading) {
		t.Fatalf("the section was read whole and is still owed (code %d): %q", code, message)
	}

	cut := t.TempDir()
	writeDecisionsFixture(t, cut)
	readPageRange(t, cut, "", decisionsPage, decisionsPageText, 1, 8)
	requireDecisionsOwed(t, cut)
	readPageRange(t, cut, "", decisionsPage, decisionsPageText, 5, 9)
	requireDecisionsOwed(t, cut)

	stray := t.TempDir()
	writeDecisionsFixture(t, stray)
	const copied = "tmp/copy/" + decisionsPage
	writeFixtureFile(t, stray, copied, decisionsPageText)
	readWholePage(t, stray, "", copied, decisionsPageText)
	requireDecisionsOwed(t, stray)

	ranged := t.TempDir()
	writeDecisionsFixture(t, ranged)
	readPageRange(t, ranged, "", decisionsPage, decisionsPageText, 5, 11)
	requireDecisionsCleared(t, ranged, "")
}

// TestDesignPageGateHoldsEachSubagentToItsOwnRead pins AC-6. Method: two
// subagents of one session each read the system door; agent1 reads the page;
// agent2's edit still owes the section, and passes once agent2 read it too.
func TestDesignPageGateHoldsEachSubagentToItsOwnRead(t *testing.T) {
	root := t.TempDir()
	writeDecisionsFixture(t, root)
	for _, agent := range []string{"agent1", "agent2"} {
		payload := map[string]any{
			"session_id": decisionsSession, "agent_id": agent, "tool_name": "Read",
			"tool_input": map[string]any{"file_path": filepath.Join(root, filepath.FromSlash(doorSystem))},
		}
		if code, _, message := runHook(t, root, "mark-source-read", payload); code != 0 {
			t.Fatalf("mark-source-read of the system door by %s: code=%d message=%q", agent, code, message)
		}
	}
	readWholePage(t, root, "agent1", decisionsPage, decisionsPageText)
	requireDecisionsCleared(t, root, "agent1")
	requireDecisionsOwedFrom(t, root, "agent2")
	requireDecisionsOwed(t, root)

	readWholePage(t, root, "agent2", decisionsPage, decisionsPageText)
	requireDecisionsCleared(t, root, "agent2")
}

// TestDesignPageGateRefusesADanglingHeader pins AC-7. Method: a Write whose
// proposed header names a page that does not exist is refused naming the
// file, the header line and the fix; the Edit that corrects such a header on
// disk is not; and a spec naming the file owes nothing for the header.
func TestDesignPageGateRefusesADanglingHeader(t *testing.T) {
	root := t.TempDir()
	writeDecisionsFixture(t, root)
	const lost = "internal/component/widget/lost.go"
	const header = "// Design: docs/architecture/missing.md -- lost"

	written := map[string]any{
		"session_id": decisionsSession, "tool_name": "Write",
		"tool_input": map[string]any{"file_path": lost, "content": header + "\n\npackage widget\n"},
	}
	code, message := editMessage(t, root, written)
	if code != 2 {
		t.Fatalf("a header naming a missing page exited %d, want 2: %q", code, message)
	}
	for _, want := range []string{lost, header, danglingRefusal} {
		if !strings.Contains(message, want) {
			t.Fatalf("the dangling-header refusal does not name %q: %q", want, message)
		}
	}

	writeFixtureFile(t, root, lost, header+"\n\npackage widget\n")
	corrected := map[string]any{
		"session_id": decisionsSession, "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": lost, "old_string": "docs/architecture/missing.md", "new_string": decisionsPage},
	}
	if _, message := editMessage(t, root, corrected); strings.Contains(message, danglingRefusal) {
		t.Fatalf("the edit correcting the header was refused as dangling: %q", message)
	}

	spec := specEditPayload("Write", specFixturePath, "## Files to Modify\n- `"+lost+"` - change\n")
	if _, message := editMessage(t, root, spec); strings.Contains(message, danglingRefusal) || strings.Contains(message, "missing.md") {
		t.Fatalf("a spec naming a file with a dangling header was refused for it: %q", message)
	}
}
