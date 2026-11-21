// VALIDATES: Stores answers where the Read hook keeps an agent's records of a
// checkout's pages, and the key each copy carries there: the checkout's own
// tmp/session under the bare path, and, for a worktree under a checkout's
// .claude/worktrees, also the holder's tmp/session under the worktree's
// label (CheckoutLabel, CheckoutKey).
// PREVENTS: a reader outside the hook (le spec design judge) that looks for a
// worktree's records only where a session started in the worktree keeps them.
package readrecord

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestStoresOfACheckoutIsItsOwnSessionDir pins the main-checkout case. Method:
// a checkout that is no worktree answers one store, its own tmp/session, whose
// key is the bare checkout-relative path.
func TestStoresOfACheckoutIsItsOwnSessionDir(t *testing.T) {
	root := t.TempDir()
	stores := Stores(root)
	if len(stores) != 1 {
		t.Fatalf("Stores(%s) answered %d stores, want 1: %#v", root, len(stores), stores)
	}
	if stores[0].Dir != SessionDir(root) {
		t.Errorf("the store is %s, want %s", stores[0].Dir, SessionDir(root))
	}
	if key := stores[0].Key("ai/doors/system.md"); key != "ai/doors/system.md" {
		t.Errorf("the key is %q, want the bare path", key)
	}
}

// TestStoresOfAWorktreeAddTheHoldersSessionDir pins the worktree case. Method:
// a worktree at <holder>/.claude/worktrees/wt answers its own store first,
// then the holder's tmp/session, whose key is the hex SHA-256 of the label
// ".claude/worktrees/wt", a slash and the path: the spelling the hook was seen
// to write, computed here without CheckoutKey.
func TestStoresOfAWorktreeAddTheHoldersSessionDir(t *testing.T) {
	holder, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(holder, ".claude", "worktrees", "wt")
	if err := os.MkdirAll(worktree, 0o750); err != nil {
		t.Fatal(err)
	}
	stores := Stores(worktree)
	if len(stores) != 2 {
		t.Fatalf("Stores(%s) answered %d stores, want 2: %#v", worktree, len(stores), stores)
	}
	if stores[0].Dir != SessionDir(worktree) {
		t.Errorf("the worktree's own store is %s, want %s", stores[0].Dir, SessionDir(worktree))
	}
	if key := stores[0].Key("ai/doors/system.md"); key != "ai/doors/system.md" {
		t.Errorf("the worktree's own key is %q, want the bare path", key)
	}
	digest := sha256.Sum256([]byte(".claude/worktrees/wt"))
	want := hex.EncodeToString(digest[:]) + "/ai/doors/system.md"
	if stores[1].Dir != SessionDir(holder) {
		t.Errorf("the second store is %s, want the holder's %s", stores[1].Dir, SessionDir(holder))
	}
	if key := stores[1].Key("ai/doors/system.md"); key != want {
		t.Errorf("the holder's key is %q, want %q", key, want)
	}
}

// TestCheckoutLabelOutsideTheSessionRootIsAbsolute pins the label of a
// checkout elsewhere on the disk. Method: a directory beside the session root
// is labeled by its absolute path, one inside it by its relative path.
func TestCheckoutLabelOutsideTheSessionRootIsAbsolute(t *testing.T) {
	if label := CheckoutLabel("/srv/ze", "/srv/other"); label != "/srv/other" {
		t.Errorf("a checkout beside the root is labeled %q, want /srv/other", label)
	}
	if label := CheckoutLabel("/srv/ze", "/srv/ze/.claude/worktrees/wt"); label != ".claude/worktrees/wt" {
		t.Errorf("a worktree inside the root is labeled %q, want .claude/worktrees/wt", label)
	}
}
