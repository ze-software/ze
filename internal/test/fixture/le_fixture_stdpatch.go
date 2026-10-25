// Design: docs/contributing/patching-the-standard-library.md -- the patched GOROOT every le checkout activates
// Related: ui_fixture_le_discovery_answers.go -- stand-in trees the discovery generators run in
// Related: ui_fixture_le_rfc_answers.go -- the owned tree the RFC gate compiles in

package fixture

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/scratch"
)

// leFixtureStdpatch makes tree, a stand-in checkout a fixture runs le in,
// carry what toolchain derivation reads there: the standard-library patch set
// of the checkout under test, and the cache/ link every le run makes in a
// checkout (scratch.EnsureCache), to the per-user cache whose stdpatch/ the
// checkout under test reaches too.
//
// le activates the patched GOROOT of whichever checkout it runs in, at startup
// and again in gotoolchain.New (gen.Activate). A tree with no patch set refuses
// every action that compiles, and the refused activation still creates
// cache/stdpatch inside the tree. A tree with the set but a cache of its own
// would build a whole GOROOT per run. The patched tree is content-addressed by
// toolchain and patch bytes, never by checkout, so the linked cache answers the
// tree the run under test already built. The link is the one le would make, so
// le finds it correct and the run leaves the tree as the fixture laid it out.
//
// A tree that already has a cache/ keeps it. Only a link survives a le run
// unchanged: le renames a real cache/ directory aside and links its place
// (docs/contributing/running-commands.md, the cache link table).
func leFixtureStdpatch(tree string) error {
	root, err := lepath.Root()
	if err != nil {
		return fmt.Errorf("locate the checkout under test: %w", err)
	}
	patches, err := gen.Patches(root)
	if err != nil {
		return err
	}
	for _, patch := range patches {
		target := filepath.Join(tree, filepath.FromSlash(patch.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fmt.Errorf("create the stand-in patch directory: %w", err)
		}
		if err := os.WriteFile(target, patch.Body, 0o600); err != nil {
			return fmt.Errorf("copy %s into the stand-in tree: %w", patch.Path, err)
		}
	}

	link := filepath.Join(tree, "cache")
	if _, err := os.Lstat(link); err == nil {
		return nil
	}
	result, err := scratch.New(tree, os.Environ()).EnsureCache(false)
	if err != nil {
		return fmt.Errorf("link the stand-in checkout's cache: %w", err)
	}
	// An absent cache/ is linked silently; a repair or a refusal is announced.
	if result.Announce() {
		return fmt.Errorf("link the stand-in checkout's cache: %s", result.Line)
	}
	return nil
}

// leFixtureStdpatchFor gives tree a patch set of its own, stamped for release:
// one patch that adds a text file outside every package, so it applies to any
// GOROOT and leaves `go list std cmd` unchanged. A fixture whose go.mod
// declares a release other than this checkout's needs it, because toolchain
// derivation pins that release and refuses a patch reviewed against another.
// The tree keeps a cache/ of its own: a tree over a stand-in patch set never
// enters the shared stdpatch cache.
func leFixtureStdpatchFor(tree, release string) error {
	const name = "fixture-stand-in.patch"
	body := "Ze fixture patch: a test artifact of internal/test/fixture, never applied to a build.\n\n" +
		"Ze-Reviewed-Against: go" + release + "\n\n" +
		"diff --git a/src/ze-fixture-stand-in.txt b/src/ze-fixture-stand-in.txt\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/src/ze-fixture-stand-in.txt\n" +
		"@@ -0,0 +1 @@\n" +
		"+stand-in patch for a fixture checkout\n"
	dir := filepath.Join(tree, filepath.FromSlash(gen.PatchDir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create the stand-in patch directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		return fmt.Errorf("write the stand-in patch: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(tree, "cache"), 0o750); err != nil {
		return fmt.Errorf("create the stand-in cache directory: %w", err)
	}
	return nil
}
