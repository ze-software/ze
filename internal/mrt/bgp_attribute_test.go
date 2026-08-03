package mrt_test

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

// truncatedMPReach builds the RFC 6396 Section 4.3.4 abbreviated MP_REACH_NLRI
// carried inside a TABLE_DUMP_V2 RIB entry: Next Hop Length + Next Hop only.
// AFI, SAFI, Reserved and NLRI are omitted (they live in the RIB record header).
func truncatedMPReach(nextHop []byte) []byte {
	v := make([]byte, 1+len(nextHop))
	v[0] = byte(len(nextHop))
	copy(v[1:], nextHop)
	return v
}

// fullMPReach builds the RFC 4760 Section 3 full MP_REACH_NLRI attribute value.
func fullMPReach(afi uint16, safi uint8, nextHop, nlri []byte) []byte {
	v := make([]byte, 0, 4+len(nextHop)+1+len(nlri))
	v = binary.BigEndian.AppendUint16(v, afi)
	v = append(v, safi, byte(len(nextHop)))
	v = append(v, nextHop...)
	v = append(v, 0) // Reserved
	v = append(v, nlri...)
	return v
}

func TestParseMPReachRIBEntry_IPv6Global(t *testing.T) {
	// VALIDATES: the abbreviated MP_REACH_NLRI in a TABLE_DUMP_V2 RIB entry
	// (RFC 6396 Section 4.3.4) yields the correct next hop.
	// PREVENTS: reading Next Hop Length from offset 3 (the full-form position),
	// which silently returns a garbage or empty next hop for every IPv6 RIB entry.
	nh := netip.MustParseAddr("2001:db8::1").As16()
	got, err := mrt.ParseMPReachRIBEntry(truncatedMPReach(nh[:]))
	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), got)
}

func TestParseMPReachRIBEntry_IPv6GlobalPlusLinkLocal(t *testing.T) {
	// VALIDATES: a 32-byte next hop (global + link-local, RFC 2545 Section 3)
	// resolves to the global address.
	// PREVENTS: rejecting the 32-byte form, which route collectors emit routinely.
	global := netip.MustParseAddr("2001:db8::1").As16()
	linkLocal := netip.MustParseAddr("fe80::1").As16()
	nh := append(append([]byte{}, global[:]...), linkLocal[:]...)

	got, err := mrt.ParseMPReachRIBEntry(truncatedMPReach(nh))
	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), got)
}

func TestParseMPReachRIBEntry_IPv4(t *testing.T) {
	// VALIDATES: a 4-byte next hop in the abbreviated form parses as IPv4.
	// PREVENTS: assuming the abbreviated form is IPv6-only.
	got, err := mrt.ParseMPReachRIBEntry(truncatedMPReach([]byte{192, 0, 2, 1}))
	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), got)
}

func TestParseMPReachRIBEntry_LengthMismatch(t *testing.T) {
	// VALIDATES: a Next Hop Length that overruns the attribute value is rejected.
	// PREVENTS: reading past the attribute into adjacent memory.
	_, err := mrt.ParseMPReachRIBEntry([]byte{16, 0x20, 0x01})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "16")
}

func TestParseMPReachRIBEntry_Empty(t *testing.T) {
	// VALIDATES: an empty attribute value is rejected rather than panicking.
	// PREVENTS: index-out-of-range on a zero-length MP_REACH in a corrupt dump.
	_, err := mrt.ParseMPReachRIBEntry(nil)
	require.Error(t, err)
}

func TestExtractNextHopRIB_TruncatedMPReach(t *testing.T) {
	// VALIDATES: next-hop extraction from RIB-entry attributes uses the
	// abbreviated MP_REACH decoder (RFC 6396 Section 4.3.4).
	// PREVENTS: `ze-analyze routes` and `show` emitting a wrong or missing
	// next hop for every IPv6 TABLE_DUMP_V2 RIB entry.
	nh := netip.MustParseAddr("2001:db8::1").As16()
	attrs := []mrt.PathAttribute{{Code: 14, Value: truncatedMPReach(nh[:])}}

	got := mrt.ExtractNextHopRIB(attrs)
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), got)
}

func TestExtractNextHopRIB_PrefersNextHopAttribute(t *testing.T) {
	// VALIDATES: a plain NEXT_HOP (type 3) attribute wins when present, which is
	// how ze's own TABLE_DUMP_V2 writer encodes IPv4 RIB entries.
	// PREVENTS: ignoring type 3 in RIB entries.
	attrs := []mrt.PathAttribute{{Code: 3, Value: []byte{10, 0, 0, 1}}}
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), mrt.ExtractNextHopRIB(attrs))
}

func TestExtractNextHopRIB_DiffersFromFullForm(t *testing.T) {
	// VALIDATES: the RIB decoder and the full-message decoder are distinct entry
	// points, as RFC 6396 Section 4.3.4 requires.
	// PREVENTS: one function silently handling both encodings and getting the
	// abbreviated case wrong.
	nh := netip.MustParseAddr("2001:db8::1").As16()
	truncated := []mrt.PathAttribute{{Code: 14, Value: truncatedMPReach(nh[:])}}

	// The full-form decoder must NOT produce the right answer here; that is the
	// whole reason a separate entry point exists.
	assert.NotEqual(t, netip.MustParseAddr("2001:db8::1"), mrt.ExtractNextHop(truncated))
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), mrt.ExtractNextHopRIB(truncated))
}

func TestParseMPReach_Full(t *testing.T) {
	// VALIDATES: the full MP_REACH_NLRI (RFC 4760 Section 3) yields AFI, SAFI,
	// next hop and IPv6 NLRI.
	// PREVENTS: dropping the announced IPv6 prefixes carried in MP_REACH.
	nh := netip.MustParseAddr("2001:db8::1").As16()
	// 2001:db8::/32 -> 4 significant bytes.
	nlri := []byte{32, 0x20, 0x01, 0x0d, 0xb8}

	mp, err := mrt.ParseMPReach(fullMPReach(2, 1, nh[:], nlri))
	require.NoError(t, err)
	assert.Equal(t, uint16(2), mp.AFI)
	assert.Equal(t, uint8(1), mp.SAFI)
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), mp.NextHop)
	require.Len(t, mp.Prefixes, 1)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), mp.Prefixes[0])
}

func TestParseMPReach_TruncatedHeader(t *testing.T) {
	// VALIDATES: a full MP_REACH shorter than its fixed header is rejected.
	// PREVENTS: out-of-range reads on a corrupt attribute.
	_, err := mrt.ParseMPReach([]byte{0, 2, 1})
	require.Error(t, err)
}

func TestParseMPUnreach(t *testing.T) {
	// VALIDATES: MP_UNREACH_NLRI (RFC 4760 Section 4) yields AFI, SAFI and the
	// withdrawn IPv6 prefixes.
	// PREVENTS: treating IPv6 withdrawals as unparsed opaque bytes.
	value := []byte{0, 2, 1, 32, 0x20, 0x01, 0x0d, 0xb8}

	mp, err := mrt.ParseMPUnreach(value)
	require.NoError(t, err)
	assert.Equal(t, uint16(2), mp.AFI)
	assert.Equal(t, uint8(1), mp.SAFI)
	require.Len(t, mp.Prefixes, 1)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), mp.Prefixes[0])
}

func TestParseMPUnreach_TooShort(t *testing.T) {
	// VALIDATES: an MP_UNREACH without its 3-byte AFI/SAFI header is rejected.
	// PREVENTS: silently returning an empty withdrawal set for corrupt input.
	_, err := mrt.ParseMPUnreach([]byte{0, 2})
	require.Error(t, err)
}

func TestParsePrefixesAFI_IPv6(t *testing.T) {
	// VALIDATES: IPv6 NLRI decodes to real IPv6 prefixes.
	// PREVENTS: the IPv4-only decoder turning 2001:db8::/32 into a bogus
	// IPv4 prefix or an invalid zero Prefix.
	data := []byte{
		32, 0x20, 0x01, 0x0d, 0xb8, // 2001:db8::/32
		48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, // 2001:db8:1::/48
	}
	pfxs, err := mrt.ParsePrefixesAFI(data, mrt.AFIIPv6, false)
	require.NoError(t, err)
	require.Len(t, pfxs, 2)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), pfxs[0])
	assert.Equal(t, netip.MustParsePrefix("2001:db8:1::/48"), pfxs[1])
	for _, p := range pfxs {
		assert.True(t, p.IsValid(), "prefix %v must be valid", p)
	}
}

func TestParsePrefixesAFI_IPv6AddPath(t *testing.T) {
	// VALIDATES: add-path IPv6 NLRI skips the 4-byte Path Identifier (RFC 8050).
	// PREVENTS: reading the Path ID as a prefix length.
	data := []byte{
		0, 0, 0, 7, // Path ID 7
		32, 0x20, 0x01, 0x0d, 0xb8,
	}
	pfxs, err := mrt.ParsePrefixesAFI(data, mrt.AFIIPv6, true)
	require.NoError(t, err)
	require.Len(t, pfxs, 1)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), pfxs[0])
}

func TestParsePrefixes_RejectsOversizedIPv4Length(t *testing.T) {
	// VALIDATES: an IPv4 prefix length above 32 is reported as an error instead
	// of emitting an invalid prefix (boundary: 32 valid, 33 invalid), and the
	// error names the offending value.
	// PREVENTS: (a) appending netip's zero Prefix, which reads as "0.0.0.0/0"
	// downstream; (b) silently truncating the walk, which would make a damaged
	// record indistinguishable from a short one.
	valid, err := mrt.ParsePrefixes([]byte{32, 10, 0, 0, 1}, false)
	require.NoError(t, err)
	require.Len(t, valid, 1)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.1/32"), valid[0])

	oversized, err := mrt.ParsePrefixes([]byte{33, 10, 0, 0, 1, 0}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "33")
	assert.Empty(t, oversized)
}

func TestParsePrefixesAFI_RejectsOversizedIPv6Length(t *testing.T) {
	// VALIDATES: an IPv6 prefix length above 128 is rejected (boundary: 128
	// valid, 129 invalid) and reported.
	// PREVENTS: an invalid zero Prefix entering the output.
	valid, err := mrt.ParsePrefixesAFI(append([]byte{128}, make([]byte, 16)...), mrt.AFIIPv6, false)
	require.NoError(t, err)
	require.Len(t, valid, 1)

	oversized, err := mrt.ParsePrefixesAFI(append([]byte{129}, make([]byte, 17)...), mrt.AFIIPv6, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "129")
	assert.Empty(t, oversized)
}

func TestParsePrefixesAFI_UnknownAFIReports(t *testing.T) {
	// VALIDATES: an unrecognized AFI returns an error naming it rather than an
	// empty result, per RFC 6396 Section 4.3.3.
	// PREVENTS: an unsupported family looking like a record with no routes.
	pfxs, err := mrt.ParsePrefixesAFI([]byte{8, 10}, 99, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "99")
	assert.Empty(t, pfxs)
}

func TestParsePrefixesAFI_SalvagesGoodEntriesBeforeDamage(t *testing.T) {
	// VALIDATES: a damaged NLRI returns BOTH the prefixes decoded before the
	// damage AND an error, so a caller can salvage and still report the file as
	// damaged.
	// PREVENTS: an all-or-nothing contract that would discard good routes, and
	// the silent-truncation contract that would hide the damage.
	data := []byte{8, 10, 16, 192, 168, 33, 1, 2, 3, 4}
	pfxs, err := mrt.ParsePrefixesAFI(data, mrt.AFIIPv4, false)
	require.Error(t, err)
	require.Len(t, pfxs, 2, "the two well-formed prefixes must survive")
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), pfxs[0])
	assert.Equal(t, netip.MustParsePrefix("192.168.0.0/16"), pfxs[1])
}

func TestParsePrefixesAFI_TruncatedPrefixIsReportedNotPadded(t *testing.T) {
	// VALIDATES: a prefix whose declared byte length runs past the end of the
	// NLRI returns mrt.ErrShortData and is NOT emitted, while the prefixes decoded
	// before it survive.
	// PREVENTS: the truncation branch going untested. The salvage test above
	// exercises only the prefix-LENGTH ceiling, so replacing either
	// mrt.ErrShortData return with a `break`, or with a clamp that zero-pads the
	// short tail and appends it, left the whole suite green. A zero-padded
	// prefix is IsValid(), so the fuzz invariant cannot catch it either: it
	// enters the salvage list as a real-looking route that is not in the file.
	// Truncation is the commonest shape of real MRT damage.
	data := []byte{
		8, 10, // 10.0.0.0/8
		16, 192, 168, // 192.168.0.0/16
		24, 172, 16, // claims /24 (3 octets) but only 2 follow
	}
	pfxs, err := mrt.ParsePrefixesAFI(data, mrt.AFIIPv4, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, mrt.ErrShortData)
	require.Len(t, pfxs, 2, "the truncated prefix must NOT be emitted, padded or otherwise")
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), pfxs[0])
	assert.Equal(t, netip.MustParsePrefix("192.168.0.0/16"), pfxs[1])
	assert.NotContains(t, pfxs, netip.MustParsePrefix("172.16.0.0/24"),
		"a zero-padded reconstruction of the truncated prefix is a fabricated route")
}

func TestParsePrefixesAFI_TruncatedAddPathIdentifierIsReported(t *testing.T) {
	// VALIDATES: NLRI that ends inside a 4-octet add-path Path Identifier
	// (RFC 8050) returns mrt.ErrShortData with the prefixes decoded so far.
	// PREVENTS: the second mrt.ErrShortData branch going untested; silently
	// dropping the tail of an add-path dump.
	data := []byte{
		0, 0, 0, 7, // Path ID 7
		8, 10, // 10.0.0.0/8
		0, 0, // a second Path ID, cut short
	}
	pfxs, err := mrt.ParsePrefixesAFI(data, mrt.AFIIPv4, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, mrt.ErrShortData)
	require.Len(t, pfxs, 1)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), pfxs[0])
}

func TestParseMPReach_SalvagesPrefixesBeforeDamage(t *testing.T) {
	// VALIDATES: the salvage contract has a real implementation -- a damaged
	// NLRI section returns the decoded prefixes AND the error.
	// PREVENTS: the documented promise going unhonored. Every caller used to
	// receive nil, so 500 good NLRIs followed by one truncated prefix lost all
	// 500 (ai/rules/completion.md: honor the contract or delete it).
	nh := netip.MustParseAddr("2001:db8::1").As16()
	value := fullMPReach(mrt.AFIIPv6, 1, nh[:], []byte{
		32, 0x20, 0x01, 0x0d, 0xb8, // 2001:db8::/32
		48, 0x20, 0x01, // /48 claims 6 octets, 2 follow
	})
	mp, err := mrt.ParseMPReach(value)
	require.Error(t, err)
	require.NotNil(t, mp, "the partial attribute MUST be returned, not discarded")
	assert.ErrorIs(t, err, mrt.ErrShortData)
	require.Len(t, mp.Prefixes, 1)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), mp.Prefixes[0])
	assert.Equal(t, mrt.AFIIPv6, mp.AFI)
}

func TestParseMPUnreach_SalvagesPrefixesBeforeDamage(t *testing.T) {
	// VALIDATES: MP_UNREACH salvages the same way as MP_REACH.
	// PREVENTS: an asymmetric contract between the two attributes.
	value := []byte{
		0, 2, // AFI IPv6
		1,                          // SAFI unicast
		32, 0x20, 0x01, 0x0d, 0xb8, // 2001:db8::/32
		48, 0x20, 0x01, // truncated
	}
	mp, err := mrt.ParseMPUnreach(value)
	require.Error(t, err)
	require.NotNil(t, mp)
	assert.ErrorIs(t, err, mrt.ErrShortData)
	require.Len(t, mp.Prefixes, 1)
}

func TestParseMPReach_HeaderFailureReturnsNil(t *testing.T) {
	// VALIDATES: a failure BEFORE the NLRI section still returns nil, because
	// nothing was decoded.
	// PREVENTS: the salvage change handing callers a half-built attribute whose
	// AFI/SAFI were never read.
	mp, err := mrt.ParseMPReach([]byte{0, 2})
	require.Error(t, err)
	assert.Nil(t, mp, "nothing decoded means nothing to salvage")
}

func TestASPathIsFourByte(t *testing.T) {
	// VALIDATES: the AS_PATH width is derived from the MRT record type/subtype
	// exactly as RFC 6396 mandates, never guessed from the attribute bytes.
	// PREVENTS: silent AS_PATH corruption from an AS-width mismatch, the
	// documented RFC 6396 pitfall.
	tests := []struct {
		name    string
		mrtType uint16
		subtype uint16
		want    bool
	}{
		{"TABLE_DUMP v1 IPv4 is 2-byte (Section 4.2)", mrt.TypeTableDump, mrt.TableDumpAFIIPv4, false},
		{"TABLE_DUMP v1 IPv6 is 2-byte (Section 4.2)", mrt.TypeTableDump, mrt.TableDumpAFIIPv6, false},
		{"TABLE_DUMP_V2 RIB is 4-byte (Section 4.3.4)", mrt.TypeTableDumpV2, mrt.TDV2RIBIPv4Unicast, true},
		{"TABLE_DUMP_V2 generic is 4-byte (Section 4.3.4)", mrt.TypeTableDumpV2, mrt.TDV2RIBGeneric, true},
		{"BGP4MP_MESSAGE is 2-byte (Section 4.4.2)", mrt.TypeBGP4MP, mrt.BGP4MPMessage, false},
		{"BGP4MP_MESSAGE_LOCAL is 2-byte (Section 4.4.2)", mrt.TypeBGP4MP, mrt.BGP4MPMessageLocal, false},
		{"BGP4MP_MESSAGE_AS4 is 4-byte (Section 4.4.3)", mrt.TypeBGP4MP, mrt.BGP4MPMessageAS4, true},
		{"BGP4MP_MESSAGE_AS4_LOCAL is 4-byte (Section 4.4.3)", mrt.TypeBGP4MP, mrt.BGP4MPMessageAS4Local, true},
		{"BGP4MP_ET inherits subtype width", mrt.TypeBGP4MPET, mrt.BGP4MPMessageAS4, true},
		{"BGP4MP_ET 2-byte subtype", mrt.TypeBGP4MPET, mrt.BGP4MPMessage, false},
		{"BGP4MP add-path AS4 is 4-byte", mrt.TypeBGP4MP, mrt.BGP4MPMessageAS4AP, true},
		{"BGP4MP add-path non-AS4 is 2-byte", mrt.TypeBGP4MP, mrt.BGP4MPMessageAP, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mrt.ASPathIsFourByte(tt.mrtType, tt.subtype))
		})
	}
}

func TestExtractAggregator_FourByte(t *testing.T) {
	// VALIDATES: AGGREGATOR (type 7) with a 4-byte ASN decodes ASN and router ID.
	// PREVENTS: leaving AGGREGATOR as opaque bytes.
	value := []byte{0, 1, 0, 0, 192, 0, 2, 1} // AS 65536, 192.0.2.1
	agg, ok := mrt.ExtractAggregator([]mrt.PathAttribute{{Code: 7, Value: value}})
	require.True(t, ok)
	assert.Equal(t, uint32(65536), agg.ASN)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), agg.Address)
}

func TestExtractAggregator_TwoByte(t *testing.T) {
	// VALIDATES: a 6-byte AGGREGATOR (2-byte ASN, TABLE_DUMP / BGP4MP_MESSAGE)
	// decodes correctly. The width is inferred from the attribute length, which
	// RFC 4271 and RFC 6793 make unambiguous (6 vs 8 octets).
	// PREVENTS: misreading a 2-byte-AS aggregator as a 4-byte one.
	value := []byte{0xFD, 0xE8, 192, 0, 2, 1} // AS 65000, 192.0.2.1
	agg, ok := mrt.ExtractAggregator([]mrt.PathAttribute{{Code: 7, Value: value}})
	require.True(t, ok)
	assert.Equal(t, uint32(65000), agg.ASN)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), agg.Address)
}

func TestExtractAggregator_Absent(t *testing.T) {
	// VALIDATES: absence reports ok=false rather than a zero-valued aggregator.
	// PREVENTS: a zero value reading as a real AS 0 aggregator.
	_, ok := mrt.ExtractAggregator([]mrt.PathAttribute{{Code: 1, Value: []byte{0}}})
	assert.False(t, ok)
}

func TestHasAtomicAggregate(t *testing.T) {
	// VALIDATES: ATOMIC_AGGREGATE (type 6) presence is reported as a flag.
	// PREVENTS: the zero-length attribute being dropped as "empty".
	assert.True(t, mrt.HasAtomicAggregate([]mrt.PathAttribute{{Code: 6, Value: nil}}))
	assert.False(t, mrt.HasAtomicAggregate([]mrt.PathAttribute{{Code: 1, Value: []byte{0}}}))
}

func TestExtractExtendedCommunities(t *testing.T) {
	// VALIDATES: EXTENDED COMMUNITIES (type 16, RFC 4360) split into 8-octet
	// records exposing type, subtype and value.
	// PREVENTS: returning the raw blob and forcing every caller to re-slice it.
	value := []byte{
		0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64, // route-target 65000:100
		0x40, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02,
	}
	ecs := mrt.ExtractExtendedCommunities([]mrt.PathAttribute{{Code: 16, Value: value}})
	require.Len(t, ecs, 2)
	assert.Equal(t, uint8(0x00), ecs[0].Type)
	assert.Equal(t, uint8(0x02), ecs[0].Subtype)
	assert.Equal(t, [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}, ecs[0].Value)
	assert.Equal(t, uint8(0x40), ecs[1].Type)
}

func TestExtractExtendedCommunities_RaggedLength(t *testing.T) {
	// VALIDATES: a value that is not a multiple of 8 yields no communities.
	// PREVENTS: reading a partial trailing record past the attribute end.
	assert.Empty(t, mrt.ExtractExtendedCommunities([]mrt.PathAttribute{{Code: 16, Value: make([]byte, 12)}}))
}
