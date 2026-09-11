// RFC: rfc/short/rfc6793.md -- AS4_PATH and AS4_AGGREGATOR on the ORIGINATING rail
//
// Goal: prove that every UpdateBuilder entry point that writes an AS_PATH for a
// route ze originates also writes the AS4_PATH RFC 6793 Section 4.2.2 obliges
// beside it, and the AS4_AGGREGATOR beside a downgraded AGGREGATOR.
//
// Method: each table row drives one builder end to end and reads the attribute
// block it returns. The table enumerates EVERY builder rather than one of them,
// because the obligation is on the attribute block a peer receives and a builder
// nobody listed is how this defect reached the wire in the first place
// (plan/journal/requirement-met-on-the-rails-the-spec-planned.md).

package message

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

const (
	// rfc6793OriginNonMappable cannot be represented in two octets, so a
	// two-octet AS_PATH carrying it must substitute AS_TRANS.
	rfc6793OriginNonMappable uint32 = 4200000001
	rfc6793OriginASTrans     uint32 = 23456
	rfc6793OriginLocalAS     uint32 = 65001
	rfc6793OriginPeerAS      uint32 = 65002
)

// rfc6793Originator is one builder that writes an originated attribute block.
// configurable reports whether the builder takes an operator-supplied AS_PATH:
// BuildFlowSpec does not, so its path can only carry the local AS.
type rfc6793Originator struct {
	name         string
	configurable bool
	build        func(t *testing.T, ub *UpdateBuilder, asPath []uint32) []byte
}

// rfc6793Originators enumerates every UpdateBuilder entry point that writes an
// AS_PATH. Adding a builder without adding it here leaves that builder unproven.
var rfc6793Originators = []rfc6793Originator{
	{
		name:         "BuildUnicast",
		configurable: true,
		build: func(_ *testing.T, ub *UpdateBuilder, asPath []uint32) []byte {
			return ub.BuildUnicast(&UnicastParams{
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.0.2.1"),
				Origin:  attribute.OriginIGP,
				ASPath:  asPath,
			}).PathAttributes
		},
	},
	{
		name:         "BuildGroupedUnicast",
		configurable: true,
		build: func(t *testing.T, ub *UpdateBuilder, asPath []uint32) []byte {
			t.Helper()
			var attrs []byte
			routes := []UnicastParams{{
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.0.2.1"),
				Origin:  attribute.OriginIGP,
				ASPath:  asPath,
			}}
			err := ub.BuildGroupedUnicast(routes, wireStandardMaxForTest, func(u *Update) error {
				attrs = append(attrs, u.PathAttributes...)
				return nil
			})
			if err != nil {
				t.Fatalf("BuildGroupedUnicast: %v", err)
			}
			return attrs
		},
	},
	{
		name:         "BuildVPN",
		configurable: true,
		build: func(_ *testing.T, ub *UpdateBuilder, asPath []uint32) []byte {
			return ub.BuildVPN(&VPNParams{
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.0.2.1"),
				Labels:  []uint32{100},
				RDBytes: [8]byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 1},
				Origin:  attribute.OriginIGP,
				ASPath:  asPath,
			}).PathAttributes
		},
	},
	{
		name:         "BuildLabeledUnicast",
		configurable: true,
		build: func(_ *testing.T, ub *UpdateBuilder, asPath []uint32) []byte {
			return ub.BuildLabeledUnicast(&LabeledUnicastParams{
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.0.2.1"),
				Labels:  []uint32{100},
				Origin:  attribute.OriginIGP,
				ASPath:  asPath,
			}).PathAttributes
		},
	},
	{
		name:         "BuildEVPN",
		configurable: true,
		build: func(_ *testing.T, ub *UpdateBuilder, asPath []uint32) []byte {
			return ub.BuildEVPN(EVPNParams{
				NLRI:    []byte{3, 9, 0, 0, 0, 0, 0, 0, 0, 0, 1, 4, 192, 0, 2, 1},
				NextHop: netip.MustParseAddr("192.0.2.1"),
				Origin:  attribute.OriginIGP,
				ASPath:  asPath,
			}).PathAttributes
		},
	},
	{
		name:         "BuildFlowSpec",
		configurable: false,
		build: func(_ *testing.T, ub *UpdateBuilder, _ []uint32) []byte {
			return ub.BuildFlowSpec(FlowSpecParams{
				NLRI:    []byte{0x05, 0x01, 0x18, 10, 0, 0},
				NextHop: netip.MustParseAddr("192.0.2.1"),
			}).PathAttributes
		},
	},
	{
		name:         "BuildPlugin",
		configurable: true,
		build: func(_ *testing.T, ub *UpdateBuilder, asPath []uint32) []byte {
			return ub.BuildPlugin(PluginParams{
				AFI:     1,
				SAFI:    133,
				NLRI:    []byte{0x05, 0x01, 0x18, 10, 0, 0},
				NextHop: netip.MustParseAddr("192.0.2.1"),
				ASPath:  asPath,
			}).PathAttributes
		},
	},
}

// wireStandardMaxForTest is the RFC 4271 maximum message size, which leaves the
// grouped builder room for one route and its attributes.
const wireStandardMaxForTest = 4096

// asPathASNs returns the AS numbers of the AS_PATH attribute in attrs, read at
// the width the peer negotiated.
func asPathASNs(t *testing.T, attrs []byte, asn4 bool) []uint32 {
	t.Helper()
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrASPath)
	if !found {
		t.Fatal("no AS_PATH attribute in the originated block")
	}
	path, err := attribute.ParseASPath(value, asn4)
	if err != nil {
		t.Fatalf("ParseASPath: %v", err)
	}
	var asns []uint32
	for _, seg := range path.Segments {
		asns = append(asns, seg.ASNs...)
	}
	return asns
}

// as4PathASNs returns the AS numbers of the AS4_PATH attribute in attrs, or nil
// when the block carries none.
func as4PathASNs(t *testing.T, attrs []byte) []uint32 {
	t.Helper()
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrAS4Path)
	if !found {
		return nil
	}
	path, err := attribute.ParseAS4Path(value)
	if err != nil {
		t.Fatalf("ParseAS4Path: %v", err)
	}
	var asns []uint32
	for _, seg := range path.Segments {
		asns = append(asns, seg.ASNs...)
	}
	return asns
}

// equalASNs reports whether two AS number lists hold the same values in order.
func equalASNs(got, want []uint32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker drives every originating
// builder toward a peer that did not negotiate four-octet AS support, with a
// non-mappable AS number in the configured AS_PATH.
//
// RFC requirement: RFC6793-4.2.2-2 positive -- every originated UPDATE toward an OLD speaker
// whose AS path holds a non-mappable four-octet AS carries the AS path again in an AS4_PATH
// encoded with four-octet AS numbers, beside an AS_PATH carrying AS_TRANS.
// RFC requirement: RFC6793-4.2.2-3 negative -- the AS4_PATH suppression is conditional: an
// originated path that is not composed of mappable AS numbers only does get an AS4_PATH.
func TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker(t *testing.T) {
	configured := []uint32{rfc6793OriginNonMappable, rfc6793OriginPeerAS}
	wantASPath := []uint32{rfc6793OriginLocalAS, rfc6793OriginASTrans, rfc6793OriginPeerAS}
	wantAS4Path := []uint32{rfc6793OriginLocalAS, rfc6793OriginNonMappable, rfc6793OriginPeerAS}

	for _, originator := range rfc6793Originators {
		if !originator.configurable {
			continue
		}
		t.Run(originator.name, func(t *testing.T) {
			ub := NewUpdateBuilder(rfc6793OriginLocalAS, false, false, false)
			attrs := originator.build(t, ub, configured)

			if got := asPathASNs(t, attrs, false); !equalASNs(got, wantASPath) {
				t.Errorf("AS_PATH = %v, want %v (AS_TRANS for the non-mappable AS)", got, wantASPath)
			}
			got := as4PathASNs(t, attrs)
			if got == nil {
				t.Fatal("no AS4_PATH: the real four-octet AS number reaches the OLD speaker nowhere")
			}
			if !equalASNs(got, wantAS4Path) {
				t.Errorf("AS4_PATH = %v, want %v", got, wantAS4Path)
			}
		})
	}
}

// TestOriginatedUpdateCarriesAS4PathForANonMappableLocalAS covers the builder
// that takes no configured AS_PATH: BuildFlowSpec prepends the local AS alone,
// so a four-octet local AS is the only way its path becomes non-mappable. The
// other builders are driven the same way, because a four-octet local AS is the
// common case rather than a corner of one builder.
//
// RFC requirement: RFC6793-4.2.2-2 positive -- a four-octet local AS prepended to an originated
// route toward an OLD speaker is sent as AS_TRANS in AS_PATH and as its real value in AS4_PATH.
func TestOriginatedUpdateCarriesAS4PathForANonMappableLocalAS(t *testing.T) {
	for _, originator := range rfc6793Originators {
		t.Run(originator.name, func(t *testing.T) {
			ub := NewUpdateBuilder(rfc6793OriginNonMappable, false, false, false)
			attrs := originator.build(t, ub, nil)

			want := []uint32{rfc6793OriginASTrans}
			if got := asPathASNs(t, attrs, false); !equalASNs(got, want) {
				t.Errorf("AS_PATH = %v, want %v", got, want)
			}
			wantAS4 := []uint32{rfc6793OriginNonMappable}
			got := as4PathASNs(t, attrs)
			if got == nil {
				t.Fatal("no AS4_PATH: ze's own four-octet AS reaches the OLD speaker nowhere")
			}
			if !equalASNs(got, wantAS4) {
				t.Errorf("AS4_PATH = %v, want %v", got, wantAS4)
			}
		})
	}
}

// TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable drives the same builders
// toward the same OLD speaker with a path of mappable AS numbers only.
//
// RFC requirement: RFC6793-4.2.2-3 positive -- when all of the originated AS path information
// is composed of mappable four-octet AS numbers only, no AS4_PATH is sent to the OLD speaker.
// RFC requirement: RFC6793-4.2.2-2 negative -- the AS4_PATH is not written into every
// OLD-speaker UPDATE: with every AS mappable the attribute is absent.
func TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable(t *testing.T) {
	for _, originator := range rfc6793Originators {
		t.Run(originator.name, func(t *testing.T) {
			ub := NewUpdateBuilder(rfc6793OriginLocalAS, false, false, false)
			attrs := originator.build(t, ub, []uint32{rfc6793OriginPeerAS})

			if got := as4PathASNs(t, attrs); got != nil {
				t.Errorf("AS4_PATH = %v, want none: every AS number is mappable", got)
			}
		})
	}
}

// TestOriginatedUpdateOmitsAS4PathTowardNewSpeaker drives the same builders
// toward a peer that negotiated four-octet AS support.
//
// RFC requirement: RFC6793-4.1-6 positive -- an originated UPDATE toward a NEW speaker carries
// no AS4_PATH, whatever the AS numbers in its path, because the AS_PATH itself is four-octet.
func TestOriginatedUpdateOmitsAS4PathTowardNewSpeaker(t *testing.T) {
	for _, originator := range rfc6793Originators {
		t.Run(originator.name, func(t *testing.T) {
			ub := NewUpdateBuilder(rfc6793OriginNonMappable, false, true, false)
			attrs := originator.build(t, ub, []uint32{rfc6793OriginNonMappable})

			if got := as4PathASNs(t, attrs); got != nil {
				t.Errorf("AS4_PATH = %v, want none: the peer negotiated four-octet AS numbers", got)
			}
			want := []uint32{rfc6793OriginNonMappable}
			if got := asPathASNs(t, attrs, true); !equalASNs(got, want) {
				t.Errorf("AS_PATH = %v, want %v", got, want)
			}
		})
	}
}

// rfc6793Aggregators enumerates every builder that writes an AGGREGATOR for an
// originated route.
var rfc6793Aggregators = []struct {
	name  string
	build func(t *testing.T, ub *UpdateBuilder, asn uint32) []byte
}{
	{
		name: "BuildUnicast",
		build: func(_ *testing.T, ub *UpdateBuilder, asn uint32) []byte {
			return ub.BuildUnicast(&UnicastParams{
				Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
				NextHop:       netip.MustParseAddr("192.0.2.1"),
				Origin:        attribute.OriginIGP,
				HasAggregator: true,
				AggregatorASN: asn,
				AggregatorIP:  [4]byte{192, 0, 2, 1},
			}).PathAttributes
		},
	},
	{
		name: "BuildGroupedUnicast",
		build: func(t *testing.T, ub *UpdateBuilder, asn uint32) []byte {
			t.Helper()
			var attrs []byte
			routes := []UnicastParams{{
				Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
				NextHop:       netip.MustParseAddr("192.0.2.1"),
				Origin:        attribute.OriginIGP,
				HasAggregator: true,
				AggregatorASN: asn,
				AggregatorIP:  [4]byte{192, 0, 2, 1},
			}}
			err := ub.BuildGroupedUnicast(routes, wireStandardMaxForTest, func(u *Update) error {
				attrs = append(attrs, u.PathAttributes...)
				return nil
			})
			if err != nil {
				t.Fatalf("BuildGroupedUnicast: %v", err)
			}
			return attrs
		},
	},
	{
		name: "BuildVPN",
		build: func(_ *testing.T, ub *UpdateBuilder, asn uint32) []byte {
			return ub.BuildVPN(&VPNParams{
				Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
				NextHop:       netip.MustParseAddr("192.0.2.1"),
				Labels:        []uint32{100},
				RDBytes:       [8]byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 1},
				Origin:        attribute.OriginIGP,
				HasAggregator: true,
				AggregatorASN: asn,
				AggregatorIP:  [4]byte{192, 0, 2, 1},
			}).PathAttributes
		},
	},
	{
		name: "BuildLabeledUnicast",
		build: func(_ *testing.T, ub *UpdateBuilder, asn uint32) []byte {
			return ub.BuildLabeledUnicast(&LabeledUnicastParams{
				Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
				NextHop:       netip.MustParseAddr("192.0.2.1"),
				Labels:        []uint32{100},
				Origin:        attribute.OriginIGP,
				HasAggregator: true,
				AggregatorASN: asn,
				AggregatorIP:  [4]byte{192, 0, 2, 1},
			}).PathAttributes
		},
	},
}

// aggregatorASN returns the AS number of the AGGREGATOR attribute in attrs, read
// at the width the peer negotiated.
func aggregatorASN(t *testing.T, attrs []byte, asn4 bool) uint32 {
	t.Helper()
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrAggregator)
	if !found {
		t.Fatal("no AGGREGATOR attribute in the originated block")
	}
	agg, err := attribute.ParseAggregator(value, asn4)
	if err != nil {
		t.Fatalf("ParseAggregator: %v", err)
	}
	return agg.ASN
}

// as4AggregatorASN returns the AS number of the AS4_AGGREGATOR attribute in
// attrs, and whether the block carries one.
func as4AggregatorASN(t *testing.T, attrs []byte) (uint32, bool) {
	t.Helper()
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrAS4Aggregator)
	if !found {
		return 0, false
	}
	agg, err := attribute.ParseAS4Aggregator(value)
	if err != nil {
		t.Fatalf("ParseAS4Aggregator: %v", err)
	}
	return agg.ASN, true
}

// TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker drives every
// builder that writes an AGGREGATOR with a non-mappable aggregating AS.
//
// RFC requirement: RFC6793-4.2.2-5 positive -- an originated AGGREGATOR whose aggregating AS is
// non-mappable is sent to an OLD speaker with AS_TRANS in the AGGREGATOR AS field and the real
// four-octet AS number in an AS4_AGGREGATOR.
// RFC requirement: RFC6793-4.2.2-6 negative -- the AS4_AGGREGATOR suppression is conditional: a
// non-mappable aggregating AS does get the companion attribute.
func TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker(t *testing.T) {
	for _, originator := range rfc6793Aggregators {
		t.Run(originator.name, func(t *testing.T) {
			ub := NewUpdateBuilder(rfc6793OriginLocalAS, false, false, false)
			attrs := originator.build(t, ub, rfc6793OriginNonMappable)

			if got := aggregatorASN(t, attrs, false); got != rfc6793OriginASTrans {
				t.Errorf("AGGREGATOR AS = %d, want AS_TRANS %d", got, rfc6793OriginASTrans)
			}
			got, found := as4AggregatorASN(t, attrs)
			if !found {
				t.Fatal("no AS4_AGGREGATOR: the real aggregating AS reaches the OLD speaker nowhere")
			}
			if got != rfc6793OriginNonMappable {
				t.Errorf("AS4_AGGREGATOR AS = %d, want %d", got, rfc6793OriginNonMappable)
			}
		})
	}
}

// TestOriginatedMappableAggregatorGetsNoCompanion drives the same builders with
// a mappable aggregating AS.
//
// RFC requirement: RFC6793-4.2.2-6 positive -- an originated AGGREGATOR whose aggregating AS is
// mappable is sent to an OLD speaker with no AS4_AGGREGATOR beside it.
// RFC requirement: RFC6793-4.2.2-5 negative -- the companion is not written into every
// OLD-speaker AGGREGATOR: a mappable aggregating AS gets none.
func TestOriginatedMappableAggregatorGetsNoCompanion(t *testing.T) {
	for _, originator := range rfc6793Aggregators {
		t.Run(originator.name, func(t *testing.T) {
			ub := NewUpdateBuilder(rfc6793OriginLocalAS, false, false, false)
			attrs := originator.build(t, ub, rfc6793OriginPeerAS)

			if got := aggregatorASN(t, attrs, false); got != rfc6793OriginPeerAS {
				t.Errorf("AGGREGATOR AS = %d, want %d", got, rfc6793OriginPeerAS)
			}
			if got, found := as4AggregatorASN(t, attrs); found {
				t.Errorf("AS4_AGGREGATOR AS = %d, want none: the aggregating AS is mappable", got)
			}
		})
	}
}
