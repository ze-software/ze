package buildtag

import (
	"go/build/constraint"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// VALIDATES: a package joins a tag's population only when one of its test
// files needs the tag: the constraint holds with the tag set and cannot hold
// without it, whatever the other tags are.
// METHOD: a fixture tree with one package per constraint shape; Packages for
// docker and for integration must answer exactly the packages that need each.
// PREVENTS: a `!docker` file joining the docker population because the scan
// counted the negated term, and `docker || integration` being claimed by either.
func TestPackagesSelectOnlyFilesThatNeedTheTag(t *testing.T) {
	root := t.TempDir()
	for dir, expression := range map[string]string{
		"docker":         "docker",
		"dockerlinux":    "docker && linux",
		"notdocker":      "!docker",
		"integonly":      "integration && !docker",
		"either":         "docker || integration",
		"integration":    "integration",
		"dockerish":      "dockerish",
		"doublenegation": "!(!docker)",
	} {
		path := filepath.Join(root, "internal", dir, "x_test.go")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("//go:build "+expression+"\n\npackage x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for tag, want := range map[string][]string{
		"docker":      {"internal/docker", "internal/dockerlinux", "internal/doublenegation"},
		"integration": {"internal/integonly", "internal/integration"},
	} {
		got, err := Packages(root, tag)
		if err != nil {
			t.Fatalf("Packages(%s): %v", tag, err)
		}
		if !slices.Equal(got, want) {
			t.Errorf("Packages(%s) = %v, want %v", tag, got, want)
		}
	}
}

// VALIDATES: Names answers whether a constraint mentions a tag under any
// operator, negation included, matching the whole word only.
// METHOD: one parsed constraint per shape, asked for docker.
// PREVENTS: the overlap guard missing a file that names docker by negation or
// disjunction, or refusing one that names only `dockerish`.
func TestNamesFindsTheTagUnderEveryOperator(t *testing.T) {
	for expression, want := range map[string]bool{
		"docker":                           true,
		"!docker":                          true,
		"integration && !docker":           true,
		"(docker && linux) || integration": true,
		"!(!docker)":                       true,
		"integration":                      false,
		"dockerish":                        false,
	} {
		parsed, err := constraint.Parse("//go:build " + expression)
		if err != nil {
			t.Fatalf("parse %q: %v", expression, err)
		}
		if got := (TestFile{Constraint: parsed}).Names("docker"); got != want {
			t.Errorf("Names(docker) on %q = %v, want %v", expression, got, want)
		}
	}
}
