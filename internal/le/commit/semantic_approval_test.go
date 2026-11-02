// Design: docs/architecture/testing/test-health.md -- semantic approvals use the selected test units.
// Related: ledger_test.go -- approval fixtures and structural weakening cases.
package commit

import (
	"strings"
	"testing"

	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

// TestCreateCarriesSemanticApproval proves that a judge's verdict survives a
// change the structural detector cannot see, and is consumed only on publication.
func TestCreateCarriesSemanticApproval(t *testing.T) {
	for _, replacement := range []string{
		"require.Equal(t, 2, got)",
		"require.Equal(t, 2, got)\n\trequire.Equal(t, 3, other)",
	} {
		t.Run(replacement, func(t *testing.T) {
			root := newLedgerRepository(t)
			const path = "pkg/one_test.go"
			writeCommitFixture(t, root, path, strings.Replace(ledgerTestBody("TestOne"),
				"require.Equal(t, 1, got)", replacement, 1))
			const reason = "independent judge approved the corrected expectation"
			if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", reason); err != nil {
				t.Fatal(err)
			}
			if _, err := testweakened.Approve(root, sessionOne, "pkg.TestTwo", "unused verdict"); err != nil {
				t.Fatal(err)
			}
			approvals := testweakened.ApprovalPath(sessionOne)
			before := readLedgerFile(t, root, approvals)
			prepared, err := ledgerCreate(t, root, path)
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Weakened) != 0 {
				t.Fatalf("fixture unexpectedly has structural findings: %v", prepared.Weakened)
			}
			trailer := testweakened.ApprovalTrailerLine("pkg.TestOne", reason)
			message := readLedgerFile(t, root, prepared.Message)
			if strings.Count(message, trailer) != 1 {
				t.Fatalf("semantic approval missing or duplicated in message:\n%s", message)
			}
			if strings.Contains(message, "pkg.TestTwo") {
				t.Fatalf("unselected unit's approval entered message:\n%s", message)
			}
			if got := readLedgerFile(t, root, approvals); got != before {
				t.Fatalf("Create changed approval file:\n%s", got)
			}
			runCommitScript(t, root, prepared.Script)
			if body := runCommitGitOutput(t, root, "log", "-1", "--format=%B"); strings.Count(body, trailer) != 1 {
				t.Fatalf("landed message lacks exact verdict:\n%s", body)
			}
			after := readLedgerFile(t, root, approvals)
			if strings.Contains(after, "| pkg.TestOne |") || !strings.Contains(after, "| pkg.TestTwo | unused verdict |") {
				t.Fatalf("script did not prune only the used verdict:\n%s", after)
			}
		})
	}
}

// TestCreateSemanticApprovalSelection proves that unchanged units, unselected
// hunks and other sessions cannot supply a verdict or lose their approval rows.
func TestCreateSemanticApprovalSelection(t *testing.T) {
	for _, mode := range []string{"unchanged unit", "excluded hunk", "foreign session", "no approval"} {
		t.Run(mode, func(t *testing.T) {
			root := newLedgerRepository(t)
			const path = "pkg/semantic_test.go"
			head := ledgerTestBody("TestSemantic") + "\nfunc TestOther(t *testing.T) {\n\trequire.Equal(t, 1, other)\n}\n"
			commitFixture(t, root, "semantic baseline", map[string]string{path: head}, nil)
			changed := strings.Replace(head, "1, got", "2, got", 1)
			options := &Options{Session: sessionOne, Subject: "selected test change", Files: []string{path}}
			session := sessionOne
			switch mode {
			case "unchanged unit":
				writeCommitFixture(t, root, path, strings.Replace(head, "1, other", "2, other", 1))
			case "excluded hunk":
				patch := writeHunkPatch(t, root, path, head+"\n// Selected file comment.\n", changed+"\n// Selected file comment.\n")
				options.Files = nil
				options.Hunks = []HunkSelection{{Path: path, Patch: patch}}
			case "foreign session":
				session = sessionTwo
				writeCommitFixture(t, root, path, changed)
			case "no approval":
				writeCommitFixture(t, root, path, changed)
			default:
				t.Fatalf("unknown fixture %q", mode)
			}
			before := ""
			if mode != "no approval" {
				if _, err := testweakened.Approve(root, session, "pkg.TestSemantic", "unused verdict"); err != nil {
					t.Fatal(err)
				}
				before = readLedgerFile(t, root, testweakened.ApprovalPath(session))
			}
			prepared, err := Create(root, options)
			if err != nil {
				t.Fatal(err)
			}
			if message := readLedgerFile(t, root, prepared.Message); strings.Contains(message, testweakened.ApprovalTrailer) {
				t.Fatalf("unused verdict entered message:\n%s", message)
			}
			runCommitScript(t, root, prepared.Script)
			if mode != "no approval" {
				if after := readLedgerFile(t, root, testweakened.ApprovalPath(session)); after != before {
					t.Fatalf("unused approval changed:\n%s", after)
				}
			}
		})
	}
}

// TestCreateSemanticApprovalPreservesOneLineUnit proves that adding a test after
// an unchanged inline body cannot publish or consume its reserved verdict.
func TestCreateSemanticApprovalPreservesOneLineUnit(t *testing.T) {
	root := newLedgerRepository(t)
	const path = "pkg/inline_test.go"
	const baseline = "package pkg\nfunc TestA(t *testing.T) { require.Equal(t, 1, got) }\n"
	commitFixture(t, root, "inline baseline", map[string]string{path: baseline}, nil)
	const added = "\nfunc TestAdded(t *testing.T) { require.Equal(t, 1, added) }\n"
	writeCommitFixture(t, root, path, baseline+added)
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestA", "reserved verdict"); err != nil {
		t.Fatal(err)
	}
	approvals := testweakened.ApprovalPath(sessionOne)
	before := readLedgerFile(t, root, approvals)
	prepared, err := ledgerCreate(t, root, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Weakened) != 0 {
		t.Fatalf("addition produced structural findings: %v", prepared.Weakened)
	}
	if message := readLedgerFile(t, root, prepared.Message); strings.Contains(message, testweakened.ApprovalTrailer) {
		t.Errorf("unchanged TestA improperly selected its reserved approval:\n%s", message)
	}
	if got := readLedgerFile(t, root, approvals); got != before {
		t.Fatalf("preparation changed reserved approval: %q", got)
	}
	runCommitScript(t, root, prepared.Script)
	if message := runCommitGitOutput(t, root, "log", "-1", "--format=%B"); strings.Contains(message, testweakened.ApprovalTrailer) {
		t.Errorf("publication carried unchanged TestA's reserved approval:\n%s", message)
	}
	if got := readLedgerFile(t, root, approvals); got != before {
		t.Errorf("publication consumed unchanged TestA's reserved approval: before %q, after %q", before, got)
	}
	if got := runCommitGitOutput(t, root, "show", "HEAD:"+path); got != baseline+added {
		t.Fatalf("published test content = %q, want %q", got, baseline+added)
	}
}

// TestCreateSemanticApprovalAfterBodylessDeclaration proves native selection
// continues past a valid external declaration to the changed existing test.
func TestCreateSemanticApprovalAfterBodylessDeclaration(t *testing.T) {
	root := newLedgerRepository(t)
	const path = "pkg/external_test.go"
	const baseline = "package pkg\nfunc external()\nfunc TestA(t *testing.T) { require.Equal(t, 1, got) }\n"
	commitFixture(t, root, "bodyless baseline", map[string]string{path: baseline}, nil)
	writeCommitFixture(t, root, path, strings.Replace(baseline, "1, got", "2, got", 1))
	const reason = "independent judge approved the corrected expectation"
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestA", reason); err != nil {
		t.Fatal(err)
	}
	prepared, err := ledgerCreate(t, root, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Weakened) != 0 {
		t.Fatalf("semantic edit produced structural findings: %v", prepared.Weakened)
	}
	trailer := testweakened.ApprovalTrailerLine("pkg.TestA", reason)
	if message := readLedgerFile(t, root, prepared.Message); strings.Count(message, trailer) != 1 {
		t.Fatalf("later test approval not selected exactly once:\n%s", message)
	}
}

// TestCreateSemanticApprovalAlreadyCarried proves that a failed prune cannot
// let the same verdict enter another commit that changes the same unit.
func TestCreateSemanticApprovalAlreadyCarried(t *testing.T) {
	root := newLedgerRepository(t)
	const path = "pkg/one_test.go"
	weakenLedgerTest(t, root, path, "TestOne")
	const reason = "independent verdict already carried"
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", reason); err != nil {
		t.Fatal(err)
	}
	first, err := ledgerCreate(t, root, path)
	if err != nil {
		t.Fatal(err)
	}
	runCommitScript(t, root, first.Script)
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", reason); err != nil {
		t.Fatal(err)
	}
	before := readLedgerFile(t, root, path)
	writeCommitFixture(t, root, path, strings.Replace(before, "1, got", "2, got", 1))
	second, err := ledgerCreate(t, root, path)
	if err != nil {
		t.Fatal(err)
	}
	if message := readLedgerFile(t, root, second.Message); strings.Contains(message, testweakened.ApprovalTrailer) {
		t.Fatalf("already-carried verdict reused:\n%s", message)
	}
}

// TestCreateSemanticApprovalAmbiguousUnit proves semantic edits cannot bypass
// the existing refusal of one approval naming units in two directories.
func TestCreateSemanticApprovalAmbiguousUnit(t *testing.T) {
	root := newLedgerRepository(t)
	paths := []string{"x/pkg/one_test.go", "y/pkg/one_test.go"}
	commitFixture(t, root, "ambiguous baseline", map[string]string{
		paths[0]: ledgerTestBody("TestOne"), paths[1]: ledgerTestBody("TestOne"),
	}, nil)
	for _, path := range paths {
		writeCommitFixture(t, root, path, strings.Replace(ledgerTestBody("TestOne"), "1, got", "2, got", 1))
	}
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", "ambiguous verdict"); err != nil {
		t.Fatal(err)
	}
	_, err := ledgerCreate(t, root, paths...)
	if err == nil {
		t.Fatal("one semantic approval admitted two directory units")
	}
	for _, want := range []string{"pkg.TestOne", "x/pkg", "y/pkg", "split the commit"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal lacks %q: %v", want, err)
		}
	}
}

// TestCreateSemanticApprovalSelectedHunk proves a selected semantic change
// carries its verdict while a structural weakening left in the checkout does not.
func TestCreateSemanticApprovalSelectedHunk(t *testing.T) {
	root := newLedgerRepository(t)
	const path = "pkg/one_test.go"
	head := ledgerTestBody("TestOne") + "\n// " + strings.ReplaceAll(strings.TrimSpace(hunkPadding), "\n", "\n// ") +
		"\nfunc TestOther(t *testing.T) {\n\trequire.Equal(t, 1, other)\n}\n"
	commitFixture(t, root, "hunk baseline", map[string]string{path: head}, nil)
	mine := strings.Replace(head, "1, got", "2, got", 1)
	both := strings.Replace(mine, "require.Equal(t, 1, other)", "t.Skip(\"unselected\")\n\trequire.Equal(t, 1, other)", 1)
	patch := writeHunkPatch(t, root, path, mine, both)
	const reason = "independent verdict for the selected expectation"
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", reason); err != nil {
		t.Fatal(err)
	}
	prepared, err := Create(root, &Options{
		Session: sessionOne, Subject: "selected semantic change",
		Hunks: []HunkSelection{{Path: path, Patch: patch}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Weakened) != 0 {
		t.Fatalf("unselected weakening reached gate: %v", prepared.Weakened)
	}
	trailer := testweakened.ApprovalTrailerLine("pkg.TestOne", reason)
	if message := readLedgerFile(t, root, prepared.Message); strings.Count(message, trailer) != 1 {
		t.Fatalf("selected semantic verdict missing or duplicated:\n%s", message)
	}
	runCommitScript(t, root, prepared.Script)
	if got := runCommitGitOutput(t, root, "show", "HEAD:"+path); got != mine {
		t.Fatalf("committed content = %q, want selected content %q", got, mine)
	}
	if got := readLedgerFile(t, root, path); got != both {
		t.Fatalf("unselected working-tree content changed: %q", got)
	}
}

// TestCreateSemanticApprovalRename proves a paired move compares unit text,
// rather than spending an approval just because the path changes.
func TestCreateSemanticApprovalRename(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "changed"}[changed], func(t *testing.T) {
			root := newLedgerRepository(t)
			const oldPath, newPath = "a/pkg/moved_test.go", "b/pkg/moved_test.go"
			head := ledgerTestBody("TestMoved") + "\n// " + strings.ReplaceAll(strings.TrimSpace(hunkPadding), "\n", "\n// ") + "\n"
			commitFixture(t, root, "rename baseline", map[string]string{oldPath: head}, nil)
			after := head
			if changed {
				after = strings.Replace(head, "1, got", "2, got", 1)
			}
			writeCommitFixture(t, root, newPath, after)
			const reason = "independent verdict for moved test"
			if _, err := testweakened.Approve(root, sessionOne, "pkg.TestMoved", reason); err != nil {
				t.Fatal(err)
			}
			prepared, err := Create(root, &Options{
				Session: sessionOne, Subject: "move test", Files: []string{newPath}, Remove: []string{oldPath},
			})
			if err != nil {
				t.Fatal(err)
			}
			trailer := testweakened.ApprovalTrailerLine("pkg.TestMoved", reason)
			message := readLedgerFile(t, root, prepared.Message)
			if strings.Contains(message, trailer) != changed {
				t.Fatalf("changed=%v, wrong approval selection:\n%s", changed, message)
			}
		})
	}
}
