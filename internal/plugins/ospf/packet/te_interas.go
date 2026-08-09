// Design: docs/architecture/ospf/ospf-ext-2-traffic-engineering.md -- RFC 5392 inter-AS TE sub-TLVs.
// RFC: rfc/short/rfc5392.md -- OSPF Extensions for Inter-AS MPLS/GMPLS TE.
//
// RFC 5392 adds NO top-level TLV: the Inter-AS-TE-v2 LSA (Opaque type 6) carries the same
// Link TLV (type 2) as RFC 3630, plus three new Link sub-TLVs identifying the remote ASBR.
// These helpers append and parse those three sub-TLVs inside the shared Link TLV codec
// (te_lsa.go). RFC 5392 sec 3.2.1: the Link ID sub-TLV MUST NOT appear here -- the caller
// (origination) leaves TELink.HasLinkID false for an inter-AS link, so it is never emitted.

package packet

// RFC 5392 sec 3.3 / sec 6.2: the inter-AS Link sub-TLV type codes. The IPv6 Remote ASBR
// ID is type 24, NOT 23: the sec 3.2.1 prose "23" is an editorial slip; sec 3.3.3 and the
// sec 6.2 IANA table both assign 24.
const (
	// TESubRemoteAS is the Remote AS Number sub-TLV (RFC 5392 sec 3.3.1), 4 octets,
	// REQUIRED in an inter-AS Link TLV. A 2-byte ASN is zero-extended into the high 16 bits.
	TESubRemoteAS uint16 = 21
	// TESubIPv4RemoteASBRID is the IPv4 Remote ASBR ID sub-TLV (RFC 5392 sec 3.3.2), 4 octets.
	TESubIPv4RemoteASBRID uint16 = 22
	// TESubIPv6RemoteASBRID is the IPv6 Remote ASBR ID sub-TLV (RFC 5392 sec 3.3.3), 16 octets.
	TESubIPv6RemoteASBRID uint16 = 24
)

// appendInterAsSubTLVs appends the RFC 5392 inter-AS sub-TLVs present on l to tlvs. It is
// called by TELink.linkSubTLVs after the RFC 3630 sub-TLVs. The Link ID sub-TLV is not
// appended here (nor anywhere for an inter-AS link): its prohibition (sec 3.2.1) is
// enforced by the origination path leaving HasLinkID false.
//
// Cold-path exception to ai/rules/buffer-first: append/make here run only on TE-LSA
// origination/refresh (see TELSA.Encode), not on packet forwarding, so the allocation is
// deliberate and left for readability.
func appendInterAsSubTLVs(tlvs []opaqueTLV, l TELink) []opaqueTLV {
	if l.HasRemoteAS {
		// RFC 5392 sec 3.3.1: 4-octet field, 2-byte ASN zero-extended into the high 16 bits.
		tlvs = append(tlvs, opaqueTLV{Type: TESubRemoteAS, Value: teU32Bytes(l.RemoteAS)})
	}
	if l.HasRemoteASBRv4 {
		tlvs = append(tlvs, opaqueTLV{Type: TESubIPv4RemoteASBRID, Value: teIPBytes(l.RemoteASBRv4)})
	}
	if l.HasRemoteASBRv6 {
		v := make([]byte, 16)
		copy(v, l.RemoteASBRv6[:])
		tlvs = append(tlvs, opaqueTLV{Type: TESubIPv6RemoteASBRID, Value: v})
	}
	return tlvs
}

// parseInterAsSubTLV decodes one RFC 5392 inter-AS sub-TLV into l. handled is true when
// typ is one of 21/22/24 (whether or not the length was valid); err is non-nil only for a
// wrong fixed length. A non-inter-AS type returns handled=false so the caller can ignore it.
func parseInterAsSubTLV(l *TELink, typ uint16, v []byte) (handled bool, err error) {
	switch typ {
	case TESubRemoteAS:
		if len(v) != 4 {
			return true, ErrLength
		}
		l.HasRemoteAS = true
		l.RemoteAS = readUint32(v, 0)
		return true, nil
	case TESubIPv4RemoteASBRID:
		if len(v) != 4 {
			return true, ErrLength
		}
		l.HasRemoteASBRv4 = true
		l.RemoteASBRv4 = readIPv4(v, 0)
		return true, nil
	case TESubIPv6RemoteASBRID:
		if len(v) != 16 {
			return true, ErrLength
		}
		l.HasRemoteASBRv6 = true
		copy(l.RemoteASBRv6[:], v)
		return true, nil
	default:
		return false, nil
	}
}
