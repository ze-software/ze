// Design: docs/architecture/plugin/plugin-system.md -- Registration fields, the runtime list equals the declared list
// Related: registered.go -- BuildZe, ListPlugins, Compare

package registered

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// TestBuiltZeRegistersTheDeclaredPlugins proves the static plugin list is the
// runtime one.
//
// VALIDATES: a `ze` built with the daemon tags answers `show plugin list`
// with exactly the plugins repoinventory.DeclaredPlugins reads from the
// committed source, each with the same description, RFCs, capability codes
// and families, and no row is a plugin that recorded setup without
// registering.
// PREVENTS: a registration the declared walk cannot see (one moved behind a
// call the walk treats as opaque) shipping unlisted, and a declared plugin
// the build never registers being published as shipped.
//
// Method: build cmd/ze from the checkout, run its `show plugin list` in the
// JSON rendering, and compare with the declared walk of HEAD. Linux only:
// the declared walk reads the Linux build context. The binary is built from
// the working tree, so on a dirty checkout a difference can be an
// uncommitted edit; the failure then lists the dirty Go files.
func TestBuiltZeRegistersTheDeclaredPlugins(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the repository root: %v", err)
	}
	declared, err := repoinventory.DeclaredPlugins(root)
	if err != nil {
		t.Fatalf("read the declared plugins of HEAD: %v", err)
	}
	binary, err := BuildZe(context.Background(), root, t.TempDir())
	if err != nil {
		t.Fatalf("build ze: %v", err)
	}
	rows, err := ListPlugins(context.Background(), binary)
	if err != nil {
		t.Fatalf("list the plugins the built ze registered: %v", err)
	}
	if err := Compare(declared, rows); err != nil {
		t.Fatalf("%v%s", err, dirtyGoFiles(root))
	}
}

// dirtyGoFiles answers a note naming the uncommitted Go files of the
// checkout, or nothing when it has none or git cannot say.
func dirtyGoFiles(root string) string {
	command := exec.CommandContext(context.Background(), "git", "status", "--porcelain", "--", "*.go")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return "\n(git status could not list uncommitted Go files: " + err.Error() + ")"
	}
	listed := strings.TrimSpace(string(output))
	if listed == "" {
		return ""
	}
	return "\nThe binary was built from the working tree and these Go files differ from HEAD:\n" + listed
}
