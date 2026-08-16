// Related: sync_web.go -- the sync this test drives from its entry point
// Related: check_web_test.go -- vendorFixture, runVendorProgram and the rest of
// the harness this file uses, which both programs in this directory share

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeGenerateRecipe returns the commands `make generate` would run, without
// running any of them.
//
// The recipe is read from make rather than from the Makefile text, so a recipe
// line built from a variable still answers. `make -n` is safe for this target
// because its recipe holds no $(MAKE): the warning above ze-precommit-verify in the
// Makefile is about that target alone.
//
// scripts/codegen/web_assets_test.go holds the same helper. The two test
// packages are separate binaries and neither can import the other, so the
// duplication is the cheaper of the two answers.
func makeGenerateRecipe(t *testing.T) string {
	t.Helper()

	out, err := runVendorCommand(t, vendorRepoRoot(t), "make", "-n", "generate")
	if err != nil {
		t.Fatalf("make -n generate: %v\n%s", err, out)
	}

	return out
}

// VALIDATES: `make generate` restores a consumer asset copy that no longer
// matches its third_party/web/ source.
// PREVENTS: consumer copies staying hand-maintained. //go:embed cannot reach
// outside its own package, so one library is vendored once per consumer, and a
// copy nobody regenerates is a copy that quietly diverges.
func TestGenerateSyncsVendoredAssets(t *testing.T) {
	t.Run("make-generate-runs-the-sync", func(t *testing.T) {
		recipe := makeGenerateRecipe(t)

		if !strings.Contains(recipe, "scripts/vendor/sync_web.go") {
			t.Fatalf("`make generate` does not run scripts/vendor/sync_web.go; its recipe is:\n%s", recipe)
		}
	})

	t.Run("the-sync-restores-an-edited-copy", func(t *testing.T) {
		const edited = "internal/component/web/assets/htmx.min.js"

		root := vendorFixture(t, map[string]string{edited: "// an edited consumer copy\n"})

		out, err := runVendorProgram(t, "sync_web.go", root)
		if err != nil {
			t.Fatalf("sync_web.go: %v\n%s", err, out)
		}

		source, err := os.ReadFile(filepath.Join(root, "third_party", "web", "htmx", "htmx.min.js"))
		if err != nil {
			t.Fatalf("read the fixture source copy: %v", err)
		}

		got, err := os.ReadFile(filepath.Join(root, edited))
		if err != nil {
			t.Fatalf("read the fixture consumer copy: %v", err)
		}

		if !bytes.Equal(got, source) {
			t.Errorf("%s was not restored\n got: %q\nwant: %q\n%s", edited, got, source, out)
		}
	})
}
