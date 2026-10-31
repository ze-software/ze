// Design: docs/architecture/testing/interop.md -- the legacy signature scenarios against strongSwan with RFC 7427 disabled.
// Related: checkers.go -- the scenario registry these checkers join.
// Related: multiauth.go -- requireLogLine and establishMultipleAuth, reused here.
// RFC: rfc/short/rfc8247.md -- Section 3.1, RSA Digital Signature (AUTH method 1) is MUST.
// RFC: rfc/short/rfc4754.md -- ECDSA-256, ECDSA-384 and ECDSA-521 (AUTH methods 9, 10 and 11).
package ipsec

import (
	"context"
)

// The identities of the legacy signature scenarios: client.pem names
// ze-test-client and server.pem names swanIP, in the shared lab PKI and in each
// scenario's own pki directory.
const (
	legacySigZeID   = "ze-test-client"
	legacySigSwanID = "172.28.0.3"
)

// checkLegacySignature answers the checker for one legacy signature scenario:
// strongSwan 6.1.0 with charon.signature_authentication = no, so neither side
// receives a SIGNATURE_HASH_ALGORITHMS notify and RFC 7427 Section 3 permits
// neither side the Digital Signature method. The checker holds for Ze as
// initiator and as responder: each side's AUTH is read from charon's log either
// way. method is charon's auth_method_names rendering of the legacy method both
// sides must use ("RSA signature", "ECDSA-256 signature", ...;
// src/libcharon/sa/authenticator.c).
//
// RFC 8247 Section 3.1: "RSA Digital Signature is widely deployed and, therefore,
// kept for interoperability." The tunnel coming up says both AUTH payloads
// verified, not which method they carried, so the checker reads charon's own
// account of each: process() in pubkey_authenticator.c names the method it
// verified Ze's AUTH under, by auth_method_names for every method but 14, which it
// names by signature scheme instead:
//
//	DBG1(DBG_IKE, "authentication of '%Y' with %N successful", id,
//		 auth_method_names, auth_method);
//
// and build() names the method it signed its own AUTH with, which Ze then verified:
//
//	DBG1(DBG_IKE, "authentication of '%Y' (myself) with %N %s", id,
//		 auth_method_names, auth_method, ...);
func checkLegacySignature(method string) scenarioChecker {
	return func(ctx context.Context, lab *scenarioLab) error {
		if err := establishMultipleAuth(ctx, lab); err != nil {
			return err
		}
		swanLogs, err := lab.logs(ctx, swanPeer)
		if err != nil {
			return err
		}
		if err := requireLogLine(swanLogs, "strongSwan did not verify Ze's AUTH under the legacy method",
			"authentication of '"+legacySigZeID+"' with "+method+" successful"); err != nil {
			return err
		}
		return requireLogLine(swanLogs, "strongSwan did not sign its own AUTH with the legacy method",
			"authentication of '"+legacySigSwanID+"' (myself) with "+method+" successful")
	}
}
