package rfc

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInsideGitCheckoutCeilings keeps discovery inside Git's declared boundary,
// including its symlink-resolution and empty-entry rules, with a corrupt marker
// immediately above an otherwise history-free fixture.
func TestInsideGitCheckoutCeilings(t *testing.T) {
	for _, name := range []string{"absolute", "symlink", "symlinked working directory", "empty before symlink", "empty after symlink", "relative"} {
		t.Run(name, func(t *testing.T) {
			outer := t.TempDir()
			root := filepath.Join(outer, "parent", "work")
			parent := filepath.Dir(root)
			if err := os.MkdirAll(root, 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(parent, ".git"), []byte("gitdir: missing\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(outer, "alias")
			if err := os.Symlink(parent, alias); err != nil {
				t.Fatal(err)
			}
			separator := string(os.PathListSeparator)
			ceiling := parent
			want := false
			switch name {
			case "absolute":
			case "symlink":
				ceiling = alias
			case "symlinked working directory":
				root = filepath.Join(alias, "work")
			case "empty before symlink":
				ceiling = separator + alias
				want = true
			case "empty after symlink":
				ceiling = alias + separator
			case "relative":
				ceiling = ".."
				want = true
			}
			// The outer boundary isolates this case from the checkout containing
			// the test's temporary storage without hiding its own corrupt marker.
			t.Setenv("GIT_CEILING_DIRECTORIES", ceiling+separator+outer)
			if got := insideGitCheckout(root); got != want {
				t.Errorf("insideGitCheckout = %v, want %v for ceiling %q", got, want, ceiling)
			}
		})
	}
}

// TestPriorRevisionStateCeilingExcludesAncestor proves the ratchet's entry point
// calls a bounded history-free fixture absent despite an ancestor's marker.
func TestPriorRevisionStateCeilingExcludesAncestor(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "work")
	if err := os.Mkdir(root, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, ".git"), []byte("gitdir: missing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", parent)
	if state, reason := priorRevisionState(root); state != priorAbsent {
		t.Errorf("bounded fixture answered %v (%s), want absent", state, reason)
	}
}

// TestPriorRevisionStateCeilingPreservesCorruptCheckout proves a ceiling never
// hides a corrupt marker below it, in the working directory, or at GIT_DIR.
func TestPriorRevisionStateCeilingPreservesCorruptCheckout(t *testing.T) {
	for _, name := range []string{"below ceiling", "current directory", "explicit git directory"} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "work")
			if err := os.Mkdir(root, 0o750); err != nil {
				t.Fatal(err)
			}
			ceiling := parent
			switch name {
			case "below ceiling", "current directory":
				if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: missing\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if name == "current directory" {
					ceiling = root
				}
			case "explicit git directory":
				t.Setenv("GIT_DIR", filepath.Join(parent, "missing"))
			}
			t.Setenv("GIT_CEILING_DIRECTORIES", ceiling)
			if state, reason := priorRevisionState(root); state != priorUnreadable {
				t.Errorf("corrupt checkout answered %v (%s), want unreadable", state, reason)
			}
		})
	}
}

// TestPriorRevisionStateCeilingRefusesNonDirectory proves a failed marker
// inspection is not absence: Git cannot use a regular file as its work tree.
func TestPriorRevisionStateCeilingRefusesNonDirectory(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "file")
	if err := os.WriteFile(root, []byte("not a directory\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", parent)
	if state, reason := priorRevisionState(root); state != priorUnreadable {
		t.Errorf("non-directory answered %v (%s), want unreadable", state, reason)
	}
}
