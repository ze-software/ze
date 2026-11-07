// Design: docs/architecture/testing/interop.md -- the RFC 8420 EdDSA signature scenarios against strongSwan.
// Related: checkers.go -- the scenario registry these checkers join.
// Related: legacysig.go -- the lab identities and the charon log lines read the same way.
// Related: multiauth.go -- requireLogLine and establishMultipleAuth, reused here.
package ipsec

import (
	"context"
	"fmt"
)

// charonEd25519 is charon's signature_scheme_names rendering of SIGN_ED25519
// (src/libstrongswan/credentials/keys/public_key.c, strongSwan 6.1.0). charon
// names a method 14 AUTH by its signature scheme, not by auth_method_names.
const charonEd25519 = "ED25519"

// charonEd448 is the same table's rendering of SIGN_ED448. strongSwan 6.1.0's
// openssl plugin provides PRIVKEY_SIGN and PUBKEY_VERIFY for SIGN_ED448 and the
// HASH_IDENTITY hasher (openssl_plugin.c, under OPENSSL_VERSION_NUMBER >=
// 0x1010100fL), which the lab image builds with --enable-openssl.
const charonEd448 = "ED448"

// charonAuthFailed is the line charon's process_i in
// src/libcharon/sa/ikev2/tasks/ike_auth.c writes for an error notify in an
// IKE_AUTH response:
//
//	DBG1(DBG_IKE, "received %N notify error", notify_type_names, type);
const charonAuthFailed = "received AUTHENTICATION_FAILED notify error"

// zeAuthVerifyFailed is the message Ze's responder logs at WARN when an
// initiator's AUTH, its certificate chain included, does not verify
// (handleAuthRequest in internal/component/ike/engine/responder.go), with the
// attribute peer. It MUST match that message byte for byte.
const zeAuthVerifyFailed = "ike: initiator AUTH verification failed"

// checkEd25519ZeSigns proves Ze signs its IKE_AUTH with an Ed25519 key under
// RFC 7427 Digital Signature (method 14) and strongSwan 6.1.0 verifies it.
//
// RFC 8420 Section 2: "Ed25519 and Ed448 are only defined with the "Identity"
// hash and MUST NOT be sent to a receiver that has not indicated support for
// the "Identity" hash." The tunnel coming up says both AUTH payloads verified,
// not which signature carried Ze's, so the checker reads charon's own account:
// process() in pubkey_authenticator.c, for an AUTH_DS payload whose
// AlgorithmIdentifier parse_signature_auth_data resolved, logs
//
//	DBG1(DBG_IKE, "authentication of '%Y' with %N successful", id,
//		 signature_scheme_names, params->scheme);
//
// only after public->verify accepted the signature, so the line naming ED25519
// for ze-test-client is strongSwan's proof that it checked Ze's Ed25519
// signature.
func checkEd25519ZeSigns(ctx context.Context, lab *scenarioLab) error {
	if err := establishMultipleAuth(ctx, lab); err != nil {
		return err
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	return requireLogLine(swanLogs, "strongSwan did not verify an Ed25519 AUTH from Ze",
		"authentication of '"+legacySigZeID+"' with "+charonEd25519+" successful")
}

// checkEd25519ZeVerifies proves Ze, as the responder, verifies an Ed25519
// IKE_AUTH that strongSwan 6.1.0 signed under method 14.
//
// RFC 8420 Section 3: "For this reason, implementations MUST have the
// "Identity" value in the SIGNATURE_HASH_ALGORITHMS notification when EdDSA is
// supported and configured." charon's build() in pubkey_authenticator.c can sign with its
// Ed25519 key only when Ze's IKE_SA_INIT response announced Identity, and it
// logs the scheme it signed with:
//
//	DBG1(DBG_IKE, "authentication of '%Y' (myself) with %N %s", id,
//		 signature_scheme_names, params->scheme, ...);
//
// That line says charon sent an Ed25519 AUTH. Ze installs the Child SA only
// after its own verification of that AUTH, so the established tunnel with
// traffic both ways is the proof that Ze verified it.
func checkEd25519ZeVerifies(ctx context.Context, lab *scenarioLab) error {
	if err := establishMultipleAuth(ctx, lab); err != nil {
		return err
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	return requireLogLine(swanLogs, "strongSwan did not sign its AUTH with its Ed25519 key",
		"authentication of '"+legacySigSwanID+"' (myself) with "+charonEd25519+" successful")
}

// checkEd448ZeSigns proves Ze signs its IKE_AUTH with an Ed448 key under RFC 7427
// Digital Signature (method 14) and strongSwan 6.1.0 verifies it: the Ed448 twin
// of checkEd25519ZeSigns, read from the same pubkey_authenticator.c line.
func checkEd448ZeSigns(ctx context.Context, lab *scenarioLab) error {
	if err := establishMultipleAuth(ctx, lab); err != nil {
		return err
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	return requireLogLine(swanLogs, "strongSwan did not verify an Ed448 AUTH from Ze",
		"authentication of '"+legacySigZeID+"' with "+charonEd448+" successful")
}

// checkEd448ZeVerifies proves Ze, as the responder, verifies an Ed448 leaf that
// the lab P-256 CA issued and the Ed448 IKE_AUTH strongSwan signed with it, and
// refuses the same identity's Ed448 leaf from a CA Ze does not trust.
//
// The trusted half reads like checkEd25519ZeVerifies: charon logs that it signed
// with ED448, and Ze installs the Child SA only after its own verification of
// that AUTH, so the tunnel carrying traffic both ways is the proof Ze verified
// it. The untrusted half initiates the `untrusted` connection, whose leaf
// untrusted-ca.example.net issued: Ze's chain check fails in getRemoteCert, Ze
// logs the refusal naming the peer and answers AUTHENTICATION_FAILED, which
// charon logs, and Ze holds no more ESP state than before.
func checkEd448ZeVerifies(ctx context.Context, lab *scenarioLab) error {
	if err := establishMultipleAuth(ctx, lab); err != nil {
		return err
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if err := requireLogLine(swanLogs, "strongSwan did not sign its AUTH with its Ed448 key",
		"authentication of '"+legacySigSwanID+"' (myself) with "+charonEd448+" successful"); err != nil {
		return err
	}

	before, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	// swanctl exits non-zero when the IKE_AUTH fails, which is the expected
	// outcome; the log lines below are the verdict.
	lab.execQuiet(ctx, swanPeer, cmdSwanctl, "--initiate", "--child", "untrusted-child")
	if err := lab.waitLog(ctx, swanPeer, charonAuthFailed, lab.timeout); err != nil {
		return fmt.Errorf("ze did not answer AUTHENTICATION_FAILED to an Ed448 leaf from an untrusted CA: %w", err)
	}
	zeLogs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := requireLogLine(zeLogs, "ze did not log the untrusted chain it refused",
		zeAuthVerifyFailed, "peer="+swanConfigPeer, "certificate signed by unknown authority"); err != nil {
		return err
	}
	after, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	if len(after) > len(before) {
		return fmt.Errorf("ze installed ESP state for the untrusted connection: %d SPIs before, %d after",
			len(before), len(after))
	}
	return nil
}
