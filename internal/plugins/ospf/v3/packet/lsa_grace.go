// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- OSPFv3 Grace-LSA body codec.
// RFC: rfc/short/rfc5187.md (§2.2 grace-LSA format: LS Type 0x000B, two mandatory TLVs).
//
// The OSPFv3 Grace-LSA is a native link-scope LSA (function code 11), unlike the OSPFv2
// Grace-LSA which rides the opaque carrier (RFC 3623 §A). Its body is exactly two TLVs:
// Grace Period (Type 1, Length 4) and Graceful Restart Reason (Type 2, Length 1), both
// MANDATORY (RFC 5187 §2.2). The RFC 3623 IP Interface Address tlv (Type 3) is NOT emitted
// or required: OSPFv3 keys neighbors by Router ID (RFC 5187 §2).

package packet

const (
	// GraceTLVPeriod is the RFC 5187 §2.2 Grace Period tlv (Type 1, Length 4): the seconds
	// neighbors keep advertising the router as fully adjacent.
	GraceTLVPeriod uint16 = 1
	// GraceTLVReason is the RFC 5187 §2.2 Graceful Restart Reason tlv (Type 2, Length 1):
	// 0 unknown, 1 software restart, 2 software reload/upgrade, 3 switch to redundant CP.
	GraceTLVReason uint16 = 2
)

// gracePeriodValueLen is the fixed Grace Period value length in octets (RFC 5187 §2.2).
const gracePeriodValueLen = 4

// GraceLSA is the OSPFv3 Grace-LSA body (RFC 5187 §2.2). The LS Type (0x000B) and the
// Link State ID (the originating Interface ID) live in the LSA header; this struct is the
// tlv-encoded body only.
type GraceLSA struct {
	GracePeriod uint32 // seconds neighbors keep advertising the router as adjacent
	Reason      uint8  // restart reason code, 0..3
}

// tlvs builds the ordered mandatory tlv set for this body: Grace Period then Restart Reason
// (RFC 5187 §2.2). Cold path (origination only), so the small per-tlv value allocation is
// acceptable; the on-wire encode itself is buffer-first via WriteTo.
func (g GraceLSA) tlvs() []tlv {
	period := []byte{byte(g.GracePeriod >> 24), byte(g.GracePeriod >> 16), byte(g.GracePeriod >> 8), byte(g.GracePeriod)}
	return []tlv{
		{Type: GraceTLVPeriod, Value: period},
		{Type: GraceTLVReason, Value: []byte{g.Reason}},
	}
}

// EncodedLen returns the Grace-LSA body length (the two 4-byte-aligned TLVs = 16 octets).
func (g GraceLSA) EncodedLen() int { return tlvsEncodedLen(g.tlvs()) }

// WriteTo serializes the Grace-LSA body (the two mandatory TLVs) into buf at off and returns
// the new offset. Buffer-first: the caller owns buf and sizes it for EncodedLen.
func (g GraceLSA) WriteTo(buf []byte, off int) int {
	for _, t := range g.tlvs() {
		off = t.WriteTo(buf, off)
	}
	return off
}

// decodeGraceLSA parses a Grace-LSA body. Both mandatory TLVs (Grace Period, Restart Reason)
// MUST be present with valid lengths or the Grace-LSA is malformed (RFC 5187 §2.2
// Validation); unknown tlv types are ignored (RFC 3623 §A). The iterator advances by the
// 4-octet-padded length, so the Reason tlv (Length 1) is walked correctly (RFC 5187
// §Pitfalls).
func decodeGraceLSA(body []byte) (GraceLSA, error) {
	var out GraceLSA
	var hasPeriod, hasReason bool
	it := newTLVIterator(body)
	for it.Next() {
		v := it.Value()
		switch it.Type() {
		case GraceTLVPeriod:
			if len(v) != gracePeriodValueLen {
				return GraceLSA{}, ErrLength
			}
			out.GracePeriod = readUint32(v, 0)
			hasPeriod = true
		case GraceTLVReason:
			if len(v) < 1 {
				return GraceLSA{}, ErrLength
			}
			out.Reason = v[0]
			hasReason = true
		}
	}
	if it.Err() != nil {
		return GraceLSA{}, it.Err()
	}
	if !hasPeriod || !hasReason {
		return GraceLSA{}, ErrLength
	}
	return out, nil
}
