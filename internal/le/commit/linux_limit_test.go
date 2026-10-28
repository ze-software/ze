// Design: docs/contributing/committing.md -- Linux-limit owner approval
package commit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const linuxLimitSummary = "rfc/short/rfc9999.md"
const linuxLimitRID = "RFC9999-2-1"
const linuxLimitRow = "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2)"
const linuxLimitAnnotation = " {linux-limit: LL-EXAMPLE; Linux cannot provide this primitive}"

// TestALinuxLimitCommitOwesTheOwnersAttestation drives Create, its generated
// commit, push derivation and Create's empty attestation path (AC-15).
func TestALinuxLimitCommitOwesTheOwnersAttestation(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-approval")
	writeCommitFixture(t, root, linuxLimitSummary, linuxLimitRow+linuxLimitAnnotation+"\n")
	prepared, err := Create(root, &Options{Subject: "record a Linux limit", Files: []string{linuxLimitSummary}})
	if err != nil {
		t.Fatal(err)
	}
	want := trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"
	if len(prepared.Trailers) != 1 || prepared.Trailers[0] != want {
		t.Fatalf("trailers %v, want %q", prepared.Trailers, want)
	}
	if output, err := runScript(t, root, prepared.Script); err != nil {
		t.Fatalf("commit failed: %v\n%s", err, output)
	}
	changed := revParse(t, root, "HEAD")
	writeOwedGreen(t, root, changed)
	debt := deriveOwed(t, root, "")
	assertOneItem(t, debt, changed, owedItemLinuxLimit, linuxLimitRID)
	if err := debt.pushRefusal(); err == nil || !strings.Contains(err.Error(), "attest") {
		t.Fatalf("unattested limit did not hold the push: %v", err)
	}
	// An ordinary RFC approval, even naming this RID, cannot approve a limit.
	commitOwed(t, root, "ordinary approval", nil, nil, "RFC-approved: "+linuxLimitRID+": reviewed")
	assertOneItem(t, deriveOwed(t, root, ""), changed, owedItemLinuxLimit, linuxLimitRID)
	attested, err := Create(root, &Options{Subject: "owner approves Linux limit", Attest: changed,
		AttestWords: "I approve this Linux limitation"})
	if err != nil {
		t.Fatal(err)
	}
	if output, err := runScript(t, root, attested.Script); err != nil {
		t.Fatalf("attestation failed: %v\n%s", err, output)
	}
	debt = deriveOwed(t, root, "")
	if err := debt.pushRefusal(); err != nil {
		t.Fatalf("owner attestation did not clear the push: %v", err)
	}
	// The same attestation lies outside a push explicitly targeting changed.
	debt = deriveOwed(t, root, changed)
	assertOneItem(t, debt, changed, owedItemLinuxLimit, linuxLimitRID)
	if debt.pushRefusal() == nil {
		t.Fatal("attestation after the push target answered the earlier range")
	}
}

// TestAnUnchangedLinuxLimitOwesNothing edits summary prose but leaves the
// annotation unchanged, so Create must not repeat its approval debt (AC-16).
func TestAnUnchangedLinuxLimitOwesNothing(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-unchanged")
	base := linuxLimitRow + linuxLimitAnnotation + "\n"
	commitOwed(t, root, "existing limit", map[string]string{linuxLimitSummary: base}, nil)
	writeCommitFixture(t, root, linuxLimitSummary, "# Changed heading\n\n"+base)
	prepared, err := Create(root, &Options{Subject: "edit summary prose", Files: []string{linuxLimitSummary}, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Trailers) != 0 {
		t.Fatalf("unchanged annotation owes %v", prepared.Trailers)
	}
}

// TestLinuxLimitCreateJudgesSelectedBlobs compares HEAD with selected hunks,
// never unselected annotations in the working tree or the shared index.
func TestLinuxLimitCreateJudgesSelectedBlobs(t *testing.T) {
	for _, change := range []string{"added", "reworded", "retargeted", "unchanged", "removed"} {
		t.Run(change, func(t *testing.T) {
			root := newOwedRepository(t)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-hunks")
			base := linuxLimitRow + linuxLimitAnnotation + "\n"
			selected := base
			want := trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"
			switch change {
			case "added":
				base = linuxLimitRow + " {gap: work remains}\n"
			case "reworded":
				selected = strings.ReplaceAll(base, "this primitive", "the required primitive")
			case "retargeted":
				selected = strings.ReplaceAll(base, "LL-EXAMPLE", "LL-OTHER")
				want = trailerLinuxLimitOwed + linuxLimitRID + ": LL-OTHER"
			case "unchanged":
				selected = "# New heading\n\n" + base
				want = ""
			case "removed":
				selected = linuxLimitRow + " {gap: implement another shape}\n"
				want = ""
			}
			commitOwed(t, root, "base summary", map[string]string{linuxLimitSummary: base}, nil)
			patch := writeHunkPatch(t, root, linuxLimitSummary, selected,
				linuxLimitRow+" {linux-limit: LL-UNSELECTED; another session's annotation}\n")
			// The index is another session's state too.
			runCommitGit(t, root, "add", "--", linuxLimitSummary)
			prepared, err := Create(root, &Options{Subject: "selected summary change", DryRun: true,
				Hunks: []HunkSelection{{Path: linuxLimitSummary, Patch: patch}}})
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(prepared.Trailers, "\n"); got != want {
				t.Fatalf("selected %s trailers %q, want %q", change, got, want)
			}
		})
	}
}

// TestLinuxLimitDeclaredTrailersFailClosed rejects malformed obligations and
// answers, and prevents an author from injecting the automatic trailer.
func TestLinuxLimitDeclaredTrailersFailClosed(t *testing.T) {
	for _, line := range []string{
		"Linux-limit-owed:", "Linux-limit-owed: ", "Linux-limit-owed: " + linuxLimitRID,
		"Linux-limit-owed: : LL-EXAMPLE", "Linux-limit-owed: " + linuxLimitRID + ": ",
		"Linux-limit-owed: " + linuxLimitRID + ": not-a-limit",
		"Linux-limit-owed: " + linuxLimitRID + ": LL-EXAMPLE extra",
		"Owner-attested: patch-id: ", "RFC-approved: unit: ",
	} {
		t.Run(line, func(t *testing.T) {
			acts := parsePersonActs("\x1ecommit\x00subject\n\n" + line)
			if len(acts.problems) == 0 {
				t.Fatalf("malformed trailer %q was ignored", line)
			}
		})
	}
	for _, line := range []string{"Linux-limit-owed:", trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"} {
		if _, err := Message("subject", []string{line}, nil); err == nil {
			t.Fatalf("handwritten trailer %q accepted", line)
		}
	}
}

// TestLinuxLimitCreateBillsEachChangedRow compares multiple annotations in one
// snapshot, preserving an unchanged row while billing a reworded and a new row.
func TestLinuxLimitCreateBillsEachChangedRow(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-rows")
	first := linuxLimitRow + linuxLimitAnnotation + "\n"
	second := strings.ReplaceAll(first, linuxLimitRID, "RFC9999-2-2")
	third := strings.ReplaceAll(first, linuxLimitRID, "RFC9999-2-3")
	commitOwed(t, root, "existing rows", map[string]string{linuxLimitSummary: first + second}, nil)
	writeCommitFixture(t, root, linuxLimitSummary, first+
		strings.ReplaceAll(second, "this primitive", "the other primitive")+third)
	prepared, err := Create(root, &Options{Subject: "record changed limits", Files: []string{linuxLimitSummary}, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	want := trailerLinuxLimitOwed + "RFC9999-2-2: LL-EXAMPLE\n" +
		trailerLinuxLimitOwed + "RFC9999-2-3: LL-EXAMPLE"
	if got := strings.Join(prepared.Trailers, "\n"); got != want {
		t.Fatalf("trailers %q, want %q", got, want)
	}
}

// TestLinuxLimitCreateRefusesMalformedAnnotations ensures the canonical parser
// refuses an unreadable claim rather than preparing a commit without its debt.
func TestLinuxLimitCreateRefusesMalformedAnnotations(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-malformed")
	writeCommitFixture(t, root, linuxLimitSummary, linuxLimitRow+" {linux-limit: LL-EXAMPLE; }\n")
	if _, err := Create(root, &Options{Subject: "malformed limit", Files: []string{linuxLimitSummary}, DryRun: true}); err == nil {
		t.Fatal("malformed Linux-limit annotation was committed without approval debt")
	}
}

// TestLinuxLimitMalformedDebtCannotBeAttested proves malformed declarations
// remain a push refusal even beside a valid owner answer, and Create refuses
// to generate an attestation over a message it cannot read.
func TestLinuxLimitMalformedDebtCannotBeAttested(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-malformed-debt")
	changed := commitOwed(t, root, "malformed debt", map[string]string{"a.txt": "a\n"}, nil,
		trailerLinuxLimitOwed+linuxLimitRID+": not-a-limit")
	if _, err := Create(root, &Options{Subject: "attest malformed debt", Attest: changed,
		AttestWords: "I approve", DryRun: true}); err == nil {
		t.Fatal("attest accepted an unreadable obligation")
	}
	attested := commitOwed(t, root, "owner answer", nil, nil,
		trailerOwnerAttested+stablePatchID(t, root, changed)+": I approve")
	writeOwedGreen(t, root, attested)
	debt := deriveOwed(t, root, "")
	if err := debt.pushRefusal(); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed obligation was discharged: %v", err)
	}
}

// TestLinuxLimitCreateRefusesSelectedSymlinkSummary exercises Create's snapshot
// of a new link and a regular summary replaced by a link to an annotated text file.
func TestLinuxLimitCreateRefusesSelectedSymlinkSummary(t *testing.T) {
	for _, prior := range []string{"absent", "regular"} {
		t.Run(prior, func(t *testing.T) {
			root := newOwedRepository(t)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-selected-link")
			if prior == "regular" {
				commitOwed(t, root, "regular summary", map[string]string{
					linuxLimitSummary: linuxLimitRow + " {gap: work remains}\n",
				}, nil)
				if err := os.Remove(filepath.Join(root, linuxLimitSummary)); err != nil {
					t.Fatal(err)
				}
			}
			const target = "docs/summary.txt"
			writeCommitFixture(t, root, target, linuxLimitRow+linuxLimitAnnotation+"\n")
			writeLinuxLimitSymlink(t, root, "../../"+target)
			_, err := Create(root, &Options{Subject: "linked summary", DryRun: true,
				Files: []string{linuxLimitSummary, target}})
			assertLinuxLimitSymlinkRefusal(t, err)
		})
	}
}

// TestLinuxLimitCreateRefusesExistingSymlinkTargetChanges proves target-only
// additions, rewordings and retargetings cannot bypass approval through a HEAD
// link, even when the unselected checkout and shared index hold a regular summary.
func TestLinuxLimitCreateRefusesExistingSymlinkTargetChanges(t *testing.T) {
	for _, change := range []string{"added", "reworded", "retargeted"} {
		for _, selection := range []string{"file", "hunks"} {
			t.Run(change+"/"+selection, func(t *testing.T) {
				root := newOwedRepository(t)
				t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-existing-link")
				const target = "docs/summary.txt"
				before := linuxLimitRow + linuxLimitAnnotation + "\n"
				after := before
				switch change {
				case "added":
					before = linuxLimitRow + " {gap: work remains}\n"
				case "reworded":
					after = strings.ReplaceAll(before, "this primitive", "the required primitive")
				case "retargeted":
					after = strings.ReplaceAll(before, "LL-EXAMPLE", "LL-OTHER")
				}
				writeCommitFixture(t, root, target, before)
				writeLinuxLimitSymlink(t, root, "../../"+target)
				runCommitGit(t, root, "add", "--", linuxLimitSummary)
				commitOwed(t, root, "legacy linked summary", map[string]string{target: before}, nil)
				if err := os.Remove(filepath.Join(root, linuxLimitSummary)); err != nil {
					t.Fatal(err)
				}
				writeCommitFixture(t, root, linuxLimitSummary, linuxLimitRow+" {gap: unselected repair}\n")
				runCommitGit(t, root, "add", "--", linuxLimitSummary)
				options := &Options{Subject: "change only the summary target", DryRun: true}
				switch selection {
				case "file":
					writeCommitFixture(t, root, target, after)
					options.Files = []string{target}
				case "hunks":
					patch := writeHunkPatch(t, root, target, after, before)
					options.Hunks = []HunkSelection{{Path: target, Patch: patch}}
				}
				_, err := Create(root, options)
				assertLinuxLimitSymlinkRefusal(t, err)
			})
		}
	}
}

// TestLinuxLimitCreateRepairsLegacySymlinkSummary permits removing the link or
// replacing it with either regular Git mode, billing the replacement's annotation.
func TestLinuxLimitCreateRepairsLegacySymlinkSummary(t *testing.T) {
	for _, repair := range []string{"remove", "regular", "executable"} {
		t.Run(repair, func(t *testing.T) {
			root := newOwedRepository(t)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-repair-link")
			const target = "docs/summary.txt"
			body := linuxLimitRow + linuxLimitAnnotation + "\n"
			writeCommitFixture(t, root, target, body)
			writeLinuxLimitSymlink(t, root, "../../"+target)
			runCommitGit(t, root, "add", "--", linuxLimitSummary)
			commitOwed(t, root, "legacy linked summary", map[string]string{target: body}, nil)
			options := &Options{Subject: "repair linked summary", DryRun: true}
			want := ""
			if repair == "remove" {
				options.Remove = []string{linuxLimitSummary}
			} else {
				if err := os.Remove(filepath.Join(root, linuxLimitSummary)); err != nil {
					t.Fatal(err)
				}
				writeCommitFixture(t, root, linuxLimitSummary, body)
				if repair == "executable" {
					if err := os.Chmod(filepath.Join(root, linuxLimitSummary), 0o750); err != nil {
						t.Fatal(err)
					}
				}
				options.Files = []string{linuxLimitSummary}
				want = trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"
			}
			prepared, err := Create(root, options)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(prepared.Trailers, "\n"); got != want {
				t.Fatalf("repair %s trailers %q, want %q", repair, got, want)
			}
		})
	}
}

// TestLinuxLimitCreateDoesNotTreatLinkTextAsPriorApproval replaces a legacy
// link whose target string itself looks like a row. Only a regular HEAD blob
// can establish an unchanged annotation; link text cannot suppress the debt.
func TestLinuxLimitCreateDoesNotTreatLinkTextAsPriorApproval(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-link-baseline")
	body := linuxLimitRow + linuxLimitAnnotation
	writeLinuxLimitSymlink(t, root, body)
	runCommitGit(t, root, "add", "--", linuxLimitSummary)
	commitOwed(t, root, "legacy link text", nil, nil)
	if err := os.Remove(filepath.Join(root, linuxLimitSummary)); err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, linuxLimitSummary, body+"\n")
	prepared, err := Create(root, &Options{Subject: "materialize summary", DryRun: true,
		Files: []string{linuxLimitSummary}})
	if err != nil {
		t.Fatal(err)
	}
	want := trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"
	if got := strings.Join(prepared.Trailers, "\n"); got != want {
		t.Fatalf("regular replacement trailers %q, want %q", got, want)
	}
}

// TestLinuxLimitCreateIgnoresUnselectedSymlinkMode judges regular HEAD and hunk
// entries rather than the symlink another session left in the checkout and index.
func TestLinuxLimitCreateIgnoresUnselectedSymlinkMode(t *testing.T) {
	for _, selection := range []string{"unselected", "hunks"} {
		t.Run(selection, func(t *testing.T) {
			root := newOwedRepository(t)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-unselected-link")
			base := linuxLimitRow + " {gap: work remains}\n"
			commitOwed(t, root, "regular summary", map[string]string{linuxLimitSummary: base}, nil)
			patch := writeHunkPatch(t, root, linuxLimitSummary, linuxLimitRow+linuxLimitAnnotation+"\n", base)
			if err := os.Remove(filepath.Join(root, linuxLimitSummary)); err != nil {
				t.Fatal(err)
			}
			const target = "docs/summary.txt"
			writeCommitFixture(t, root, target, linuxLimitRow+linuxLimitAnnotation+"\n")
			writeLinuxLimitSymlink(t, root, "../../"+target)
			runCommitGit(t, root, "add", "--", linuxLimitSummary)
			options := &Options{Subject: "regular prospective summary", DryRun: true}
			want := ""
			switch selection {
			case "unselected":
				options.Files = []string{target}
			case "hunks":
				options.Hunks = []HunkSelection{{Path: linuxLimitSummary, Patch: patch}}
				want = trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"
			}
			prepared, err := Create(root, options)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(prepared.Trailers, "\n"); got != want {
				t.Fatalf("selection %s trailers %q, want %q", selection, got, want)
			}
		})
	}
}

func writeLinuxLimitSymlink(t *testing.T, root, target string) {
	t.Helper()
	name := filepath.Join(root, linuxLimitSummary)
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
}

func assertLinuxLimitSymlinkRefusal(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("symlink summary was accepted without approval debt")
	}
	for _, want := range []string{"Linux-limit approval", linuxLimitSummary, "120000", "regular file", "remove"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q lacks %q", err, want)
		}
	}
}

// TestLinuxLimitCreateRefusesLinkedAncestorTargetChanges judges target-only
// file and hunk changes against both committed ancestors, even when unselected
// checkout directories already contain a regular replacement summary.
func TestLinuxLimitCreateRefusesLinkedAncestorTargetChanges(t *testing.T) {
	for _, ancestor := range []string{"rfc", "rfc/short"} {
		for _, selection := range []string{"file", "hunks"} {
			for _, change := range []string{"added", "reworded", "retargeted"} {
				t.Run(ancestor+"/"+selection+"/"+change, func(t *testing.T) {
					root := newOwedRepository(t)
					t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-ancestor-target")
					const target = "docs/short/rfc9999.md"
					before := linuxLimitRow + linuxLimitAnnotation + "\n"
					after := before
					switch change {
					case "added":
						before = linuxLimitRow + " {gap: work remains}\n"
					case "reworded":
						after = strings.ReplaceAll(before, "this primitive", "the required primitive")
					case "retargeted":
						after = strings.ReplaceAll(before, "LL-EXAMPLE", "LL-OTHER")
					}
					writeCommitFixture(t, root, target, before)
					writeLinuxLimitAncestorSymlink(t, root, ancestor)
					runCommitGit(t, root, "add", "--", ancestor)
					commitOwed(t, root, "legacy linked ancestor", map[string]string{target: before}, nil)
					if err := os.Remove(filepath.Join(root, ancestor)); err != nil {
						t.Fatal(err)
					}
					writeCommitFixture(t, root, linuxLimitSummary, before)
					options := &Options{Subject: "change only ancestor target", DryRun: true}
					switch selection {
					case "file":
						writeCommitFixture(t, root, target, after)
						options.Files = []string{target}
					case "hunks":
						patch := writeHunkPatch(t, root, target, after, before)
						options.Hunks = []HunkSelection{{Path: target, Patch: patch}}
					}
					_, err := Create(root, options)
					assertLinuxLimitAncestorRefusal(t, ancestor, err)
				})
			}
		}
	}
}

// TestLinuxLimitCreateRefusesSelectedAncestorSymlink selects a mode-120000
// ancestor through hunks while HEAD and the checkout hold regular files. The
// selected mode, not either unselected representation, must refuse approval.
func TestLinuxLimitCreateRefusesSelectedAncestorSymlink(t *testing.T) {
	for _, ancestor := range []string{"rfc", "rfc/short"} {
		t.Run(ancestor, func(t *testing.T) {
			root := newOwedRepository(t)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-ancestor-hunks")
			commitOwed(t, root, "regular ancestor file", map[string]string{
				ancestor:                "regular ancestor file\n",
				"docs/short/rfc9999.md": linuxLimitRow + linuxLimitAnnotation + "\n",
			}, nil)
			if err := os.Remove(filepath.Join(root, ancestor)); err != nil {
				t.Fatal(err)
			}
			writeLinuxLimitAncestorSymlink(t, root, ancestor)
			writeCommitFixture(t, root, "tmp/ancestor.patch", runCommitGitOutput(t, root, "diff", "--", ancestor))
			if err := os.Remove(filepath.Join(root, ancestor)); err != nil {
				t.Fatal(err)
			}
			writeCommitFixture(t, root, ancestor, "unselected regular ancestor\n")
			_, err := Create(root, &Options{Subject: "selected linked ancestor", DryRun: true,
				Hunks: []HunkSelection{{Path: ancestor, Patch: filepath.Join(root, "tmp/ancestor.patch")}}})
			assertLinuxLimitAncestorRefusal(t, ancestor, err)
		})
	}
}

// TestLinuxLimitCreateRemovesLinkedAncestor permits removing either committed
// link, without demanding approval for annotations outside the resulting tree.
func TestLinuxLimitCreateRemovesLinkedAncestor(t *testing.T) {
	for _, ancestor := range []string{"rfc", "rfc/short"} {
		t.Run(ancestor, func(t *testing.T) {
			root := newOwedRepository(t)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-ancestor-removal")
			const target = "docs/short/rfc9999.md"
			body := linuxLimitRow + linuxLimitAnnotation + "\n"
			writeCommitFixture(t, root, target, body)
			writeLinuxLimitAncestorSymlink(t, root, ancestor)
			runCommitGit(t, root, "add", "--", ancestor)
			commitOwed(t, root, "legacy linked ancestor", map[string]string{target: body}, nil)
			prepared, err := Create(root, &Options{Subject: "remove linked ancestor", DryRun: true,
				Remove: []string{ancestor}})
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Trailers) != 0 {
				t.Fatalf("removal owes %v", prepared.Trailers)
			}
		})
	}
}

// TestLinuxLimitApprovalMaterializesAncestorTrees exercises the approval
// producer with exact immutable replacement entries and removals. Descendants
// establish real tree ancestors, and every newly materialized annotation owes.
func TestLinuxLimitApprovalMaterializesAncestorTrees(t *testing.T) {
	for _, ancestor := range []string{"rfc", "rfc/short"} {
		t.Run(ancestor, func(t *testing.T) {
			root := newOwedRepository(t)
			const target = "docs/short/rfc9999.md"
			body := linuxLimitRow + linuxLimitAnnotation + "\n"
			writeCommitFixture(t, root, target, body)
			writeLinuxLimitAncestorSymlink(t, root, ancestor)
			runCommitGit(t, root, "add", "--", ancestor)
			head := commitOwed(t, root, "legacy linked ancestor", map[string]string{target: body}, nil)
			if err := os.Remove(filepath.Join(root, ancestor)); err != nil {
				t.Fatal(err)
			}
			writeCommitFixture(t, root, linuxLimitSummary, body)
			if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
				t.Fatal(err)
			}
			entries, err := snapshotIndexEntries(root, []string{linuxLimitSummary})
			if err != nil {
				t.Fatal(err)
			}
			trailers, err := linuxLimitTrailers(root, head, entries, []string{ancestor})
			if err != nil {
				t.Fatal(err)
			}
			want := trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"
			if got := strings.Join(trailers, "\n"); got != want {
				t.Fatalf("materialized summary trailers %q, want %q", got, want)
			}
		})
	}
}

// TestLinuxLimitCreateIgnoresUnselectedAncestorLinks keeps the committed tree
// ancestors while another session links the checkout elsewhere. Target-only
// edits owe nothing; a selected regular summary hunk still owes its annotation.
func TestLinuxLimitCreateIgnoresUnselectedAncestorLinks(t *testing.T) {
	for _, ancestor := range []string{"rfc", "rfc/short"} {
		for _, selection := range []string{"unselected", "hunks"} {
			t.Run(ancestor+"/"+selection, func(t *testing.T) {
				root := newOwedRepository(t)
				t.Setenv("CLAUDE_CODE_SESSION_ID", "linux-limit-unselected-ancestor")
				const target = "docs/short/rfc9999.md"
				before := linuxLimitRow + " {gap: work remains}\n"
				after := linuxLimitRow + linuxLimitAnnotation + "\n"
				commitOwed(t, root, "real summary directories", map[string]string{
					linuxLimitSummary: before, target: before,
				}, nil)
				patch := writeHunkPatch(t, root, linuxLimitSummary, after, before)
				if err := os.Rename(filepath.Join(root, ancestor), filepath.Join(root, "tmp/saved-ancestor")); err != nil {
					t.Fatal(err)
				}
				writeLinuxLimitAncestorSymlink(t, root, ancestor)
				options := &Options{Subject: "keep recorded summary directories", DryRun: true}
				want := ""
				switch selection {
				case "unselected":
					writeCommitFixture(t, root, target, after)
					options.Files = []string{target}
				case "hunks":
					options.Hunks = []HunkSelection{{Path: linuxLimitSummary, Patch: patch}}
					want = trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"
				}
				prepared, err := Create(root, options)
				if err != nil {
					t.Fatal(err)
				}
				if got := strings.Join(prepared.Trailers, "\n"); got != want {
					t.Fatalf("selection %s trailers %q, want %q", selection, got, want)
				}
			})
		}
	}
}

func writeLinuxLimitAncestorSymlink(t *testing.T, root, ancestor string) {
	t.Helper()
	target := "docs"
	if ancestor == "rfc/short" {
		target = "../docs/short"
	}
	name := filepath.Join(root, ancestor)
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
}

func assertLinuxLimitAncestorRefusal(t *testing.T, ancestor string, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("linked summary ancestor was accepted without approval debt")
	}
	for _, want := range []string{"Linux-limit approval", ancestor, "120000", "directory", "remove"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q lacks %q", err, want)
		}
	}
}
