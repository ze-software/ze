package srpolicy

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRFC9830BindingSIDSRv6Encode pins the type-13 Binding SID sub-TLV carrying an
// SRv6 BSID, the backward-compatible form RFC 9830 Section 2.4.2 retains beside the
// RECOMMENDED type-20 sub-TLV. Method: build the Tunnel Encapsulation attribute from
// a route config through parseConfigRoute, the parser the config and `update text`
// paths call, and compare its octets with the RFC layout written out by hand.
func TestRFC9830BindingSIDSRv6Encode(t *testing.T) {
	t.Parallel()

	got := attr(t, "distinguisher 0 color 100 endpoint 10.0.0.1 binding-sid srv6 2001:db8::b5")

	// Tunnel Type 15, TLV length 20; sub-TLV Type 13, Length 18, Flags 0,
	// RESERVED 0, then the 16 octets of 2001:db8::b5.
	want := "000f0014" + "0d120000" + "20010db80000000000000000000000b5"

	// RFC requirement: RFC9830-2.4.2-12 positive -- an SRv6 BSID in the Binding SID sub-TLV is written with Length 18: Flags, RESERVED and the 16-octet SID, octet for octet
	assert.Equal(t, want, hex.EncodeToString(got))

	// RFC requirement: RFC9830-2.4.2-12 negative -- the same sub-TLV carrying an SR-MPLS BSID is written with Length 6, so the 18 is the SRv6 form's length and not a constant
	mpls := attr(t, "distinguisher 0 color 100 endpoint 10.0.0.1 binding-sid mpls 24000")
	assert.Equal(t, "000f0008"+"0d060000"+"05dc0000", hex.EncodeToString(mpls))

	// The operator chose the type-13 form explicitly, so no type-20 sub-TLV is added,
	// and srv6-binding-sid alone still writes only the RECOMMENDED type 20.
	assert.Empty(t, srpAll(srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 binding-sid srv6 2001:db8::b5"), subTLVSRv6BindingSID))
	assert.Empty(t, srpAll(srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 srv6-binding-sid 2001:db8::b5"), subTLVBindingSID))
}

// TestRFC9830BindingSIDSRv6Refused pins that 'binding-sid srv6' takes only an IPv6
// address: an SRv6 SID is 128 bits, so anything else is refused before any UPDATE is
// built, naming the token and the fix.
func TestRFC9830BindingSIDSRv6Refused(t *testing.T) {
	t.Parallel()

	for _, sid := range []string{"10.0.0.1", "::ffff:10.0.0.1", "fe80::1%eth0", "nonsense"} {
		_, err := parseConfigRoute(cr(strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1 binding-sid srv6 "+sid), "192.0.2.1", false))
		require.Error(t, err, sid)
		assert.Contains(t, err.Error(), "binding-sid srv6", sid)
	}
	_, err := parseConfigRoute(cr(strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1 binding-sid srv6"), "192.0.2.1", false))
	require.Error(t, err)
}

// TestRFC9830BindingSIDSRv6Decode pins the receive-side reading of the Binding SID
// sub-TLV that `ze bgp decode` shows. Method: hand-built Tunnel Encapsulation
// attributes whose type-13 value is 18, 6, 2 and 20 octets.
func TestRFC9830BindingSIDSRv6Decode(t *testing.T) {
	t.Parallel()

	decode := func(h string) map[string]any {
		t.Helper()
		raw, err := hex.DecodeString(h)
		require.NoError(t, err)
		out := TunnelEncapToJSON(raw)
		require.NotNil(t, out)
		tunnels, ok := out["tunnels"].([]any)
		require.True(t, ok)
		require.Len(t, tunnels, 1)
		tunnel, ok := tunnels[0].(map[string]any)
		require.True(t, ok)
		subs, ok := tunnel["sub-tlvs"].([]any)
		require.True(t, ok)
		require.Len(t, subs, 1)
		sub, ok := subs[0].(map[string]any)
		require.True(t, ok)
		return sub
	}

	srv6 := decode("000f0014" + "0d120000" + "20010db80000000000000000000000b5")
	// RFC requirement: RFC9830-2.4.2-12 positive -- a received Binding SID sub-TLV of Length 18 is read as the 16-octet SRv6 SID it carries
	assert.Equal(t, "2001:db8::b5", srv6["srv6-sid"])
	assert.Nil(t, srv6["malformed"])

	mpls := decode("000f0008" + "0d060000" + "05dc0000")
	assert.Equal(t, uint32(24000), mpls["mpls-label"])
	assert.Nil(t, mpls["srv6-sid"])

	none := decode("000f0004" + "0d020000")
	assert.Nil(t, none["srv6-sid"])
	assert.Nil(t, none["mpls-label"])
	assert.Nil(t, none["malformed"])

	// RFC requirement: RFC9830-2.4.2-12 negative -- a Binding SID sub-TLV holding the same 16 SID octets plus two more (Length 20) is none of the defined lengths, so it is shown malformed and never as an SRv6 SID
	long := decode("000f0016" + "0d140000" + "20010db80000000000000000000000b5" + "0000")
	assert.Equal(t, true, long["malformed"])
	assert.Nil(t, long["srv6-sid"])
	assert.Equal(t, "000020010db80000000000000000000000b50000", long["value"])
}
