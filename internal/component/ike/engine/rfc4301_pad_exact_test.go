// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- remote identity policy
// Related: remote_id.go -- remoteIDMatches and certificateCarriesIdentity, the producers
// RFC: rfc/short/rfc4301.md -- PAD exact-match name type (Section 4.4.3.1), IKE ID to certificate binding (Section 4.4.3.2)
package engine

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// VALIDATES: RFC4301-4.4.3.1-3. A PAD entry for the "other ID type", which is ID_KEY_ID,
// admits the peer whose asserted octets equal the entry exactly, whatever those octets
// spell: plain text, a sub-tree marker, or a value with a trailing dot.
// PREVENTS: an exact key id being refused because its text happens to look like one of
// the structured forms the other name types give a meaning to.
// RFC requirement: RFC4301-4.4.3.1-3 positive -- a key id equal to the PAD entry is admitted, whatever the text spells.
func TestRFC4301PadOtherIDTypeAdmitsAnExactMatch(t *testing.T) {
	for _, id := range []string{"branch-7", ".example.com", "@example.com", "vpn.example.com."} {
		t.Run(id, func(t *testing.T) {
			sa := padSA(t, id, wire.IDTypeKeyID, []byte(id))
			if err := verifyRemoteAuth(sa, ridPSKAuth(t, sa)); err != nil {
				t.Fatalf("an ID_KEY_ID equal to the entry %q was refused: %v", id, err)
			}
		})
	}
}

// VALIDATES: RFC4301-4.4.3.1-3. Only exact-match syntax exists for ID_KEY_ID: a peer
// whose key id is a prefix, a suffix, a case variant, or a sub-tree descendant of the
// entry is refused, and so is a key id asserted against an entry of another name type.
// PREVENTS: the structured matching of the name types (sub-tree, case folding, trailing
// dot) leaking onto the one type the section says has no structure.
// RFC requirement: RFC4301-4.4.3.1-3 negative -- a key id that is not octet-for-octet equal to the PAD entry is refused.
func TestRFC4301PadOtherIDTypeRefusesEverythingButExact(t *testing.T) {
	for _, tc := range []struct {
		name     string
		remoteID string
		asserted string
	}{
		{"prefix", "branch-7", "branch"},
		{"suffix", "branch-7", "7"},
		{"longer", "branch-7", "branch-7-lab"},
		{"case variant", "branch-7", "BRANCH-7"},
		{"trailing dot", "branch-7", "branch-7."},
		{"sub-tree descendant", ".example.com", "vpn.example.com"},
		{"mail sub-tree descendant", "@example.com", "gw@example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := padSA(t, tc.remoteID, wire.IDTypeKeyID, []byte(tc.asserted))
			if err := verifyRemoteAuth(sa, ridPSKAuth(t, sa)); err == nil {
				t.Fatalf("an ID_KEY_ID %q was admitted by the entry %q", tc.asserted, tc.remoteID)
			}
		})
	}
}

// VALIDATES: RFC4301-4.4.3.2-1. With remote-id configured on a certificate-authenticated
// peer, the asserted IKE ID is required to appear in the certificate: a peer asserting
// the name its certificate's subject alternative name carries is admitted, and so is one
// whose certificate has no alternative name and carries the name as its subject common
// name.
// PREVENTS: the binding half denying a legitimate peer whose authority attested the name
// in either of the two places the section names.
// RFC requirement: RFC4301-4.4.3.2-1 positive -- an asserted IKE ID carried by the certificate's subject alt name, or by its subject name when no alt name exists, is admitted.
func TestRFC4301PadRequiresTheCertificateToCarryTheIKEID(t *testing.T) {
	anchor, sign := ridAnchor(t)

	for _, tc := range []struct {
		name string
		cn   string
		dns  []string
	}{
		{"subject alt name", "ignored", []string{"vpn.example.com"}},
		{"subject name, no alt name", "vpn.example.com", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			der, key := sign(t, tc.cn, nil, tc.dns)
			sa := ridCertSA(t, anchor, "vpn.example.com")
			sa.RemoteCertRaw = der
			ridAssert(sa, wire.IDTypeFQDN, []byte("vpn.example.com"))
			if err := verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, key)); err != nil {
				t.Fatalf("a peer whose certificate carries the asserted id was refused: %v", err)
			}
		})
	}
}

// VALIDATES: RFC4301-4.4.3.2-1. The required match is enforced: a peer asserting the
// configured IKE ID with a valid signature from a certificate the same authority issued
// to ANOTHER name is refused, and the refusal names the asserted identity. A common name
// is not consulted once an alternative name extension exists.
// PREVENTS: any certificate the authority issued satisfying the entry, which is the hole
// the section's control exists to close.
// RFC requirement: RFC4301-4.4.3.2-1 negative -- a certificate that does not carry the asserted IKE ID in its subject alt name, or in its subject name when no alt name exists, is refused.
func TestRFC4301PadRefusesACertificateWithoutTheIKEID(t *testing.T) {
	anchor, sign := ridAnchor(t)

	for _, tc := range []struct {
		name string
		cn   string
		dns  []string
	}{
		{"alt name for another peer", "vpn.example.com", []string{"other.example.com"}},
		{"subject name for another peer, no alt name", "other.example.com", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			der, key := sign(t, tc.cn, nil, tc.dns)
			sa := ridCertSA(t, anchor, "vpn.example.com")
			sa.RemoteCertRaw = der
			ridAssert(sa, wire.IDTypeFQDN, []byte("vpn.example.com"))
			err := verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, key))
			if err == nil {
				t.Fatal("a certificate that does not carry the asserted id authenticated the peer")
			}
			if !strings.Contains(err.Error(), "vpn.example.com") {
				t.Errorf("the refusal %q does not name the asserted identity", err)
			}
		})
	}
}
