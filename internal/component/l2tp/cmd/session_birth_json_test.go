// Design: docs/guide/l2tp.md -- displayed immutable disconnect reference

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestSessionJSONPublishesOriginalBirthWhenPresent checks the real list and
// detail renderer, so the guard accepted by clear can be copied from its row.
func TestSessionJSONPublishesOriginalBirthWhenPresent(t *testing.T) {
	const reference = "l2tp-00000000000000000000000000000001"
	birth, err := subscriberid.ParseL2TPBirth(reference)
	require.NoError(t, err)
	for _, detail := range []bool{false, true} {
		row := sessionJSON(&l2tp.SessionSnapshot{Birth: birth, LocalSID: 42}, detail)
		require.Equal(t, reference, row["birth"])
		require.Equal(t, 42, row["local-sid"])
	}
}

// TestSessionJSONOmitsAbsentBirth preserves rows with no original reference;
// the renderer must not manufacture a guard from an ordinary numeric SID.
func TestSessionJSONOmitsAbsentBirth(t *testing.T) {
	for _, detail := range []bool{false, true} {
		row := sessionJSON(&l2tp.SessionSnapshot{LocalSID: 42}, detail)
		_, present := row["birth"]
		require.False(t, present)
		require.Equal(t, 42, row["local-sid"])
	}
}
