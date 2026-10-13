// Design: docs/architecture/testing/interop.md -- live BFD profile edit against FRR.
// Related: check_rpki_reload.go -- applyZeReload, the SIGHUP reload this checker drives.
// Related: check_special.go -- checkBFDFailover, the same FRR BFD lab without a reload.
// RFC: rfc/short/rfc5880.md Section 6.8.3
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const bfdProfileEditScenario = "bgp-bfd-profile-edit-live-frr"

// bfdProfileEditBefore and bfdProfileEditAfter are what FRR reads from ze's
// packets under ze.conf and under ze-reload.conf: ze's Desired Min TX and
// Required Min RX in milliseconds, and ze's Detect Mult. Both configs name the
// same profile "fast", so these numbers are the only thing the reload changes.
var (
	bfdProfileEditBefore = bfdRemoteTimers{transmitMs: 300, receiveMs: 300, detectMultiplier: 3}
	bfdProfileEditAfter  = bfdRemoteTimers{transmitMs: 200, receiveMs: 200, detectMultiplier: 5}
)

// bfdRemoteTimers are the intervals and multiplier FRR reports for its remote
// system, ze, in `show bfd peer <ze> json`.
type bfdRemoteTimers struct {
	transmitMs       int
	receiveMs        int
	detectMultiplier int
}

// bfdPeerIdentity is what identifies one BFD session and one BGP session as
// FRR sees them. A BFD session ze tore down and rebuilt carries a new remote
// discriminator, a flap FRR recovered from advances its session-down counter,
// and a BGP reset advances connectionsEstablished.
type bfdPeerIdentity struct {
	localDiscriminator  uint64
	remoteDiscriminator uint64
	sessionDowns        uint64
	bgpGeneration       uint64
}

// RFC 5880 Section 6.8.3
//
// checkBFDProfileEditLive proves a BFD profile edit applied by reload reaches
// a foreign BFD peer while the session stays Up: FRR reads ze's new intervals
// and multiplier from ze's packets, its BFD session keeps both discriminators
// and never counts a Down, and its BGP session to ze is never re-established.
func checkBFDProfileEditLive(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("BFD profile-edit scenario has no selected IPv4 network")
	}
	neighbor := networkHostAddress(check.Network, 2)
	var text textbuf.Buffer
	session := []string{cmdVtysh, "-c", text.Str("show bgp neighbor ").Str(neighbor).String()}
	peer := []string{cmdVtysh, "-c", text.Reset().Str("show bfd peer ").Str(neighbor).Str(" json").String()}
	counters := []string{cmdVtysh, "-c", text.Reset().Str("show bfd peer ").Str(neighbor).Str(" counters json").String()}

	if err := waitContains(ctx, check.Lab, peerFRR, session, 90*time.Second, "BGP state = Established"); err != nil {
		return checkerFailure(ctx, check.Lab, bfdProfileEditScenario, 1, err)
	}
	// Status up comes first at the slow-start intervals (RFC 5880 Section
	// 6.8.3 keeps them at one second or more until Up), so the before-state
	// is read only once ze advertises the configured ones.
	if err := waitBFDRemoteTimers(ctx, check.Lab, peer, bfdProfileEditBefore); err != nil {
		return checkerFailure(ctx, check.Lab, bfdProfileEditScenario, 2, err)
	}
	before, err := readBFDPeerIdentity(ctx, check.Lab, neighbor, peer, counters)
	if err != nil {
		return checkerFailure(ctx, check.Lab, bfdProfileEditScenario, 3, err)
	}
	if err := applyZeReload(ctx, check.Lab, zeMountedReloadConfig); err != nil {
		return checkerFailure(ctx, check.Lab, bfdProfileEditScenario, 4, fmt.Errorf("reload with the edited profile: %w", err))
	}
	if err := waitBFDRemoteTimers(ctx, check.Lab, peer, bfdProfileEditAfter); err != nil {
		return checkerFailure(ctx, check.Lab, bfdProfileEditScenario, 5, fmt.Errorf("FRR never read the edited profile from ze: %w", err))
	}
	after, err := readBFDPeerIdentity(ctx, check.Lab, neighbor, peer, counters)
	if err != nil {
		return checkerFailure(ctx, check.Lab, bfdProfileEditScenario, 6, err)
	}
	if err := requireBFDProfileEditContinuity(before, after); err != nil {
		return checkerFailure(ctx, check.Lab, bfdProfileEditScenario, 7, err)
	}
	return nil
}

// waitBFDRemoteTimers waits until FRR reports the BFD session up with want as
// ze's intervals and multiplier, and names FRR's last answer when it never does.
func waitBFDRemoteTimers(ctx context.Context, lab interoplab.CheckerLab, peer []string, want bfdRemoteTimers) error {
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "FRR BFD remote timers",
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peerFRR, peer, nil)
	}, func(output string) bool {
		return bfdRemoteTimersMatch(output, want) == nil
	})
	return withLastOutput(err, last)
}

// readBFDPeerIdentity reads FRR's BFD session (status up required) and BGP
// session generation for neighbor.
func readBFDPeerIdentity(ctx context.Context, lab interoplab.CheckerLab, neighbor string, peer, counters []string) (bfdPeerIdentity, error) {
	peerOutput, err := lab.Query(ctx, peerFRR, peer, nil)
	if err != nil {
		return bfdPeerIdentity{}, fmt.Errorf("read FRR BFD peer: %w", err)
	}
	counterOutput, err := lab.Query(ctx, peerFRR, counters, nil)
	if err != nil {
		return bfdPeerIdentity{}, fmt.Errorf("read FRR BFD counters: %w", err)
	}
	identity, err := parseBFDPeerIdentity(peerOutput, counterOutput)
	if err != nil {
		return bfdPeerIdentity{}, err
	}
	generation, err := queryFRRSessionGeneration(ctx, lab, neighbor)
	if err != nil {
		return bfdPeerIdentity{}, fmt.Errorf("read FRR BGP session generation: %w", err)
	}
	identity.bgpGeneration = generation
	return identity, nil
}

// frrBFDPeer is the part of FRR's `show bfd peer <addr> json` the BFD
// scenarios read. Each field is a pointer so a field FRR did not print is told apart
// from a zero it did.
type frrBFDPeer struct {
	Status                 *string `json:"status"`
	ID                     *uint64 `json:"id"`
	RemoteID               *uint64 `json:"remote-id"`
	RemoteTransmitInterval *int    `json:"remote-transmit-interval"`
	RemoteReceiveInterval  *int    `json:"remote-receive-interval"`
	RemoteDetectMultiplier *int    `json:"remote-detect-multiplier"`
	RemoteDiagnostic       *string `json:"remote-diagnostic"`
}

// frrBFDCounters is the part of `show bfd peer <addr> counters json` read here.
type frrBFDCounters struct {
	SessionDown *uint64 `json:"session-down"`
}

// decodeFRRBFDPeer decodes FRR's BFD peer JSON and refuses a session that is
// not up.
func decodeFRRBFDPeer(output string) (frrBFDPeer, error) {
	var peer frrBFDPeer
	if err := json.Unmarshal([]byte(output), &peer); err != nil {
		return frrBFDPeer{}, fmt.Errorf("decode FRR BFD peer JSON: %w; output: %s", err, strings.TrimSpace(output))
	}
	if peer.Status == nil {
		return frrBFDPeer{}, fmt.Errorf("FRR BFD peer JSON has no status: %s", strings.TrimSpace(output))
	}
	if *peer.Status != "up" {
		return frrBFDPeer{}, fmt.Errorf("FRR BFD peer status is %q, want up", *peer.Status)
	}
	return peer, nil
}

// bfdRemoteTimersMatch reports nil when output is FRR's BFD peer, up, with
// want as ze's transmit and receive intervals and detect multiplier.
func bfdRemoteTimersMatch(output string, want bfdRemoteTimers) error {
	peer, err := decodeFRRBFDPeer(output)
	if err != nil {
		return err
	}
	for _, field := range []struct {
		name string
		got  *int
		want int
	}{
		{"remote-transmit-interval", peer.RemoteTransmitInterval, want.transmitMs},
		{"remote-receive-interval", peer.RemoteReceiveInterval, want.receiveMs},
		{"remote-detect-multiplier", peer.RemoteDetectMultiplier, want.detectMultiplier},
	} {
		if field.got == nil {
			return fmt.Errorf("FRR BFD peer JSON has no %s", field.name)
		}
		if *field.got != field.want {
			return fmt.Errorf("FRR BFD peer %s = %d, want %d", field.name, *field.got, field.want)
		}
	}
	return nil
}

// parseBFDPeerIdentity reads both discriminators of an up session and its
// session-down count; the BGP generation is filled by the caller.
func parseBFDPeerIdentity(peerOutput, counterOutput string) (bfdPeerIdentity, error) {
	peer, err := decodeFRRBFDPeer(peerOutput)
	if err != nil {
		return bfdPeerIdentity{}, err
	}
	if peer.ID == nil {
		return bfdPeerIdentity{}, errors.New("FRR BFD peer JSON has no id")
	}
	if peer.RemoteID == nil {
		return bfdPeerIdentity{}, errors.New("FRR BFD peer JSON has no remote-id")
	}
	var counters frrBFDCounters
	if err := json.Unmarshal([]byte(counterOutput), &counters); err != nil {
		return bfdPeerIdentity{}, fmt.Errorf("decode FRR BFD counters JSON: %w; output: %s", err, strings.TrimSpace(counterOutput))
	}
	if counters.SessionDown == nil {
		return bfdPeerIdentity{}, fmt.Errorf("FRR BFD counters JSON has no session-down: %s", strings.TrimSpace(counterOutput))
	}
	return bfdPeerIdentity{
		localDiscriminator:  *peer.ID,
		remoteDiscriminator: *peer.RemoteID,
		sessionDowns:        *counters.SessionDown,
	}, nil
}

// requireBFDProfileEditContinuity refuses any sign that the edit replaced or
// flapped a session: a new discriminator on either side, a Down FRR counted,
// or a re-established BGP session.
func requireBFDProfileEditContinuity(before, after bfdPeerIdentity) error {
	for _, field := range []struct {
		name          string
		before, after uint64
	}{
		{"FRR BFD id (FRR's discriminator)", before.localDiscriminator, after.localDiscriminator},
		{"FRR BFD remote-id (ze's discriminator)", before.remoteDiscriminator, after.remoteDiscriminator},
		{"FRR BFD session-down counter", before.sessionDowns, after.sessionDowns},
		{"FRR BGP connectionsEstablished", before.bgpGeneration, after.bgpGeneration},
	} {
		if field.before != field.after {
			var tb textbuf.Buffer
			return errors.New(tb.Str(field.name).Str(" moved from ").Uint(field.before).
				Str(" to ").Uint(field.after).
				Str(" across the profile-edit reload, so a session was replaced or went down").String())
		}
	}
	return nil
}
