// VALIDATES: spec-ospf-ext-9 IPv4 Grace-LSA body codec (RFC 3623 sec A) -- the Grace-LSA body
// is a 4-byte-aligned Type/Length/Value stream over the ext-1 generic builder; the mandatory
// Grace Period (type 1) and Reason (type 2) TLVs are always emitted, the optional IP interface
// address (type 3) only on shared media; the 1-byte Reason value round-trips through 4-octet
// padding; and a body missing a mandatory TLV is reported as malformed, never panicked.
// PREVENTS: a Grace-LSA that omits a mandatory TLV, a Reason TLV whose padding desyncs a
// receiver, or a decoder that accepts an incomplete Grace-LSA.
package packet

import "testing"

func TestGraceLSARoundTrip(t *testing.T) {
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

func TestGraceLSADecodeMissingMandatory(t *testing.T) {
	// A body carrying only the Grace Period TLV (no mandatory Reason TLV) is malformed.
	var period [4]byte
	writeUint32(period[:], 0, 120)
	body := make([]byte, opaqueTLVsLen([]opaqueTLV{{Type: GraceTLVPeriod, Value: period[:]}}))
	writeOpaqueTLVs(body, []opaqueTLV{{Type: GraceTLVPeriod, Value: period[:]}})
	if _, err := DecodeGraceLSA(body); err == nil {
		t.Fatalf("expected error for Grace-LSA missing the Reason TLV, got nil")
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
