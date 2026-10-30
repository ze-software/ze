package buildflavor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: Tags reads the tags a conjunction requires true and refuses a
// line it cannot read as one flavor: a disjunction, no //go:build line, a
// line that does not parse, and tags that do not satisfy their own line.
// METHOD: Tags over one body per case.
func TestTagsReadsOnlyAConjunctionItsTagsSatisfy(t *testing.T) {
	tags, err := Tags("// Copyright\n\n//go:build ze_core && ze_distro && !ze_le\n\npackage main\n")
	if err != nil || !slices.Equal(tags, []string{"ze_core", "ze_distro"}) {
		t.Errorf("a conjunction answered %v, %v; want [ze_core ze_distro]", tags, err)
	}
	refusals := []struct{ body, says string }{
		{"//go:build ze_core || ze_distro\n", "is not a conjunction of tags and negations"},
		{"package main\n", "carries no //go:build line"},
		{"//go:build ze_core &&\n", "does not parse"},
		{"//go:build ze_core && !ze_core\n", "do not satisfy its own //go:build line"},
	}
	for _, one := range refusals {
		if _, err := Tags(one.body); err == nil || !strings.Contains(err.Error(), one.says) {
			t.Errorf("Tags(%q) answered %v, want an error saying %q", one.body, err, one.says)
		}
	}
}

// VALIDATES: Shipped names each flavor from its file and reads its tags, a
// tree with no flavor file is an error, and an unreadable flavor's error
// names the file.
// METHOD: Shipped over temporary trees.
func TestShippedNamesEachFlavorFile(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, "cmd", "ze", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Shipped(root); err == nil || !strings.Contains(err.Error(), "declares a shipped flavor") {
		t.Errorf("a tree with no flavor file answered %v, want an error", err)
	}
	write("build_tag_setup_test.go", "//go:build ze_setup\n\npackage main\n")
	write("build_tag_core_test.go", "//go:build ze_core\n\npackage main\n")
	flavors, err := Shipped(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []Flavor{{Name: "core", Tags: []string{"ze_core"}}, {Name: "setup", Tags: []string{"ze_setup"}}}
	if !slices.EqualFunc(flavors, want, func(a, b Flavor) bool { return a.Name == b.Name && slices.Equal(a.Tags, b.Tags) }) {
		t.Errorf("Shipped answered %v, want %v", flavors, want)
	}
	write("build_tag_bad_test.go", "package main\n")
	if _, err := Shipped(root); err == nil || !strings.Contains(err.Error(), "cmd/ze/build_tag_bad_test.go") {
		t.Errorf("an unreadable flavor answered %v, want an error naming its file", err)
	}
}
