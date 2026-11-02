// Design: docs/architecture/zefs-format.md -- secure framed-tree traversal refusals.

package zefs

import (
	"errors"
	"io/fs"
	"testing"
)

// TestComponentErrorWords pins the refusal a caller reads when the walk
// refuses one component, and what it matches.
//
// VALIDATES: ComponentError.Error names the parent and the component by the
// absolute paths the walk built, then the cause, then fs.ErrPermission, in
// those exact words; the error matches both its cause and fs.ErrPermission.
// PREVENTS: the words a local caller prints, which storage passes through
// unchanged, drifting with no test going red.
// Method: a refusal is built directly, so the words are compared on every
// platform, including one where OpenDirectory never returns it.
func TestComponentErrorWords(t *testing.T) {
	cause := errors.New("too many levels of symbolic links")
	refusal := &ComponentError{Parent: "/srv", Component: "/srv/link", Folder: "/srv/link/store", Err: cause}

	const want = "zefs: open /srv: unsafe component /srv/link: too many levels of symbolic links: permission denied"
	if got := refusal.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(refusal, cause) {
		t.Errorf("%q does not match its cause", refusal)
	}
	if !errors.Is(refusal, fs.ErrPermission) {
		t.Errorf("%q does not match %v", refusal, fs.ErrPermission)
	}
}
