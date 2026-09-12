// VALIDATES: AC-9 -- every registered le area publishes the action table the
// manifest renders, unless it is named on the migration list.
// PREVENTS: an area reaching the registry with no published grammar, so the
// only way to learn what it takes is to invoke it. A stale migration row
// surviving after its area started publishing.
package le

import (
	"testing"

	"github.com/ze-software/ze/internal/le/leroot"
)

// areasWithoutAnActionTable names every registered area that declares no
// leaction table, so the manifest publishes its description and not its
// grammar. Each one writes its own dispatch: a gate that refuses every argument
// writes the refusal by hand, and the rest parse their own keywords.
//
// The list is a ratchet in both directions. An area that is not on it and
// registers no table fails the test below, and so does a name on it whose area
// now registers one. plan/spec-le-every-area-dispatches-through-one-table.md
// empties it.
var areasWithoutAnActionTable = []string{
	"cli-grammar",
	"command list",
	"command ownership",
	"commit",
	"config claims",
	"consistency",
	"digest",
	"doc check",
	"docvalid",
	"go-extract",
	"gokrazy-gosum",
	"iface-resolution",
	"inventory",
	"job",
	"spec citation",
	"spec session",
	"spec status",
	"stress-repro",
	"test-helper",
	"token-economy",
	"tracked",
	"verify lock",
	"verify summary",
	"weekly",
	"working-tree",
}

// TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList holds le to
// publishing what it declares. An area that registers a table is in the
// manifest with its keywords, and `le <area> <verb> --help` renders that
// grammar rather than the area's node page.
//
// It runs here rather than in package leroot because the registry is populated
// by the blank imports in register.go. In leroot's own test binary no area is
// registered at all, and the same assertion would pass over an empty set.
func TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList(t *testing.T) {
	if len(commandsAtStart) == 0 {
		t.Fatal("le registered no local-data command")
	}

	seen := make(map[string]bool, len(areasWithoutAnActionTable))
	for _, name := range areasWithoutAnActionTable {
		seen[name] = false
	}

	for _, tool := range commandsAtStart {
		_, declared := leroot.ActionsOf(tool.Name)
		_, listed := seen[tool.Name]
		if listed {
			seen[tool.Name] = true
		}
		if declared && listed {
			t.Errorf("area %q registers an action table, so delete its row from areasWithoutAnActionTable", tool.Name)
			continue
		}
		if !declared && !listed {
			t.Errorf("area %q publishes no grammar: call leroot.RegisterActions(%q, Actions) in its register.go", tool.Name, tool.Name)
		}
	}

	for name, found := range seen {
		if !found {
			t.Errorf("areasWithoutAnActionTable names %q, which no area registers under", name)
		}
	}
}
