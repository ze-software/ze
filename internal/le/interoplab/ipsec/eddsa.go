// Design: docs/architecture/testing/interop.md -- the RFC 8420 EdDSA signature scenarios against strongSwan.
// Related: checkers.go -- the scenario registry these checkers join.
// Related: legacysig.go -- the lab identities and the charon log lines read the same way.
// Related: multiauth.go -- requireLogLine and establishMultipleAuth, reused here.
package ipsec

import (
	"context"
)

// charonEd25519 is charon's signature_scheme_names rendering of SIGN_ED25519
// (src/libstrongswan/credentials/keys/public_key.c, strongSwan 6.1.0). charon
// names a method 14 AUTH by its signature scheme, not by auth_method_names.
const charonEd25519 = "ED25519"

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
