// Design: docs/architecture/testing/interop.md -- the peer certificate revocation and purpose scenarios against strongSwan.
// Related: checkers.go -- the scenario registry these checkers join.
// Related: eddsa.go -- the other scenarios where Ze refuses a strongSwan certificate.
// Related: multiauth.go -- establishMultipleAuth and requireLogLine, reused here.
package ipsec

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// zeRemoteAuthVerifyFailed is the message Ze's initiator logs at WARN when the
// responder's AUTH, its certificate checks included, does not verify
// (handleAuthResponse in internal/component/ike/engine/fsm.go), with the
// attributes peer and error. It MUST match that message byte for byte.
const zeRemoteAuthVerifyFailed = "ike: remote AUTH verification failed"

// zeRevokedCause is the fragment Ze's revocation refusal carries, from the
// revocation check internal/core/eap shares between EAP-TLS and IKE
// (crlSet.checkChain): "<certificate> (serial <n>) was revoked by <CRL issuer>".
const zeRevokedCause = "was revoked by"

// zeWrongPurposeCause is the fragment Ze's purpose refusal carries, matched
// without regard to case. The refusal names the extension it found and the
// fix (getRemoteCert in internal/component/ike/engine).
const zeWrongPurposeCause = "extended key usage"

// The two identities of the scenarios' AUTH exchange, as charon's
// pubkey_authenticator.c writes them: process logs "authentication of '%Y'
// with %N successful" once it verified the peer's AUTH, and build logs
// "authentication of '%Y' (myself) with %N successful" once it signed its own.
const (
	charonVerifiedZe = "authentication of '" + legacySigZeID + "' with "
	charonSignedSelf = "authentication of '" + legacySigSwanID + "' (myself) with "
)

// checkPeerCertificateAccepted proves Ze, as the initiator, accepts strongSwan's
// certificate under the scenario's revocation list or extended key usage, and
// that strongSwan signed its AUTH with that certificate.
//
// It is the control of a refusing twin whose inputs differ by one file. The
// strongSwan SA alone proves nothing here: charon's responder holds an
// ESTABLISHED IKE SA as soon as it sent its IKE_AUTH response, before Ze checked
// it. Ze's own ESP state and traffic both ways are what only an accepting Ze
// produces.
func checkPeerCertificateAccepted(ctx context.Context, lab *scenarioLab) error {
	if err := establishMultipleAuth(ctx, lab); err != nil {
		return err
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	return requireLogLine(swanLogs, "strongSwan did not sign its AUTH with the certificate under test",
		charonSignedSelf, "successful")
}

// checkPeerCertificateRefused answers a checker that proves Ze, as the
// initiator, refuses strongSwan's certificate for the reason cause names, and
// installs no SA.
//
// Ze sends no notification for a responder AUTH it refuses on the first round
// (RFC 7296 Section 2.21.2 makes the separate INFORMATIONAL a MAY, and
// handleAuthResponse ends the SA), so charon's responder keeps the IKE SA it
// established when it answered. The checker therefore asserts nothing about
// strongSwan's SA list or kernel. The assertions are:
//
//  1. Ze logged the refusal: the positive fact, which a Ze without the check
//     never writes, so a scenario run against it fails here.
//  2. The refusal names the peer and the cause, so an earlier failure (an
//     unknown authority, a remote-id mismatch) cannot pass for it.
//  3. charon verified Ze's AUTH and signed its own with the certificate under
//     test, so the exchange reached Ze's check of that certificate.
//  4. Ze holds no ESP state.
func checkPeerCertificateRefused(cause string) scenarioChecker {
	return func(ctx context.Context, lab *scenarioLab) error {
		if err := lab.waitLog(ctx, zePeer, zeRemoteAuthVerifyFailed, lab.timeout); err != nil {
			return fmt.Errorf("ze did not refuse strongSwan's certificate: %w", err)
		}
		zeLogs, err := lab.logs(ctx, zePeer)
		if err != nil {
			return err
		}
		if err := requireRefusalCause(zeLogs, cause); err != nil {
			return err
		}
		swanLogs, err := lab.logs(ctx, swanPeer)
		if err != nil {
			return err
		}
		if err := requireLogLine(swanLogs, "strongSwan never verified Ze's AUTH, so Ze refused before the certificate check",
			charonVerifiedZe, "successful"); err != nil {
			return err
		}
		if err := requireLogLine(swanLogs, "strongSwan never signed its AUTH with the certificate under test",
			charonSignedSelf, "successful"); err != nil {
			return err
		}
		return lab.checkXFRMCount(ctx, zePeer, 0)
	}
}

// requireRefusalCause finds Ze's refusal line for the lab peer and requires it
// to carry cause, compared without regard to case, so the line says why Ze
// refused and not only that it did.
func requireRefusalCause(zeLogs, cause string) error {
	want := strings.ToLower(cause)
	for line := range strings.SplitSeq(zeLogs, "\n") {
		if !lineHasAll(line, []string{zeRemoteAuthVerifyFailed, "peer=" + swanConfigPeer}) {
			continue
		}
		if strings.Contains(strings.ToLower(line), want) {
			return nil
		}
	}
	return interoplab.WithSearchedOutput(
		errors.New("ze refused strongSwan's AUTH, but no refusal line names the cause "+cause), zeLogs)
}
