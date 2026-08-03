// Design: rfc/short/rfc7296.md -- Section 2.16, EAP responder authentication
// Related: responder_eap.go -- computeServerAuth, the guard these tests drive

package engine

import (
	"bytes"
	"strings"
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// eapAuthCfg builds a peer auth config for an EAP mode. A PSK is always present,
// because eap-mschapv2 carries the user password there. That matters for the
// negative case. The pre-shared credential a fallback would reach for is
// available. A refusal therefore proves a deliberate choice, not a missing key.
// An empty certName is the misconfiguration the guard must refuse.
func eapAuthCfg(mode ipsec.AuthMode, certName string) ipsec.AuthConfig {
	cfg := ipsec.AuthConfig{Mode: mode, PSK: "eap-user-password"}
	if certName != "" {
		cfg.Certificate = certName
		cfg.CACertificate = autCAName
	}
	return cfg
}

// eapAuthAllPresent is the PKI lookup trio for ValidatePKIRefs when every named
// object resolves. The tests here are about the certificate being ABSENT from the
// config, not about it being absent from the store.
func eapAuthAllPresent() (hasCA, hasCert func(string) bool, certCN func(string) string) {
	return func(string) bool { return true },
		func(string) bool { return true },
		func(string) string { return "" }
}

// eapAuthForgedPSKAuth builds the AUTH payload a non-conformant responder would
// send. It is a shared-secret MAC over the REMOTE role's signed octets, computed
// as RFC 7296 Section 2.15 specifies. computePSKAuth cannot stand in for this. It
// signs the LOCAL role's octets, so verifyRemoteAuth would reject the result on the
// MAC comparison rather than on the rule under test.
//
// The secret is the peer's PSK, which for eap-mschapv2 is the user password. That
// is the point of the RFC obligation: password knowledge must not let a responder
// pass for the network.
func eapAuthForgedPSKAuth(t *testing.T, sa *SA) *wire.PayloadAUTH {
	t.Helper()
	signedOctets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}
	prfID := sa.Proposal.PRF.ID
	derivedKey, err := ikecrypto.PRF(prfID, []byte(sa.PeerCfg.Auth.PSK), []byte("Key Pad for IKEv2"))
	if err != nil {
		t.Fatalf("PRF over the key pad: %v", err)
	}
	authData, err := ikecrypto.PRF(prfID, derivedKey, signedOctets)
	if err != nil {
		t.Fatalf("PRF over the signed octets: %v", err)
	}
	return &wire.PayloadAUTH{AuthMethod: wire.AuthMethodPSK, AuthData: authData}
}

// eapAuthPeerConfig wraps one peer in an IPsecConfig for ValidatePKIRefs.
func eapAuthPeerConfig(name string, auth ipsec.AuthConfig) *ipsec.IPsecConfig {
	return &ipsec.IPsecConfig{
		Peers: map[string]ipsec.SiteToSitePeer{
			name: {Name: name, Auth: auth},
		},
	}
}

// VALIDATES: RFC7296-2.16-11. When EAP authenticates the initiator, the responder
// authenticates itself with a public-key signature. The AUTH payload carries the
// digital-signature method. The signature verifies against the public key of the
// configured certificate, over the IKE signed octets.
// PREVENTS: an EAP exchange whose responder AUTH is a shared-secret MAC. Such an
// exchange gives the initiator no signature to check.
// RFC requirement: RFC7296-2.16-11 positive -- computeServerAuth (responder_eap.go:79-101) sends
// every EAP exchange to computeX509Auth (auth.go:262). The AUTH in the responder's
// first EAP message is therefore an RFC 7427 signature. It verifies under the
// certificate public key, and it differs from the pre-shared-key MAC of the same SA.
func TestEapAuthResponderSignsWithPublicKey(t *testing.T) {
	autLoadPKI(t)
	entry := pki.GetCertificate(autCertName)
	if entry == nil || entry.Certificate == nil {
		t.Fatalf("test fixture: certificate %q absent from the PKI store", autCertName)
	}

	for _, mode := range []ipsec.AuthMode{ipsec.AuthEAPMSCHAPv2, ipsec.AuthEAPTLS} {
		t.Run(mode.String(), func(t *testing.T) {
			_, resp, _ := autSAInitPair(t, eapAuthCfg(mode, autCertName))

			auth, err := computeServerAuth(resp)
			if err != nil {
				t.Fatalf("computeServerAuth: %v", err)
			}
			if auth.AuthMethod != wire.AuthMethodDigitalSig {
				t.Fatalf("AuthMethod = %d, want %d (digital signature)",
					auth.AuthMethod, wire.AuthMethodDigitalSig)
			}

			// The pre-shared credential is present, so a fallback would have
			// succeeded. Prove the responder did not take it.
			pskAuth, pskErr := computePSKAuth(resp)
			if pskErr != nil {
				t.Fatalf("computePSKAuth control: %v", pskErr)
			}
			if bytes.Equal(auth.AuthData, pskAuth.AuthData) {
				t.Fatal("responder AUTH equals the pre-shared-key MAC for the same SA")
			}

			// RFC 7427 Section 3: authData is a length octet, then the algorithm
			// identifier, then the signature. Verify the signature under the
			// certificate's public key. Nothing weaker proves "public-key
			// signature" as the RFC uses that phrase.
			if len(auth.AuthData) < 2 {
				t.Fatalf("authData too short for RFC 7427 form: %d bytes", len(auth.AuthData))
			}
			algLen := int(auth.AuthData[0])
			if algLen == 0 || 1+algLen >= len(auth.AuthData) {
				t.Fatalf("authData algorithm identifier length %d does not fit %d bytes",
					algLen, len(auth.AuthData))
			}
			algID := auth.AuthData[1 : 1+algLen]
			sig := auth.AuthData[1+algLen:]

			hashFunc := hashFromAlgID(algID)
			if hashFunc == nil {
				t.Fatalf("unrecognized RFC 7427 algorithm identifier %x", algID)
			}
			signedOctets, err := computeSignedOctets(resp, resp.IsInitiator)
			if err != nil {
				t.Fatalf("computeSignedOctets: %v", err)
			}
			h := hashFunc()
			h.Write(signedOctets)
			if err := verifySignature(entry.Certificate.PublicKey, h.Sum(nil), sig, hashFunc); err != nil {
				t.Fatalf("responder AUTH does not verify under the certificate public key: %v", err)
			}
		})
	}
}

// VALIDATES: RFC7296-2.16-11. An EAP peer with no certificate is refused at session
// setup. The responder returns an error and no AUTH payload at all, and in
// particular it does not answer with a pre-shared-key AUTH.
// PREVENTS: the fall-through that signed the responder AUTH with the pre-shared
// key. For eap-mschapv2 that is the same secret the user holds as a password.
// RFC requirement: RFC7296-2.16-11 negative -- computeServerAuth (responder_eap.go:79-101) returns
// an error and a nil payload when Auth.Certificate is empty. computePSKAuth on the same
// SA still succeeds, so the pre-shared credential was available and refused.
func TestEapAuthResponderRefusesWithoutCertificate(t *testing.T) {
	autLoadPKI(t)

	for _, mode := range []ipsec.AuthMode{ipsec.AuthEAPMSCHAPv2, ipsec.AuthEAPTLS} {
		t.Run(mode.String(), func(t *testing.T) {
			_, resp, _ := autSAInitPair(t, eapAuthCfg(mode, ""))

			// Control first. A PSK AUTH is computable on this SA, so the refusal
			// below cannot be explained by a missing pre-shared key.
			pskAuth, pskErr := computePSKAuth(resp)
			if pskErr != nil {
				t.Fatalf("control: computePSKAuth must succeed on this SA, got %v", pskErr)
			}

			auth, err := computeServerAuth(resp)
			if err == nil {
				t.Fatal("computeServerAuth accepted an EAP peer with no certificate")
			}
			// rfc-test-change-approved: 2026-07-31 the owner gave standing approval, for the
			// whole of plan/learned/1313-rfcgate-1b-rfc7296-pilot.md, to strengthen a tagged test
			// whose arm no input reaches. The AUTH-data comparison used to sit after an
			// unconditional failure on the same `auth != nil` condition, so `auth` was
			// provably nil by the time the comparison ran and it was dead in both directions
			// (go vet: "impossible condition: nil != nil"). Nesting it makes a reintroduced
			// pre-shared-key fallback fail on THAT line, which is what the arm was written
			// for. The approval covers strengthening only, never weakening.
			if auth != nil {
				// Name the specific outcome the RFC forbids before the general one, so a
				// reintroduced fallback is reported as itself and not as "an AUTH payload".
				if bytes.Equal(auth.AuthData, pskAuth.AuthData) {
					t.Fatal("responder answered an EAP exchange with a pre-shared-key AUTH")
				}
				t.Fatalf("refusal still returned an AUTH payload, method %d", auth.AuthMethod)
			}

			msg := err.Error()
			for _, want := range []string{"ze", "certificate", "RFC 7296 Section 2.16"} {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q does not mention %q", msg, want)
				}
			}
			if !strings.Contains(msg, mode.String()) {
				t.Errorf("error %q does not name the auth mode %q", msg, mode.String())
			}
		})
	}
}

// VALIDATES: RFC7296-2.16-11 at the config boundary. ValidatePKIRefs rejects an EAP
// peer that names no certificate, so the operator learns at verify time rather than
// at session setup.
// PREVENTS: a config that passes `ze config verify` and then leaves the daemon
// unable to meet the RFC obligation for every session it accepts.
// RFC requirement: RFC7296-2.16-11 negative -- ValidatePKIRefs (ipsec/validate.go) refuses an EAP
// peer whose Auth.Certificate is empty. It accepts the same peer once a certificate is
// named, so the rejection tracks the missing credential and not the mode alone.
func TestEapAuthConfigRejectsEAPWithoutCertificate(t *testing.T) {
	hasCA, hasCert, certCN := eapAuthAllPresent()

	for _, mode := range []ipsec.AuthMode{ipsec.AuthEAPMSCHAPv2, ipsec.AuthEAPTLS} {
		t.Run(mode.String(), func(t *testing.T) {
			cfg := eapAuthPeerConfig("branch", eapAuthCfg(mode, ""))
			err := cfg.ValidatePKIRefs(hasCA, hasCert, certCN)
			if err == nil {
				t.Fatal("ValidatePKIRefs accepted an EAP peer with no certificate")
			}
			msg := err.Error()
			for _, want := range []string{"branch", "certificate", "RFC 7296 Section 2.16"} {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q does not mention %q", msg, want)
				}
			}

			// The same peer with a certificate passes, so the rule is about the
			// missing credential rather than about the mode.
			ok := eapAuthPeerConfig("branch", eapAuthCfg(mode, autCertName))
			if err := ok.ValidatePKIRefs(hasCA, hasCert, certCN); err != nil {
				t.Fatalf("ValidatePKIRefs rejected a complete EAP peer: %v", err)
			}
		})
	}
}

// VALIDATES: the RFC 7296 Section 2.16 obligation is scoped to EAP. A
// pre-shared-secret peer, which runs no EAP exchange, still authenticates.
// PREVENTS: a fail-closed guard written too wide. Such a guard would break every
// PSK site-to-site peer as a side effect of the EAP fix.
// RFC requirement: RFC7296-2.16-11 positive -- a pre-shared-secret peer completes the handshake
// end to end with no certificate anywhere. computeLocalAuth (auth.go:217) routes the
// non-EAP responder AUTH to computePSKAuth, and never through computeServerAuth.
func TestEapAuthNonEAPPreSharedKeyStillAuthenticates(t *testing.T) {
	ini, resp, _ := establishPSK(t)
	if ini.State != StateEstablished || resp.State != StateEstablished {
		t.Fatalf("PSK handshake did not establish: ini=%v resp=%v", ini.State, resp.State)
	}
	if resp.PeerCfg.Auth.Certificate != "" {
		t.Fatalf("fixture drift: the PSK peer names a certificate %q", resp.PeerCfg.Auth.Certificate)
	}

	auth, err := computeLocalAuth(resp)
	if err != nil {
		t.Fatalf("computeLocalAuth on a pre-shared-secret responder: %v", err)
	}
	if auth.AuthMethod != wire.AuthMethodPSK {
		t.Fatalf("AuthMethod = %d, want %d (pre-shared key)", auth.AuthMethod, wire.AuthMethodPSK)
	}
}

// VALIDATES: RFC7296-2.16-11 on the receive side. When ze is the EAP initiator, a
// responder that authenticates with a pre-shared key is refused. Only a
// public-key signature satisfies the obligation.
// PREVENTS: ze completing an EAP exchange against a responder that proved nothing
// more than knowledge of the user password. That is the substitution RFC 7296
// Section 2.16 exists to stop.
// RFC requirement: RFC7296-2.16-11 negative -- verifyRemoteAuth (auth.go:306) refuses a PSK AUTH on
// an EAP SA whose MSK is not yet derived. That state is the responder AUTH of the first
// EAP message, and it is the only AUTH the initiator checks before EAP completes.
func TestEapAuthInitiatorRefusesPreSharedKeyResponder(t *testing.T) {
	autLoadPKI(t)
	ini, _, _ := autSAInitPair(t, eapAuthCfg(ipsec.AuthEAPMSCHAPv2, autCertName))

	// The initiator has derived no MSK yet, which is the state of the first EAP
	// message.
	if ini.EAPMSK != ([64]byte{}) {
		t.Fatal("fixture drift: the initiator already holds an EAP MSK")
	}

	// Build the AUTH a non-conformant responder would send, over the REMOTE
	// role's signed octets, so verifyPSKAuth would accept it. Anything the MAC
	// comparison rejects anyway would make this test pass for the wrong reason.
	pskAuth := eapAuthForgedPSKAuth(t, ini)
	if pskAuth.AuthMethod != wire.AuthMethodPSK {
		t.Fatalf("control AUTH method = %d, want %d", pskAuth.AuthMethod, wire.AuthMethodPSK)
	}
	// Prove the PSK verifier permits this payload. Without the check, removing
	// the guard under test would leave the AUTH rejected anyway.
	remoteOctets, err := computeSignedOctets(ini, !ini.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}
	if err := verifyPSKAuth(ini, pskAuth.AuthData, remoteOctets); err != nil {
		t.Fatalf("the forged AUTH is not acceptable to verifyPSKAuth: %v", err)
	}

	if err := verifyRemoteAuth(ini, pskAuth); err == nil {
		t.Fatal("the initiator accepted a pre-shared-key AUTH from an EAP responder")
	} else if !strings.Contains(err.Error(), "RFC 7296 Section 2.16") {
		t.Errorf("error %q does not cite the requirement", err)
	}

	// Isolate the guard. The auth mode is the only variable it reads. The same SA
	// and the same AUTH payload under AuthPreSharedSecret must therefore not
	// produce that refusal. It reaches verifyPSKAuth instead and fails
	// the MAC comparison there, because a self-computed AUTH carries the local
	// role's signed octets rather than the remote's. That different error is the
	// proof the non-EAP path is untouched. The real non-EAP handshake is covered
	// by TestEapAuthNonEAPPreSharedKeyStillAuthenticates.
	ini.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	pskModeErr := verifyRemoteAuth(ini, pskAuth)
	if pskModeErr != nil && strings.Contains(pskModeErr.Error(), "RFC 7296 Section 2.16") {
		t.Fatalf("a pre-shared-secret peer hit the EAP-only refusal: %v", pskModeErr)
	}
	if pskModeErr != nil && !strings.Contains(pskModeErr.Error(), "authentication failed") {
		t.Fatalf("the non-EAP path gave %v, want the verifyPSKAuth MAC comparison", pskModeErr)
	}
}

// VALIDATES: ValidatePKIRefs leaves a pre-shared-secret peer alone. The new EAP
// rule must not reach a mode that never runs EAP.
// PREVENTS: a config-time rejection of every PSK peer, which is the widest possible
// form of this fix going wrong.
// RFC requirement: RFC7296-2.16-11 positive -- ValidatePKIRefs skips AuthPreSharedSecret entirely,
// so a PSK peer with no certificate and no CA passes validation unchanged. The conforming
// configuration is accepted, which is what a positive tag names.
func TestEapAuthConfigAcceptsPreSharedKeyPeer(t *testing.T) {
	hasCA, hasCert, certCN := eapAuthAllPresent()
	cfg := eapAuthPeerConfig("branch", ipsec.AuthConfig{
		Mode: ipsec.AuthPreSharedSecret,
		PSK:  "site-to-site-secret",
	})
	if err := cfg.ValidatePKIRefs(hasCA, hasCert, certCN); err != nil {
		t.Fatalf("ValidatePKIRefs rejected a pre-shared-secret peer: %v", err)
	}
}
