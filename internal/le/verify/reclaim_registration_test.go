// Design: docs/contributing/running-commands.md -- reclaim only proven-owned artifacts.
// Related: cleanup.go -- directoryless registrations outlive their checkout.

package verify

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// directorylessRegistration builds Git's actual administrative file format in
// an isolated fixture repository. It creates no checkout or live Git worktree.
func directorylessRegistration(t *testing.T, repo fixtureRepo, name, path string, locked bool) string {
	t.Helper()
	admin := filepath.Join(repo.root, ".git", "worktrees", name)
	if err := os.MkdirAll(admin, 0o750); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"gitdir":    filepath.Join(path, ".git") + "\n",
		"HEAD":      repo.git(t, "rev-parse", "HEAD") + "\n",
		"commondir": "../..\n",
	}
	if locked {
		files["locked"] = "initializing\n"
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(admin, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return admin
}

// TestReclaimRemovesLockedDirectorylessRegistration proves reclamation removes
// the dead owner's locked Git registration before discarding its retry marker.
func TestReclaimRemovesLockedDirectorylessRegistration(t *testing.T) {
	repo := newFixtureRepo(t)
	path := filepath.Join(repo.root, "tmp", "verify-worktree", "interrupted-add")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	admin := directorylessRegistration(t, repo, "interrupted-add", path, true)
	if err := os.WriteFile(ownerMarker(path), []byte("0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := Reclaim(t.Context(), repo.root)
	if report.Code != 0 {
		t.Errorf("owned locked reclamation failed: %+v", report)
	}
	for _, gone := range []string{admin, ownerMarker(path)} {
		if _, err := os.Lstat(gone); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("owned registration or marker remains: %s: %v", gone, err)
		}
	}
}

// TestReclaimPreservesUnownedRegistrations tests actual Git registrations whose
// missing directories do not grant ownership. Repository-wide prune would erase
// each unlocked registration without consulting its marker or containment.
func TestReclaimPreservesUnownedRegistrations(t *testing.T) {
	for _, kind := range []string{"unknown", "unreadable", "live", "outside-area"} {
		t.Run(kind, func(t *testing.T) {
			repo := newFixtureRepo(t)
			path := filepath.Join(repo.root, "tmp", "verify-worktree", kind)
			if kind == "outside-area" {
				path = filepath.Join(repo.root, "foreign-checkout")
			} else {
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				if kind == "unreadable" {
					if err := os.Mkdir(ownerMarker(path), 0o700); err != nil {
						t.Fatal(err)
					}
				} else {
					owner := "foreign identity\n"
					if kind == "live" {
						owner = strconv.Itoa(os.Getpid()) + "\n"
					}
					if err := os.WriteFile(ownerMarker(path), []byte(owner), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			admin := directorylessRegistration(t, repo, kind, path, false)
			before, err := os.ReadFile(filepath.Join(admin, "gitdir"))
			if err != nil {
				t.Fatal(err)
			}
			report := Reclaim(t.Context(), repo.root)
			if kind == "unknown" || kind == "unreadable" {
				if report.Code == 0 {
					t.Errorf("unknown ownership was not refused: %+v", report)
				}
			}
			after, err := os.ReadFile(filepath.Join(admin, "gitdir"))
			if err != nil || !bytes.Equal(after, before) {
				t.Errorf("reclamation changed unowned registration: before=%q after=%q err=%v", before, after, err)
			}
		})
	}
}
