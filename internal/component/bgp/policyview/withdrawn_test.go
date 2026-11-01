package policyview

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// TestWithdrawnFieldWritesWithdrawals proves the withdrawn field carries the
// NLRI an UPDATE withdraws, the body Withdrawn Routes and MP_UNREACH_NLRI, in
// the family-keyed object prefix carries the announced ones, and that an
// UPDATE withdrawing nothing leaves the field out rather than send it empty.
//
// VALIDATES: the withdrawn read field (spec D-12: IRR's keep-withdrawals
// verdict needs the withdrawals the route carries).
// PREVENTS: a verdict built from the announced half alone.
func TestWithdrawnFieldWritesWithdrawals(t *testing.T) {
	family.RegisterTestFamilies()
	unreach := pathAttr(0x80, attribute.AttrMPUnreachNLRI, []byte{0, 2, 1, 32, 0x20, 0x01, 0x0d, 0xb8})
	both := withdrawing(updatePayload([][]byte{unreach}, []byte{24, 10, 0, 0}), []byte{24, 10, 9, 0})
	cases := []struct {
		name    string
		payload []byte
		want    string
	}{
		{"body and MP_UNREACH_NLRI", both,
			`{"withdrawn":{"ipv4/unicast":["10.9.0.0/24"],"ipv6/unicast":["2001:db8::/32"]}}`},
		{"no withdrawal", updatePayload(nil, []byte{24, 10, 0, 0}), `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var view View
			view.Reset(wireu.NewWireUpdate(tc.payload, fourOctet(t)), Session{})
			defer view.Clear()
			got, err := AppendFields(nil, &view, []string{"withdrawn"})
			if err != nil {
				t.Fatalf("AppendFields: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("fields = %s, want %s", got, tc.want)
			}
		})
	}
}
