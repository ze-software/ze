// The goal is the refusals, because those are what the old layout sites did not
// have: a glob over an absent directory and a regex that fails to match both
// answer "no spec" with no error. Each test below drives one function over a
// temporary checkout and asserts that an unplaceable path, an absent name and a
// name in two buckets are each REFUSED rather than answered.
package specpath

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// tree writes one temporary checkout holding the named root-relative files.
func tree(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, file := range files {
		full := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("make %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte("| Field | Value |\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
	return root
}

func TestDirsAndGlobsAreInBucketOrder(t *testing.T) {
	dirs := Dirs()
	want := []string{"plan/release/fixes", "plan/release/validation", "plan/release/delivery", "plan/post-release/features", "plan/post-release/engineering", "plan/proposals"}
	if !slices.Equal(dirs, want) {
		t.Fatalf("Dirs() = %v, want %v", dirs, want)
	}
	globs := Globs()
	if len(globs) != len(dirs) {
		t.Fatalf("Globs() = %v, want one glob for each of %v", globs, dirs)
	}
	for i, glob := range globs {
		if glob != dirs[i]+"/spec-*.md" {
			t.Errorf("Globs()[%d] = %q, want %q", i, glob, dirs[i]+"/spec-*.md")
		}
	}
}

func TestBucketPlacesASpecAndRefusesEverythingElse(t *testing.T) {
	cases := []struct {
		path   string
		bucket string
		ok     bool
	}{
		{"plan/release/fixes/spec-now.md", Fixes, true},
		{"plan/release/validation/spec-proof.md", Validation, true},
		{"plan/release/delivery/spec-image.md", Delivery, true},
		{"plan/post-release/features/spec-feature.md", Features, true},
		{"plan/post-release/engineering/spec-tool.md", Engineering, true},
		{"plan/proposals/spec-idea.md", Proposals, true},
		{"./plan/release/fixes/spec-now.md", Fixes, true},
		{"plan/spec-after.md", "", false},
		{"plan/immediate/spec-now.md", "", false},
		{"plan/pre-release/spec-blocking.md", "", false},
		{"plan/to-review/spec-idea.md", "", false},
		{"plan/journal/spec-lookalike.md", "", false},
		{"plan/release/fixes/deep/spec-deeper.md", "", false},
		{"plan/README.md", "", false},
		{"plan/release/fixes/README.md", "", false},
		{"docs/spec-elsewhere.md", "", false},
		{"spec-at-the-root.md", "", false},
		{"plan/release/fixes/../fixes/spec-traversal.md", "", false},
		{"plan/release/fixes/spec-no-suffix", "", false},
		{"plan/release/fixes/not-a-spec.md", "", false},
	}
	for _, one := range cases {
		bucket, ok := Bucket(one.path)
		if bucket != one.bucket || ok != one.ok {
			t.Errorf("Bucket(%q) = %q, %v; want %q, %v", one.path, bucket, ok, one.bucket, one.ok)
		}
		if IsSpec(one.path) != one.ok {
			t.Errorf("IsSpec(%q) = %v, want %v", one.path, IsSpec(one.path), one.ok)
		}
	}
}

func TestAllReadsEveryBucketSortedAndRecursesIntoNone(t *testing.T) {
	root := tree(t,
		"plan/post-release/features/spec-zulu.md",
		"plan/post-release/features/spec-alpha.md",
		"plan/release/fixes/spec-now.md",
		"plan/release/validation/spec-blocking.md",
		"plan/release/delivery/spec-image.md",
		"plan/post-release/engineering/spec-tool.md",
		"plan/proposals/spec-idea.md",
		"plan/journal/spec-lookalike.md",
		"plan/release/fixes/deep/spec-deeper.md",
		"plan/README.md",
	)
	specs, err := All(root)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	want := []string{
		"plan/post-release/engineering/spec-tool.md",
		"plan/post-release/features/spec-alpha.md",
		"plan/post-release/features/spec-zulu.md",
		"plan/proposals/spec-idea.md",
		"plan/release/delivery/spec-image.md",
		"plan/release/fixes/spec-now.md",
		"plan/release/validation/spec-blocking.md",
	}
	if !slices.Equal(specs, want) {
		t.Fatalf("All = %v, want %v", specs, want)
	}
}

func TestAllAnswersABucketDirectoryThatDoesNotExist(t *testing.T) {
	root := tree(t, "plan/release/fixes/spec-alpha.md")
	specs, err := All(root)
	if err != nil {
		t.Fatalf("All over a tree with absent empty buckets: %v", err)
	}
	if !slices.Equal(specs, []string{"plan/release/fixes/spec-alpha.md"}) {
		t.Fatalf("All = %v, want the one spec that exists", specs)
	}
}

func TestAllRefusesATreeWithNoPlanDirectory(t *testing.T) {
	if _, err := All(t.TempDir()); err == nil {
		t.Fatal("All over a tree holding no plan/ answered an inventory; it must refuse")
	}
}

func TestFindResolvesABareNameInEveryBucket(t *testing.T) {
	root := tree(t,
		"plan/post-release/features/spec-after.md",
		"plan/release/fixes/spec-now.md",
		"plan/release/validation/spec-blocking.md",
		"plan/release/delivery/spec-image.md",
		"plan/post-release/engineering/spec-tool.md",
		"plan/proposals/spec-idea.md",
	)
	cases := map[string]string{
		"spec-after.md":    "plan/post-release/features/spec-after.md",
		"after":            "plan/post-release/features/spec-after.md",
		"spec-now.md":      "plan/release/fixes/spec-now.md",
		"blocking":         "plan/release/validation/spec-blocking.md",
		"spec-blocking":    "plan/release/validation/spec-blocking.md",
		"spec-blocking.md": "plan/release/validation/spec-blocking.md",
		"image":            "plan/release/delivery/spec-image.md",
		"tool":             "plan/post-release/engineering/spec-tool.md",
		"idea":             "plan/proposals/spec-idea.md",
	}
	for name, want := range cases {
		got, err := Find(root, name)
		if err != nil {
			t.Errorf("Find(%q): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("Find(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestFindRefusesAbsentDoubledAndPathSpellings(t *testing.T) {
	root := tree(t,
		"plan/post-release/features/spec-doubled.md",
		"plan/release/fixes/spec-doubled.md",
		"plan/release/fixes/spec-alone.md",
	)
	for _, name := range []string{"spec-absent.md", "spec-doubled.md", "plan/spec-alone.md"} {
		got, err := Find(root, name)
		if err == nil {
			t.Errorf("Find(%q) = %q with no error; it must refuse", name, got)
		}
		if got != "" {
			t.Errorf("Find(%q) = %q beside its error; a refusal answers no path", name, got)
		}
	}
	_, err := Find(root, "spec-doubled.md")
	if err == nil || !strings.Contains(err.Error(), "two buckets") {
		t.Errorf("Find over a doubled stem said %v; the error must name the collision", err)
	}
}

func TestStemDropsTheAffixesAndTheBucket(t *testing.T) {
	for _, name := range []string{"foo", "spec-foo.md", "plan/spec-foo.md", "plan/pre-release/spec-foo.md"} {
		if got := Stem(name); got != "foo" {
			t.Errorf("Stem(%q) = %q, want %q", name, got, "foo")
		}
	}
}

// TestObsoletePlacementFailsCurrentInventory keeps old work from silently
// disappearing, even when a valid destination has already been prepared.
func TestObsoletePlacementFailsCurrentInventory(t *testing.T) {
	for _, directory := range []string{"plan", "plan/immediate", "plan/pre-release", "plan/to-review"} {
		t.Run(directory, func(t *testing.T) {
			old := directory + "/spec-move.md"
			root := tree(t, old, "plan/release/fixes/spec-move.md")
			if _, err := All(root); err == nil || !strings.Contains(err.Error(), old) {
				t.Fatalf("All failed to identify obsolete placement %s: %v", old, err)
			}
			if _, err := Find(root, "move"); err == nil || !strings.Contains(err.Error(), old) {
				t.Fatalf("Find failed to identify obsolete placement %s: %v", old, err)
			}
		})
	}
}

// TestHistoricalRecognitionPreservesOldClassifications pins history semantics
// separately from current placement, including uncounted old proposals.
func TestHistoricalRecognitionPreservesOldClassifications(t *testing.T) {
	for rel, want := range map[string]string{
		"plan/spec-old.md":                          "after",
		"plan/immediate/spec-old.md":                "immediate",
		"plan/pre-release/spec-old.md":              "pre-release",
		"plan/to-review/spec-old.md":                "to-review",
		"plan/post-release/engineering/spec-new.md": Engineering,
	} {
		got, ok := HistoricalBucket(rel)
		if !ok || got != want {
			t.Errorf("HistoricalBucket(%s) = %s, %v; want %s", rel, got, ok, want)
		}
	}
	if len(HistoricalCategories()) != 3 {
		t.Fatal("historically excluded proposals entered the old inventory")
	}
}

// TestAllRefusesDuplicateIdentityAndUnreadableBucket exercises population
// boundaries independently of Find and metadata parsing.
func TestAllRefusesDuplicateIdentityAndUnreadableBucket(t *testing.T) {
	root := tree(t, "plan/release/fixes/spec-same.md", "plan/proposals/spec-same.md")
	if _, err := All(root); err == nil || !strings.Contains(err.Error(), "two buckets") {
		t.Fatalf("duplicate identity admitted: %v", err)
	}
	root = tree(t, "plan/release/fixes")
	if _, err := All(root); err == nil {
		t.Fatal("bucket file reported as an empty directory")
	}
}

// TestCategoriesOwnClassificationAndCopies ensures consumers cannot mutate the
// declaration and each bucket has one explicit required/optional decision.
func TestCategoriesOwnClassificationAndCopies(t *testing.T) {
	categories := Categories()
	for index, category := range categories {
		if category.Dir != Dirs()[index] || category.Label == "" {
			t.Fatalf("incomplete declaration: %+v", category)
		}
		want := index < 3
		if category.Required != want {
			t.Fatalf("%s required = %v, want %v", category.Name, category.Required, want)
		}
	}
	categories[0].Name = "changed"
	if Categories()[0].Name != Fixes {
		t.Fatal("caller changed the shared declaration")
	}
}
