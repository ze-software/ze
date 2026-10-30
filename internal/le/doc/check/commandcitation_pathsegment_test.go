// Design: docs/contributing/documentation-testing.md -- the retired and unheld le command citation check
// Related: commandcitation.go -- citationsOf, argvElementLater

package doccheck

import "testing"

// TestPathSegmentLeIsNotACitation proves a "le" that is a later element of a Go
// list, a path segment joined by filepath.Join, cites no command, while an argv
// whose first element is "le" still does. Method: run citationsOf over both
// lines of a Go file and count what each cites.
// PREVENTS: wireguardproof.go's testdata Dockerfile path read as the command
// `le test deployment testdata`, which the unheld-citation check refused.
func TestPathSegmentLeIsNotACitation(t *testing.T) {
	path := `Dockerfile: filepath.Join(root, "internal", "le", "test", "deployment", "testdata", "Dockerfile.wireguard"),`
	if got := citationsOf("internal/le/test/deployment/x.go", path); len(got) != 0 {
		t.Errorf("a path segment cited %q", got)
	}
	argv := `command := exec.CommandContext(ctx, "le", "test", "deployment")`
	got := citationsOf("internal/le/test/deployment/x.go", argv)
	if len(got) != 1 {
		t.Fatalf("an argv whose program is le cited %q, want one citation", got)
	}
	if got[0][0] != "test" {
		t.Errorf("argv citation words = %q, want them to start with test", got[0])
	}
}
