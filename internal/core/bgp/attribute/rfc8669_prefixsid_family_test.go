// Design: docs/architecture/wire/attributes.md -- BGP Prefix-SID receive actions per family
// Overview: prefixsid_classify.go -- ClassifyPrefixSID, the family-aware verdict

package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParsePrefixSIDAcceptsOffFamilyLabelIndexLength states the family split: a
// type 1 or type 3 TLV whose length breaks RFC 8669 Sections 3.1 and 3.2 is
// unrecognized off labeled unicast, so the parser every reader goes through
// keeps it, and the JSON renders it as the unknown TLV it is. A SID Structure
// whose length is not 6 is likewise kept and rendered as hex.
//
// RFC requirement: RFC8669-3.1-2 positive -- a 6-octet Label-Index TLV, ignored off labeled unicast, is accepted by ParsePrefixSID, re-emitted octet-equal, and rendered as attribute-not-implemented-1 rather than sr-label-index.
// RFC requirement: RFC8669-3.2-4 positive -- a 3-octet Originator SRGB TLV, ignored off labeled unicast, is accepted by ParsePrefixSID and rendered as attribute-not-implemented-3.
func TestParsePrefixSIDAcceptsOffFamilyLabelIndexLength(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ wire, json string }{
		"label-index of 6": {"010006000000000064", `{"attribute-not-implemented-1":"000000000064"}`},
		"srgb of 3":        {"030003000000", `{"attribute-not-implemented-3":"000000"}`},
		"sid structure of 5": {
			"050021" + "00" + "01001d" + "00" + "20010db8000100010000000000000000" + "00" + "0048" + "00" + "010005" + "2018100000",
			`{"l3-service":[{"sid":"2001:db8:1:1::","flags":0,"endpoint_behavior":72,"sub-sub-tlv-1":"2018100000"}]}`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			wire := mustHex(t, tc.wire)
			sid, err := ParsePrefixSID(wire)
			require.NoError(t, err)
			out := make([]byte, sid.Len())
			sid.WriteTo(out, 0)
			assert.Equal(t, wire, out, "the attribute must be re-emitted unmodified")
			assert.Equal(t, tc.json, renderPrefixSID(t, wire))
		})
	}
}

// TestClassifyPrefixSIDAllocatesNothing pins the receive-path cost: a
// well-formed value on labeled unicast and on IPv4 VPN is judged without an
// allocation.
func TestClassifyPrefixSIDAllocatesNothing(t *testing.T) {
	labeled := mustHex(t, "010007"+"00000000000064"+"030008"+"0000"+"003e80"+"0003e8")
	vpn := mustHex(t, "050019"+"00"+"010015"+"00"+"20010db8000100010000000000000000"+"00"+"0048"+"00")
	for name, tc := range map[string]struct {
		value []byte
		safi  SAFI
	}{"labeled": {labeled, SAFIMPLSLabel}, "vpn": {vpn, SAFIVPN}} {
		verdict, err := ClassifyPrefixSID(tc.value, AFIIPv4, tc.safi)
		require.NoError(t, err, name)
		require.Equal(t, PrefixSIDWellFormed, verdict, name)
		allocs := testing.AllocsPerRun(100, func() {
			_, _ = ClassifyPrefixSID(tc.value, AFIIPv4, tc.safi)
		})
		assert.Zero(t, allocs, name)
	}
}
