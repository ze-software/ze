// VALIDATES: the read gates (writeStyleGuideRead, writeDesignDoorsRead,
// writeSpecDoorsRead, writeDesignEvidence) hold an edit to the copies of the
// style guide, the doors and the producers in the checkout the edited file
// lies in, and count a read, through Read or a Bash print, for the checkout
// whose copy it read; writeSpecMechanisms and writeSpecWIP judge a worktree's
// spec instead of skipping it.
// PREVENTS: an agent in a worktree made to read the main checkout's copies
// before it may edit, and a read of one checkout's copy, which its branch may
// have changed, counted for another checkout's edits.
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// worktreeFixture is the name of the fixture worktree under the session's
// .claude/worktrees.
const worktreeFixture = "wt"

// worktreeLabel is how the gates name the fixture worktree.
const worktreeLabel = ".claude/worktrees/" + worktreeFixture

// writeCheckoutFixture makes dir a git checkout of Ze the way gatedCheckout
// recognizes one (the lepath markers and a .git entry, here a linked
// worktree's file), holding the fixture doors and a copy of the style guide.
func writeCheckoutFixture(t *testing.T, dir string) {
	t.Helper()
	writeFixtureFile(t, dir, "go.mod", "module example.com/ze\n")
	writeFixtureFile(t, dir, "feature-gates.txt", "\n")
	writeFixtureFile(t, dir, ".git", "gitdir: /elsewhere/.git/worktrees/"+worktreeFixture+"\n")
	writeDoors(t, dir)
	writeFixtureFile(t, dir, styleGuidePath, strings.Repeat("A line of the guide.\n", guideLines))
}

// sessionWithWorktree lays out a session root holding its own doors and style
// guide, with a worktree under its .claude/worktrees, and answers both.
func sessionWithWorktree(t *testing.T) (root, worktree string) {
	t.Helper()
	root = t.TempDir()
	writeDoors(t, root)
	writeFixtureFile(t, root, styleGuidePath, strings.Repeat("A line of the guide.\n", guideLines))
	worktree = filepath.Join(root, ".claude", "worktrees", worktreeFixture)
	writeCheckoutFixture(t, worktree)
	return root, worktree
}

// readCopy runs the Read hook of session sess-a, rooted at root, over the file
// at the absolute path document, read whole.
func readCopy(t *testing.T, root, document string) {
	t.Helper()
	payload := map[string]any{"session_id": "sess-a", "tool_name": "Read", "tool_input": map[string]any{"file_path": document}}
	if code, _, message := runHook(t, root, "mark-source-read", payload); code != 0 {
		t.Fatalf("mark-source-read: code=%d message=%q", code, message)
	}
}

// requireReadGate asserts that the pre-write hook refuses the payload with
// refusal, the text only one gate writes, exactly when names is not empty, and
// that the refusal carries each of names, the copies it must name.
func requireReadGate(t *testing.T, root string, payload map[string]any, refusal string, names []string) {
	t.Helper()
	code, _, message := runHook(t, root, "pretool-writeedit", payload)
	refused := strings.Contains(message, refusal)
	if refused != (len(names) != 0) {
		t.Fatalf("refused = %v (code %d), want a refusal naming %v: %q", refused, code, names, message)
	}
	if !refused {
		return
	}
	if code != 2 {
		t.Fatalf("a refusal must exit 2, got %d: %q", code, message)
	}
	for _, name := range names {
		if !strings.Contains(message, name) {
			t.Fatalf("the refusal does not name %q: %q", name, message)
		}
	}
}

// listedDoor is how a door refusal lists an unread door.
func listedDoor(name string) string { return "  - " + name + "\n" }

// TestDoorGateCountsAWorktreeReadForAWorktreeEdit: an edit in a worktree is
// refused until the context read the worktree's own copies of system.md and
// the covering door, the refusal names those copies, and reading them clears
// it.
func TestDoorGateCountsAWorktreeReadForAWorktreeEdit(t *testing.T) {
	root, worktree := sessionWithWorktree(t)
	edit := codeEditPayload("sess-a", "", "Edit", filepath.Join(worktree, "internal", "component", "bgp", "plugins", "rib", "rib.go"))
	requireReadGate(t, root, edit, doorGateRefusal, []string{
		listedDoor(worktreeLabel + "/" + doorSystem),
		listedDoor(worktreeLabel + "/" + doorBGPPlugins),
		worktreeLabel + "/internal/component/bgp/plugins/rib/rib.go",
	})
	readCopy(t, root, filepath.Join(worktree, filepath.FromSlash(doorSystem)))
	readCopy(t, root, filepath.Join(worktree, filepath.FromSlash(doorBGPPlugins)))
	requireReadGate(t, root, edit, doorGateRefusal, nil)
}

// TestDoorGateHoldsEachCheckoutToItsOwnCopies pins the cross-checkout case
// both ways: a read of the session root's copies does not open the gate for a
// worktree edit, and a read of the worktree's copies does not open it for an
// edit in the session root.
func TestDoorGateHoldsEachCheckoutToItsOwnCopies(t *testing.T) {
	file := filepath.Join("internal", "component", "bgp", "plugins", "rib", "rib.go")

	root, worktree := sessionWithWorktree(t)
	readDoor(t, root, "sess-a", doorSystem, nil)
	readDoor(t, root, "sess-a", doorBGPPlugins, nil)
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(root, file)), doorGateRefusal, nil)
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(worktree, file)), doorGateRefusal, []string{
		listedDoor(worktreeLabel + "/" + doorSystem),
		listedDoor(worktreeLabel + "/" + doorBGPPlugins),
	})

	root, worktree = sessionWithWorktree(t)
	readCopy(t, root, filepath.Join(worktree, filepath.FromSlash(doorSystem)))
	readCopy(t, root, filepath.Join(worktree, filepath.FromSlash(doorBGPPlugins)))
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(worktree, file)), doorGateRefusal, nil)
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(root, file)), doorGateRefusal, []string{
		listedDoor(doorSystem),
		listedDoor(doorBGPPlugins),
	})
}

// TestDoorGateTakesTheDoorsOfTheWorktreeBranch: a door the worktree's branch
// added, which the session root does not hold, covers the worktree's files.
func TestDoorGateTakesTheDoorsOfTheWorktreeBranch(t *testing.T) {
	root, worktree := sessionWithWorktree(t)
	widget := "ai/doors/widget.md"
	writeFixtureFile(t, worktree, widget, "# Door\n\n<!-- door-base: internal/component/widget -->\n")
	readCopy(t, root, filepath.Join(worktree, filepath.FromSlash(doorSystem)))
	edit := codeEditPayload("sess-a", "", "Write", filepath.Join(worktree, "internal", "component", "widget", "widget.go"))
	requireReadGate(t, root, edit, doorGateRefusal, []string{listedDoor(worktreeLabel + "/" + widget)})
	readCopy(t, root, filepath.Join(worktree, filepath.FromSlash(widget)))
	requireReadGate(t, root, edit, doorGateRefusal, nil)
}

// TestDoorGateGatesACheckoutOutsideTheSessionRoot: a worktree elsewhere on the
// disk (ai/owner/machines.md) is gated by its own doors, named by its absolute
// path, and a Bash print of its copies clears the gate.
func TestDoorGateGatesACheckoutOutsideTheSessionRoot(t *testing.T) {
	root := t.TempDir()
	writeDoors(t, root)
	other, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeCheckoutFixture(t, other)
	label := filepath.ToSlash(other)
	edit := codeEditPayload("sess-a", "", "Edit", filepath.Join(other, "internal", "component", "foo", "foo.go"))
	requireReadGate(t, root, edit, doorGateRefusal, []string{listedDoor(label + "/" + doorSystem)})
	runBash(t, root, "", "cat "+filepath.Join(other, filepath.FromSlash(doorSystem)))
	requireReadGate(t, root, edit, doorGateRefusal, nil)
}

// TestDoorGatePlacesABashReadWhereItRuns: a relative cat run in a worktree
// records the worktree's copy, never the session root's, and a print of a
// path that is no checkout's own copy records nothing.
func TestDoorGatePlacesABashReadWhereItRuns(t *testing.T) {
	file := filepath.Join("internal", "component", "foo", "foo.go")
	root, worktree := sessionWithWorktree(t)
	payload := map[string]any{
		"session_id": "sess-a",
		"tool_name":  "Bash",
		"cwd":        worktree,
		"tool_input": map[string]any{"command": "cat " + doorSystem},
	}
	runHook(t, root, "pretool-bash", payload)
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(worktree, file)), doorGateRefusal, nil)
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(root, file)), doorGateRefusal, []string{listedDoor(doorSystem)})

	root = t.TempDir()
	writeDoors(t, root)
	scratch := t.TempDir()
	writeFixtureFile(t, scratch, doorSystem, "# A copy in no checkout\n")
	runBash(t, root, "", "cat "+filepath.Join(scratch, filepath.FromSlash(doorSystem)))
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", file), doorGateRefusal, []string{listedDoor(doorSystem)})
}

// requireDoorCredit asserts which checkout's copy of system.md the context has
// read: the worktree's when worktreeRead is set, else the session root's, and
// the other checkout's not at all.
func requireDoorCredit(t *testing.T, root, worktree string, worktreeRead bool) {
	t.Helper()
	file := filepath.Join("internal", "component", "foo", "foo.go")
	worktreeUnread := []string{listedDoor(worktreeLabel + "/" + doorSystem)}
	rootUnread := []string(nil)
	if worktreeRead {
		worktreeUnread, rootUnread = nil, []string{listedDoor(doorSystem)}
	}
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(worktree, file)), doorGateRefusal, worktreeUnread)
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(root, file)), doorGateRefusal, rootUnread)
}

// requireNoDoorCredit asserts that the context has read neither checkout's
// copy of system.md.
func requireNoDoorCredit(t *testing.T, root, worktree string) {
	t.Helper()
	file := filepath.Join("internal", "component", "foo", "foo.go")
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(worktree, file)), doorGateRefusal, []string{listedDoor(worktreeLabel + "/" + doorSystem)})
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(root, file)), doorGateRefusal, []string{listedDoor(doorSystem)})
}

// TestDoorGateCreditsOnlyAPlainReadLine: a plain read line (an optional first
// `cd <path>`, then `cat [-n]` or `nl` of plain paths, joined by `&&` or `;`)
// records the copy each operand opens, in the checkout it lies in. Every other
// line records nothing in either checkout, absolute operands included: a cd
// the hook cannot place (behind another word, in a group, not first, a pushd,
// spelled by a variable, run by `.`, `eval` or `bash -c`), a print that never
// runs (inside quotes, after an escaped separator, in a heredoc body, behind a
// comment, a here-string), a plain print beside any other command, a print
// after one whose operand is missing or a directory, which `&&` skips, and a
// path under /proc or /dev. The test runs from the session root, so
// /proc/self/cwd reaches the root's copy for the hook while the shell reads
// the worktree's. %s is the worktree and %r the session root.
func TestDoorGateCreditsOnlyAPlainReadLine(t *testing.T) {
	for _, test := range []struct {
		line     string
		worktree bool
	}{
		{"cd %s && cat " + doorSystem, true},
		{"cd %s; cat -n " + doorSystem, true},
		{"cat %s/" + doorSystem, true},
		{"nl " + doorSystem, false},
		{"cd %s && nl %r/" + doorSystem, false},
	} {
		root, worktree := sessionWithWorktree(t)
		runBash(t, root, "", strings.NewReplacer("%s", worktree, "%r", root).Replace(test.line))
		requireDoorCredit(t, root, worktree, test.worktree)
	}

	for _, line := range []string{
		"builtin cd %s && cat " + doorSystem,
		"command cd %s && cat " + doorSystem,
		"if true; then cd %s; cat " + doorSystem + "; fi",
		"{ cd %s; cat " + doorSystem + "; }",
		"cd %s & cat " + doorSystem,
		"cd %s || cat " + doorSystem,
		"pushd %s && cat " + doorSystem,
		"cd %r && cd %s && cat " + doorSystem,
		"x=c; ${x}d %s; cat " + doorSystem,
		". ./enter.sh; cat " + doorSystem,
		"echo 'x; cd %s'; cat " + doorSystem,
		"cd %s && echo \"; cd %r\" && cat " + doorSystem,
		"bash -c 'cd %s; cat " + doorSystem + "'",
		"eval 'cd %s'; cat " + doorSystem,
		"cat <<EOF\ncd %s\nEOF\ncat " + doorSystem,
		"cd %s\\; cat " + doorSystem,
		"echo `cd %s`; cat " + doorSystem,
		"echo 'x; cat %s/" + doorSystem + "'",
		"echo x\\; cat %s/" + doorSystem,
		"cat <<EOF\ncat %s/" + doorSystem + "\nEOF",
		"cat <<< %s/" + doorSystem,
		"true # ; cat " + doorSystem,
		"cat " + doorSystem + "; echo \"--\"; cat %s/" + doorSystem,
		"cat /nonexistent && cat %s/" + doorSystem,
		"cat missing.md && cat " + doorSystem,
		"cat -n %s/ai/doors && cat %s/" + doorSystem,
		"cat ai/doors && cat " + doorSystem,
		"cd %s && cat /proc/self/cwd/" + doorSystem,
		"cd /proc/self/cwd && cat " + doorSystem,
		"cat /proc/self/cwd/" + doorSystem,
	} {
		root, worktree := sessionWithWorktree(t)
		t.Chdir(root)
		runBash(t, root, "", strings.NewReplacer("%s", worktree, "%r", root).Replace(line))
		requireNoDoorCredit(t, root, worktree)
	}
}

// TestDoorGateCreditsNoReadThroughAPlantedLink: a link planted in the worktree
// that points at /proc/self/cwd hides the /proc path from the line's text. The
// hook resolves it against its own working directory, the session root here,
// while the shell resolves it against its own, so neither a cd through the
// link nor an operand through it records a read in either checkout.
func TestDoorGateCreditsNoReadThroughAPlantedLink(t *testing.T) {
	for _, line := range []string{
		"cd %s && cat here/" + doorSystem,
		"cd %s/here && cat " + doorSystem,
		"cat %s/here/" + doorSystem,
		"cd %s && cat hop/" + doorSystem,
	} {
		root, worktree := sessionWithWorktree(t)
		t.Chdir(root)
		if err := os.Symlink("/proc/self/cwd", filepath.Join(worktree, "here")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("here", filepath.Join(worktree, "hop")); err != nil {
			t.Fatal(err)
		}
		runBash(t, root, "", strings.ReplaceAll(line, "%s", worktree))
		requireNoDoorCredit(t, root, worktree)
	}
}

// padding answers size bytes of text in 100-byte lines, the last one cut short
// and unterminated when size is not a multiple of 100, and its line count.
func padding(size int) (string, int) {
	text := strings.Repeat(strings.Repeat("x", 99)+"\n", size/100) + strings.Repeat("x", size%100)
	lines := size / 100
	if size%100 != 0 {
		lines++
	}
	return text, lines
}

// TestDoorGateCreditsNoPrintPastTheHarnessCut: the harness cuts a long Bash
// output, so a line whose prints total more than plainReadPrintMax bytes
// records nothing, and one at the limit records each door. The line numbers
// `cat -n` and `nl` add (six columns and a tab per line) count, so a pair of
// files a plain cat may print together is too long numbered. The pad file sits
// in the session root beside its system.md.
func TestDoorGateCreditsNoPrintPastTheHarnessCut(t *testing.T) {
	probe, _ := sessionWithWorktree(t)
	door, err := os.ReadFile(filepath.Join(probe, filepath.FromSlash(doorSystem)))
	if err != nil {
		t.Fatal(err)
	}
	doorLines := strings.Count(string(door), "\n")
	if len(door) != 0 && door[len(door)-1] != '\n' {
		doorLines++
	}
	numberedFit := 0
	for size := plainReadPrintMax - len(door); size > 0; size-- {
		_, lines := padding(size)
		if size+len(door)+7*(lines+doorLines) <= plainReadPrintMax {
			numberedFit = size
			break
		}
	}
	for _, test := range []struct {
		line     string
		pad      int
		credited bool
	}{
		{"cat pad.md " + doorSystem, plainReadPrintMax - len(door), true},
		{"cat pad.md " + doorSystem, plainReadPrintMax - len(door) + 1, false},
		{"cat pad.md && cat " + doorSystem, plainReadPrintMax - len(door) + 1, false},
		{"cat -n pad.md " + doorSystem, plainReadPrintMax - len(door), false},
		{"nl pad.md; nl " + doorSystem, plainReadPrintMax - len(door), false},
		{"cat -n pad.md " + doorSystem, numberedFit, true},
		{"nl pad.md; nl " + doorSystem, numberedFit + 1, false},
	} {
		root, worktree := sessionWithWorktree(t)
		text, _ := padding(test.pad)
		writeFixtureFile(t, root, "pad.md", text)
		runBash(t, root, "", test.line)
		if test.credited {
			requireDoorCredit(t, root, worktree, false)
			continue
		}
		requireNoDoorCredit(t, root, worktree)
	}
}

// TestDoorGateCreditsNothingPastAnUnreadableFile: a cat of a file it cannot
// open fails, and `&&` skips the print after it, so the line records no read
// of the door that print names.
func TestDoorGateCreditsNothingPastAnUnreadableFile(t *testing.T) {
	root, worktree := sessionWithWorktree(t)
	writeFixtureFile(t, worktree, "locked.md", "A page nobody may read.\n")
	locked := filepath.Join(worktree, "locked.md")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o600); err != nil {
			t.Error(err)
		}
	})
	if file, err := os.Open(locked); err == nil {
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		t.Skip("the current user can open a mode-000 file; an unreadable fixture is unavailable")
	}
	runBash(t, root, "", "cat "+locked+" && cat "+filepath.Join(worktree, filepath.FromSlash(doorSystem)))
	requireNoDoorCredit(t, root, worktree)
}

// TestDoorGatePlacesAReadPhysically: the kernel resolves a path by following
// each link before the `..` after it, so with link in the session root
// pointing at a worktree's internal/, `cat ../ai/doors/system.md` after `cd
// link` and `cat <root>/link/../ai/doors/system.md` both read, and must
// record, the worktree's copy, never the root's copy beside the link.
func TestDoorGatePlacesAReadPhysically(t *testing.T) {
	for _, line := range []string{
		"cd %l && cat ../" + doorSystem,
		"cat %l/../" + doorSystem,
	} {
		root, worktree := sessionWithWorktree(t)
		target := filepath.Join(worktree, "internal")
		if err := os.MkdirAll(target, 0o750); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		runBash(t, root, "", strings.ReplaceAll(line, "%l", link))
		requireDoorCredit(t, root, worktree, true)
	}
}

// TestStyleGateCountsTheWorktreeCopyForAWorktreeEdit: a Go edit in a worktree
// owes the worktree's copy of the style guide, the refusal names it, and a
// read of the session root's copy does not count.
func TestStyleGateCountsTheWorktreeCopyForAWorktreeEdit(t *testing.T) {
	root, worktree := sessionWithWorktree(t)
	edit := codeEditPayload("sess-a", "", "Edit", filepath.Join(worktree, "internal", "core", "sample.go"))
	guide := worktreeLabel + "/" + styleGuidePath
	requireReadGate(t, root, edit, styleGateRefusal, []string{"read " + guide + " in full"})
	readCopy(t, root, filepath.Join(root, filepath.FromSlash(styleGuidePath)))
	requireReadGate(t, root, edit, styleGateRefusal, []string{"read " + guide + " in full"})
	readCopy(t, root, filepath.Join(worktree, filepath.FromSlash(styleGuidePath)))
	requireReadGate(t, root, edit, styleGateRefusal, nil)
	requireReadGate(t, root, codeEditPayload("sess-a", "", "Edit", filepath.Join(root, "internal", "core", "sample.go")), styleGateRefusal, nil)
}

// worktreeDesignGate probes writeDesignEvidence on a Write of spec, an
// absolute path, with a proposal naming evidenceProbeA under Files to Modify,
// and asserts the exit code it answers.
func worktreeDesignGate(t *testing.T, root, spec string, want int) string {
	t.Helper()
	proposed := "## Files to Modify\n`" + evidenceProbeA + "`\n"
	code, message, found := Probe("writeDesignEvidence", Payload{
		SessionID: evidenceProbeSession, ToolName: toolWrite,
		ToolInput: map[string]any{"file_path": spec, "content": proposed},
	}, root)
	if !found || code != want {
		t.Fatalf("design gate on %s = %d, found %v, want %d: %s", spec, code, found, want, message)
	}
	return message
}

// TestDesignEvidenceHoldsEachCheckoutToItsOwnSource: a spec in a worktree owes
// a read of the worktree's producer, which the refusal names, and a read of the
// session root's copy of the same path does not count; nor does a worktree
// read count for a spec in the session root.
func TestDesignEvidenceHoldsEachCheckoutToItsOwnSource(t *testing.T) {
	root, worktree := sessionWithWorktree(t)
	writeHookFixture(t, root, evidenceProbeA, "package le\n")
	writeHookFixture(t, worktree, evidenceProbeA, "package le\n")
	worktreeSpec := filepath.Join(worktree, filepath.FromSlash(evidenceProbeSpec))
	rootSpec := filepath.Join(root, filepath.FromSlash(evidenceProbeSpec))

	recordObservedRead(t, root, filepath.Join(root, filepath.FromSlash(evidenceProbeA)), "package le\n", 1, 2, 2)
	message := worktreeDesignGate(t, root, worktreeSpec, 2)
	if !strings.Contains(message, worktreeLabel+"/"+evidenceProbeA) {
		t.Fatalf("refusal does not name the worktree's producer: %s", message)
	}
	worktreeDesignGate(t, root, rootSpec, 0)

	root, worktree = sessionWithWorktree(t)
	writeHookFixture(t, root, evidenceProbeA, "package le\n")
	writeHookFixture(t, worktree, evidenceProbeA, "package le\n")
	worktreeSpec = filepath.Join(worktree, filepath.FromSlash(evidenceProbeSpec))
	rootSpec = filepath.Join(root, filepath.FromSlash(evidenceProbeSpec))
	recordObservedRead(t, root, filepath.Join(worktree, filepath.FromSlash(evidenceProbeA)), "package le\n", 1, 2, 2)
	worktreeDesignGate(t, root, worktreeSpec, 0)
	message = worktreeDesignGate(t, root, rootSpec, 2)
	if !strings.Contains(message, evidenceProbeA) || strings.Contains(message, worktreeLabel) {
		t.Fatalf("refusal does not name the session root's producer: %s", message)
	}
}

// TestSpecMechanismsGateJudgesAWorktreeSpec: a spec in a worktree that claims
// a design without an Existing Mechanisms row is refused, not skipped, and
// passes once it carries the row.
func TestSpecMechanismsGateJudgesAWorktreeSpec(t *testing.T) {
	root, worktree := sessionWithWorktree(t)
	spec := filepath.Join(worktree, filepath.FromSlash(specFixturePath))
	withSection := strings.Replace(mechanismsDesignSpec, "## Data Flow", mechanismsSection+"## Data Flow", 1)
	for _, test := range []struct {
		content string
		refused bool
	}{{mechanismsDesignSpec, true}, {withSection, false}} {
		input := map[string]any{"file_path": spec, "content": test.content}
		result := writeSpecMechanisms(context{root: root, tool: toolWrite, input: input, path: spec})
		if (result != nil) != test.refused {
			t.Fatalf("refused = %v, want %v: %#v", result != nil, test.refused, result)
		}
		if result != nil && result.code != 2 {
			t.Fatalf("a refusal must exit 2: %#v", result)
		}
	}
}

// TestSpecWIPGateJudgesAWorktreeSpec: a direct ready to in-progress edit of a
// spec in a worktree is refused and sent to the claim publisher, as it is in
// the session root, rather than skipped.
func TestSpecWIPGateJudgesAWorktreeSpec(t *testing.T) {
	root, worktree := sessionWithWorktree(t)
	const ready = "| Field | Value |\n|---|---|\n| Status | ready |\n| Updated | 2026-11-04 |\n"
	writeFixtureFile(t, worktree, specFixturePath, ready)
	spec := filepath.Join(worktree, filepath.FromSlash(specFixturePath))
	input := map[string]any{"file_path": spec, "content": strings.Replace(ready, "ready", "in-progress", 1)}
	code, message, found := Probe("writeSpecWIP", Payload{ToolName: toolWrite, ToolInput: input}, root)
	if !found || code != 2 || !strings.Contains(message, "./le spec claim "+filepath.Base(specFixturePath)) {
		t.Fatalf("worktree transition = %d, found %v; want the native claim: %s", code, found, message)
	}
	if !strings.Contains(message, worktreeLabel+"/"+specFixturePath) {
		t.Fatalf("refusal does not name the worktree's spec: %s", message)
	}
}

// TestSpecDoorGateHoldsAWorktreeSpecToTheWorktreeDoors: a spec in a worktree
// owes the worktree's copies of the doors covering the paths it names.
func TestSpecDoorGateHoldsAWorktreeSpecToTheWorktreeDoors(t *testing.T) {
	root, worktree := sessionWithWorktree(t)
	readDoor(t, root, "sess-a", doorSystem, nil)
	readDoor(t, root, "sess-a", doorBGPEngine, nil)
	spec := filepath.Join(worktree, filepath.FromSlash(specFixturePath))
	edit := specEditPayload("Write", spec, "- `internal/component/bgp/reactor/peer.go`\n")
	requireReadGate(t, root, edit, doorGateRefusal, []string{
		listedDoor(worktreeLabel + "/" + doorSystem),
		listedDoor(worktreeLabel + "/" + doorBGPEngine),
	})
	readCopy(t, root, filepath.Join(worktree, filepath.FromSlash(doorSystem)))
	readCopy(t, root, filepath.Join(worktree, filepath.FromSlash(doorBGPEngine)))
	requireReadGate(t, root, edit, doorGateRefusal, nil)
}
