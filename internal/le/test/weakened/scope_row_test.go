// Design: docs/architecture/testing/test-health.md -- the owner approval route
//
// VALIDATES: an approval row's name matches a finding by its
// `<package>.<TestName>` form, and never a different package or name. A bare
// name never reaches rowMatches: LoadApprovals refuses it.
// PREVENTS: a row for one package's test approving the same-named test of
// another package.
package testweakened

import "testing"

func TestRowMatchesExactAndQualifiedFormsAreUnaffectedByScope(t *testing.T) {
	t.Parallel()

	finding := Finding{Path: "pkg/a_test.go", Package: "pkg", Name: "TestA"}
	if !rowMatches("pkg.TestA", finding) {
		t.Fatal("a package-qualified name must still match")
	}
	if rowMatches("other.TestA", finding) {
		t.Fatal("a package-qualified name for a different package must not match")
	}
	if rowMatches("TestOther", finding) {
		t.Fatal("a different name must not match")
	}
}
