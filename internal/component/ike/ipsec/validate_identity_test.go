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

// VALIDATES: a distinguished-name identity is refused at verify, and the refusal names
// the peer, the leaf, and the value.
// PREVENTS: a config that commits cleanly and then denies every peer. Ze compares
// ID_IPV4_ADDR, ID_IPV6_ADDR, ID_FQDN, ID_RFC822_ADDR and ID_KEY_ID, and states that it
// cannot compare ID_DER_ASN1_DN. A peer whose identity is a distinguished name asserts
// ID_DER_ASN1_DN, so remote-id "CN=hq,O=Example" denied it at IKE_AUTH with a message
// only the log carried (ai/rules/exact-or-reject.md).
func TestVidDistinguishedNameIdentityIsRefusedAtVerify(t *testing.T) {
	for _, tc := range []struct {
		name  string
		leaf  string
		value string
	}{
		{"remote-id", "remote-id", "CN=hq,O=Example"},
		{"remote-id lower case", "remote-id", "cn=hq"},
		{"remote-id with spaces", "remote-id", "C = FR, O = Example"},
		{"local-id", "local-id", "CN=branch,OU=Field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := vidPeer("", tc.value)
			if tc.leaf == "local-id" {
				cfg = vidPeer(tc.value, "")
			}
			err := cfg.ValidateIdentities()
			if err == nil {
				t.Fatalf("%s %q was accepted, and no peer can ever satisfy it", tc.leaf, tc.value)
			}
			for _, want := range []string{"branch", tc.leaf, tc.value, "ID_DER_ASN1_DN"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal %q does not name %q", err, want)
				}
			}
		})
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
