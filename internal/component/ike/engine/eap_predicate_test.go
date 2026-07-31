package engine

import (
	"strings"
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// eapmAllModes returns every declared authentication mode. It walks the enum until a
// value has no name, so a mode added later is covered without this list being
// edited. A hardcoded list is what lets the engine and the config gate drift apart
// (ai/rules/derive-not-hardcode.md).
func eapmAllModes() []ipsec.AuthMode {
	var out []ipsec.AuthMode
	for m := ipsec.AuthMode(1); m.String() != "unknown"; m++ {
		out = append(out, m)
	}
	return out
}

// eapmSharedKeyAuth builds the shared-key AUTH the remote party would send on this
// SA, using the configured pre-shared key. RFC 7296 Section 2.15.
func eapmSharedKeyAuth(t *testing.T, sa *SA) *wire.PayloadAUTH {
	t.Helper()
	octets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		t.Fatalf("compute the signed octets: %v", err)
	}
	derived, err := ikecrypto.PRF(sa.Proposal.PRF.ID, []byte(sa.PeerCfg.Auth.PSK), []byte("Key Pad for IKEv2"))
	if err != nil {
		t.Fatalf("derive the key: %v", err)
	}
	data, err := ikecrypto.PRF(sa.Proposal.PRF.ID, derived, octets)
	if err != nil {
		t.Fatalf("compute the AUTH data: %v", err)
	}
	return &wire.PayloadAUTH{AuthMethod: wire.AuthMethodPSK, AuthData: data}
}

// eapmSection216 is the requirement an EAP refusal must cite.
const eapmSection216 = "RFC 7296 Section 2.16"

// VALIDATES: the engine refuses a shared-key AUTH for exactly the modes the config
// layer calls EAP. One predicate decides both, so the two cannot disagree. The
// pre-shared-secret mode still verifies, so the refusal is the EAP branch and not a
// blanket refusal of every shared-key AUTH.
// PREVENTS: a private copy of the predicate that goes stale. A third EAP mode added
// to ipsec.IsEAPMode alone leaves verifyRemoteAuth computing false for it. The
// shared-key AUTH that RFC 7296 Section 2.16 forbids then verifies again, while the
// config gate calls the peer compliant.
//
// The walk asks a mode-specific question rather than one question for every mode. An
// earlier form set a pre-shared key on EVERY mode and required each non-EAP mode to
// ACCEPT the shared-key AUTH. For x509 that made "an x509 peer with a pre-shared key
// configured authenticates by that key" a REQUIRED behavior, which is the mirror of the
// EAP defect this test exists to catch. It is unreachable today only because
// parseAuthConfig never fills PSK for x509 (ipsec/config.go). A later change that filled
// it would find this test demanding the cross-mode acceptance, and
// ai/rules/no-test-deletion.md would make the demand hard to remove. The x509 arm now
// asserts only that the EAP branch did not fire, which is this test's subject.
func TestEapmEngineAgreesWithTheCanonicalEAPPredicate(t *testing.T) {
	modes := eapmAllModes()
	if len(modes) < 4 {
		t.Fatalf("the enum walk found %d modes, want at least the four declared", len(modes))
	}
	sawEAP, sawShared, sawOther := false, false, false

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			sa := testSAWithKeys(t)
			sa.PeerCfg.Auth.Mode = mode
			sa.PeerCfg.Auth.PSK = "shared-secret"

			err := verifyRemoteAuth(sa, eapmSharedKeyAuth(t, sa))

			if ipsec.IsEAPMode(mode) {
				sawEAP = true
				if err == nil {
					t.Fatal("a shared-key AUTH verified on an EAP peer")
				}
				if !strings.Contains(err.Error(), eapmSection216) {
					t.Errorf("the refusal %q does not cite the requirement", err)
				}
				return
			}

			if mode == ipsec.AuthPreSharedSecret {
				// The one mode a shared key is the credential for. Its acceptance is
				// what keeps the EAP arm above from passing on a blanket refusal.
				sawShared = true
				if err != nil {
					t.Fatalf("a shared-key AUTH was refused on a pre-shared-secret peer: %v", err)
				}
				return
			}

			// Every other non-EAP mode. Whether a shared-key AUTH verifies here is not
			// this test's subject. Asserting either answer would pin a behavior this test
			// was not written to decide. Only the EAP branch is in scope.
			sawOther = true
			if err != nil && strings.Contains(err.Error(), eapmSection216) {
				t.Errorf("the EAP refusal fired on a %s peer: %v", mode, err)
			}
		})
	}

	if !sawEAP || !sawShared || !sawOther {
		t.Errorf("the walk saw eap=%v pre-shared-secret=%v other=%v, want all three",
			sawEAP, sawShared, sawOther)
	}
}
