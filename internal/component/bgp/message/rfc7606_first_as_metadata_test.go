// RFC naming: untagged -- parser metadata invariant supporting separately tagged Session error-handling tests.

package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC 7606 Sections 3(g) and 7.2: admission must inspect the first AS_PATH
// from the original attribute bytes, before duplicate stripping or coalescing.
func TestRFC7606FirstASMetadata(t *testing.T) {
	path := []byte{2, 1, 0, 0, 0xfd, 0xea}
	attrs := optAttr(0x40, 1, []byte{0})
	attrs = append(attrs, optAttr(0x40, 2, path)...)
	attrs = append(attrs, optAttr(0x40, 3, []byte{192, 0, 2, 1})...)
	for _, tc := range []struct {
		name       string
		extra      []byte
		wantAS4    bool
		wantAction RFC7606Action
	}{
		{"ordinary", nil, false, RFC7606ActionNone},
		{"duplicate_path", optAttr(0x40, 2, []byte{2, 1, 0, 0, 0xfd, 0xeb}), false, RFC7606ActionNone},
		{"as4_path", optAttr(0xc0, 17, path), true, RFC7606ActionNone},
		{"as4_aggregator", optAttr(0xc0, 18, []byte{0, 0, 0xfd, 0xea, 192, 0, 2, 1}), true, RFC7606ActionNone},
		{"completed_discard", optAttr(0x40, 6, []byte{0}), false, RFC7606ActionAttributeDiscard},
		{"completed_withdraw", optAttr(0x80, 4, []byte{0}), false, RFC7606ActionTreatAsWithdraw},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := append(append([]byte(nil), attrs...), tc.extra...)
			// RFC 7606 Sections 3(g) and 7.2: collect on the existing walk.
			result := ValidateUpdateRFC7606AddPath(input, true, false, true, nil)
			require.Equal(t, tc.wantAction, result.Action)
			require.Equal(t, AttrRange{Start: 7, End: 13}, result.ASPath)
			require.Equal(t, path, input[result.ASPath.Start:result.ASPath.End])
			require.Equal(t, tc.wantAS4, result.AS4Present)
		})
	}

	t.Run("empty_value_present", func(t *testing.T) {
		// RFC 7606 Section 4 permits an empty AS_PATH value.
		result := ValidateUpdateRFC7606(rfc7606MandatoryAttrs, true, false, true)
		require.Equal(t, RFC7606ActionNone, result.Action)
		require.Equal(t, AttrRange{Start: 7, End: 7}, result.ASPath)
	})
	t.Run("extended_length", func(t *testing.T) {
		input := []byte{0x50, 2, 0, byte(len(path))}
		input = append(input, path...)
		input = append(input, optAttr(0x40, 1, []byte{0})...)
		input = append(input, optAttr(0x40, 3, []byte{192, 0, 2, 1})...)
		// RFC 4271 Section 4.3: the value follows both length octets.
		result := ValidateUpdateRFC7606(input, true, false, true)
		require.Equal(t, RFC7606ActionNone, result.Action)
		require.Equal(t, AttrRange{Start: 4, End: 10}, result.ASPath)
	})
	t.Run("abandoned_walk_publishes_neither", func(t *testing.T) {
		input := append(append([]byte(nil), attrs...), optAttr(0xc0, 17, path)...)
		input = append(input, 0xc0, 8, 4, 0)
		// RFC 7606 Section 4: later framing damage abandons metadata publication.
		result := ValidateUpdateRFC7606(input, true, false, true)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
		require.Equal(t, AttrRange{}, result.ASPath)
		require.False(t, result.AS4Present)
	})
	t.Run("absent", func(t *testing.T) {
		// RFC 7606 Section 3(d): a missing AS_PATH has no value location.
		result := ValidateUpdateRFC7606(optAttr(0x40, 1, []byte{0}), true, false, true)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
		require.Equal(t, AttrRange{}, result.ASPath)
		require.False(t, result.AS4Present)
	})
}
