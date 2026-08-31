// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- remote identity policy
// Related: remote_id.go -- remoteIDMatches and certificateCarriesIdentity, the producers
// RFC: rfc/short/rfc4301.md -- Peer Authorization Database entry matching (Section 4.4.3.1)
package engine

import (
	"crypto/x509/pkix"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// padSA builds a pre-shared-secret SA whose remote-id holds the PAD entry under test.
// Pre-shared secret is used because it isolates the POLICY half: no certificate exists,
// so a refusal can only come from the identity comparison.
func padSA(t *testing.T, remoteID string, idType uint8, data []byte) *SA {
	t.Helper()
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "shared-secret"
	sa.PeerCfg.Auth.RemoteID = remoteID
	ridAssert(sa, idType, data)
	return sa
}

// VALIDATES: RFC4301-4.4.3.1-1. A PAD entry written as a sub-tree matches every peer
// under that sub-tree, for each of the three name types Section 4.4.3.1 names: a domain
// name, an RFC 822 mail address, and a distinguished name.
// PREVENTS: an operator having to write one PAD entry per peer. Every comparison in
// remoteIDMatches was exact, so ".example.com" matched the single peer whose ID_FQDN is
// the literal text ".example.com" and no other.
// RFC requirement: RFC4301-4.4.3.1-1 positive -- a sub-tree entry admits a peer beneath it.
func TestPadSubtreeAdmitsAPeerBeneathIt(t *testing.T) {
	dnBelow, _ := cfmDN(t, pkix.Name{
		Country:      []string{"US"},
		Province:     []string{"MA"},
		Organization: []string{"BBN Technologies"},
		CommonName:   "Stephen",
	})

	for _, tc := range []struct {
		name     string
		remoteID string
		idType   uint8
		asserted []byte
	}{
		{"domain name", ".example.com", wire.IDTypeFQDN, []byte("vpn.example.com")},
		{"domain name, deeper", ".example.com", wire.IDTypeFQDN, []byte("gw.branch.example.com")},
		{"mail address", "@example.com", wire.IDTypeRFC822Addr, []byte("gateway@example.com")},
		{"distinguished name", ",ST=MA,C=US", wire.IDTypeDERASN1DN, dnBelow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := padSA(t, tc.remoteID, tc.idType, tc.asserted)
			if err := verifyRemoteAuth(sa, ridPSKAuth(t, sa)); err != nil {
				t.Fatalf("the sub-tree entry %q refused a peer beneath it: %v", tc.remoteID, err)
			}
		})
	}
}

// VALIDATES: RFC4301-4.4.3.1-1. A sub-tree entry refuses a peer outside the sub-tree,
// including the near misses a suffix comparison alone would admit.
// PREVENTS: the sub-tree widening into a substring match. Section 4.4.3.1 makes substring
// matching optional ("MAY be supported, but is not required"), so "notexample.com" must
// not be admitted by ".example.com", and the bare apex must not be admitted either.
// RFC requirement: RFC4301-4.4.3.1-1 negative -- a sub-tree entry refuses a peer outside it.
func TestPadSubtreeRefusesAPeerOutsideIt(t *testing.T) {
	dnOutside, _ := cfmDN(t, pkix.Name{
		Country:    []string{"US"},
		Province:   []string{"CA"},
		CommonName: "Stephen",
	})
	dnApex, _ := cfmDN(t, pkix.Name{
		Country:  []string{"US"},
		Province: []string{"MA"},
	})

	for _, tc := range []struct {
		name     string
		remoteID string
		idType   uint8
		asserted []byte
	}{
		{"another domain", ".example.com", wire.IDTypeFQDN, []byte("vpn.example.net")},
		{"substring, not a label boundary", ".example.com", wire.IDTypeFQDN, []byte("notexample.com")},
		{"the apex itself", ".example.com", wire.IDTypeFQDN, []byte("example.com")},
		{"an empty label", ".example.com", wire.IDTypeFQDN, []byte(".example.com")},
		{"another mail domain", "@example.com", wire.IDTypeRFC822Addr, []byte("gateway@example.net")},
		{"no local part", "@example.com", wire.IDTypeRFC822Addr, []byte("@example.com")},
		{"a different province", ",ST=MA,C=US", wire.IDTypeDERASN1DN, dnOutside},
		{"the sub-tree root itself", ",ST=MA,C=US", wire.IDTypeDERASN1DN, dnApex},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := padSA(t, tc.remoteID, tc.idType, tc.asserted)
			if err := verifyRemoteAuth(sa, ridPSKAuth(t, sa)); err == nil {
				t.Fatalf("the sub-tree entry %q admitted %q, which is outside it",
					tc.remoteID, tc.asserted)
			}
		})
	}
}

// VALIDATES: RFC4301-4.4.3.1-1. A KEY_ID entry stays exact even when its text carries a
// sub-tree marker.
// PREVENTS: the sub-tree syntax leaking onto the one ID type Section 4.4.3.1 excludes:
// "For this name type, only exact-match syntax MUST be supported (since there is no
// explicit structure for this ID type)."
// RFC requirement: RFC4301-4.4.3.1-1 negative -- a key id is never matched as a sub-tree.
func TestPadKeyIDStaysExact(t *testing.T) {
	sa := padSA(t, ".example.com", wire.IDTypeKeyID, []byte("vpn.example.com"))
	if err := verifyRemoteAuth(sa, ridPSKAuth(t, sa)); err == nil {
		t.Fatal("an ID_KEY_ID was matched against a sub-tree entry")
	}

	exact := padSA(t, ".example.com", wire.IDTypeKeyID, []byte(".example.com"))
	if err := verifyRemoteAuth(exact, ridPSKAuth(t, exact)); err != nil {
		t.Fatalf("an ID_KEY_ID equal to the entry text was refused: %v", err)
	}
}

// VALIDATES: RFC4301-4.4.3.1-2. An address-valued PAD entry accepts the address range
// syntax Ze's SPD entries use, which is the CIDR prefix of zt:ip-prefix, and a peer whose
// asserted address falls inside the prefix is admitted for both address families.
// PREVENTS: an operator having to write one PAD entry per tunnel endpoint. netip.ParseAddr
// rejects a prefix, so "10.0.0.0/24" classified as text and matched no address at all.
// RFC requirement: RFC4301-4.4.3.1-2 positive -- an address range admits an address inside it.
func TestPadAddressRangeAdmitsAnAddressInsideIt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		remoteID string
		idType   uint8
		asserted []byte
	}{
		{"ipv4 prefix", "10.0.0.0/24", wire.IDTypeIPv4Addr, net.IPv4(10, 0, 0, 7).To4()},
		{"ipv4 host prefix", "10.0.0.7/32", wire.IDTypeIPv4Addr, net.IPv4(10, 0, 0, 7).To4()},
		{"ipv6 prefix", "2001:db8::/32", wire.IDTypeIPv6Addr, net.ParseIP("2001:db8::1").To16()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := padSA(t, tc.remoteID, tc.idType, tc.asserted)
			if err := verifyRemoteAuth(sa, ridPSKAuth(t, sa)); err != nil {
				t.Fatalf("the address range %q refused an address inside it: %v", tc.remoteID, err)
			}
		})
	}
}

// VALIDATES: RFC4301-4.4.3.1-2. An address-valued PAD entry refuses an address outside the
// range, refuses an address of the other family, and refuses a text identity spelling an
// address inside the range.
// PREVENTS: the range widening the class gate. An address entry accepts ID_IPV4_ADDR and
// ID_IPV6_ADDR alone, whether it is written as a single address or as a prefix.
// RFC requirement: RFC4301-4.4.3.1-2 negative -- an address range refuses what is outside it.
func TestPadAddressRangeRefusesWhatIsOutsideIt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		remoteID string
		idType   uint8
		asserted []byte
	}{
		{"outside the prefix", "10.0.0.0/24", wire.IDTypeIPv4Addr, net.IPv4(10, 0, 1, 7).To4()},
		{"another family", "10.0.0.0/24", wire.IDTypeIPv6Addr, net.ParseIP("2001:db8::1").To16()},
		{"text spelling an address inside", "10.0.0.0/24", wire.IDTypeFQDN, []byte("10.0.0.7")},
		{"text spelling the prefix", "10.0.0.0/24", wire.IDTypeFQDN, []byte("10.0.0.0/24")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := padSA(t, tc.remoteID, tc.idType, tc.asserted)
			if err := verifyRemoteAuth(sa, ridPSKAuth(t, sa)); err == nil {
				t.Fatalf("the address range %q admitted %q", tc.remoteID, tc.asserted)
			}
		})
	}
}

// VALIDATES: RFC4301-4.4.3.1-1. The certificate half binds the ASSERTED identity, so a
// sub-tree entry still requires the authority to have attested this peer's own name, and
// a peer beneath the sub-tree holding another peer's certificate is denied.
// PREVENTS: the policy half and the binding half disagreeing. certificateCarriesIdentity
// bound on the CONFIGURED value, which is correct while the two are equal and wrong the
// moment an entry admits a set. A sub-tree entry bound on ".example.com" would have looked
// for that literal in the certificate and denied every legitimate peer, or, read the other
// way, admitted any certificate the authority issued.
// RFC requirement: RFC4301-4.4.3.1-1 positive -- a sub-tree entry still binds the certificate.
func TestPadSubtreeStillBindsTheCertificate(t *testing.T) {
	anchor, sign := ridAnchor(t)

	der, key := sign(t, "vpn.example.com", nil, []string{"vpn.example.com"})
	admitted := ridCertSA(t, anchor, ".example.com")
	admitted.RemoteCertRaw = der
	ridAssert(admitted, wire.IDTypeFQDN, []byte("vpn.example.com"))
	if err := verifyRemoteAuth(admitted, rctDigitalSigAuth(t, admitted, key)); err != nil {
		t.Fatalf("a peer beneath the sub-tree with its own certificate was refused: %v", err)
	}

	// The same authority, a certificate issued to another name under the same sub-tree.
	// The peer asserts a name the sub-tree admits and its certificate does not carry.
	otherDER, otherKey := sign(t, "other.example.com", nil, []string{"other.example.com"})
	impostor := ridCertSA(t, anchor, ".example.com")
	impostor.RemoteCertRaw = otherDER
	ridAssert(impostor, wire.IDTypeFQDN, []byte("vpn.example.com"))
	err := verifyRemoteAuth(impostor, rctDigitalSigAuth(t, impostor, otherKey))
	if err == nil {
		t.Fatal("a certificate issued to another name authenticated under a sub-tree entry")
	}
	if !strings.Contains(err.Error(), "vpn.example.com") {
		t.Errorf("the refusal %q does not name the asserted identity", err)
	}
}
