package mup

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/locatororigin"
	"github.com/ze-software/ze/internal/core/report"
)

// These tests drive the UPDATE-level check this plugin's init registers, through
// nlritype.Rulings and nlritype.RetainUpdate, the calls the reactor's typed NLRI pass
// makes (session_validation_nlritype.go, typedNLRIEdit). The receive path itself is
// covered in the reactor package with whole UPDATEs.

// dsdAttrsNoPrefixSID is ORIGIN IGP and an empty AS_PATH: no Prefix-SID attribute.
var dsdAttrsNoPrefixSID = []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00}

// dsdAttrsPrefixSID adds a Prefix-SID attribute (type 40) holding an SRv6 L3 Service
// TLV (type 5, RFC 9252 Section 2) with one SRv6 SID Information Sub-TLV for 2001:db8::1.
var dsdAttrsPrefixSID = append(append([]byte{}, dsdAttrsNoPrefixSID...),
	0xc0, 0x28, 0x1c, // optional transitive, type 40, length 28
	0x05, 0x00, 0x19, 0x00, // SRv6 L3 Service TLV, length 25, reserved
	0x01, 0x00, 0x15, 0x00, // SRv6 SID Information Sub-TLV, length 21, reserved
	0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01, // SID
	0x00, 0x00, 0x13, 0x00, // flags, endpoint behavior End.DT4 (0x0013), reserved
)

// VALIDATES: draft-ietf-bess-mup-safi Section 3.3.6, "When a BGP speaker receives a
// MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without
// a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix
// SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied." The DSD
// route of an announcement with no Prefix-SID is withdrawn and its T1ST and T2ST
// siblings are kept; with a Prefix-SID every route is kept; a Prefix-SID that
// attribute-discard removes counts as absent. Both families, with and without ADD-PATH.
// PREVENTS: a DSD route with no Prefix-SID being installed and relayed, and the check
// reaching a route type neither Section 3.3.3 nor Section 3.3.6 names.
func TestMUPDSDWithoutPrefixSIDTreatAsWithdraw(t *testing.T) {
	t.Cleanup(locatororigin.ResetForTest)
	t.Cleanup(report.ResetForTest)
	locatororigin.ResetForTest()
	report.ResetForTest()

	cases := []struct {
		name     string
		fam      Family
		dsd      []byte
		siblings [][]byte
	}{
		{"IPv4", IPv4MUP, dsdRoute(192, 0, 2, 1), [][]byte{
			t1stRoute(nil, teid1, endpoint4NoSource...), t2stRoute(64, 192, 0, 2, 1, 0, 0, 0, 1),
		}},
		{"IPv6", IPv6MUP, dsdRoute(ipv6Addr...), [][]byte{
			t2stRoute(160, append(append([]byte{}, ipv6Addr...), 0, 0, 0, 1)...),
		}},
	}
	for _, tc := range cases {
		for _, addPath := range []bool{false, true} {
			name := tc.name
			if addPath {
				name += " ADD-PATH"
			}
			t.Run(name, func(t *testing.T) {
				recognize, check := nlritype.Rulings(tc.fam)
				require.NotNil(t, recognize, "the mup plugin registers its recognizer for %s", tc.fam)
				require.NotNil(t, check, "the mup plugin registers its UPDATE-level check for %s", tc.fam)

				frame := func(nlri []byte) []byte {
					if !addPath {
						return nlri
					}
					return concatNLRI([]byte{0, 0, 0, 7}, nlri)
				}
				siblings := make([]byte, 0)
				for _, s := range tc.siblings {
					siblings = concatNLRI(siblings, frame(s))
				}
				dsd := frame(tc.dsd)
				section := concatNLRI(siblings[:0:0], dsd, siblings)
				var scratch [nlrisplit.PrefixKeyScratchSize]byte

				without := nlritype.NewUpdateAttrs(dsdAttrsNoPrefixSID)
				r, err := nlritype.RetainUpdate(recognize, check, &without, tc.fam, section, addPath, true, scratch[:])
				require.NoError(t, err)
				assert.Equal(t, siblings, r.Kept, "only the DSD route leaves the announcement")
				assert.Equal(t, dsd, r.Withdrawn, "the DSD route is withdrawn, not discarded")
				assert.Zero(t, r.Discarded)

				with := nlritype.NewUpdateAttrs(dsdAttrsPrefixSID)
				r, err = nlritype.RetainUpdate(recognize, check, &with, tc.fam, section, addPath, true, scratch[:])
				require.NoError(t, err)
				assert.Equal(t, section, r.Kept, "a DSD route with a Prefix-SID is kept")
				assert.False(t, r.Removed())

				discarded := nlritype.NewUpdateAttrs(dsdAttrsPrefixSID)
				discarded.Discarding(attribute.AttrPrefixSID)
				r, err = nlritype.RetainUpdate(recognize, check, &discarded, tc.fam, section, addPath, true, scratch[:])
				require.NoError(t, err)
				assert.Equal(t, dsd, r.Withdrawn, "a Prefix-SID being discarded leaves the DSD route without one")

				r, err = nlritype.RetainUpdate(recognize, check, &without, tc.fam, section, addPath, false, nil)
				require.NoError(t, err)
				assert.False(t, r.Removed(), "a withdrawal needs no Prefix-SID")
			})
		}
	}
}

// VALIDATES: 16-octet and 32-octet IPv4-mapped IPv6 next hops (::ffff:x.x.x.x)
// against a proven locator originator withdraw the DSD route even when the
// originating LSP carries undecoded TLV 237, and keep the route with warning
// when the originator is unresolved.
// PREVENTS: ::ffff:x.x.x.x being misclassified as a native IPv6 address and
// slipping through as ReasonUndecodedTLV237 instead of MatchMismatch.
func TestMUPDSDIPv4MappedIPv6NextHopMismatch(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.55")
	nh16Mapped := netip.MustParseAddr("::ffff:192.0.2.1").AsSlice()
	nh32Mapped := append(append([]byte(nil), nh16Mapped...), netip.MustParseAddr("fe80::1").AsSlice()...)
	dsd := dsdRoute(192, 0, 2, 1)
	sibling := t2stRoute(64, 192, 0, 2, 1, 0, 0, 0, 1)
	section := concatNLRI(dsd, sibling)
	recognize, check := nlritype.Rulings(IPv4MUP)
	require.NotNil(t, recognize)
	require.NotNil(t, check)
	var scratch [nlrisplit.PrefixKeyScratchSize]byte

	cases := []struct {
		name      string
		nh        []byte
		hasTLV237 bool
	}{
		{name: "16-octet IPv4-mapped without TLV 237", nh: nh16Mapped, hasTLV237: false},
		{name: "16-octet IPv4-mapped with TLV 237", nh: nh16Mapped, hasTLV237: true},
		{name: "32-octet IPv4-mapped global half without TLV 237", nh: nh32Mapped, hasTLV237: false},
		{name: "32-octet IPv4-mapped global half with TLV 237", nh: nh32Mapped, hasTLV237: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capture, _ := setupMUPOriginTestEnv(t)
			seedMUPOriginSnapshot(locatororigin.LocatorEntry{
				Prefix:             netip.MustParsePrefix("2001:db8::/48"),
				Known:              true,
				Originator:         "0100.0000.0001",
				Addresses:          []netip.Addr{netip.MustParseAddr("2001:db8:140::1")},
				HasUndecodedTLV237: tc.hasTLV237,
			})
			attrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, tc.nh)
			r, err := nlritype.RetainUpdate(recognize, check, &attrs, IPv4MUP, section, false, true, scratch[:])
			require.NoError(t, err)
			assert.Equal(t, sibling, r.Kept)
			assert.Equal(t, dsd, r.Withdrawn)
			assert.Zero(t, r.Discarded)
			assert.Equal(t, 1, capture.countByCode(codeDoctorBGPMUPLocatorMismatch))
			assert.Equal(t, 0, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))
		})
	}

	t.Run("unresolved originator keeps 16-octet IPv4-mapped next hop with warning", func(t *testing.T) {
		capture, _ := setupMUPOriginTestEnv(t)
		attrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, nh16Mapped)
		r, err := nlritype.RetainUpdate(recognize, check, &attrs, IPv4MUP, dsd, false, true, scratch[:])
		require.NoError(t, err)
		assert.Equal(t, dsd, r.Kept)
		assert.False(t, r.Removed())
		assert.Equal(t, 1, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))
	})
}
