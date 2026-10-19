// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- ESP proposal negotiation
// Related: initiator.go -- wireESPOffer, the SAi2 producer
// Related: child.go -- installChildSA, the SAParams the dataplane receives
// RFC: rfc/short/rfc4303.md -- integrity-only ESP negotiable (Section 1)
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// integrityOnlyESPGroup is an ESP group whose one proposal is the null cipher with
// HMAC-SHA2-256-128: integrity-only ESP.
func integrityOnlyESPGroup() ipsec.ESPGroup {
	return ipsec.ESPGroup{
		Name:     "esp-integrity-only",
		Lifetime: 3600,
		Proposals: []ipsec.ESPProposal{
			{Number: 1, Encryption: ipsec.EncryptionNull, Hash: ipsec.HashSHA256},
		},
	}
}

// VALIDATES: RFC 4303 Section 1, "Integrity-only ESP MUST be offered as a service
// selection option, e.g., it must be negotiable in SA management protocols". Method: two
// Ze engines run IKE_SA_INIT and IKE_AUTH in process, both carrying an ESP group whose
// only proposal is NULL encryption with SHA-256 integrity. The test reads the SAi2 offer
// the initiator builds, completes the exchange, installs the first Child SA into a mock
// dataplane, and reads the SAParams Ze hands the dataplane.
// PREVENTS: an ESP vocabulary or proposal matcher that cannot negotiate ENCR_NULL, which
// leaves integrity-only ESP configurable only by manual keying.
//
// RFC requirement: RFC4303-1-2 positive -- an IKE_AUTH between two Ze engines whose ESP group offers only NULL encryption with SHA-256 integrity offers ENCR_NULL (Transform ID 11, no Key Length attribute) beside AUTH_HMAC_SHA2_256_128, establishes the IKE SA, and installs an inbound and an outbound Child SA carrying the null cipher with no encryption key and sha256 integrity with a 32-octet key.
func TestRFC4303IntegrityOnlyESPIsNegotiated(t *testing.T) {
	log := slogutil.DiscardLogger()
	group := integrityOnlyESPGroup()

	offer := buildWireESPProposals(group, 0x11223344, dhGroupNone)
	if len(offer) != 1 {
		t.Fatalf("SAi2 carries %d proposals, want 1", len(offer))
	}
	var encrOffered, integOffered bool
	for _, tr := range offer[0].Transforms {
		switch tr.Type {
		case wire.TransformTypeENCR:
			if tr.ID != uint16(crypto.ENCR_NULL) {
				t.Fatalf("SAi2 offers ENCR %d, want ENCR_NULL (%d)", tr.ID, crypto.ENCR_NULL)
			}
			if len(tr.Attrs) != 0 {
				t.Fatalf("SAi2 ENCR_NULL carries attributes %+v, want none: the null cipher takes no key", tr.Attrs)
			}
			encrOffered = true
		case wire.TransformTypeINTG:
			if tr.ID != uint16(crypto.AUTH_HMAC_SHA2_256_128) {
				t.Fatalf("SAi2 offers INTEG %d, want AUTH_HMAC_SHA2_256_128 (%d)", tr.ID, crypto.AUTH_HMAC_SHA2_256_128)
			}
			integOffered = true
		}
	}
	if !encrOffered || !integOffered {
		t.Fatalf("SAi2 transforms %+v, want ENCR_NULL and an integrity transform", offer[0].Transforms)
	}

	ini, answer := negAuthExchange(t, group, group)
	handleAuthResponse(ini, parseMsg(t, answer), answer, nil, nil, log)
	if ini.State != StateEstablished {
		t.Fatalf("state = %v, want established: both ends offer integrity-only ESP", ini.State)
	}

	dp := &mockDP{}
	if _, err := initiatorFirstChildSA(ini, ini.PeerCfg, 7, dp, log); err != nil {
		t.Fatalf("initiatorFirstChildSA: %v", err)
	}
	if len(dp.sas) != 2 {
		t.Fatalf("the dataplane holds %d ESP states, want 2 (inbound and outbound)", len(dp.sas))
	}
	for i, sa := range dp.sas {
		if sa.EncAlgo != "null" {
			t.Errorf("SA[%d] cipher = %q, want the null cipher", i, sa.EncAlgo)
		}
		if len(sa.EncKey) != 0 {
			t.Errorf("SA[%d] carries a %d-octet encryption key, want none for the null cipher", i, len(sa.EncKey))
		}
		if sa.AuthAlgo != "sha256" {
			t.Errorf("SA[%d] integrity = %q, want sha256", i, sa.AuthAlgo)
		}
		if len(sa.AuthKey) != 32 {
			t.Errorf("SA[%d] integrity key = %d octets, want 32", i, len(sa.AuthKey))
		}
	}
}
