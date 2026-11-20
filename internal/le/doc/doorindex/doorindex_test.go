// VALIDATES: the door index answers the doors whose base is the longest whole
// component prefix of a file, and refuses a door that lost its header.
// PREVENTS: the derivation of `le spec design documents` and the door read gate
// disagreeing about which door a file sits under.
package doorindex

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// writeDoor writes one page under the fixture's ai/doors.
func writeDoor(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCoveringTakesTheLongestBase is the rule. Method: a system door over
// internal and a command door over internal/component/command; a file under
// the command tree answers the command door alone, a sibling tree whose name
// extends it answers the system door, and a door with no header is an error.
func TestCoveringTakesTheLongestBase(t *testing.T) {
	root := t.TempDir()
	writeDoor(t, root, "README.md", "# Doors\n")
	writeDoor(t, root, "system.md", "# System\n\n<!-- door-base: pkg cmd/ze internal -->\n")
	writeDoor(t, root, "commands-cli.md", "# Commands\n\n<!-- door-base: internal/component/command/ -->\n")
	index, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := index.Covering("internal/component/command/pipe.go"); !slices.Equal(got, []string{Dir + "/commands-cli.md"}) {
		t.Fatalf("pipe.go is covered by %q", got)
	}
	if got := index.Covering("internal/component/commander/x.go"); !slices.Equal(got, []string{SystemDoor}) {
		t.Fatalf("a sibling tree is covered by %q", got)
	}
	if got := index.Covering("docs/architecture/x.md"); len(got) != 0 {
		t.Fatalf("a path no base reaches is covered by %q", got)
	}

	writeDoor(t, root, "broken.md", "# Broken\n")
	_, err = Load(root)
	if _, ok := errors.AsType[*HeaderError](err); !ok {
		t.Fatalf("a door with no header loaded (err %v)", err)
	}
}
