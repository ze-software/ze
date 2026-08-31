// The wiki catalog's one hand-written declaration, held against the source it
// mirrors. It lives here rather than beside Collect because docvalid imports
// wikicatalog (command_surfaces.go), so the test cannot sit on the other side of
// that edge.

package docvalid

import (
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/wikicatalog"
)

// VALIDATES: the four local commands Collect declares by hand carry the path,
// the summary and the long help cmd/ze registers for them.
// PREVENTS: the drift the mirror invites. Go forbids importing a main package,
// so those four registrations reach no linker, and the copy here is what the
// published catalog prints. A copy with no arbiter disagrees with its source the
// first time one side is edited, and the reader of the catalog cannot tell which
// side is wrong (ai/rules/principles.md).
func TestBuiltinsAgreeWithTheMainPackage(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout root: %v", err)
	}
	declared, err := mainPackageLocalCommands(root)
	if err != nil {
		t.Fatalf("read the cmd/ze registrations: %v", err)
	}

	registered := make(map[string]string, len(declared))
	longHelp := make(map[string]string, len(declared))
	for _, entry := range declared {
		registered[entry.Path] = entry.Meta.Description
		longHelp[entry.Path] = entry.Meta.LongHelp
	}

	published := map[string]bool{}
	for _, entry := range wikicatalog.Collect() {
		want, ok := registered[entry.Path]
		if !ok {
			continue
		}
		published[entry.Path] = true
		// A path the YANG tree also holds is published from the NODE, and the
		// hand-written entry is never reached for it (Collect, the seen map).
		// A wire method is what says the tree answered first.
		if entry.WireMethod != "" {
			continue
		}
		if entry.Description != want {
			t.Errorf("%s publishes %q, cmd/ze registers %q", entry.Path, entry.Description, want)
		}
		if entry.LongHelp != longHelp[entry.Path] {
			t.Errorf("%s publishes the long help %q, cmd/ze registers %q",
				entry.Path, entry.LongHelp, longHelp[entry.Path])
		}
	}
	for path := range registered {
		if !published[path] {
			t.Errorf("the catalog does not carry %q, which cmd/ze registers", path)
		}
	}
}
