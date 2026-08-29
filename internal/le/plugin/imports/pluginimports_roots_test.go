// Related: pluginimports.go -- pluginDirs, the policy this test keeps honest

package pluginimports

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// treeRoot walks up from the working directory to the checkout that holds
// go.mod. The package's other tests build a fixture tree; this one judges the
// real one, because the thing it guards is a directory somebody adds.
func treeRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("read the working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

// VALIDATES: every internal/component/<domain>/plugins directory in the tree is
// named in pluginDirs.
// PREVENTS: a new component plugins subtree that the composition root never
// walks, so its plugins are absent from the built binary with nothing red.
//
// pluginDirs is a policy and stays written out: the tier gate and the
// process-boundary gate judge the tree against it, so a list derived from the
// tree would agree with the tree by construction. This test is the other half
// of that choice. It does not derive the policy; it refuses to let the tree
// hold a plugins root the policy has not decided about.
//
// It replaced nestedPluginDomains, which derived internal/component/<domain>/plugins
// for a hand-written pair of domains. That is a list of the same kind wearing a
// derivation, and it had already gone stale in both directions: it named "ike",
// which has no plugins subtree, while l2tp/plugins was reachable only through it.
func TestEveryComponentPluginsTreeIsNamed(t *testing.T) {
	root := treeRoot(t)
	componentDir := filepath.Join(root, "internal", "component")

	entries, err := os.ReadDir(componentDir)
	if err != nil {
		t.Fatalf("read %s: %v", componentDir, err)
	}

	declared := pluginSearchRoots()
	found := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("internal", "component", entry.Name(), "plugins"))
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			continue
		}
		found++
		if !slices.Contains(declared, rel) {
			t.Errorf("%s holds plugins and no search root names it, so the composition root never walks it; add it to pluginDirs", rel)
		}
	}

	if found == 0 {
		t.Fatal("no internal/component/<domain>/plugins directory was found, so this test asserted nothing")
	}
}
