// Design: docs/architecture/testing/interop.md -- the RFC 4739 multiple-authentication scenarios against strongSwan.
// Related: checkers.go -- the scenario registry these two checkers join.
// Related: helpers.go -- establish, waitXFRM, checkXFRMCount and verifyTunnelTraffic.
// RFC: rfc/short/rfc4739.md -- Section 2.1, a second authentication round in one IKE_AUTH phase.
package ipsec

import (
	"context"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The identities of the two RFC 4739 scenarios. Round 1 uses the lab's usual
// identities: ze-test-client is client.pem's subjectAltName, and strongSwan's
// server.pem names swanIP. Round 2 gives Ze, and in multiple-auth-ze-initiator
// strongSwan too, an identity no round 1 uses, so a charon or Ze log line that
// names it can only come from the second round.
const (
	multipleAuthZeCertificateID = "ze-test-client"
	multipleAuthZeRoundTwoID    = "ze-user.example.com"
	multipleAuthSwanRoundTwoID  = "swan-user.example.com"
)

// zeSecondRoundVerified is the message Ze's IKE engine logs at INFO when the
// peer's second authentication round (RFC 4739) verifies, with the attributes
// peer (the configured peer name) and id (the identity that round proved). It
// MUST match the log message in internal/component/ike/engine byte for byte:
// it is Ze's own account that it checked the peer's second AUTH, and a reworded
// message there turns both multiple-auth scenarios red here. Each scenario's
// ze-env sets ZE_LOG_IKE=info, because the ike subsystem logs at WARN by default.
const zeSecondRoundVerified = "ike: second authentication round verified"

// swanVerifiedZeRoundTwo is charon's line for the pre-shared key AUTH of Ze's second
// round, which both scenarios run. charon writes it from process() in
// src/libcharon/sa/ikev2/authenticators/psk_authenticator.c (strongSwan 6.1.0):
//
//	DBG1(DBG_IKE, "authentication of '%Y' with %N successful",
//		 other_id, auth_method_names, AUTH_PSK);
//
// auth_method_names renders AUTH_PSK as "pre-shared key"
// (src/libcharon/sa/authenticator.c). other_id is the peer identity of the
// round charon is verifying, so the identity says which round the line is for.
const swanVerifiedZeRoundTwo = "authentication of '" + multipleAuthZeRoundTwoID + "' with pre-shared key successful"

// swanBuiltRoundTwo is charon's line for the pre-shared key AUTH it builds for its
// own second round, which both scenarios run, written by build() in the same
// psk_authenticator.c:
//
//	DBG1(DBG_IKE, "authentication of '%Y' (myself) with %N",
//		 my_id, auth_method_names, AUTH_PSK);
const swanBuiltRoundTwo = "authentication of '" + multipleAuthSwanRoundTwoID + "' (myself) with pre-shared key"

// swanVerifiedZeCertificatePrefix opens charon's line for the signature AUTH of
// Ze's first, certificate, round, which both scenarios run, written by process() in
// src/libcharon/sa/ikev2/authenticators/pubkey_authenticator.c:
//
//	DBG1(DBG_IKE, "authentication of '%Y' with %N successful", id,
//		 signature_scheme_names, params->scheme);
//
// The scheme name depends on the key and on what the peers negotiated, so the
// checker matches the identity before it and the "successful" after it, on one
// line.
const swanVerifiedZeCertificatePrefix = "authentication of '" + multipleAuthZeCertificateID + "' with "

// requireLogLine fails unless one line of logs carries every part, and the
// failure names the parts and carries the text it searched. One line, because a
// Ze line's attributes belong to its message only when they share the line.
func requireLogLine(logs, message string, parts ...string) error {
	for line := range strings.SplitSeq(logs, "\n") {
		if lineHasAll(line, parts) {
			return nil
		}
	}
	return fmt.Errorf("%s: no line carries all of %q; searched output:\n%s", message, parts, interoplab.OutputExcerpt(logs))
}

// lineHasAll reports whether line contains every part.
func lineHasAll(line string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(line, part) {
			return false
		}
	}
	return true
}

// requireZeSecondRoundVerified reads Ze's log for its account that the peer's
// second round proved identity.
func requireZeSecondRoundVerified(ctx context.Context, lab *scenarioLab, identity string) error {
	zeLogs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	return requireLogLine(zeLogs, "ze did not log the peer's second authentication round",
		zeSecondRoundVerified, " peer="+swanConfigPeer, " id="+identity)
}

// establishMultipleAuth is the common half of the two RFC 4739 checkers: the
// IKE and Child SAs establish, both kernels hold the Child SA's ESP states, and
// ESP carries traffic both ways. It proves the exchange succeeded, not which
// rounds it held: the log assertions in each checker carry that.
func establishMultipleAuth(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	if err := lab.checkXFRMCount(ctx, swanPeer, 2); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "traffic did not flow through the multiple-authentication tunnel")
}

// checkMultipleAuthZeInitiator proves RFC 4739 multiple authentication with Ze
// as the initiator against a strongSwan 6.1.0 responder: round 1 is X.509 both
// ways, round 2 a pre-shared key both ways under identities of its own.
//
// RFC 4739 Section 2.1: "The next IKE_AUTH message sent by this peer will
// contain a second identity payload (IDi or IDr) and starts another
// authentication exchange." A tunnel coming up says the exchange succeeded, not
// which rounds it held, so the checker reads each side's own account of the
// round it ran: charon verified ze-user.example.com and built its AUTH as
// swan-user.example.com, and Ze verified swan-user.example.com. The round-1 line
// for ze-test-client is read too, so a round 2 that replaced round 1 fails.
func checkMultipleAuthZeInitiator(ctx context.Context, lab *scenarioLab) error {
	if err := establishMultipleAuth(ctx, lab); err != nil {
		return err
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if err := requireLogLine(swanLogs, "strongSwan did not verify Ze's first, certificate, round",
		swanVerifiedZeCertificatePrefix, " successful"); err != nil {
		return err
	}
	if err := requireContains(swanLogs, swanVerifiedZeRoundTwo,
		"strongSwan did not verify Ze's second authentication round"); err != nil {
		return err
	}
	if err := requireContains(swanLogs, swanBuiltRoundTwo,
		"strongSwan did not run its own second authentication round"); err != nil {
		return err
	}
	return requireZeSecondRoundVerified(ctx, lab, multipleAuthSwanRoundTwoID)
}

// checkMultipleAuthZeResponder proves RFC 4739 multiple authentication with
// strongSwan 6.1.0 as the initiator and Ze as the responder: round 1 is X.509
// both ways, round 2 a pre-shared key both ways under identities of their own.
//
// RFC 4739 Section 2.1: "The peers announce support for this IKEv2 extension by
// including a MULTIPLE_AUTH_SUPPORTED notification in the IKE_SA_INIT response
// (responder) and the first IKE_AUTH request (initiator)." charon starts its
// local-2 round only when Ze announced support (ike_auth.c do_another_auth
// returns FALSE without EXT_MULTIPLE_AUTH), so the checker reads charon
// verifying both of Ze's rounds (ze-test-client by signature, then
// ze-user.example.com by pre-shared key), charon building its own round-2 AUTH as
// swan-user.example.com, and Ze verifying that round.
func checkMultipleAuthZeResponder(ctx context.Context, lab *scenarioLab) error {
	if err := establishMultipleAuth(ctx, lab); err != nil {
		return err
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if err := requireLogLine(swanLogs, "strongSwan did not verify Ze's first, certificate, round",
		swanVerifiedZeCertificatePrefix, " successful"); err != nil {
		return err
	}
	if err := requireContains(swanLogs, swanVerifiedZeRoundTwo,
		"strongSwan did not verify Ze's second authentication round"); err != nil {
		return err
	}
	if err := requireContains(swanLogs, swanBuiltRoundTwo,
		"strongSwan did not run its own second authentication round"); err != nil {
		return err
	}
	return requireZeSecondRoundVerified(ctx, lab, multipleAuthSwanRoundTwoID)
}
