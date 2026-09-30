// Design: docs/architecture/wire/isis.md -- obsolete TLV 133 is ignored
// Related: auth_verify.go -- VerifyPDU and AuthTLVIndex, the producers under test

package packet

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// tlv133Password is the value both tests place in TLV 133, laid out the way the
// old RFC 1195 authentication TLV carried a cleartext password: an auth type
// octet of 1 followed by the password.
var tlv133Password = []byte{1, 's', 'e', 'c', 'r', 'e', 't'}

// lspWithTLV133First encodes a Level 2 LSP whose first TLV is TLV 133, the place
// an authentication TLV sits, followed by one TLV 135 prefix.
func lspWithTLV133First(t *testing.T) []byte {
	t.Helper()
	sys := types.SystemID{1, 1, 1, 1, 1, 1}
	in := &LSP{
		PDUType:           PDUTypeL2LSP,
		RemainingLifetime: 600,
		LSPID:             types.NewLSPID(types.NewSourceID(sys, 0), 0),
		SequenceNumber:    1,
		TLVs: []TLV{
			{Type: 133, Value: tlv133Password},
			ext135TLV(netip.MustParsePrefix("203.0.113.0/24"), 5),
		},
	}
	buf := make([]byte, in.EncodedLen())
	return buf[:in.WriteTo(buf, 0)]
}

// VALIDATES: RFC 3787 section 3.2, "TLV 133 is not used, and MUST be ignored."
// A received LSP carrying TLV 133 is accepted as if the TLV were absent: it
// decodes, TLV 133 is kept as an opaque span, and it is not taken for
// authentication.
// PREVENTS: a receiver that refuses a PDU because it carries TLV 133.
//
// RFC requirement: RFC3787-3.2-1 positive -- an LSP whose first TLV is TLV 133 decodes without error, AuthTLVIndex finds no authentication TLV in it, and VerifyPDU with no keys configured accepts it.
func TestRFC3787TLV133IgnoredOnReceipt(t *testing.T) {
	pdu := lspWithTLV133First(t)

	dec, err := DecodePDU(pdu)
	if err != nil {
		t.Fatalf("DecodePDU of an LSP carrying TLV 133: %v", err)
	}
	defer dec.Release()
	if dec.LSP.TLVs[0].Type != 133 {
		t.Fatalf("first TLV = %d, want 133 kept as an opaque span", dec.LSP.TLVs[0].Type)
	}
	if got := AuthTLVIndex(dec.LSP.TLVs); got != -1 {
		t.Fatalf("AuthTLVIndex = %d, want -1: TLV 133 is not authentication", got)
	}
	if err := VerifyPDU(pdu, nil); err != nil {
		t.Fatalf("VerifyPDU with no keys = %v, want the LSP accepted", err)
	}
}

// VALIDATES: RFC 3787 section 3.2 from the other side. Ignoring TLV 133 means it
// cannot stand in for authentication: under a configured cleartext key equal to
// the password TLV 133 carries, the LSP is refused as unauthenticated.
// PREVENTS: a receiver that honors the obsolete TLV 133 as an authentication TLV.
//
// RFC requirement: RFC3787-3.2-1 negative -- under a configured cleartext key whose secret is the password TLV 133 carries, VerifyPDU refuses the LSP with ErrAuthMissing rather than authenticating it through TLV 133.
func TestRFC3787TLV133NeverAuthenticates(t *testing.T) {
	pdu := lspWithTLV133First(t)
	key := Key{Algorithm: AuthAlgoCleartext, Secret: tlv133Password[1:]}

	if err := VerifyPDU(pdu, []Key{key}); !errors.Is(err, ErrAuthMissing) {
		t.Fatalf("VerifyPDU = %v, want ErrAuthMissing: TLV 133 must be ignored", err)
	}
}
