// Design: docs/labs/pppoe-interop.md -- pppoe-padr-replay: a replayed PADR gets
// the existing session id, and a MAC at the per-MAC cap is refused with a
// wire-level PADS error, both proved against a real pppd client.
// RFC: rfc/short/rfc2516.md -- Section 5.4: a retransmitted PADR gets the
// existing session id back, and a refusal is a PADS carrying an error tag with
// SESSION_ID 0x0000, never a dropped frame.
// Related: check_ac.go -- the general Ze access-concentrator checker this one
// shares its dial, session-wait and LCP/auth/IPCP helpers with.
// Related: check_service_name.go -- the wire-capture shape this one follows.
package pppoe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	discovery "github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// replayRoundTripBound is how long the checker waits, after handing a captured
// PADR back to the AC, for the PADS it provokes to appear in the fresh
// capture. The exchange is a single, synchronous, loopback-network reply with
// no negotiation on either side, so this is generous rather than tight.
const replayRoundTripBound = 5 * time.Second

// replayInterface is the interface inside the client container's own network
// namespace that the replay is sent on -- the same one pppd dials through.
const replayInterface = "eth0"

// checkZeAccessConcentratorPADRReplay proves two things a unit test cannot,
// both against a real pppd client on the wire:
//
//  1. A PADR captured from the client's own first (and only) dial, replayed
//     verbatim after that session has left discovery and is live in PPP, gets
//     back a PADS carrying the SAME session id -- not a second session.
//  2. A second, genuinely distinct dial from the same MAC, once the AC's
//     per-MAC cap is already at its configured value of one, is refused with
//     a PADS carrying session id 0x0000 and an AC-System-Error tag -- a real
//     frame a pppd client parses, never a dropped one.
//
// A session count that stays at one is not enough evidence for either claim
// on its own (docs/architecture/testing/interop.md, "Prove a scenario
// discriminates": an absence needs positive proof the query mechanism ran).
// Both claims are read off frames captured on the wire.
func checkZeAccessConcentratorPADRReplay(
	ctx context.Context,
	check *interoplab.CheckContext,
) error {
	return checkZeAccessConcentratorPADRReplayWith(ctx, check, sendCapturedFrame)
}

// checkZeAccessConcentratorPADRReplayWith is checkZeAccessConcentratorPADRReplay
// with the frame-replay mechanics injected, so a unit test can substitute a
// fake for the real AF_PACKET/network-namespace send.
func checkZeAccessConcentratorPADRReplayWith(
	ctx context.Context,
	check *interoplab.CheckContext,
	send frameSender,
) (err error) {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE PADR-replay checker has no lab")
	}
	defer func() {
		err = appendPPPDLog(ctx, check.Lab, err)
		err = appendDiagnostics(ctx, check.Lab, err, zeImageName, clientImageName)
	}()

	if err := waitLogsContain(
		ctx,
		check.Lab,
		zeImageName,
		"PPPoE interface configured",
		60*time.Second,
	); err != nil {
		return fmt.Errorf("ze PPPoE AC did not bind its access interface: %w", err)
	}
	if err := waitZeRESTReady(ctx, check.Lab, 60*time.Second); err != nil {
		return err
	}

	sid, originalPADR, err := dialFirstSessionAndCapturePADR(ctx, check.Lab)
	if err != nil {
		return err
	}

	if err := checkReplayReturnsExistingSID(ctx, check.Lab, sid, originalPADR, send); err != nil {
		return err
	}

	return checkSecondDialAtCapIsRefused(ctx, check.Lab)
}

// frameSender replays a previously captured frame as the peer at pid, on its
// own interfaceName. The type exists so a unit test can substitute a fake for
// the real AF_PACKET/network-namespace mechanics, the same shape
// isis_inject.go's isisPurgeSender already uses for the BGP lab's own
// namespace injector.
type frameSender func(pid int, interfaceName string, frame []byte) error

// sendCapturedFrame is the production frameSender: it needs nothing from
// inside the peer's namespace, because frame already carries every header a
// real send there produced.
func sendCapturedFrame(pid int, interfaceName string, frame []byte) error {
	return interoplab.SendFrameInNamespace(pid, interfaceName, func(*net.Interface) ([]byte, error) {
		return frame, nil
	})
}

// dialFirstSessionAndCapturePADR dials the one session the whole scenario
// admits, waits until it is live in PPP (past StateDiscovery, so the later
// replay exercises the state-independent dedup branch rather than the
// original discovery-stage match), and returns its session id together with
// the raw bytes of the PADR the client sent for it.
func dialFirstSessionAndCapturePADR(
	ctx context.Context,
	lab interoplab.CheckerLab,
) (uint16, []byte, error) {
	if err := startDiscoveryCapture(ctx, lab); err != nil {
		return 0, nil, err
	}
	if err := pppdDial(ctx, lab, pppdUsername, pppdPassword, pppoeService); err != nil {
		return 0, nil, err
	}
	sessions, err := waitZeSession(ctx, lab, 45*time.Second)
	if err != nil {
		return 0, nil, err
	}
	if len(sessions) != 1 {
		return 0, nil, fmt.Errorf("ze allocated %d PPPoE sessions before the replay, expected exactly one", len(sessions))
	}
	sid := sessions[0].SID

	// checkLCPAuthIPCP waits out LCP, CHAP and IPCP, so by the time it
	// returns the session is live in PPP and no longer in StateDiscovery.
	if _, err := checkLCPAuthIPCP(ctx, lab); err != nil {
		return 0, nil, err
	}

	capture, err := stopDiscoveryCapture(ctx, lab)
	if err != nil {
		return 0, nil, err
	}
	observed, err := observeDiscoveryFrames(capture)
	if err != nil {
		return 0, nil, err
	}
	if observed.padrCount != 1 {
		return 0, nil, fmt.Errorf("original dial's capture carries %d PADR frames, expected exactly 1", observed.padrCount)
	}
	if observed.padsCount != 1 {
		return 0, nil, fmt.Errorf("original dial's capture carries %d PADS frames, expected exactly 1", observed.padsCount)
	}
	if observed.padsSID != uint16(sid) { //nolint:gosec // REST reports the same 16-bit session id the wire carries
		return 0, nil, fmt.Errorf(
			"original PADS on the wire carries session id %#04x, REST reported %d",
			observed.padsSID, sid,
		)
	}
	if len(observed.firstPADR) == 0 {
		return 0, nil, errors.New("original dial's capture carries no PADR bytes to replay")
	}
	return observed.padsSID, observed.firstPADR, nil
}

// checkReplayReturnsExistingSID replays the exact bytes of originalPADR from
// inside the client container's own network namespace -- so the AC sees the
// same source MAC and the same AC-Cookie tag the original dial produced --
// once the session it opened is already live in PPP, and requires the PADS it
// provokes to name that session's existing id rather than allocate a new one.
func checkReplayReturnsExistingSID(
	ctx context.Context,
	lab interoplab.CheckerLab,
	sid uint16,
	originalPADR []byte,
	send frameSender,
) error {
	clientPID, err := lab.PeerPID(ctx, clientImageName)
	if err != nil {
		return fmt.Errorf("resolve %s network namespace: %w", clientImageName, err)
	}

	if err := startDiscoveryCapture(ctx, lab); err != nil {
		return err
	}
	if err := send(clientPID, replayInterface, originalPADR); err != nil {
		return fmt.Errorf("replay the original PADR: %w", err)
	}
	if err := waitFixed(ctx, replayRoundTripBound); err != nil {
		return err
	}
	capture, err := stopDiscoveryCapture(ctx, lab)
	if err != nil {
		return err
	}
	observed, err := observeDiscoveryFrames(capture)
	if err != nil {
		return err
	}
	if observed.padrCount != 1 {
		return fmt.Errorf("replay capture carries %d PADR frames, expected exactly the one replayed", observed.padrCount)
	}
	if observed.padsCount != 1 {
		return fmt.Errorf("replayed PADR provoked %d PADS frames, expected exactly 1", observed.padsCount)
	}
	if observed.padsSID != sid {
		return fmt.Errorf(
			"replayed PADR got session id %#04x, want the existing session %#04x",
			observed.padsSID, sid,
		)
	}

	sessions, err := zeSessions(ctx, lab)
	if err != nil {
		return err
	}
	if len(sessions) != 1 || sessions[0].SID != int(sid) {
		return fmt.Errorf("replay changed ze's session table: %+v, want exactly session %#04x", sessions, sid)
	}
	links, err := pppLinks(ctx, lab, clientImageName)
	if err != nil {
		return err
	}
	if len(links) != 1 {
		return fmt.Errorf("replay left %d PPP interfaces on the client, want exactly 1", len(links))
	}
	return nil
}

// checkSecondDialAtCapIsRefused dials a second, genuinely independent session
// from the same client MAC -- its own PADI/PADO round trip, so its own
// AC-Cookie -- while the MAC already holds one session and the scenario's
// max-sessions-per-mac is 1, and requires the PADS it provokes to carry
// session id 0x0000 and an AC-System-Error tag rather than allocate a second
// session.
func checkSecondDialAtCapIsRefused(ctx context.Context, lab interoplab.CheckerLab) error {
	if err := startDiscoveryCapture(ctx, lab); err != nil {
		return err
	}
	if err := pppdDial(ctx, lab, pppdUsername, pppdPassword, pppoeService); err != nil {
		return err
	}
	if err := waitFixed(ctx, replayRoundTripBound); err != nil {
		return err
	}
	// Best-effort cleanup: this scenario proves what the AC put on the wire,
	// not how the refused client reacts to it, so a dial that has already
	// given up on its own leaves pkill nothing to signal.
	_, _ = exec(ctx, lab, clientImageName, []string{commandPkill, "-TERM", "-x", pppdExecutable})

	capture, err := stopDiscoveryCapture(ctx, lab)
	if err != nil {
		return err
	}
	observed, err := observeDiscoveryFrames(capture)
	if err != nil {
		return err
	}
	if observed.padrCount == 0 {
		return errors.New("no PADR observed for the over-cap dial")
	}
	if observed.padsCount != 1 {
		return fmt.Errorf("over-cap dial's capture carries %d PADS frames, expected exactly 1", observed.padsCount)
	}
	if observed.padsSID != 0 {
		return fmt.Errorf("over-cap PADR got session id %#04x, want 0x0000", observed.padsSID)
	}
	if !observed.padsHadACSystemError {
		return errors.New("over-cap PADS carries no AC-System-Error tag")
	}

	sessions, err := zeSessions(ctx, lab)
	if err != nil {
		return err
	}
	if len(sessions) != 1 {
		return fmt.Errorf("the per-MAC cap did not hold: ze now reports %d sessions, want 1", len(sessions))
	}
	return nil
}

// discoveryObservation is what one capture window's frames say about the PADR
// and PADS exchange it carried.
type discoveryObservation struct {
	padrCount            int
	firstPADR            []byte
	padsCount            int
	padsSID              uint16
	padsHadACSystemError bool
}

// observeDiscoveryFrames reads every frame in capture and requires each one to
// parse as PPPoE discovery: the filter already narrows the capture to that
// EtherType, so a frame that fails to parse is a corrupt capture rather than
// noise to skip past (matching checkDiscoveryServiceNameTags).
func observeDiscoveryFrames(capture []byte) (discoveryObservation, error) {
	reader, err := pcap.NewReader(bytes.NewReader(capture))
	if err != nil {
		return discoveryObservation{}, fmt.Errorf("discovery capture: %w", err)
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return discoveryObservation{}, fmt.Errorf("discovery capture link type %d, expected Ethernet", reader.LinkType())
	}

	var observed discoveryObservation
	var record pcap.Record
	for {
		if err := reader.Next(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return discoveryObservation{}, fmt.Errorf("discovery capture: %w", err)
		}
		packet, parseErr := discovery.ParseDiscovery(record.Data)
		if parseErr != nil {
			return discoveryObservation{}, fmt.Errorf("discovery capture: frame matching the PPPoE filter did not parse: %w", parseErr)
		}
		switch packet.Code {
		case discovery.CodePADR:
			observed.padrCount++
			if observed.firstPADR == nil {
				observed.firstPADR = append([]byte(nil), record.Data...)
			}
		case discovery.CodePADS:
			observed.padsCount++
			observed.padsSID = packet.SID
			observed.padsHadACSystemError = packet.FindTag(discovery.TagACSystemError) != nil
		}
	}
	return observed, nil
}

// waitFixed blocks for d or until ctx is done, whichever comes first. The two
// callers above are not polling Ze for a value that becomes true: they are
// giving a synchronous wire exchange time to happen before the capture that
// observes it is stopped, so there is nothing to poll.
func waitFixed(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
