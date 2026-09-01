// VALIDATES: two things one shared declaration now decides. The engine's two EAP
// producers read ipsec.IsEAPPasswordMode and nothing else. The pre-shared-key AUTH is
// the same RFC 7296 Section 2.15 construction the EAP AUTH uses.
// PREVENTS: the state before 2026-09-01. eapMethodConfig (responder_eap.go) and
// startEAPExchange (fsm.go) each carried their own two-item list of password methods.
// computePSKAuth and verifyPSKAuth (auth.go) each wrote Section 2.15's formula by hand
// beside computeAuthFromSharedSecret (eap_auth.go), pad string included. A change to one
// copy left the others answering the old way, and nothing compared them
// (ai/rules/principles.md).
package engine

import (
	"bytes"
	"log/slog"
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// eapPasswordSecret is the credential every case below configures. A producer that
// stopped reading it answers with the empty string instead.
const eapPasswordSecret = "engine-eap-password-secret" //nolint:gosec // test fixture

// rfc7296Section215Pad is the pad string of RFC 7296 Section 2.15, written out here
// rather than read from keyPadForIKEv2. A test that took the string from the code it
// judges would pass over any string at all.
const rfc7296Section215Pad = "Key Pad for IKEv2"

// section215AUTH states RFC 7296 Section 2.15's formula independently of the engine:
// "AUTH = prf(prf(Shared Secret, "Key Pad for IKEv2"), <InitiatorSignedOctets>)". It is
// the oracle both AUTH producers are held to below.
func section215AUTH(t *testing.T, prfID ikecrypto.PRFID, secret, signedOctets []byte) []byte {
	t.Helper()

	inner, err := ikecrypto.PRF(prfID, secret, []byte(rfc7296Section215Pad))
	if err != nil {
		t.Fatalf("prf(secret, pad): %v", err)
	}
	auth, err := ikecrypto.PRF(prfID, inner, signedOctets)
	if err != nil {
		t.Fatalf("prf(inner, signed octets): %v", err)
	}
	return auth
}

// TestEAPPasswordModeDecidesBothEngineProducers walks every mode the ipsec package names
// and checks the authenticator's method configuration and the peer's session constructor
// against the one predicate.
func TestEAPPasswordModeDecidesBothEngineProducers(t *testing.T) {
	passwordModes, otherModes := 0, 0
	for _, mode := range declaredAuthModes(t) {
		if ipsec.IsEAPPasswordMode(mode) {
			passwordModes++
		} else {
			otherModes++
		}
		t.Run(authModeCaseName(mode), func(t *testing.T) {
			assertMethodConfigCarriesThePassword(t, mode)
			assertPeerSessionRunsOnThePassword(t, mode)
		})
	}

	// Without these two counts the assertions above are satisfied by a tree in which
	// the predicate names every mode, or none.
	if passwordModes == 0 {
		t.Error("ipsec.IsEAPPasswordMode names no declared mode, so no password assertion ran")
	}
	if otherModes == 0 {
		t.Error("ipsec.IsEAPPasswordMode names every declared mode, so no refusal assertion ran")
	}
}

// assertMethodConfigCarriesThePassword checks the authenticator producer: eapMethodConfig
// builds a method configuration from the configured secret for exactly the modes
// ipsec.IsEAPPasswordMode names.
//
// The SA names no certificate, so EAP-TLS fails in eapTLSServerConfig and a non-EAP mode
// fails at the last line. Neither can leave a password behind, which is what makes this a
// question about the password list rather than about the PKI store.
func assertMethodConfigCarriesThePassword(t *testing.T, mode ipsec.AuthMode) {
	t.Helper()

	sa := &SA{
		PeerName: "engine-password-walk",
		PeerCfg: ipsec.SiteToSitePeer{
			Auth: ipsec.AuthConfig{Mode: mode, PSK: eapPasswordSecret},
		},
	}

	config, err := eapMethodConfig(sa)
	got := err == nil && config.Password == eapPasswordSecret
	if got != ipsec.IsEAPPasswordMode(mode) {
		t.Errorf("eapMethodConfig built a password method = %v for mode %s, and "+
			"ipsec.IsEAPPasswordMode says %v (error %v)",
			got, mode, ipsec.IsEAPPasswordMode(mode), err)
	}
}

// assertPeerSessionRunsOnThePassword checks the peer producer: startEAPExchange builds an
// eap.PeerSession from the configured secret for exactly the modes
// ipsec.IsEAPPasswordMode names.
//
// The SA names no client certificate, so buildPeerTLSConfig refuses EAP-TLS and the
// default arm refuses every non-EAP mode. Both leave sa.EAPSession nil, so a session on
// the SA can only have come from the password arm.
//
// The EAP payload carries an EAP-Success, which RFC 3748 Section 4.2 makes the peer
// discard at the start of a conversation. The arm has already chosen the method and
// stored the session by then, and no response leaves the peer, so no transport is needed.
func assertPeerSessionRunsOnThePassword(t *testing.T, mode ipsec.AuthMode) {
	t.Helper()

	sa := &SA{
		PeerName: "engine-password-walk",
		PeerCfg: ipsec.SiteToSitePeer{
			Auth: ipsec.AuthConfig{
				Mode:    mode,
				PSK:     eapPasswordSecret,
				LocalID: "engine-password-user",
			},
		},
	}
	startEAPExchange(sa, &wire.PayloadEAP{Code: eap.CodeSuccess, Identifier: 1}, nil, slog.New(slog.DiscardHandler))

	if peer, ok := sa.EAPSession.(*eap.PeerSession); ok {
		t.Cleanup(peer.Close)
	}
	started := sa.EAPSession != nil && sa.State == StateEAPInProgress
	if started != ipsec.IsEAPPasswordMode(mode) {
		t.Errorf("startEAPExchange started a password peer session = %v for mode %s (state %v), "+
			"and ipsec.IsEAPPasswordMode says %v",
			started, mode, sa.State, ipsec.IsEAPPasswordMode(mode))
	}
}

// TestPSKAuthIsTheOneSharedSecretConstruction checks that a pre-shared key travels the
// same RFC 7296 Section 2.15 construction an EAP secret does. It also checks that the
// verifier answers over the value that construction produces.
//
// Both producers are compared against section215AUTH rather than against each other. Two
// producers that agree prove only that they agree. The oracle is what says they agree on
// the formula the RFC writes.
func TestPSKAuthIsTheOneSharedSecretConstruction(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = eapPasswordSecret

	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}
	want := section215AUTH(t, sa.Proposal.PRF.ID, []byte(sa.PeerCfg.Auth.PSK), signedOctets)

	auth, err := computePSKAuth(sa)
	if err != nil {
		t.Fatalf("computePSKAuth: %v", err)
	}
	if !bytes.Equal(auth.AuthData, want) {
		t.Errorf("computePSKAuth produced %x, and RFC 7296 Section 2.15 over the same secret "+
			"gives %x", auth.AuthData, want)
	}

	shared, err := computeAuthFromSharedSecret(sa.Proposal.PRF.ID, []byte(sa.PeerCfg.Auth.PSK), signedOctets)
	if err != nil {
		t.Fatalf("computeAuthFromSharedSecret: %v", err)
	}
	if !bytes.Equal(shared, want) {
		t.Errorf("computeAuthFromSharedSecret produced %x over the pre-shared key, and "+
			"RFC 7296 Section 2.15 gives %x", shared, want)
	}

	if err := verifyPSKAuth(sa, want, signedOctets); err != nil {
		t.Errorf("verifyPSKAuth refused the value RFC 7296 Section 2.15 gives: %v", err)
	}

	// The two ways an authenticator can differ. Without them the acceptance above is
	// satisfied by a verifier that compares nothing.
	flipped := bytes.Clone(want)
	flipped[len(flipped)-1] ^= 0x01
	if err := verifyPSKAuth(sa, flipped, signedOctets); err == nil {
		t.Error("verifyPSKAuth accepted an authenticator differing in one bit")
	}
	if err := verifyPSKAuth(sa, want[:len(want)-1], signedOctets); err == nil {
		t.Error("verifyPSKAuth accepted a truncated authenticator")
	}
}
