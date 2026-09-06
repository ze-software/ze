// VALIDATES: the DISCARD disposition of RFC 4301 Section 4.4.1 reaches VPP as
// IPSEC_API_SPD_ACTION_DISCARD, and the other two dispositions keep their own actions.
// PREVENTS: a discard entry projected onto VPP's bypass action. The four VPP actions
// are numbered so that BYPASS is 0 and DISCARD is 1, which is the reverse of the Ze
// enum's neighbors, so a pass-through of the numeric value would swap the two.

//go:build ze_vpp

package dataplane

import (
	"testing"

	"go.fd.io/govpp/binapi/ipsec_types"
)

// VALIDATES: RFC4301-7.4-1. Each of the three Ze dispositions maps onto its own VPP
// SPD action, and an unknown one is refused rather than defaulted.
// PREVENTS: a silent downgrade. vppSPDAction hardcoded PROTECT once, and a bypass
// then reached VPP as a protect policy and black-holed the traffic it was meant to
// let through; the same mistake over a discard would pass the traffic the operator
// asked to stop.
// RFC requirement: RFC4301-7.4-1 positive -- a discard entry reaches VPP as a discard.
func TestVPPSPDActionCarriesEveryDisposition(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   SPAction
		want ipsec_types.IpsecSpdAction
	}{
		{"protect", SPActionProtect, ipsec_types.IPSEC_API_SPD_ACTION_PROTECT},
		{"bypass", SPActionBypass, ipsec_types.IPSEC_API_SPD_ACTION_BYPASS},
		{"discard", SPActionDiscard, ipsec_types.IPSEC_API_SPD_ACTION_DISCARD},
	} {
		got, err := vppSPDAction(tc.in)
		if err != nil {
			t.Errorf("%s: vppSPDAction: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: action = %v, want %v", tc.name, got, tc.want)
		}
	}

	// The negative half. A disposition this backend cannot express is refused, never
	// mapped onto whichever action a default happened to name.
	if _, err := vppSPDAction(SPAction(200)); err == nil {
		t.Error("vppSPDAction accepted an unknown disposition; it must refuse rather than pick one")
	}
}
