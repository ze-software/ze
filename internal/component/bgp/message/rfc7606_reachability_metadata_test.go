// RFC naming: untagged -- original-reachability metadata invariant; tagged Session tests prove the resulting error action.

package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC 7606 Section 5.2: later Session checks need the original reachable-NLRI
// fact, not MP attribute presence or contents left after downstream rewrites.
func TestRFC7606OriginalReachabilityMetadata(t *testing.T) {
	for _, tc := range []struct {
		name       string
		legacy     bool
		mpNLRI     []byte
		abandoned  bool
		wantReach  bool
		wantAction RFC7606Action
	}{
		{"empty_mp", false, nil, false, false, RFC7606ActionNone},
		{"legacy_with_empty_mp", true, nil, false, true, RFC7606ActionNone},
		{"nonempty_mp", false, []byte{24, 192, 0, 2}, false, true, RFC7606ActionNone},
		{"default_route_mp", false, []byte{0}, false, true, RFC7606ActionNone},
		{"empty_mp_abandoned", false, nil, true, false, RFC7606ActionSessionReset},
		{"legacy_abandoned", true, nil, true, true, RFC7606ActionTreatAsWithdraw},
		{"nonempty_mp_abandoned", false, []byte{24, 192, 0, 2}, true, true, RFC7606ActionTreatAsWithdraw},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// RFC 4760 Section 3: MP_REACH value header followed by NLRI.
			mp := append([]byte{0, 1, 1, 4, 192, 0, 2, 1, 0}, tc.mpNLRI...)
			attrs := updateWith(optAttr(0x80, 14, mp))
			if tc.abandoned {
				attrs = append(attrs, 0xc0, 8, 4, 0)
			}
			// RFC 7606 Sections 4 and 5.2: preserve observed reachability.
			result := ValidateUpdateRFC7606AddPath(attrs, tc.legacy, false, false, nil)
			require.Equal(t, tc.wantAction, result.Action, result.Description)
			require.Equal(t, tc.wantReach, result.HasReachableNLRI)
		})
	}
	t.Run("legacy_without_attributes", func(t *testing.T) {
		// RFC 7606 Section 3(d): missing attributes do not erase the legacy NLRI.
		result := ValidateUpdateRFC7606(nil, true, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
		require.True(t, result.HasReachableNLRI)
	})
	t.Run("empty_update", func(t *testing.T) {
		// RFC 7606 Section 5.2: a legacy EOR has no reachable NLRI.
		result := ValidateUpdateRFC7606(nil, false, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action)
		require.False(t, result.HasReachableNLRI)
	})
}
