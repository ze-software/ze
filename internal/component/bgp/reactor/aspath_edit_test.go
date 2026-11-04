// Design: docs/architecture/core-design.md -- policy filter wire-level dirty tracking
// Related: aspath_edit.go -- prependASPathOps, removePrivateASOps under test
// Related: policy_test_helpers_test.go -- prependFixture

package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// aspathOp answers the bytes of the last Set or Prepend recorded on code, and
// whether one was.
func aspathOp(mods *filterapi.ModAccumulator, code attribute.AttributeCode) ([]byte, bool) {
	var buf []byte
	found := false
	for _, op := range mods.Ops() {
		if op.Code == byte(code) {
			buf, found = op.Buf, true
		}
	}
	return buf, found
}

// TestPrependASPathOpsAtTheTwoOctetBoundary drives the library prepend's wire
// half at two-octet width on both sides of the mappable boundary. 65535 is the
// last mappable AS: it is written as itself and owes no AS4_PATH. 65536 is the
// first that is not: it is written as AS_TRANS, and the real value travels in
// the AS4_PATH beside it.
//
// RFC 6793 Section 4.2.2: "In the AS_PATH attribute encoded with two-octet
// AS numbers, non-mappable four-octet AS numbers are represented by the
// well-known two-octet AS number, AS_TRANS."
//
// VALIDATES: the mappable boundary sits between 65535 and 65536.
// PREVENTS: an off-by-one that writes 65535 as AS_TRANS, or 65536 truncated.
func TestPrependASPathOpsAtTheTwoOctetBoundary(t *testing.T) {
	cases := []struct {
		name    string
		localAS uint32
		wantAS  []byte
		wantAS4 []byte // nil: no AS4_PATH
	}{
		{"65535 is mappable", 65535, []byte{byte(attribute.ASSequence), 1, 0xff, 0xff}, nil},
		{"65536 becomes AS_TRANS", 65536, []byte{byte(attribute.ASSequence), 1, 0x5b, 0xa0},
			[]byte{byte(attribute.ASSequence), 2, 0, 1, 0, 0, 0, 0, 0xfb, 0xf0}}, // 65536, 64496
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs, _ := prependFixture([]byte{byte(attribute.ASSequence), 1, 0xfb, 0xf0}, nil)
			var mods filterapi.ModAccumulator
			prependASPathOps(newTestScratch(t), attrs, false, tc.localAS, 1, &mods)

			if got, ok := aspathOp(&mods, attribute.AttrASPath); !ok || !bytes.Equal(got, tc.wantAS) {
				t.Fatalf("AS_PATH prepend = %x (recorded %v), want %x", got, ok, tc.wantAS)
			}
			got, ok := aspathOp(&mods, attribute.AttrAS4Path)
			if tc.wantAS4 == nil {
				if ok {
					t.Fatalf("AS4_PATH %x recorded for a mappable AS", got)
				}
				return
			}
			if !ok || !bytes.Equal(got, tc.wantAS4) {
				t.Fatalf("AS4_PATH = %x (recorded %v), want %x", got, ok, tc.wantAS4)
			}
		})
	}
}

// TestPrependASPathOpsAS4PathDerivation covers the AS4_PATH the prepend owes
// at two-octet width: none for a mappable local AS, a received AS4_PATH
// extended by the real local AS, and the whole path when the received
// AS4_PATH is shorter than AS_PATH.
//
// RFC 6793 Section 4.2.2: "The NEW BGP speaker MUST also send the AS path
// information in the AS4_PATH attribute (encoded with four-octet AS numbers),
// except for the case where all of the AS path information is composed of
// mappable four-octet AS numbers only. In this case, the NEW BGP speaker MUST
// NOT send the AS4_PATH attribute."
//
// VALIDATES: the AS4_PATH is withheld, extended or rebuilt whole.
// PREVENTS: a two-octet peer reading AS_TRANS with the real AS lost, or an
// AS4_PATH the RFC 6793 Section 4.2.3 reconstruction misreads.
func TestPrependASPathOpsAS4PathDerivation(t *testing.T) {
	cases := []struct {
		name    string
		localAS uint32
		count   uint8
		asPath  []byte
		as4Path []byte
		wantAS  []byte
		wantAS4 []byte // nil: no AS4_PATH
	}{
		{
			name: "mappable local AS owes no AS4_PATH", localAS: 65000, count: 3,
			asPath: []byte{byte(attribute.ASSequence), 1, 0xfb, 0xf0},
			wantAS: []byte{byte(attribute.ASSequence), 3, 0xfd, 0xe8, 0xfd, 0xe8, 0xfd, 0xe8},
		},
		{
			name: "extends a received AS4_PATH", localAS: 131072, count: 1,
			asPath:  []byte{byte(attribute.ASSequence), 1, 0x5b, 0xa0}, // AS_TRANS from the sender
			as4Path: []byte{byte(attribute.ASSequence), 1, 0, 2, 0, 1}, // 131073
			wantAS:  []byte{byte(attribute.ASSequence), 1, 0x5b, 0xa0},
			wantAS4: []byte{byte(attribute.ASSequence), 2, 0, 2, 0, 0, 0, 2, 0, 1}, // 131072, 131073
		},
		{
			name: "a shorter received AS4_PATH carries the whole path", localAS: 131072, count: 1,
			asPath: []byte{byte(attribute.ASSequence), 3, 0xfb, 0xf4, 0x5b, 0xa0, 0xfb, 0xf0}, // 64500, AS_TRANS, 64496
			as4Path: []byte{byte(attribute.ASSequence), 2,
				0, 2, 0, 1, // 131073
				0, 0, 0xfb, 0xf0}, // 64496
			wantAS: []byte{byte(attribute.ASSequence), 1, 0x5b, 0xa0},
			wantAS4: []byte{byte(attribute.ASSequence), 4,
				0, 2, 0, 0, // 131072, the real local AS
				0, 0, 0xfb, 0xf4, // 64500, from the leading part of AS_PATH
				0, 2, 0, 1, // 131073
				0, 0, 0xfb, 0xf0}, // 64496
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs, _ := prependFixture(tc.asPath, tc.as4Path)
			var mods filterapi.ModAccumulator
			prependASPathOps(newTestScratch(t), attrs, false, tc.localAS, tc.count, &mods)

			if got, ok := aspathOp(&mods, attribute.AttrASPath); !ok || !bytes.Equal(got, tc.wantAS) {
				t.Fatalf("AS_PATH prepend = %x (recorded %v), want %x", got, ok, tc.wantAS)
			}
			got, ok := aspathOp(&mods, attribute.AttrAS4Path)
			if tc.wantAS4 == nil {
				if ok {
					t.Fatalf("AS4_PATH %x recorded for an all-mappable path", got)
				}
				return
			}
			if !ok || !bytes.Equal(got, tc.wantAS4) {
				t.Fatalf("AS4_PATH = %x (recorded %v), want %x", got, ok, tc.wantAS4)
			}
		})
	}
}

// TestPrependASPathOpsFailsClosed proves the prepend records nothing when the
// AS4_PATH it owes cannot be derived: a malformed AS_PATH, or no attribute
// section at all. Recording the AS_PATH alone would put AS_TRANS on the wire
// with the real local AS carried nowhere.
//
// RFC 6793 Section 4.2.2: "The NEW BGP speaker MUST also send the AS path
// information in the AS4_PATH attribute (encoded with four-octet AS numbers)".
//
// VALIDATES: no half-formed AS-path family is recorded.
// PREVENTS: an AS_TRANS prepend with no AS4_PATH beside it.
func TestPrependASPathOpsFailsClosed(t *testing.T) {
	// An AS_SEQUENCE header claiming five AS numbers over two octets of value.
	malformed, _ := prependFixture([]byte{byte(attribute.ASSequence), 5, 0xfb, 0xf0}, nil)
	for name, attrs := range map[string]*attribute.AttributesWire{"malformed AS_PATH": malformed, "no attribute section": nil} {
		t.Run(name, func(t *testing.T) {
			var mods filterapi.ModAccumulator
			prependASPathOps(newTestScratch(t), attrs, false, 131072, 2, &mods)
			if mods.Len() != 0 {
				t.Fatalf("recorded %d operations, want none", mods.Len())
			}
		})
	}
}

// TestLibraryRemovePrivateRunsBeforePrepend drives a library chain member that
// asks remove-private strip and a prepend of one, toward a two-octet
// destination, and reads the wire the export step sends. remove-private runs
// first (libraryASPathOps), and the prepend's AS4_PATH is derived from what it
// left, so the private AS 64512 is not restored, and the real local AS leads
// the AS4_PATH. The unit half calls the two producers in the same order.
//
// RFC 6996 Section 4: "Private Use ASNs MUST be removed from AS path attributes
// (including AS4_PATH if utilizing a four-octet AS number space) before being
// advertised to the global Internet."
//
// VALIDATES: the prepend's AS4_PATH reads the AS-path family remove-private
// recorded.
// PREVENTS: a private ASN leaking back through the prepend's AS4_PATH.
func TestLibraryRemovePrivateRunsBeforePrepend(t *testing.T) {
	// AS_PATH (two-octet): 64496 public, 64512 private.
	// AS4_PATH: 64512 private, 131073 public and non-mappable.
	asPath := []byte{byte(attribute.ASSequence), 2, 0xfb, 0xf0, 0xfc, 0x00}
	as4Path := []byte{byte(attribute.ASSequence), 2, 0, 0, 0xfc, 0x00, 0, 2, 0, 1}
	wantAS := []byte{byte(attribute.ASSequence), 2, 0x5b, 0xa0, 0xfb, 0xf0}  // AS_TRANS, 64496
	wantAS4 := []byte{byte(attribute.ASSequence), 2, 0, 2, 0, 0, 0, 2, 0, 1} // 131072, 131073

	t.Run("producers", func(t *testing.T) {
		attrs, _ := prependFixture(asPath, as4Path)
		scratch := newTestScratch(t)
		var mods filterapi.ModAccumulator
		removePrivateASOps(scratch, attrs, false, policychain.RemovePrivateStrip, 65002, &mods)
		prependASPathOps(scratch, attrs, false, 131072, 1, &mods)
		if got, ok := aspathOp(&mods, attribute.AttrAS4Path); !ok || !bytes.Equal(got, wantAS4) {
			t.Fatalf("AS4_PATH = %x (recorded %v), want %x", got, ok, wantAS4)
		}
	})

	t.Run("library export", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
		libraryIntentEval = func(_ *policyview.View, s *policychain.Sets) error {
			if err := s.RemovePrivateAS(policychain.RemovePrivateStrip); err != nil {
				return err
			}
			return s.PrependASPath(1)
		}
		_, received := prependFixture(asPath, as4Path)
		policy := libraryPolicy(t, libraryIntentExport, libraryTypeIntents, routepolicy.FinalAccept)

		out := r.runEgressPolicyChainASN4(policy, "10.0.0.5", 65002, 131072, false,
			policyview.Session{EBGP: true}, testWireUpdate(received), false)

		if !out.accept || out.wireOverride == nil {
			t.Fatalf("export = %+v, want accept with the AS-path edits", out)
		}
		attrs := attrValues(t, out.wireOverride.Payload())
		if got := attrs[byte(attribute.AttrASPath)]; !bytes.Equal(got, wantAS) {
			t.Fatalf("AS_PATH = %x, want %x", got, wantAS)
		}
		if got := attrs[byte(attribute.AttrAS4Path)]; !bytes.Equal(got, wantAS4) {
			t.Fatalf("AS4_PATH = %x, want %x", got, wantAS4)
		}
	})
}

// TestRemovePrivateWireRewriteBoundaries preserves the wire parser's refusal
// boundary and every segment kind while the rewrite avoids a parsed path.
func TestRemovePrivateWireRewriteBoundaries(t *testing.T) {
	encode := func(asn4 bool, segments ...attribute.ASPathSegment) []byte {
		path := attribute.ASPath{Segments: segments}
		buf := make([]byte, path.LenWithASN4(asn4))
		path.WriteToWithASN4(buf, 0, asn4)
		return buf
	}
	atLimit := make([]uint32, attribute.MaxASPathTotalLength)
	for i := range atLimit {
		atLimit[i] = 64512
	}
	atLimit[0] = 64496
	overLimit := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xfc, 0}
	overLimit = append(encode(true, attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: atLimit}), overLimit...)
	cases := []struct {
		name    string
		value   []byte
		asn4    bool
		mode    policychain.RemovePrivateMode
		peerAS  uint32
		want    []byte
		changed bool
	}{
		{
			name: "segment-kinds", asn4: true, mode: policychain.RemovePrivateStrip, changed: true,
			value: encode(true,
				attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{64496, 64512}},
				attribute.ASPathSegment{Type: attribute.ASSet, ASNs: []uint32{64513}},
				attribute.ASPathSegment{Type: attribute.ASConfedSequence, ASNs: []uint32{64497, 4200000000}},
				attribute.ASPathSegment{Type: attribute.ASConfedSet, ASNs: []uint32{65535, 4294967294}}),
			want: encode(true,
				attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{64496}},
				attribute.ASPathSegment{Type: attribute.ASConfedSequence, ASNs: []uint32{64497}},
				attribute.ASPathSegment{Type: attribute.ASConfedSet, ASNs: []uint32{65535}}),
		},
		{
			name: "two-octet-peer-as", mode: policychain.RemovePrivatePeerAS, peerAS: 65536, changed: true,
			value: encode(false, attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{64512, 64496}}),
			want:  encode(false, attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{attribute.ASTrans, 64496}}),
		},
		{
			name: "all-private", mode: policychain.RemovePrivateStrip, changed: true,
			value: encode(false, attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{64512, 65534}}),
		},
		{
			name: "unchanged-empty-segment", asn4: true, mode: policychain.RemovePrivateStrip,
			value: append([]byte{byte(attribute.ASSequence), 0}, encode(true,
				attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{64511, 65535, 4199999999, 4294967295}})...),
		},
		{
			name: "truncated-tail", asn4: true, mode: policychain.RemovePrivateStrip,
			value: append(encode(true, attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{64512}}), byte(attribute.ASSequence)),
		},
		{
			name: "invalid-kind", asn4: true, mode: policychain.RemovePrivateStrip,
			value: []byte{5, 1, 0, 0, 0xfc, 0},
		},
		{
			name: "short-asn", asn4: true, mode: policychain.RemovePrivateStrip,
			value: []byte{byte(attribute.ASSequence), 1, 0xfc, 0},
		},
		{
			name: "maximum-count", asn4: true, mode: policychain.RemovePrivateStrip, changed: true,
			value: encode(true, attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: atLimit}),
			want:  encode(true, attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{64496}}),
		},
		{name: "over-maximum-count", value: overLimit, asn4: true, mode: policychain.RemovePrivateStrip},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := bytes.Clone(tc.value)
			got, changed := rewriteASPathRemovePrivate(newTestScratch(t), tc.value, tc.asn4, tc.mode, tc.peerAS)
			if changed != tc.changed || !bytes.Equal(got, tc.want) {
				t.Fatalf("rewrite = %x, changed %v; want %x, changed %v", got, changed, tc.want, tc.changed)
			}
			if !bytes.Equal(tc.value, original) {
				t.Fatalf("rewrite modified the received AS_PATH: %x, want %x", tc.value, original)
			}
		})
	}
}
