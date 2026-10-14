// VALIDATES: AC-27, applyVendorPatches checks each vendor patch's review stamp
// against vendor/modules.txt before any git apply, the already-applied probe
// included, and refuses a patch the set names that is missing on disk.
// PREVENTS: a re-vendor at a new module version patched by a diff nobody
// re-reviewed, and a missing patch skipped as if it were optional.

package setup

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/vendorpatch"
)

// fixtureModule is the module the fixture vendor tree holds, at fixtureVersion.
const (
	fixtureModule  = "github.com/vishvananda/netlink"
	fixtureVersion = "v1.3.1"
	fixtureStamp   = "Ze-Reviewed-Against: " + fixtureModule + " " + fixtureVersion + "\n"
)

// writeVendorPatchSet writes vendor/modules.txt holding fixtureModule and one
// patch per entry of leading (file name to the text before the diff), and
// answers the set as vendorpatch.Patches reads it.
func writeVendorPatchSet(t *testing.T, root string, leading map[string]string) []string {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(vendorpatch.Dir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	modules := "# " + fixtureModule + " " + fixtureVersion + "\n## explicit\n" + fixtureModule + "\n"
	if err := os.MkdirAll(filepath.Join(root, "vendor"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "modules.txt"), []byte(modules), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, text := range leading {
		body := text + "diff --git a/vendor/x.go b/vendor/x.go\n--- a/vendor/x.go\n+++ b/vendor/x.go\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := vendorpatch.Patches(root)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// TestVendorPatchRefusesAStaleReviewStamp proves each refusal of AC-27 happens
// before any git command, names what it found, and ends with the command line.
func TestVendorPatchRefusesAStaleReviewStamp(t *testing.T) {
	for name, tc := range map[string]struct {
		leading string
		applied bool // the --reverse --check probe would answer "already applied"
		want    []string
	}{
		"stale":           {leading: "Ze-Reviewed-Against: " + fixtureModule + " v1.3.0\n", want: []string{fixtureModule, "v1.3.0", fixtureVersion}},
		"stale-applied":   {leading: "Ze-Reviewed-Against: " + fixtureModule + " v1.3.0\n", applied: true, want: []string{fixtureModule, "v1.3.0", fixtureVersion}},
		"missing-stamp":   {leading: "a patch with no stamp\n", want: []string{"Ze-Reviewed-Against"}},
		"unknown-module":  {leading: "Ze-Reviewed-Against: example.com/absent v1.0.0\n", want: []string{"example.com/absent", "holds no"}},
		"missing-on-disk": {want: []string{"no such file"}},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			var relative string
			if name == "missing-on-disk" {
				writeVendorPatchSet(t, root, nil)
				relative = vendorpatch.Dir + "/gone.patch"
				if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, filepath.FromSlash(relative))); err != nil {
					t.Fatal(err)
				}
			} else {
				relative = writeVendorPatchSet(t, root, map[string]string{"p.patch": tc.leading})[0]
			}
			rec := rootShell("go", "git")
			if tc.applied {
				rec.answers["git apply --reverse --check "+relative] = Result{}
			}
			report := &Report{}
			if (&Setup{Root: root, Shell: rec.shell()}).applyVendorPatches(report) {
				t.Fatalf("the patch was accepted:\n%s", report.Text())
			}
			if ran := rec.ran(); slices.ContainsFunc(ran, func(c string) bool { return strings.HasPrefix(c, "git ") }) {
				t.Errorf("git ran before the stamp refusal: %v", ran)
			}
			text := strings.TrimSpace(report.Text())
			for _, want := range append(tc.want, relative) {
				if !strings.Contains(text, want) {
					t.Errorf("refusal does not name %q:\n%s", want, text)
				}
			}
			if !strings.HasSuffix(text, "Then run: ./le setup install") {
				t.Errorf("refusal does not end with the command line:\n%s", text)
			}
		})
	}

	t.Run("equal", func(t *testing.T) {
		root := t.TempDir()
		relative := writeVendorPatchSet(t, root, map[string]string{"p.patch": fixtureStamp})[0]
		rec := rootShell("go", "git")
		rec.answers["git apply --reverse --check "+relative] = Result{}
		report := &Report{}
		if !(&Setup{Root: root, Shell: rec.shell()}).applyVendorPatches(report) {
			t.Fatalf("an equal stamp refused:\n%s", report.Text())
		}
	})
}
