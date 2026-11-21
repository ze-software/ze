// VALIDATES: WorktreeMainCheckout answers the checkout that holds a worktree
// opened at <checkout>/.claude/worktrees/<name>, and the root itself for any
// other path.
// PREVENTS: a directory deeper under a worktree read as a worktree, and a
// worktree read as its own main checkout.
package lepath

import "testing"

// TestWorktreeMainCheckout pins the path rule the Bash hook and the read
// record stores share. Method: one row per shape of root.
func TestWorktreeMainCheckout(t *testing.T) {
	for _, row := range []struct {
		root string
		want string
	}{
		{root: "/srv/ze", want: "/srv/ze"},
		{root: "/srv/ze/.claude/worktrees/wt", want: "/srv/ze"},
		{root: "/srv/ze/.claude/worktrees/wt/tmp/fixture", want: "/srv/ze/.claude/worktrees/wt/tmp/fixture"},
		{root: "/srv/ze/.claude/worktrees/", want: "/srv/ze/.claude/worktrees/"},
	} {
		if got := WorktreeMainCheckout(row.root); got != row.want {
			t.Errorf("WorktreeMainCheckout(%q) = %q, want %q", row.root, got, row.want)
		}
	}
}
