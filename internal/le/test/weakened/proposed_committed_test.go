package testweakened

import (
	"errors"
	"strings"
	"testing"
)

// committedTag spells the fixture tag as a concatenation, so this file is not
// itself read as a carrier for the fixture requirement.
const committedTag = "// RFC requirement: " + "RFC2119-1-1 positive\n"

// TestProposedRFCLockFollowsTheCommittedTag proves that the owner lock on an
// RFC-tagged unit holds what HEAD records and nothing the author has not yet
// committed. Method: a real Git fixture, then Edit and Write proposals against
// four working-tree shapes the journal class
// plan/journal/guard-blocks-its-own-authors-repair.md records, plus the one
// shape the lock exists for, which MUST still be refused.
// VALIDATES: the owner lock reads its baseline from HEAD.
// PREVENTS: an author locked out of repairing a test HEAD has never recorded.
func TestProposedRFCLockFollowsTheCommittedTag(t *testing.T) {
	root := t.TempDir()
	committedPath := "pkg/rfc_test.go"
	committedText := "package a\n" + committedTag +
		"func TestOld(t *testing.T) { require.Equal(t, 1, got) }\n" +
		"func TestPlain(t *testing.T) { require.Equal(t, 3, got) }\n"
	writeProposedFile(t, root, committedPath, committedText)
	commitProposedFixture(t, root)

	// The shape the lock exists for: a committed tagged unit changed in place,
	// by each of the three tools the hook judges.
	weakened := strings.Replace(committedText, "Equal(t, 1, got)", "Equal(t, 2, got)", 1)
	for _, request := range []ProposedRequest{
		{Path: committedPath, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "Equal(t, 1, got)", NewString: "Equal(t, 2, got)",
		}},
		{Path: committedPath, Tool: "Write", ToolInput: ProposedToolInput{Content: weakened}},
		{Path: committedPath, Tool: "MultiEdit", ToolInput: ProposedToolInput{Edits: []ProposedEdit{
			{OldString: "Equal(t, 3, got)", NewString: "Equal(t, 4, got)"},
			{OldString: "Equal(t, 1, got)", NewString: "Equal(t, 2, got)"},
		}}},
	} {
		refused, err := proposedFixture(root, request)
		if err != nil || refused.ExitCode() != 2 || len(refused.RFCChanges) != 1 ||
			refused.RFCChanges[0].Name != "TestOld" {
			t.Fatalf("a committed tagged unit changed in place by %s = %#v, %v", request.Tool, refused, err)
		}
	}

	// 2026-08-30, 2026-09-04, 2026-09-08: a new untracked file the author is
	// still writing. HEAD holds no record of it, so nothing in it is locked.
	untrackedPath := "pkg/rfc_new_test.go"
	untrackedText := "package a\n" + committedTag +
		"func TestNew(t *testing.T) { require.Equal(t, 1, got) }\n"
	writeProposedFile(t, root, untrackedPath, untrackedText)
	edit, err := proposedFixture(root, ProposedRequest{
		Path: untrackedPath, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "Equal(t, 1, got)", NewString: "Equal(t, 2, got)",
		},
	})
	if err != nil || edit.ExitCode() != 0 || len(edit.RFCChanges) != 0 {
		t.Fatalf("an Edit inside an untracked tagged unit = %#v, %v", edit, err)
	}
	rewrite, err := proposedFixture(root, ProposedRequest{
		Path: untrackedPath, Tool: "Write", ToolInput: ProposedToolInput{
			Content: strings.Replace(untrackedText, committedTag, "", 1),
		},
	})
	if err != nil || rewrite.ExitCode() != 0 || len(rewrite.RFCChanges) != 0 {
		t.Fatalf("a Write dropping an untracked tag = %#v, %v", rewrite, err)
	}

	// 2026-08-10, fixit-isis-hostname-ascii: a tagged unit appended to a
	// tracked file and not yet committed.
	appended := committedText + committedTag +
		"func TestAdded(t *testing.T) { require.Equal(t, 5, got) }\n"
	writeProposedFile(t, root, committedPath, appended)
	added, err := proposedFixture(root, ProposedRequest{
		Path: committedPath, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "Equal(t, 5, got)", NewString: "Equal(t, 6, got)",
		},
	})
	if err != nil || added.ExitCode() != 0 || len(added.RFCChanges) != 0 {
		t.Fatalf("an Edit inside an uncommitted appended unit = %#v, %v", added, err)
	}

	// 2026-08-10, fixit-bgp-shutdown-cease-notification: a tag written into a
	// committed untagged unit before its assertion, then removed again.
	tagged := strings.Replace(committedText,
		"func TestPlain", strings.TrimSuffix(committedTag, "\n")+"\nfunc TestPlain", 1)
	writeProposedFile(t, root, committedPath, tagged)
	retag, err := proposedFixture(root, ProposedRequest{
		Path: committedPath, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "Equal(t, 3, got)", NewString: "Equal(t, 4, got)",
		},
	})
	if err != nil || retag.ExitCode() != 0 || len(retag.RFCChanges) != 0 {
		t.Fatalf("an Edit inside a unit tagged since HEAD = %#v, %v", retag, err)
	}

	// Another session's uncommitted change to the committed tagged unit does
	// not lock an edit that leaves that unit alone.
	foreign := strings.Replace(committedText, "Equal(t, 1, got)", "Equal(t, 1, got); require.NoError(t, err)", 1)
	writeProposedFile(t, root, committedPath, foreign)
	beside, err := proposedFixture(root, ProposedRequest{
		Path: committedPath, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "Equal(t, 3, got)", NewString: "Equal(t, 4, got)",
		},
	})
	if err != nil || beside.ExitCode() != 0 || len(beside.RFCChanges) != 0 {
		t.Fatalf("an Edit beside a foreign change to a tagged unit = %#v, %v", beside, err)
	}

	// The committed tagged unit stays locked whatever the working tree holds.
	again, err := proposedFixture(root, ProposedRequest{
		Path: committedPath, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "Equal(t, 1, got); require.NoError(t, err)", NewString: "Equal(t, 2, got)",
		},
	})
	if err != nil || again.ExitCode() != 2 || len(again.RFCChanges) != 1 ||
		again.RFCChanges[0].Name != "TestOld" {
		t.Fatalf("a committed tagged unit changed after a foreign edit = %#v, %v", again, err)
	}
}

// TestProposedRFCLockSurvivesAFileScopeTagAddedSinceHEAD proves that a tag
// written at file scope since HEAD cannot unlock a committed tagged function.
// VALIDATES: the working-tree text and the HEAD text are cut into units the
// same way, so their changes are matched by one name.
// PREVENTS: a two-step bypass, where the first edit adds a file-scope tag and
// the second weakens the committed unit unseen.
func TestProposedRFCLockSurvivesAFileScopeTagAddedSinceHEAD(t *testing.T) {
	root := t.TempDir()
	path := "pkg/rfc_test.go"
	committedText := "package a\n" + committedTag +
		"func TestOld(t *testing.T) { require.Equal(t, 1, got) }\n"
	writeProposedFile(t, root, path, committedText)
	commitProposedFixture(t, root)

	fixtureTag := "package a\n"
	withFixture := fixtureTag + "\n" + committedTag + "var fixture = 1\n\n"
	first, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", ToolInput: ProposedToolInput{OldString: fixtureTag, NewString: withFixture},
	})
	if err != nil || first.ExitCode() != 0 {
		t.Fatalf("adding a file-scope tagged fixture = %#v, %v", first, err)
	}
	writeProposedFile(t, root, path, strings.Replace(committedText, fixtureTag, withFixture, 1))

	second, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "Equal(t, 1, got)", NewString: "Equal(t, 2, got)",
		},
	})
	if err != nil || second.ExitCode() != 2 || len(second.RFCChanges) != 1 {
		t.Fatalf("weakening the committed unit after a file-scope tag = %#v, %v", second, err)
	}
}

// TestProposedRFCLockHoldsATagRemovedSinceHEAD proves that a tag HEAD records
// still locks its unit after the working tree lost it, by Bash or by another
// session, for a tag on a function and for a tag at file scope.
// VALIDATES: the lock reads the committed tag, not the working-tree one, and
// cuts both texts the way HEAD cuts them when HEAD alone has a file-scope tag.
// PREVENTS: a two-step bypass, where the tag goes first and the weakening
// follows unseen.
func TestProposedRFCLockHoldsATagRemovedSinceHEAD(t *testing.T) {
	body := "func TestOld(t *testing.T) { require.Equal(t, 1, got) }\n"
	for name, committedText := range map[string]string{
		"function":   "package a\n" + committedTag + body,
		"file-scope": "package a\n\n" + committedTag + "var fixture = 1\n\n" + body,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := "pkg/rfc_test.go"
			writeProposedFile(t, root, path, committedText)
			commitProposedFixture(t, root)
			writeProposedFile(t, root, path, strings.Replace(committedText, committedTag, "", 1))

			report, err := proposedFixture(root, ProposedRequest{
				Path: path, Tool: "Edit", ToolInput: ProposedToolInput{
					OldString: "Equal(t, 1, got)", NewString: "Equal(t, 2, got)",
				},
			})
			if err != nil || report.ExitCode() != 2 || len(report.RFCChanges) != 1 {
				t.Fatalf("weakening a unit whose tag left the tree = %#v, %v", report, err)
			}
		})
	}
}

// TestProposedRFCLockFailsClosedWithoutACommittedText proves that a checkout
// whose HEAD cannot be read refuses to judge an edit to a tagged unit.
// VALIDATES: a git failure is an error, which writeWeakening turns into a
// refusal.
// PREVENTS: an unreadable HEAD read as an empty text, which unlocks every
// tagged unit in the file.
func TestProposedRFCLockFailsClosedWithoutACommittedText(t *testing.T) {
	root := t.TempDir()
	path := "pkg/rfc_test.go"
	writeProposedFile(t, root, path, "package a\n"+committedTag+
		"func TestOld(t *testing.T) { require.Equal(t, 1, got) }\n")
	if !runSelfTestGit(root, "init", "-q") {
		t.Fatal("initialize a repository with no commit")
	}
	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "Equal(t, 1, got)", NewString: "Equal(t, 2, got)",
		},
	})
	if !errors.Is(err, errCommittedUnreadable) {
		t.Fatalf("an edit judged with no readable HEAD = %#v, %v, want %v", report, err, errCommittedUnreadable)
	}
}

// commitProposedFixture makes the files under root the HEAD of a new repository.
func commitProposedFixture(t *testing.T, root string) {
	t.Helper()
	if !runSelfTestGit(root, "init", "-q") || !runSelfTestGit(root, "add", "-A") ||
		!runSelfTestGit(root,
			"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "baseline") {
		t.Fatal("initialize the committed fixture")
	}
}
