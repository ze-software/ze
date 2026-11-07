// Design: docs/architecture/testing/interop.md -- the IPsec interop suite
// Related: checkers.go -- scenarioCheckers, which names this checker
//
// A gateway that reauthenticates a tunnel Ze started (spec-home-cpe-22).
package ipsec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// reauthWait bounds the wait for strongSwan's reauthentication: the scenario sets
// reauth_time = 60s with rand_time = 0s, so it starts a minute after establishment.
const reauthWait = 3 * time.Minute

const (
	// swanReauthLine is what strongSwan 6.1.0 logs when it starts a reauthentication
	// (src/libcharon/sa/ike_sa.c, reauth).
	swanReauthLine = "reauthenticating IKE_SA"
	// zeAdoptLine is what Ze logs when an `initiate` session adopts the gateway's
	// successor (adoptPeerStartedSuccessor, reauth_receive.go).
	zeAdoptLine = "adopting the IKE SA the gateway created to reauthenticate"
	// zeReconnectLine is what Ze logs when a peer session ends and reconnects
	// (PeerSession.run, reconcile.go): the outage the adoption exists to avoid.
	zeReconnectLine = "ike: peer session ended, reconnecting"
)

// checkResponderReauthsZeInitiator proves AC-1 to AC-3 of spec-home-cpe-22 against
// strongSwan: Ze initiates, strongSwan as the original responder reauthenticates by
// creating a new IKE SA toward Ze, Ze adopts it with its new Child SA, strongSwan
// deletes the old IKE SA, and Ze never reconnects. The ESP SPIs change and traffic
// crosses the tunnel built on the successor.
//
// RFC 7296 Section 2.8.3: "While creation of a new IKE SA can be initiated by either
// party (initiator or responder in the original IKE SA), the use of EAP and/
// or Configuration payloads means in practice that reauthentication has
// to be initiated by the same party as the original IKE SA."
func checkResponderReauthsZeInitiator(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	before, err := lab.espSPIs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if len(before) == 0 {
		return fmt.Errorf("empty ESP SPI snapshot before strongSwan's reauthentication")
	}
	if err := lab.waitLog(ctx, swanPeer, swanReauthLine, reauthWait); err != nil {
		return fmt.Errorf("strongSwan never started its reauthentication (assumption A-1): %w", err)
	}
	if err := lab.waitLog(ctx, zePeer, zeAdoptLine, lab.timeout); err != nil {
		return fmt.Errorf("ze did not adopt the IKE SA strongSwan created to reauthenticate: %w", err)
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: lab.timeout, Interval: 2 * time.Second, Description: "new ESP SAs after strongSwan's reauthentication",
	}, func(probe context.Context) (map[string]struct{}, error) { return lab.espSPIs(probe, swanPeer) }, func(after map[string]struct{}) bool {
		return newStrings(before, after)
	})
	if err != nil {
		return err
	}
	if err := lab.verifyTunnelTraffic(ctx, "traffic did not cross the tunnel Ze adopted from strongSwan's reauthentication"); err != nil {
		return err
	}
	zeLogs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	if strings.Contains(zeLogs, zeReconnectLine) {
		return interoplab.WithSearchedOutput(fmt.Errorf("ze reconnected around strongSwan's reauthentication instead of adopting it"), zeLogs)
	}
	return nil
}
