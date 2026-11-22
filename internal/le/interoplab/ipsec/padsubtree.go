// Design: docs/architecture/testing/interop.md -- pad-subtree-identity, a PAD sub-tree entry against strongSwan's ID_FQDN
// Related: helpers.go -- the IKE SA reads, inner addresses, ping and ESP counters
// Related: fqdn.go -- lineCarriesAll, the one-line log match this reuses
// Related: checkers.go -- scenarioCheckers names the checker

package ipsec

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	padScenario = "pad-subtree-identity"
	// padSubtree is the remote-id ze.conf configures: every DNS name ending in the two
	// components example.com.
	padSubtree = ".example.com"
	// padInsideIdentity is the ID_FQDN swanctl.conf gives strongSwan, under padSubtree.
	padInsideIdentity = "vpn.example.com"
	// padOutsideIdentity is the ID_FQDN the checker gives strongSwan second. It is
	// outside padSubtree yet ends in the characters "example.com", so a match that
	// dropped the label boundary would admit it.
	padOutsideIdentity = "vpn.notexample.com"
	// padOutsideFile is the conf.d file that changes strongSwan's local id alone. swanctl
	// --load-conns reads every file there, and the later file's value wins.
	padOutsideFile = "/etc/swanctl/conf.d/zz-outside-identity.conf"
	// The scenario's inner addresses, inside 10.2.0.0/24 <-> 10.1.0.0/24.
	padZeInner   = "10.2.0.1"
	padSwanInner = "10.1.0.1"
	// zeRemoteAuthRefused is the WARN line handleAuthResponse
	// (internal/component/ike/engine/fsm.go) writes when verifyRemoteAuth refuses the
	// responder; checkRemoteIdentity's error, which the line carries, names the asserted
	// identity and the remote-id it was refused against.
	zeRemoteAuthRefused = "ike: remote AUTH verification failed"
	// zeRefusedIdentity is the phrase of checkRemoteIdentity's refusal that introduces
	// the asserted identity.
	zeRefusedIdentity = "asserted identity"
	// swanLoadedConnection is what swanctl --load-conns prints for the loaded
	// connection.
	swanLoadedConnection = "loaded connection 'ze'"
)

// checkPADSubtreeIdentity proves RFC 4301 Section 4.4.3.1 against strongSwan (AC-6 of
// spec-rfc4301-architecture-gaps): a PAD entry naming the DNS sub-tree ".example.com"
// admits strongSwan asserting the ID_FQDN vpn.example.com and refuses it asserting
// vpn.notexample.com, the only change between the two being strongSwan's local id.
//
// RFC 4301 Section 4.4.3.1: "But, at a minimum, sub-tree matching of the sort described
// above MUST be supported."
//
// Each assertion is numbered, so a red names the observation that failed:
//
//  1. Under the sub-tree: the IKE SA and the Child SA are up on strongSwan, Ze holds
//     its ESP state, and Ze lists the IKE SA established for peer swan.
//  2. charon authenticated as vpn.example.com (psk_authenticator.c build logs
//     "authentication of '<id>' (myself) with ..."), so the identity Ze admitted is
//     the one under the sub-tree.
//  3. Traffic crosses the tunnel between the inner addresses in all four directions.
//  4. strongSwan loads the second file, which changes its local id alone.
//  5. Ze clears its SAs and redials (the clear-reestablish scenario proves the redial).
//  6. charon authenticated as vpn.notexample.com after the clear.
//  7. Ze refused the responder's AUTH after the clear.
//  8. That refusal names the asserted vpn.notexample.com and the remote-id
//     ".example.com", so the identity decided it and not the shared key.
//  9. Ze lists no established IKE SA for peer swan.
//  10. Ze holds no ESP state.
//
// charon keeps its own side of the refused exchange established: as the responder it
// installs the Child SA once it sends its IKE_AUTH response, and Ze sends nothing when
// it refuses the response, so strongSwan's SA list is not read after the refusal.
//
// Red proof: ze.conf names the two breaks of textIdentityMatches. Answering true for a
// sub-tree whatever the name lets Ze authenticate vpn.notexample.com, so no refusal is
// logged and assertion 7 is red; answering false refuses vpn.example.com, and
// assertion 1 is red.
func checkPADSubtreeIdentity(ctx context.Context, lab *scenarioLab) error {
	fail := func(assertion int, cause error) error {
		var tb textbuf.Buffer
		tb.Str("scenario ").Str(padScenario).Str(" assertion ").Str(strconv.Itoa(assertion)).Str(": ").Err(cause)
		return errors.New(tb.String())
	}
	if err := establish(ctx, lab); err != nil {
		return fail(1, err)
	}
	// charon lists its side established as soon as it has answered, whatever Ze makes
	// of the answer, so Ze's own Child SA is what is waited for: Ze installs it only
	// after it verified the responder.
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return fail(1, err)
	}
	if err := lab.requireZeIKEEstablished(ctx); err != nil {
		return fail(1, err)
	}
	if err := lab.waitLog(ctx, swanPeer, swanAuthenticatesAs(padInsideIdentity), lab.timeout); err != nil {
		return fail(2, err)
	}
	if err := lab.natInnerAddress(ctx, zePeer, padZeInner, padSwanInner); err != nil {
		return fail(3, err)
	}
	if err := lab.natInnerAddress(ctx, swanPeer, padSwanInner, padZeInner); err != nil {
		return fail(3, err)
	}
	probe := pingProbe{peer: zePeer, target: padSwanInner, source: padZeInner}
	if err := lab.verifyESPDirectionsToward(ctx, "traffic did not cross the tunnel Ze authenticated under "+padSubtree, probe, espBothDirections); err != nil {
		return fail(3, err)
	}
	if err := lab.loadOutsideIdentity(ctx); err != nil {
		return fail(4, err)
	}
	// The waits after the clear count only lines logged after it: the first exchange
	// logged the same lines for the first identity.
	swanMark, err := lab.logMark(ctx, swanPeer, swanAuthenticatesAs(padOutsideIdentity))
	if err != nil {
		return fail(5, err)
	}
	zeMark, err := lab.logMark(ctx, zePeer, zeRemoteAuthRefused)
	if err != nil {
		return fail(5, err)
	}
	if _, err := lab.zeCLI(ctx, "clear vpn ipsec sa all"); err != nil {
		return fail(5, err)
	}
	if err := lab.waitLogAfter(ctx, swanPeer, swanAuthenticatesAs(padOutsideIdentity), swanMark, lab.timeout); err != nil {
		return fail(6, err)
	}
	if err := lab.waitLogAfter(ctx, zePeer, zeRemoteAuthRefused, zeMark, lab.timeout); err != nil {
		return fail(7, err)
	}
	logs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return fail(8, err)
	}
	if !lineCarriesAll(logs, zeRemoteAuthRefused, zeRefusedIdentity, padOutsideIdentity, padSubtree) {
		return fail(8, interoplab.WithSearchedOutput(fmt.Errorf(
			"ze logged no %q line naming the asserted %s and the remote-id %s", zeRemoteAuthRefused, padOutsideIdentity, padSubtree), logs))
	}
	if err := lab.requireZeIKENotEstablished(ctx, swanConfigPeer); err != nil {
		return fail(9, err)
	}
	if err := lab.checkXFRMCount(ctx, zePeer, 0); err != nil {
		return fail(10, err)
	}
	return nil
}

// swanAuthenticatesAs is the start of the line charon logs when it authenticates as id
// with the shared key: psk_authenticator.c build, strongSwan 6.1.0, "authentication of
// '%Y' (myself) with %N" over AUTH_PSK, logged before it looks the key up. The method
// name is left out, as charonSignedSelf (certpolicy.go) leaves it: the identity is what
// this scenario reads.
func swanAuthenticatesAs(id string) string {
	var tb textbuf.Buffer
	return tb.Str("authentication of '").Str(id).Str("' (myself) with ").String()
}

// loadOutsideIdentity writes the conf.d file that changes strongSwan's local id to
// padOutsideIdentity and loads it. vici_config.c merge_config replaces the connection
// ("replaced vici connection: ze"), because its peer config no longer equals the
// loaded one; the established IKE SA keeps the configuration it was built from until
// Ze's clear ends it.
func (l *scenarioLab) loadOutsideIdentity(ctx context.Context) error {
	var tb textbuf.Buffer
	command := tb.Str("cat > ").Str(padOutsideFile).Str(" <<'EOF'\n").
		Str("connections {\n    ze {\n        local {\n            id = ").Str(padOutsideIdentity).
		Str("\n        }\n    }\n}\nEOF").String()
	if _, err := l.exec(ctx, swanPeer, "sh", "-c", command); err != nil {
		return fmt.Errorf("write strongSwan's %s: %w", padOutsideFile, err)
	}
	loaded, err := l.query(ctx, swanPeer, cmdSwanctl, "--load-conns")
	if err != nil {
		return fmt.Errorf("swanctl --load-conns: %w", err)
	}
	if !strings.Contains(loaded, swanLoadedConnection) {
		return interoplab.WithSearchedOutput(fmt.Errorf("strongSwan did not load the connection with the local id %s", padOutsideIdentity), loaded)
	}
	return nil
}

// requireZeIKENotEstablished refuses when `show vpn ipsec sa` lists an established IKE
// SA for the configured peer.
func (l *scenarioLab) requireZeIKENotEstablished(ctx context.Context, peer string) error {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record["peer-name"] == peer && record["state"] == zeStateEstablished {
			return fmt.Errorf("ze lists an established IKE SA for %s after refusing the identity outside %s; answer: %s", peer, padSubtree, answer)
		}
	}
	return nil
}
