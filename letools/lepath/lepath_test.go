// The root is what every le tool resolves a path against, so a wrong answer
// here is a tool reading or writing the wrong tree. These cases pin the three
// ways it can be wrong: the environment ignored, a directory that is not a
// checkout accepted, and a checkout not found from inside it.

package lepath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestRootFindsTheCheckoutFromInsideIt walks up from this package's own
// directory, which is what a tool run from anywhere in the tree does.
func TestRootFindsTheCheckoutFromInsideIt(t *testing.T) {
	t.Setenv("ZE_REPO_ROOT", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	root, err := Root()
	if err != nil {
		t.Fatalf("Root from %s: %v", mustGetwd(t), err)
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(root, marker)); err != nil {
			t.Errorf("Root answered %s, which has no %s", root, marker)
		}
	}
}

// TestRootPrefersTheEnvironment pins the contract scripts/le/paths.py states:
// ZE_REPO_ROOT wins, because the environment knows about a container mount, a
// worktree or a fixture that the filesystem walk cannot see.
func TestRootPrefersTheEnvironment(t *testing.T) {
	named := t.TempDir()
	t.Setenv("ZE_REPO_ROOT", named)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	root, err := Root()
	if err != nil {
		t.Fatalf("Root with ZE_REPO_ROOT set: %v", err)
	}
	want, err := filepath.EvalSymlinks(named)
	if err != nil {
		want = named
	}
	got, err := filepath.EvalSymlinks(root)
	if err != nil {
		got = root
	}
	if got != want {
		t.Errorf("Root answered %s, want the named %s: the environment was ignored", got, want)
	}
}

// TestAncestorWithMarkersRefusesADirectoryThatIsNotACheckout is the guard that
// makes go.mod alone insufficient: a vendored module directory has one, and
// answering it would point every tool at the wrong tree.
func TestAncestorWithMarkersRefusesADirectoryThatIsNotACheckout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if found := ancestorWithMarkers(dir); found == dir {
		t.Errorf("a directory with go.mod alone was accepted as a checkout: %s", found)
	}

	if err := os.WriteFile(filepath.Join(dir, "feature-gates.txt"), []byte("\n"), 0o600); err != nil {
		t.Fatalf("write feature-gates.txt: %v", err)
	}
	if found := ancestorWithMarkers(dir); found != dir {
		t.Errorf("a directory with both markers was not accepted: got %q, want %s", found, dir)
	}
}

// VALIDATES: Module answers the path go.mod declares, and answers an ERROR
// rather than an empty string when it declares none.
// PREVENTS: a generator writing blank imports rooted at "/internal/...", which
// compiles nowhere and says nothing about why. An empty string would be a
// plausible-looking answer to a caller that only checks the error it never
// received.
func TestModuleReadsGoModAndRefusesOneWithoutIt(t *testing.T) {
	cases := map[string]struct {
		gomod string
		want  string
	}{
		"a plain declaration":    {"module example.test/m\n\ngo 1.26\n", "example.test/m"},
		"leading blank lines":    {"\n\nmodule example.test/m\n", "example.test/m"},
		"extra spacing":          {"module    example.test/m   \ngo 1.26\n", "example.test/m"},
		"no module directive":    {"go 1.26\n\nrequire ()\n", ""},
		"a comment naming it":    {"// module example.test/m\ngo 1.26\n", ""},
		"an empty file entirely": {"", ""},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(one.gomod), 0o600); err != nil {
				t.Fatalf("write go.mod: %v", err)
			}

			got, err := Module(dir)
			if one.want == "" {
				if err == nil {
					t.Fatalf("Module answered %q for a go.mod declaring no module", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Module: %v", err)
			}
			if got != one.want {
				t.Errorf("Module answered %q, want %q", got, one.want)
			}
		})
	}

	if _, err := Module(t.TempDir()); err == nil {
		t.Error("Module answered no error for a directory holding no go.mod")
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}
