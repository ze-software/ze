// Design: third_party/vpp-patches/README.md -- the complete baseline API source.
// VALIDATES: API generation receives both ordered patch layers and no proof cut.
package buildvpp

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestVPPAPIRecipeCopiesCompleteBaselineCatalog adds implementation and owned
// API patches to the original fixture, then checks their exact copied bytes.
func TestVPPAPIRecipeCopiesCompleteBaselineCatalog(t *testing.T) {
	root, work := apiFixture(t)
	for _, name := range []string{
		"0098-fixture.patch", "0099-fixture.patch",
		"ze/0100-fixture.patch", "ze/0103-fixture.patch", "ze/0104-fixture.patch",
	} {
		apiWriteFixture(t, filepath.Join(root, "third_party/vpp-patches", name), name+"\n")
	}
	apiWriteFixture(t, filepath.Join(root, "third_party/vpp-patches/proof/metadata.diff"), "proof mutation\n")
	build, err := apiBuildRecipe(root, work, apiPins{commit: "fixture", sha256: "digest"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"0098-fixture.patch", "0099-fixture.patch",
		"ze/0100-fixture.patch", "ze/0103-fixture.patch", "ze/0104-fixture.patch",
	} {
		data, err := os.ReadFile(filepath.Join(build.Context, name))
		if err != nil {
			t.Fatalf("baseline catalog omitted %s: %v", name, err)
		}
		if string(data) != name+"\n" {
			t.Fatalf("baseline patch %s changed: %q", name, data)
		}
	}
	if _, err := os.Lstat(filepath.Join(build.Context, "proof")); !os.IsNotExist(err) {
		t.Fatalf("proof mutation entered the API recipe: %v", err)
	}
}

// TestVPPAPIBaselineCatalogOrdersEachLayer checks root patches precede the Ze
// layer even when the Ze names sort before the root names globally.
func TestVPPAPIBaselineCatalogOrdersEachLayer(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"0099-second.patch", "ze/0000-first.patch", "0001-first.patch", "ze/0103-second.patch"} {
		apiWriteFixture(t, filepath.Join(root, "third_party/vpp-patches", name), "fixture\n")
	}
	names, err := apiBaselinePatchNames(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"0001-first.patch", "0099-second.patch", filepath.Join("ze", "0000-first.patch"), filepath.Join("ze", "0103-second.patch")}
	if !slices.Equal(names, want) {
		t.Fatalf("baseline patch order = %v, want %v", names, want)
	}
}

// TestVPPAPIBaselineCatalogRefusesIncompleteLayers checks a missing or empty Ze
// layer cannot silently produce bindings from an earlier patch catalog.
func TestVPPAPIBaselineCatalogRefusesIncompleteLayers(t *testing.T) {
	for _, kind := range []string{"missing", "empty", "symlink-directory", "symlink-patch", "directory-patch"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "third_party/vpp-patches")
			apiWriteFixture(t, filepath.Join(base, "0001-fixture.patch"), "fixture\n")
			ze := filepath.Join(base, "ze")
			if kind == "symlink-directory" {
				if err := os.Symlink(t.TempDir(), ze); err != nil {
					t.Fatal(err)
				}
			} else if kind != "missing" {
				if err := os.Mkdir(ze, 0o750); err != nil {
					t.Fatal(err)
				}
				if kind == "symlink-patch" {
					target := filepath.Join(t.TempDir(), "original")
					apiWriteFixture(t, target, "original\n")
					if err := os.Symlink(target, filepath.Join(ze, "0103-fixture.patch")); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "directory-patch" {
					if err := os.Mkdir(filepath.Join(ze, "0103-fixture.patch"), 0o750); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := apiBaselinePatchNames(root); err == nil {
				t.Fatal("incomplete or unowned patch layer accepted")
			}
		})
	}
}
