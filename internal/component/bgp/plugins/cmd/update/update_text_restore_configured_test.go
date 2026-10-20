// Design: docs/architecture/api/update-syntax.md — withdrawing a route the config also announces
// Related: update_text.go — kwRestoreConfigured, errRestoreConfiguredAfterNLRI

package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRestoreConfiguredBeforeSectionMarksEveryGroup proves the keyword,
// written before the first section, reaches every group the command parses,
// and that a command without it leaves every group unmarked.
// VALIDATES: `restore-configured nlri del ... mp ipv6/unicast del ...` marks
// both groups RestoreConfigured; the same command without it marks none.
// PREVENTS: the keyword being dropped on the way to WithdrawNLRIBatch, so an
// exiting ExaBGP script's withdrawal takes a configured route off the wire.
func TestRestoreConfiguredBeforeSectionMarksEveryGroup(t *testing.T) {
	sections := []string{"nlri", "del", "1.1.1.1/32", "mp", "ipv6/unicast", "del", "2001:db8::/32"}

	marked, err := ParseUpdateText(append([]string{"restore-configured"}, sections...))
	require.NoError(t, err)
	require.Len(t, marked.Groups, 2)
	for i, group := range marked.Groups {
		assert.True(t, group.RestoreConfigured, "group %d (%s)", i, group.Family)
	}

	plain, err := ParseUpdateText(sections)
	require.NoError(t, err)
	require.Len(t, plain.Groups, 2)
	for i, group := range plain.Groups {
		assert.False(t, group.RestoreConfigured, "group %d (%s)", i, group.Family)
	}
}

// TestRestoreConfiguredAfterSectionRefused proves the keyword is refused once
// a section was parsed: it says how the whole command withdraws, so a group
// parsed before it would withdraw differently from one parsed after.
// VALIDATES: after `nlri` and after `mp`, ParseUpdateText answers
// errRestoreConfiguredAfterNLRI, whose text names the form to write.
// PREVENTS: a command whose groups silently disagree about the keyword.
func TestRestoreConfiguredAfterSectionRefused(t *testing.T) {
	tests := [][]string{
		{"nlri", "del", "1.1.1.1/32", "restore-configured"},
		{"mp", "ipv6/unicast", "del", "2001:db8::/32", "restore-configured", "nlri", "del", "1.1.1.1/32"},
	}
	for _, args := range tests {
		_, err := ParseUpdateText(args)
		require.ErrorIs(t, err, errRestoreConfiguredAfterNLRI, "%v", args)
		// The fix it names must be a form the parser accepts: `nlri <family>`
		// is refused, so the legacy section takes no family.
		assert.Contains(t, err.Error(), "update text restore-configured nlri del <prefix>")
		assert.Contains(t, err.Error(), "update text restore-configured mp <family> del <prefix>")
	}
}
