// Design: docs/contributing/rfc-implementation-guide.md -- request-local canonical approval sources.
package rfc

import (
	"strings"
	"testing"
)

// TestApprovalRequestPinsBaseAndStartsFresh proves one request's unread
// package still uses its initial immutable HEAD, while a new request observes
// a later base. Method: resolve one package, advance real Git history for a
// different package, and compare its old and fresh Content digests.
func TestApprovalRequestPinsBaseAndStartsFresh(t *testing.T) {
	const first = "package pkg\nfunc TestA(t *testing.T) { check(1) }\n"
	const second = "package other\nfunc TestB(t *testing.T) { check(1) }\n"
	const moved = "package other\nfunc TestB(t *testing.T) { check(3) }\n"
	root, parent := fixtureCommit(t, map[string]string{
		"pkg/first_test.go": first, "other/second_test.go": second,
	})
	writeApprovalIdentityFixture(t, root, "pkg/first_test.go", strings.ReplaceAll(first, "check(1)", "check(2)"))
	writeApprovalIdentityFixture(t, root, "other/second_test.go", strings.ReplaceAll(second, "check(1)", "check(2)"))
	want, err := ResolveApprovalUnitContent(root, "", "other.TestB")
	if err != nil {
		t.Fatal(err)
	}
	request := NewApprovalResolver(root, "")
	if _, err := request.Resolve("pkg.TestA", ""); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "", "read-tree", "HEAD")
	blob := fixtureGit(t, root, moved, "hash-object", "-w", "--stdin")
	fixtureGit(t, root, "", "update-index", "--cacheinfo", "100644", blob, "other/second_test.go")
	tree := fixtureGit(t, root, "", "write-tree")
	tip := fixtureGit(t, root, "new base\n", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test",
		"commit-tree", tree, "-p", parent)
	fixtureGit(t, root, "", "update-ref", "HEAD", tip, parent)
	got, err := request.Resolve("other.TestB", "")
	if err != nil || got != want {
		t.Fatalf("same request after HEAD moved = %+v, %v; want original source view %+v", got, err, want)
	}
	fresh, err := NewApprovalResolver(root, "").Resolve("other.TestB", "")
	if err != nil || fresh.Path != want.Path || fresh.Current != want.Current || fresh.Base == want.Base {
		t.Fatalf("fresh request = %+v, %v; want the new base with the same current declaration", fresh, err)
	}
}

// TestApprovalRequestPreservesMalformedNameBoundaries proves cached parsing
// does not turn a malformed file into an unconditional failure or a successful
// matched declaration. Method: ask one real malformed carrier for a missing
// name, its found inexact declaration, and its existing whole-file stem.
func TestApprovalRequestPreservesMalformedNameBoundaries(t *testing.T) {
	root := t.TempDir()
	const path = "pkg/broken_test.go"
	writeApprovalIdentityFixture(t, root, path, "package pkg\nfunc TestBroken(t *testing.T) { check(1)\n")
	request := NewApprovalResolver(root, "")
	if _, err := request.Resolve("pkg.TestAbsent", ""); err == nil || !strings.Contains(err.Error(), "has no current/parent carrier") ||
		strings.Contains(err.Error(), "no exact Go declaration boundary") {
		t.Fatalf("absent name = %v; want absence rather than another name's parse failure", err)
	}
	if _, err := request.Resolve("pkg.TestBroken", ""); err == nil || !strings.Contains(err.Error(), "no exact Go declaration boundary") {
		t.Fatalf("found inexact name = %v; want exact-boundary refusal", err)
	}
	got, err := request.Resolve("pkg.broken_test", "")
	if err != nil || got.Path != path || got.Base != approvalAbsent || got.Current == approvalAbsent {
		t.Fatalf("whole-file stem = %+v, %v; want the existing full-content fallback", got, err)
	}
}
