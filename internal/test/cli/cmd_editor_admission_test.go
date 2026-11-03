// Design: docs/architecture/testing/runner-architecture.md -- editor discovery admission.

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
)

// TestEditorDiscoveryRefusesDeclarations checks admission before any execution,
// including a malformed declaration after a capability marker.
// The authored timeout takes parallel headroom here.
func TestEditorDiscoveryRefusesDeclarations(t *testing.T) {
	for _, declaration := range []string{
		"option=needs-linux:caps=misspelled",
		"option=needs-linux:caps=",
		"option=needs-linux:cap=net-admin",
		"option=needs-linux:caps=net-admin:caps=bpf",
		"option=exclusive",
		"option=exclusive:group=a:group=b",
		"option=needs-linux:caps=sys-time\noption=exclusive:wrong=a",
		"option=needs-linux:caps=sys-time\ninvalid-action=x",
		"option=timeout:value=banana",
		"option=timeout:value=0s",
		"option=timeout:value=9223372036854775807ns",
	} {
		t.Run(declaration, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "admission.et")
			if err := os.WriteFile(path, []byte(declaration+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			tests := runner.NewEditorTests()
			if err := discoverEditorTests(tests, root, root); err != nil {
				t.Fatal(err)
			}
			if _, err := tests.Select(runner.Selection{All: true}); err != nil {
				t.Fatal(err)
			}
			selected := tests.Selected()
			if len(selected) != 1 {
				t.Fatalf("discovery lost the malformed file: %d records", len(selected))
			}
			if selected[0].GetError() == nil {
				t.Fatal("discovery accepted an invalid declaration")
			}
		})
	}
}

// TestEditorDiscoverySnapshot checks defaults and that embedded file text cannot
// become a resource declaration. The stored snapshot survives a later file edit.
func TestEditorDiscoverySnapshot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "snapshot.et")
	content := "tmpfs=test.conf:terminator=END\noption=needs-linux:caps=typo\nEND\noption=mode:value=command\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := runner.NewEditorTests()
	if err := discoverEditorTests(tests, root, root); err != nil {
		t.Fatal(err)
	}
	if _, err := tests.Select(runner.Selection{All: true}); err != nil {
		t.Fatal(err)
	}
	test := tests.Selected()[0]
	if test.GetError() != nil {
		t.Fatal(test.GetError())
	}
	if test.Admission.NeedsLinux {
		t.Fatal("embedded text became resource metadata")
	}
	if test.Admission.ExclusiveGroup != "" {
		t.Fatal("editor acquired an implicit group")
	}
	if len(test.Admission.RequiredCaps) != 0 {
		t.Fatal("editor acquired implicit capabilities")
	}
	if err := os.WriteFile(path, []byte("invalid-action=changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if test.Content != content {
		t.Fatal("discovery snapshot changed")
	}
}
