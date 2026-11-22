// Design: docs/architecture/plugin/rib-storage-design.md -- explicit source catch-up.
package ribevents

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// A provider cannot turn missing processing evidence into route absence or
// eligibility. Known lag is one whole unanswered request, then normal answers.
func TestEligibilityMinimumNeedsAuthoritativeOriginalProgress(t *testing.T) {
	family.RegisterTestFamilies()
	input := EligibilityRequest{Queries: []EligibilityQuery{{Route: ValidationRoute{PeerOwner: 7, ConnectionID: 9, Family: family.IPv4Unicast}, AfterMsgID: 11}}}
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	asked := 0
	eligible := func(ValidationRoute, uint64) bool { asked++; return true }
	present := func(ValidationRoute) bool { return false }
	_, err = AnswerEligibility(string(raw), eligible, present, nil)
	require.Error(t, err)
	require.Zero(t, asked)
	answer, err := AnswerEligibility(string(raw), eligible, present, func(route ValidationRoute, min uint64) (bool, error) {
		require.Equal(t, uint64(7), route.PeerOwner)
		require.Equal(t, uint64(9), route.ConnectionID)
		require.Equal(t, uint64(11), min)
		return false, nil
	})
	require.NoError(t, err)
	require.True(t, answer.Pending)
	require.Nil(t, answer.Answers)
	require.Zero(t, asked)
	answer, err = AnswerEligibility(string(raw), eligible, present, func(ValidationRoute, uint64) (bool, error) { return true, nil })
	require.NoError(t, err)
	require.False(t, answer.Pending)
	require.Equal(t, []bool{true}, answer.Answers)
	require.Equal(t, 1, asked)
	for _, alter := range []func(*ValidationRoute){func(r *ValidationRoute) { r.PeerOwner = 0 }, func(r *ValidationRoute) { r.ConnectionID = 0 }} {
		candidate := input
		candidate.Queries = append([]EligibilityQuery(nil), input.Queries...)
		alter(&candidate.Queries[0].Route)
		raw, err := json.Marshal(candidate)
		require.NoError(t, err)
		_, err = AnswerEligibility(string(raw), eligible, present, func(ValidationRoute, uint64) (bool, error) {
			t.Error("unrecorded identity reached progress reader")
			return true, nil
		})
		require.Error(t, err)
	}
}
