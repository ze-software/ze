// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- RFC 3623 Grace-LSA body codec (IPv4).
// RFC: rfc/short/rfc3623.md -- the OSPFv2 Grace-LSA: a link-local (Type 9) Opaque LSA with
// Opaque Type 3 and Opaque ID 0, whose body is a TLV stream (RFC 3630 sec 2.3 4-byte-aligned
// format). This file codes the Grace-LSA body ON TOP of the generic opaque_tlv.go
// builder/iterator (spec-ospf-ext-1); it never re-implements TLV framing or the 4-octet
// alignment. It exposes a Grace-specific surface (the exported GraceLSA type + encode/decode)
// for the graceful-restart consumer in package ospf, which owns the Grace-LSA carriage.

package packet

// GraceOpaqueType is the RFC 3623 sec A Grace-LSA Opaque Type (Opaque ID is always 0).
const GraceOpaqueType uint8 = 3

// RFC 3623 sec A: the Grace-LSA top-level TLV type codes.
const (
	// GraceTLVPeriod is the Grace Period TLV (Type 1, Length 4): seconds neighbors keep
	// advertising the router as fully adjacent. MUST always be present.
	GraceTLVPeriod uint16 = 1
	// GraceTLVReason is the Graceful restart reason TLV (Type 2, Length 1). MUST always be
	// present.
	GraceTLVReason uint16 = 2
	// GraceTLVInterfaceAddr is the IP interface address TLV (Type 3, Length 4): required on
	// broadcast / NBMA / Point-to-MultiPoint segments so the helper can identify the
	// restarting neighbor by its interface address (RFC 3623 sec A, sec 3.1).
	GraceTLVInterfaceAddr uint16 = 3
)

// GraceLSA is the decoded body of an OSPFv2 Grace-LSA (RFC 3623 sec A). The Grace Period and
// Reason are mandatory; the IP interface address is present only on shared media and is
// tracked by HasInterfaceAddr.
type GraceLSA struct {
	GracePeriod      uint32
	Reason           uint8
	InterfaceAddr    [4]byte
	HasInterfaceAddr bool
}

// EncodeGraceLSA renders a Grace-LSA body (the bytes after the 20-byte Opaque LSA header) from
// g using the ext-1 4-byte-aligned builder. The type-1 Grace Period and type-2 Reason TLVs are
// always emitted, in ascending type order; the type-3 IP interface address TLV follows only
// when HasInterfaceAddr is set (RFC 3623 sec A). The result is handed to the opaque carrier
// verbatim. This is a cold path (once per restart per interface); the single allocation mirrors
// EncodeRITLVs.
func EncodeGraceLSA(g GraceLSA) []byte {
	var period [4]byte
	writeUint32(period[:], 0, g.GracePeriod)
	tlvs := []opaqueTLV{
		{Type: GraceTLVPeriod, Value: period[:]},
		{Type: GraceTLVReason, Value: []byte{g.Reason}},
	}
	if g.HasInterfaceAddr {
		tlvs = append(tlvs, opaqueTLV{Type: GraceTLVInterfaceAddr, Value: g.InterfaceAddr[:]})
	}
	b := make([]byte, opaqueTLVsLen(tlvs))
	writeOpaqueTLVs(b, tlvs)
	return b
}

// DecodeGraceLSA walks a Grace-LSA body's TLV stream over the ext-1 bound-checked iterator and
// returns the decoded fields. It NEVER panics on malformed input; unrecognized TLV types are
// ignored (RFC 3623 sec A). Both mandatory TLVs (Grace Period, Reason) MUST be present, else
// the Grace-LSA is malformed and ErrLength is returned. The Reason value is returned verbatim
// (a value > 3 is clamped by helper policy, not here).
func DecodeGraceLSA(body []byte) (GraceLSA, error) {
	var g GraceLSA
	var hasPeriod, hasReason bool
	it := newOpaqueTLVIterator(body)
	for it.Next() {
		v := it.Value()
		switch it.Type() {
		case GraceTLVPeriod:
			if len(v) < 4 {
				return GraceLSA{}, ErrLength
			}
			g.GracePeriod = readUint32(v, 0)
			hasPeriod = true
		case GraceTLVReason:
			if len(v) < 1 {
				return GraceLSA{}, ErrLength
			}
			g.Reason = v[0]
			hasReason = true
		case GraceTLVInterfaceAddr:
			if len(v) < 4 {
				return GraceLSA{}, ErrLength
			}
			g.InterfaceAddr = readIPv4(v, 0)
			g.HasInterfaceAddr = true
		}
	}
	if it.Err() != nil {
		return GraceLSA{}, it.Err()
	}
	if !hasPeriod || !hasReason {
		return GraceLSA{}, ErrLength
	}
	return g, nil
}
