package engine

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// VALIDATES: buildPeerTLSConfig, the second runtime enforcement point named in
// ipsec/validate.go, refuses an EAP-TLS peer that has no certificate, and
// refuses one whose ca-certificate cannot be resolved in the PKI store.
// PREVENTS: The RFC 5216 Section 5.3 fail-open the peer-side change removed
// (eap/peer.go) surviving on the initiator-config path. A nil return here drives
// sa.State = StateDead at the call site (fsm.go), so the session never starts
// rather than starting with no trust anchor.
func TestBuildPeerTLSConfigRefusesWithoutMaterial(t *testing.T) {
	log := slogutil.DiscardLogger()

	tests := []struct {
		name string
		auth ipsec.AuthConfig
	}{
		{"no certificate", ipsec.AuthConfig{Mode: ipsec.AuthEAPTLS}},
		{"certificate not in store", ipsec.AuthConfig{Mode: ipsec.AuthEAPTLS, Certificate: "absent-cert"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sa := &SA{PeerName: "branch", PeerCfg: ipsec.SiteToSitePeer{Auth: tc.auth}}
			if cfg := buildPeerTLSConfig(sa, log); cfg != nil {
				t.Errorf("buildPeerTLSConfig returned a config for %s; want nil so the SA dies", tc.name)
			}
		})
	}
}
