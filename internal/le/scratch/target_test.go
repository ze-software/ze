package scratch

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSameDirectory pins the one answer cache-clean's ambient row and
// `le setup`'s GOCACHE row both rely on. Method: a real directory, a link to
// it, a link that dangles and a path that does not exist, each pair judged,
// and a path under a directory this user cannot search, which is an error.
func TestSameDirectory(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(base, "dangling")
	if err := os.Symlink(filepath.Join(base, "gone"), dangling); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(base, "missing")

	cases := []struct {
		name          string
		first, second string
		want          bool
	}{
		{"link reaches its target", link, target, true},
		{"target reaches its link", target, link, true},
		{"spelled alike after cleaning", target + "/./", target, true},
		{"missing path spelled alike", missing, missing + "/", true},
		{"missing path is not the target", missing, target, false},
		{"dangling link is not its absent target", dangling, filepath.Join(base, "gone"), false},
		{"two different missing paths", missing, filepath.Join(base, "other"), false},
		{"two different directories", target, base, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SameDirectory(tc.first, tc.second)
			if err != nil {
				t.Fatalf("SameDirectory(%q, %q): %v", tc.first, tc.second, err)
			}
			if got != tc.want {
				t.Errorf("SameDirectory(%q, %q) = %v, want %v", tc.first, tc.second, got, tc.want)
			}
		})
	}
}

// TestSameDirectoryUnresolvableIsAnError: a path that exists but cannot be
// resolved (its parent cannot be searched) is neither the same directory nor
// a different one, so SameDirectory answers an error rather than false.
// VALIDATES: SameDirectory refuses to answer a sameness it could not judge.
// PREVENTS: cache-clean cleaning, or setup rewriting, on a guess.
func TestSameDirectoryUnresolvableIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root searches a directory with no permissions")
	}
	base := t.TempDir()
	locked := filepath.Join(base, "locked")
	if err := os.MkdirAll(filepath.Join(locked, "inner"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) }) //nolint:gosec // G302: restored so TempDir can remove it

	got, err := SameDirectory(filepath.Join(locked, "inner"), base)
	if err == nil {
		t.Fatalf("SameDirectory under an unsearchable directory = %v with no error, want an error", got)
	}
}
