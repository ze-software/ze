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
	want := []string{"plan", "plan/immediate", "plan/pre-release"}
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
		{"plan/spec-after.md", After, true},
		{"plan/immediate/spec-now.md", Immediate, true},
		{"plan/pre-release/spec-blocking.md", PreRelease, true},
		{"./plan/spec-after.md", After, true},
		{"plan/journal/spec-lookalike.md", "", false},
		{"plan/immediate/deep/spec-deeper.md", "", false},
		{"plan/README.md", "", false},
		{"plan/immediate/README.md", "", false},
		{"docs/spec-elsewhere.md", "", false},
		{"spec-at-the-root.md", "", false},
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
		"plan/spec-zulu.md",
		"plan/spec-alpha.md",
		"plan/immediate/spec-now.md",
		"plan/pre-release/spec-blocking.md",
		"plan/journal/spec-lookalike.md",
		"plan/immediate/deep/spec-deeper.md",
		"plan/README.md",
	)
	specs, err := All(root)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	want := []string{
		"plan/immediate/spec-now.md",
		"plan/pre-release/spec-blocking.md",
		"plan/spec-alpha.md",
		"plan/spec-zulu.md",
	}
	if !slices.Equal(specs, want) {
		t.Fatalf("All = %v, want %v", specs, want)
	}
}

func TestAllAnswersABucketDirectoryThatDoesNotExist(t *testing.T) {
	root := tree(t, "plan/spec-alpha.md")
	specs, err := All(root)
	if err != nil {
		t.Fatalf("All over a tree with no immediate or pre-release directory: %v", err)
	}
	if !slices.Equal(specs, []string{"plan/spec-alpha.md"}) {
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
		"plan/spec-after.md",
		"plan/immediate/spec-now.md",
		"plan/pre-release/spec-blocking.md",
	)
	cases := map[string]string{
		"spec-after.md":    "plan/spec-after.md",
		"after":            "plan/spec-after.md",
		"spec-now.md":      "plan/immediate/spec-now.md",
		"blocking":         "plan/pre-release/spec-blocking.md",
		"spec-blocking":    "plan/pre-release/spec-blocking.md",
		"spec-blocking.md": "plan/pre-release/spec-blocking.md",
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
		"plan/spec-doubled.md",
		"plan/immediate/spec-doubled.md",
		"plan/spec-alone.md",
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
