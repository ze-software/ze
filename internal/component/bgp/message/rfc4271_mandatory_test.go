// Design: docs/architecture/route-selection.md -- mandatory attribute validation
package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC4271-6.3-7 positive -- MP_REACH's next hop cannot satisfy the separate NEXT_HOP obligation for legacy NLRI.
// RFC requirement: RFC4271-6.3-7 negative -- adding NEXT_HOP permits mixed legacy/MP announcements; MP-only routes do not require legacy NEXT_HOP.
func TestRFC4271MandatoryAttributesAcrossUpdateForms(t *testing.T) {
	originPath := []byte{0x40, 1, 1, 0, 0x40, 2, 4, 2, 1, 0xfd, 0xea}
	nextHop := []byte{0x40, 3, 4, 192, 0, 2, 1}
	mp := []byte{0x80, 14, 11, 0, 1, 1, 4, 192, 0, 2, 1, 0, 8, 10}
	for _, tc := range []struct {
		name   string
		legacy bool
		extra  []byte
		action RFC7606Action
	}{
		{"mixed missing legacy next hop", true, mp, RFC7606ActionTreatAsWithdraw},
		{"mixed complete", true, append(append([]byte{}, mp...), nextHop...), RFC7606ActionNone},
		{"MP only", false, mp, RFC7606ActionNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attrs := append(append([]byte{}, originPath...), tc.extra...)
			result := ValidateUpdateRFC7606(attrs, tc.legacy, false, false)
			require.Equal(t, tc.action, result.Action)
		})
	}
}
