// VALIDATES: spec-ospf-ext-9 IPv4 Grace-LSA body codec (RFC 3623 sec A) -- the Grace-LSA body
// is a 4-byte-aligned Type/Length/Value stream over the ext-1 generic builder; the mandatory
// Grace Period (type 1) and Reason (type 2) TLVs are always emitted, the optional IP interface
// address (type 3) only on shared media; the 1-byte Reason value round-trips through 4-octet
// padding; and a body missing a mandatory TLV is reported as malformed, never panicked.
// PREVENTS: a Grace-LSA that omits a mandatory TLV, a Reason TLV whose padding desyncs a
// receiver, or a decoder that accepts an incomplete Grace-LSA.
package packet

import "testing"

// RFC requirement: RFC5187-2.2-1 positive -- the Grace Period TLV (Type 1) always
// appears in a grace-LSA (RFC 5187 sec 2.2, same format as RFC 3623): EncodeGraceLSA
// always emits it (internal/plugins/ospf/packet/grace_lsa.go:47-49) and the body
// round-trips its GracePeriod through DecodeGraceLSA.
// RFC requirement: RFC5187-2.2-2 positive -- the Graceful Restart Reason TLV (Type 2)
// always appears in a grace-LSA (RFC 5187 sec 2.2): EncodeGraceLSA always emits it and
// the body round-trips its Reason.
func TestGraceLSARoundTrip(t *testing.T) {
	// RFC requirement: RFC3623-A-2 positive -- the Grace Period TLV (type 1) always appears
	// in a grace-LSA (RFC 3623 sec A): EncodeGraceLSA always emits it
	// (internal/plugins/ospf/packet/grace_lsa.go:47-49); the asserted body length includes
	// the type-1 TLV and DecodeGraceLSA round-trips GracePeriod=120.
	// RFC requirement: RFC3623-A-3 positive -- the Graceful restart reason TLV (type 2) always
	// appears in a grace-LSA (RFC 3623 sec A): EncodeGraceLSA always emits it and the body
	// round-trips Reason=2.
	in := GraceLSA{GracePeriod: 120, Reason: 2, HasInterfaceAddr: true, InterfaceAddr: [4]byte{192, 0, 2, 1}}
	body := EncodeGraceLSA(in)
	// RFC 3630 sec 2.3: each TLV is 4-octet aligned; the whole body is therefore aligned.
	if len(body)%4 != 0 {
		t.Fatalf("body length %d is not 4-octet aligned", len(body))
	}
	want := opaqueTLVsLen([]opaqueTLV{
		{Type: GraceTLVPeriod, Value: make([]byte, 4)},
		{Type: GraceTLVReason, Value: make([]byte, 1)},
		{Type: GraceTLVInterfaceAddr, Value: make([]byte, 4)},
	})
	if len(body) != want {
		t.Fatalf("body length %d, want %d", len(body), want)
	}
	got, err := DecodeGraceLSA(body)
	if err != nil {
		t.Fatalf("decode error on well-formed body: %v", err)
	}
	if got != in {
		t.Fatalf("round-trip = %+v, want %+v", got, in)
	}
}

func TestGraceLSANoInterfaceAddr(t *testing.T) {
	in := GraceLSA{GracePeriod: 1800, Reason: 0}
	body := EncodeGraceLSA(in)
	got, err := DecodeGraceLSA(body)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.HasInterfaceAddr {
		t.Fatalf("HasInterfaceAddr = true, want false")
	}
	if got.GracePeriod != 1800 || got.Reason != 0 {
		t.Fatalf("decoded = %+v, want period 1800 reason 0", got)
	}
}

// RFC requirement: RFC5187-2.2-1 negative -- a grace-LSA body that OMITS the Grace
// Period TLV is malformed and rejected (RFC 5187 sec 2.2 requires it to always appear):
// DecodeGraceLSA returns an error rather than accepting a Reason-only body
// (internal/plugins/ospf/packet/grace_lsa.go:94 requires hasPeriod).
// RFC requirement: RFC5187-2.2-2 negative -- a grace-LSA body that OMITS the Reason TLV
// is likewise malformed and rejected (grace_lsa.go:94 requires hasReason).
func TestGraceLSADecodeMissingMandatory(t *testing.T) {
	// RFC requirement: RFC3623-A-3 negative -- a grace-LSA body that OMITS the Reason TLV is
	// malformed and rejected (RFC 3623 sec A requires it to always appear): DecodeGraceLSA
	// returns an error for the periodOnly body (grace_lsa.go:94 requires hasReason).
	// RFC requirement: RFC3623-A-2 negative -- a grace-LSA body that OMITS the Grace Period
	// TLV is likewise malformed and rejected: DecodeGraceLSA returns an error for the
	// reasonOnly body (grace_lsa.go:94 requires hasPeriod).
	// A body carrying only the Grace Period TLV (no mandatory Reason TLV) is malformed.
	var period [4]byte
	writeUint32(period[:], 0, 120)
	periodOnly := make([]byte, opaqueTLVsLen([]opaqueTLV{{Type: GraceTLVPeriod, Value: period[:]}}))
	writeOpaqueTLVs(periodOnly, []opaqueTLV{{Type: GraceTLVPeriod, Value: period[:]}})
	if _, err := DecodeGraceLSA(periodOnly); err == nil {
		t.Fatalf("expected error for Grace-LSA missing the Reason TLV, got nil")
	}

	// A body carrying only the Reason TLV (no mandatory Grace Period TLV) is also malformed.
	reasonOnly := make([]byte, opaqueTLVsLen([]opaqueTLV{{Type: GraceTLVReason, Value: []byte{2}}}))
	writeOpaqueTLVs(reasonOnly, []opaqueTLV{{Type: GraceTLVReason, Value: []byte{2}}})
	if _, err := DecodeGraceLSA(reasonOnly); err == nil {
		t.Fatalf("expected error for Grace-LSA missing the Grace Period TLV, got nil")
	}
}

func TestGraceLSADecodeReasonPadding(t *testing.T) {
	// RFC 3623 sec A / RFC 3630 sec 2.3: the Reason TLV declares Length 1 but occupies 4
	// padded octets; a decoder must advance by the padded length, not the raw Length.
	in := GraceLSA{GracePeriod: 60, Reason: 3}
	body := EncodeGraceLSA(in)
	got, err := DecodeGraceLSA(body)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Reason != 3 || got.GracePeriod != 60 {
		t.Fatalf("decoded = %+v, want reason 3 period 60", got)
	}
}
