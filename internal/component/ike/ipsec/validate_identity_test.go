package ipsec

import (
	"strings"
	"testing"
)

// vidPeer builds one peer carrying the given identities.
func vidPeer(localID, remoteID string) *IPsecConfig {
	return &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"branch": {Auth: AuthConfig{
				Mode: AuthPreSharedSecret, PSK: "secret",
				LocalID: localID, RemoteID: remoteID,
			}},
		},
	}
}

// VALIDATES: a distinguished-name LOCAL-id is refused at verify, and the refusal names
// the peer, the leaf, and the value.
// PREVENTS: a config that commits cleanly and then asserts an identity no peer expecting
// a distinguished name accepts. encodeIKEID derives the type ze SENDS from the shape of
// the value, and it has no ID_DER_ASN1_DN branch. A distinguished-name local-id therefore
// goes out as ID_FQDN carrying the literal text (ai/rules/exact-or-reject.md).
func TestVidDistinguishedNameLocalIDIsRefusedAtVerify(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"local-id", "CN=branch,OU=Field"},
		{"local-id lower case", "cn=branch"},
		{"local-id with spaces", "C = FR, O = Example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := vidPeer(tc.value, "").ValidateIdentities()
			if err == nil {
				t.Fatalf("local-id %q was accepted, and ze cannot send it as ID_DER_ASN1_DN", tc.value)
			}
			for _, want := range []string{"branch", "local-id", tc.value, "ID_DER_ASN1_DN"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal %q does not name %q", err, want)
				}
			}
		})
	}
}

// VALIDATES: a distinguished-name REMOTE-id commits.
// PREVENTS: the commit-time refusal outliving the engine capability it existed to
// announce. Ze used to refuse both leaves, because it had no way to compare an
// ID_DER_ASN1_DN at all. RFC 7296 Section 4 requires it be possible to configure ze to
// accept a PKIX certificate "where the ID passed is any of ID_KEY_ID, ID_FQDN,
// ID_RFC822_ADDR, or ID_DER_ASN1_DN", and assertedIdentity plus certificateCarriesIdentity
// now do that. A refusal left here would keep the MUST unreachable from config while the
// engine can serve it (RFC7296-4-4).
func TestVidDistinguishedNameRemoteIDCommits(t *testing.T) {
	for _, value := range []string{
		"CN=hq,O=Example", "cn=hq", "C = FR, O = Example",
	} {
		if err := vidPeer("", value).ValidateIdentities(); err != nil {
			t.Errorf("remote-id %q was refused, so RFC 7296 Section 4's ID_DER_ASN1_DN "+
				"acceptance cannot be configured: %v", value, err)
		}
	}
}

// VALIDATES: every identity form Ze does compare is accepted.
// PREVENTS: the refusal above turning into a blanket rejection. An address, a name, a
// mail address and an opaque key id are all comparable, and an unset identity means the
// operator stated no expectation.
func TestVidComparableIdentitiesAreAccepted(t *testing.T) {
	for _, value := range []string{
		"", "172.28.0.3", "2001:db8::3", "vpn.example.com", "user@example.com",
		"branch-42", "testuser", "gw=1", "serial=ABC123",
	} {
		cfg := vidPeer(value, value)
		if err := cfg.ValidateIdentities(); err != nil {
			t.Errorf("identity %q was refused: %v", value, err)
		}
	}
}
