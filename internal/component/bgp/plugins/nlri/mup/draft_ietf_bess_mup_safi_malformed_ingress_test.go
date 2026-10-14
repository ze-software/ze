package mup

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
)

// These tests drive the recognizer this plugin's init registers, through
// nlritype.Retain, the call the reactor's typed NLRI pass makes before an UPDATE is
// installed or relayed (session_validation_nlritype.go, typedNLRIEdit).

// mupRD is RD 65001:100 (type 0), the Route Distinguisher of every route below.
var mupRD = []byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 0x64}

// mupBody joins an RD and the route type specific octets that follow it.
func mupBody(parts ...[]byte) []byte {
	return append(bytes.Clone(mupRD), bytes.Join(parts, nil)...)
}

// isdRoute is an Interwork Segment Discovery route (Section 3.1.1): prefix length,
// then the prefix octets.
func isdRoute(prefixBits byte, prefix ...byte) []byte {
	return mupWireNLRI(1, uint16(MUPISD), mupBody([]byte{prefixBits}, prefix)...)
}

// dsdRoute is a Direct Segment Discovery route (Section 3.1.2): the address alone.
func dsdRoute(address ...byte) []byte {
	return mupWireNLRI(1, uint16(MUPDSD), mupBody(address)...)
}

// t1stRoute is an IPv4 Type 1 Session Transformed route (Sections 3.1.3, 3.1.3.1):
// prefix 10.0.0.1/32 unless prefix is given, the TEID, QFI 9, then the endpoint
// octets (Endpoint Address Length, Endpoint Address, optional Source Address Length
// and Source Address, TLVs) exactly as given.
func t1stRoute(prefix, teid []byte, endpoint ...byte) []byte {
	if prefix == nil {
		prefix = []byte{32, 10, 0, 0, 1}
	}
	return mupWireNLRI(1, uint16(MUPT1ST), mupBody(prefix, teid, []byte{9}, endpoint)...)
}

// t2stRoute is a Type 2 Session Transformed route (Sections 3.1.4, 3.1.4.1): the
// Endpoint Length, then the endpoint address, TEID and TLV octets exactly as given.
func t2stRoute(endpointBits byte, rest ...byte) []byte {
	return mupWireNLRI(1, uint16(MUPT2ST), mupBody([]byte{endpointBits}, rest)...)
}

var (
	teid1     = []byte{0, 0, 0, 1}
	endpoint4 = []byte{32, 192, 0, 2, 1}
	ipv6Addr  = []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
)

// mupSiblings are the two well-formed routes each malformed route sits between.
func mupSiblings(fam Family) (l, l2 []byte) {
	if fam == IPv6MUP {
		return isdRoute(32, 0x20, 0x01, 0x0d, 0xb8), isdRoute(48, 0x20, 0x01, 0x0d, 0xb8, 0, 1)
	}
	return isdRoute(24, 10, 0, 0), isdRoute(24, 10, 0, 1)
}

// mupMalformed is one route ParseMUP refuses. unkeyable marks a route whose key
// cannot form: no withdrawal can name it, so it is discarded instead.
type mupMalformed struct {
	name      string
	fam       Family
	route     []byte
	unkeyable bool
}

// mupMalformedRoutes lists one route per malformed shape the draft answers with
// treat-as-withdraw.
func mupMalformedRoutes() []mupMalformed {
	return []mupMalformed{
		{"ISD IPv4 prefix length 33", IPv4MUP, isdRoute(33, 10, 0, 0, 0, 0), false},
		{"ISD IPv6 prefix length 129", IPv6MUP, isdRoute(129, append(bytes.Clone(ipv6Addr), 0)...), true},
		{"DSD IPv4 with a 16-octet address", IPv4MUP, dsdRoute(ipv6Addr...), false},
		{"DSD IPv6 with a 4-octet address", IPv6MUP, dsdRoute(192, 0, 2, 1), false},
		{"T1ST IPv4 prefix length 33", IPv4MUP, t1stRoute([]byte{33, 10, 0, 0, 1, 0}, teid1, endpoint4...), false},
		{"T1ST TEID 0", IPv4MUP, t1stRoute(nil, []byte{0, 0, 0, 0}, endpoint4...), false},
		{"T1ST Endpoint Address Length 24", IPv4MUP, t1stRoute(nil, teid1, 24, 192, 0, 2, 1), false},
		{"T1ST Source Address Length 16", IPv4MUP, t1stRoute(nil, teid1, append(bytes.Clone(endpoint4), 16, 10, 1)...), false},
		{"T2ST IPv4 Endpoint Length 65", IPv4MUP, t2stRoute(65, 192, 0, 2, 1, 0, 0, 0, 1, 0x80), false},
		{"T2ST IPv6 Endpoint Length 161", IPv6MUP, t2stRoute(161, append(bytes.Clone(ipv6Addr), 0, 0, 0, 1, 0x80)...), true},
		{"T2ST TEID 0", IPv4MUP, t2stRoute(64, 192, 0, 2, 1, 0, 0, 0, 0), false},
		{"T2ST TEID overruns the Length", IPv4MUP, t2stRoute(64, 192, 0, 2, 1, 0, 1), true},
		{"T2ST one octet after the TEID", IPv4MUP, t2stRoute(64, 192, 0, 2, 1, 0, 0, 0, 1, 0xf0), false},
	}
}

// TestMUPMalformedRouteTreatedAsWithdrawnAtIngress pins the receive answer to every
// malformed route type body ParseMUP refuses (owner ruling Q-1 (a),
// spec-bgp-update-propagation-rfc-defects).
//
// VALIDATES: each malformed route leaves the received section and is withdrawn (or,
// when its key cannot form, discarded), and the well-formed routes on either side of
// it are kept, in order, byte for byte.
// PREVENTS: a malformed MUP route being stored or relayed, or ending the processing
// of the routes after it.
func TestMUPMalformedRouteTreatedAsWithdrawnAtIngress(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1 positive -- an ISD route whose Prefix Length is 33 under AFI IPv4 leaves the received section and is withdrawn, and one whose Prefix Length is 129 under AFI IPv6 leaves it and is discarded because no route key forms; the routes beside it are kept
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2 positive -- with a malformed route between two well-formed ones, the ingress pass removes only the malformed route and keeps both others in order, byte for byte, for every malformed shape in the table
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1 positive -- a DSD route carrying a 16-octet address under AFI IPv4, or a 4-octet address under AFI IPv6, leaves the received section and is withdrawn
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1 positive -- a Type 1 ST route whose Prefix Length is 33 under AFI IPv4 leaves the received section and is withdrawn
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1 positive -- a Type 1 ST route whose TEID is 0 leaves the received section and is withdrawn
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2 positive -- a Type 1 ST route whose Endpoint Address Length is 24 leaves the received section and is withdrawn
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3 positive -- a Type 1 ST route whose Source Address Length is 16 leaves the received section and is withdrawn
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1 positive -- a Type 2 ST route whose Endpoint Length is 65 under AFI IPv4 leaves the received section and is withdrawn, and one whose Endpoint Length is 161 under AFI IPv6 leaves it and is discarded because no route key forms; the routes beside it are kept
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1 positive -- a Type 2 ST route whose 4-octet TEID is 0 leaves the received section and is withdrawn
	for _, tc := range mupMalformedRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			recognize := nlritype.Get(tc.fam)
			require.NotNil(t, recognize, "the mup plugin registers its ingress recognizer for %s", tc.fam)
			l, l2 := mupSiblings(tc.fam)
			var scratch [nlrisplit.PrefixKeyScratchSize]byte
			r, err := nlritype.Retain(recognize, tc.fam, concatNLRI(l, tc.route, l2), false, true, scratch[:])
			require.NoError(t, err, "a well-framed section is never an error")
			assert.Equal(t, concatNLRI(l, l2), r.Kept, "only the malformed route leaves the section")
			if tc.unkeyable {
				assert.Equal(t, tc.route, r.Unkeyable)
				assert.Empty(t, r.Withdrawn)
				return
			}
			assert.Equal(t, tc.route, r.Withdrawn, "the malformed route is withdrawn")
			assert.Equal(t, 1, r.WithdrawnCount)
			assert.Zero(t, r.Discarded)
		})
	}
}

// TestMUPRecognizerVerdicts pins the registered recognizer's three answers and the
// cost of reaching them (R-8, spec-bgp-update-propagation-rfc-defects).
//
// VALIDATES: Keep for a well-formed route, with and without an unknown TLV; Discard
// for an unimplemented route type; Withdraw for every malformed shape, including the
// ones whose key cannot form; a Type 2 ST route with a 160-bit Endpoint Length, the
// 32-octet key, keys in a PrefixKeyScratchSize scratch; the verdict and the key check
// together allocate nothing.
// PREVENTS: the receive path allocating per MUP route, or a valid route judged
// unkeyable because the scratch was too short.
func TestMUPRecognizerVerdicts(t *testing.T) {
	t2st160 := t2stRoute(160, append(bytes.Clone(ipv6Addr), 0, 0, 0, 1)...)
	type verdictCase struct {
		name  string
		fam   Family
		route []byte
		want  nlritype.Verdict
	}
	cases := []verdictCase{
		{"T1ST well-formed", IPv4MUP, t1stRoute(nil, teid1, endpoint4...), nlritype.Keep},
		{"T1ST with an unknown TLV", IPv4MUP, t1stRoute(nil, teid1, append(bytes.Clone(endpoint4), 0, 0xf0, 1, 0xaa)...), nlritype.Keep},
		{"T2ST IPv6 Endpoint Length 160", IPv6MUP, t2st160, nlritype.Keep},
		{"route type 5", IPv4MUP, mupWireNLRI(1, 5, mupRD...), nlritype.Discard},
		{"T1ST IPv6 Prefix Length 129", IPv6MUP, t1stRoute(append([]byte{129}, make([]byte, 17)...), teid1, endpoint4...), nlritype.Withdraw},
		{"ISD octet past the prefix", IPv4MUP, isdRoute(24, 10, 0, 0, 0xff), nlritype.Withdraw},
	}
	for _, m := range mupMalformedRoutes() {
		cases = append(cases, verdictCase{m.name, m.fam, m.route, nlritype.Withdraw})
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recognize := nlritype.Get(tc.fam)
			require.NotNil(t, recognize)
			assert.Equal(t, tc.want, recognize(tc.route, false))
			key := nlrisplit.GetPrefixKey(tc.fam)
			require.NotNil(t, key)
			allocs := testing.AllocsPerRun(100, func() {
				recognize(tc.route, false)
				_, _ = key(tc.route, scratch[:], true) //nolint:errcheck // only the cost is measured here
			})
			assert.Zero(t, allocs, "the verdict and its key check allocate nothing")
		})
	}
	t.Run("160-bit T2ST keys", func(t *testing.T) {
		got, err := nlrisplit.GetPrefixKey(IPv6MUP)(t2st160, scratch[:], true)
		require.NoError(t, err, "the 32-octet key fits the PrefixKeyScratchSize scratch")
		assert.Len(t, got, 32)
	})
}

// TestMUPWellFormedRouteKeptAtIngress is the other side of each boundary
// TestMUPMalformedRouteTreatedAsWithdrawnAtIngress crosses: the largest legal
// length, every legal address size, a non-zero TEID.
//
// VALIDATES: each route at a legal boundary is kept between its two siblings, and
// the section leaves the ingress pass whole, with nothing withdrawn or discarded.
// PREVENTS: a check that withdraws every route of a type passing as the rule.
func TestMUPWellFormedRouteKeptAtIngress(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1 negative -- an ISD route whose Prefix Length is 32 under AFI IPv4, or 128 under AFI IPv6, is kept, and the section is returned whole
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2 negative -- a section of well-formed routes, one of each legal boundary shape in the table between two well-formed siblings, leaves the ingress pass whole with nothing withdrawn or discarded
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1 negative -- a DSD route carrying a 4-octet address under AFI IPv4, or a 16-octet address under AFI IPv6, is kept
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1 negative -- a Type 1 ST route whose Prefix Length is 32 under AFI IPv4 is kept
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1 negative -- a Type 1 ST route whose TEID is 1 is kept
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2 negative -- a Type 1 ST route whose Endpoint Address Length is 32 with an IPv4 endpoint, or 128 with an IPv6 endpoint, is kept
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3 negative -- a Type 1 ST route whose Source Address Length is 0, 32 with an IPv4 source, or 128 with an IPv6 source, is kept
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1 negative -- a Type 2 ST route whose Endpoint Length is 64 under AFI IPv4, or 160 under AFI IPv6, is kept
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1 negative -- a Type 2 ST route whose 4-octet TEID is 1 is kept, and so is one that carries no TEID
	cases := []struct {
		name  string
		fam   Family
		route []byte
	}{
		{"ISD IPv4 prefix length 32", IPv4MUP, isdRoute(32, 10, 0, 0, 1)},
		{"ISD IPv6 prefix length 128", IPv6MUP, isdRoute(128, ipv6Addr...)},
		{"DSD IPv4 with a 4-octet address", IPv4MUP, dsdRoute(192, 0, 2, 1)},
		{"DSD IPv6 with a 16-octet address", IPv6MUP, dsdRoute(ipv6Addr...)},
		{"T1ST IPv4 prefix length 32, TEID 1, IPv4 endpoint", IPv4MUP, t1stRoute(nil, teid1, endpoint4...)},
		{"T1ST IPv6 endpoint", IPv4MUP, t1stRoute(nil, teid1, append([]byte{128}, ipv6Addr...)...)},
		{"T1ST Source Address Length 0", IPv4MUP, t1stRoute(nil, teid1, append(bytes.Clone(endpoint4), 0)...)},
		{"T1ST IPv4 source", IPv4MUP, t1stRoute(nil, teid1, append(bytes.Clone(endpoint4), 32, 10, 0, 0, 9)...)},
		{"T1ST IPv6 source", IPv4MUP, t1stRoute(nil, teid1, append(append(bytes.Clone(endpoint4), 128), ipv6Addr...)...)},
		{"T2ST IPv4 Endpoint Length 64, TEID 1", IPv4MUP, t2stRoute(64, 192, 0, 2, 1, 0, 0, 0, 1)},
		{"T2ST IPv6 Endpoint Length 160", IPv6MUP, t2stRoute(160, append(bytes.Clone(ipv6Addr), 0, 0, 0, 1)...)},
		{"T2ST without a TEID", IPv4MUP, t2stRoute(32, 192, 0, 2, 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recognize := nlritype.Get(tc.fam)
			require.NotNil(t, recognize, "the mup plugin registers its ingress recognizer for %s", tc.fam)
			l, l2 := mupSiblings(tc.fam)
			section := concatNLRI(l, tc.route, l2)
			var scratch [nlrisplit.PrefixKeyScratchSize]byte
			r, err := nlritype.Retain(recognize, tc.fam, section, false, true, scratch[:])
			require.NoError(t, err)
			assert.Equal(t, section, r.Kept, "a well-formed route is kept")
			assert.Empty(t, r.Withdrawn)
			assert.Empty(t, r.Unkeyable)
			assert.Zero(t, r.Discarded)
		})
	}
}
