// Design: docs/architecture/ospf/ospf-2-wire.md -- fuzz targets for packet and LSA decode

package packet

import "testing"

func FuzzOSPFDecodePacket(f *testing.F) {
	f.Add(fuzzSeedPacket())
	f.Add([]byte{0x02})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodePacket(data)
	})
}

func FuzzOSPFLSAIterator(f *testing.F) {
	f.Add(fuzzSeedLSA())
	f.Add([]byte{0, 1, 2})
	f.Fuzz(func(t *testing.T, data []byte) {
		it := NewLSAIterator(data)
		for it.Next() {
			_ = it.LSA()
		}
		_ = it.Err()
	})
}

func FuzzOSPFRoundTrip(f *testing.F) {
	f.Add(fuzzSeedPacket())
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := DecodePacket(data)
		if err != nil {
			return
		}
		buf := make([]byte, p.EncodedLen())
		(&p).WriteTo(buf, 0)
		_, _ = DecodePacket(buf)
	})
}

// FuzzOSPFTEBody fuzzes the RFC 3630 / RFC 5392 TE LSA body decoder: it must never panic
// on a malformed or truncated body or sub-TLV (AC-18 / R-8), only ever returning an error.
func FuzzOSPFTEBody(f *testing.F) {
	f.Add(TELSA{IsRouterAddress: true, RouterAddress: [4]byte{192, 0, 2, 1}}.Encode())
	f.Add(TELSA{IsLink: true, Link: TELink{
		HasLinkType: true, LinkType: TELinkTypePointToPoint, HasLinkID: true, LinkID: [4]byte{2, 2, 2, 2},
		HasMaxBandwidth: true, MaxBandwidth: 1.25e9, HasUnreserved: true,
	}}.Encode())
	f.Add(TELSA{IsLink: true, Link: TELink{
		HasLinkType: true, HasRemoteAS: true, RemoteAS: 65001, HasRemoteASBRv6: true,
	}}.Encode())
	f.Add([]byte{0x00, 0x02, 0x00, 0xff})
	f.Fuzz(func(t *testing.T, data []byte) {
		// The only contract is "no panic"; a returned error for malformed input is expected.
		if _, err := DecodeTELSA(data); err != nil {
			return
		}
	})
}

// FuzzOSPFRIBody fuzzes the RFC 7770 Router Information LSA body decoder: it must never
// panic on a malformed or truncated TLV stream (AC-14), only ever returning an error.
func FuzzOSPFRIBody(f *testing.F) {
	f.Add(EncodeRITLVs([]RITLV{{Type: RITLVInformationalCapabilities, Value: RICapabilitiesValue(RIInfoBitMask(RIInfoBitStubRouter))}}))
	f.Add(EncodeRITLVs([]RITLV{
		{Type: RITLVInformationalCapabilities, Value: RICapabilitiesValue(0)},
		{Type: RITLVFunctionalCapabilities, Value: RICapabilitiesValue(0)},
		{Type: 8, Value: []byte{1, 2, 3}},
	}))
	f.Add([]byte{0x00, 0x01, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, data []byte) {
		// The only contract is "no panic"; a returned error for malformed input is expected.
		tlvs, err := DecodeRITLVStream(data)
		for _, tlv := range tlvs {
			if tlv.Type == RITLVInformationalCapabilities {
				_ = RIReadCapabilities(tlv.Value)
			}
		}
		_ = err
	})
}

// FuzzOSPFExtPrefixBody fuzzes the RFC 7684 Extended Prefix Opaque LSA body decoder: a
// malformed TLV/sub-TLV must never panic (AC-7/R-2), only ever returning an error.
func FuzzOSPFExtPrefixBody(f *testing.F) {
	f.Add(EncodeExtPrefixLSA(ExtPrefixLSA{Prefixes: []ExtPrefixTLV{{
		RouteType: ExtRouteTypeIntraArea, PrefixLength: 24, AddressPrefix: [4]byte{10, 1, 2, 0},
		SubTLVs: []ExtSubTLV{{Type: 1, Value: []byte{1, 2, 3}}},
	}}}))
	f.Add(EncodeExtPrefixLSA(ExtPrefixLSA{Ranges: []ExtPrefixRangeTLV{{Value: []byte{0x20, 0, 0, 5}}}}))
	f.Add([]byte{0x00, 0x01, 0xff, 0xff})
	// RFC requirement: RFC7684-5-1 negative -- arbitrary, truncated, or overrunning Extended
	// Prefix bodies and sub-TLVs never panic; the bound-checked decoder only ever returns an
	// error, so a malformed permutation cannot crash the routing process (§5).
	f.Fuzz(func(t *testing.T, data []byte) {
		lsa, err := DecodeExtPrefixLSA(data)
		for i := range lsa.Prefixes {
			_ = lsa.Prefixes[i].HasFlag(ExtPrefixFlagN)
		}
		_ = err
	})
}

// FuzzOSPFExtLinkBody fuzzes the RFC 7684 Extended Link Opaque LSA body decoder: it must
// never panic on a malformed TLV/sub-TLV (AC-7/R-2), only ever returning an error.
func FuzzOSPFExtLinkBody(f *testing.F) {
	f.Add(EncodeExtLinkLSA(ExtLinkTLV{
		LinkType: 1, LinkID: [4]byte{2, 2, 2, 2}, LinkData: [4]byte{10, 0, 0, 1},
		SubTLVs: []ExtSubTLV{{Type: 2, Value: []byte{1, 2, 3, 4}}},
	}))
	f.Add([]byte{0x00, 0x01, 0xff, 0xff})
	// RFC requirement: RFC7684-5-1 negative -- arbitrary, truncated, or overrunning Extended
	// Link bodies and sub-TLVs never panic; the bound-checked decoder only ever returns an
	// error, so a malformed permutation cannot crash the routing process (§5).
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := DecodeExtLinkLSA(data)
		_ = err
	})
}

func fuzzSeedPacket() []byte {
	p := Packet{Header: Header{Type: PacketTypeHello}, Hello: &Hello{}}
	buf := make([]byte, p.EncodedLen())
	(&p).WriteTo(buf, 0)
	return buf
}

func fuzzSeedLSA() []byte {
	lsa := LSA{Header: LSAHeader{Type: 1}, Router: &RouterLSA{}}
	buf := make([]byte, lsa.EncodedLen())
	(&lsa).WriteTo(buf, 0)
	return buf
}
