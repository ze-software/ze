// Design: docs/architecture/config/change-file-structural-ops.md -- pending path identity.

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSavedMemberPendingPathsPreserveTokenBoundaries reads actual saved member
// operations before projecting their paths. Even a quoted name that no schema
// declares must remain one parseable token in the pending-change display.
func TestSavedMemberPendingPathsPreserveTokenBoundaries(t *testing.T) {
	for _, operation := range []string{ChangeFileInsertMemberToken, ChangeFileDeactivateMemberToken, ChangeFileActivateMemberToken} {
		for _, name := range []string{"send", "two words", `"unterminated`, "'unterminated"} {
			t.Run(operation+"/"+name, func(t *testing.T) {
				words := []string{operation, "bgp", "peer west", name, "red"}
				if operation == ChangeFileInsertMemberToken {
					words = append(words, InsertFirst)
				}
				schema := NewSchema()
				_, _, operations, err := ParseCandidateChangeFile("#alice "+FormatEditPath(words)+"\n", schema)
				require.NoError(t, err, "the structural input has valid token syntax")
				require.Len(t, operations, 1)
				pending := operations[0].PendingChange(schema)
				decoded, err := ParseEditPath(pending.Path)
				require.NoError(t, err, "a pending-change renderer must emit valid path syntax")
				assert.Equal(t, []string{"bgp", "peer west", name}, decoded)
			})
		}
	}
}
