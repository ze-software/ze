// Design: docs/labs/pppoe-interop.md -- pppoe-pap-ze-ac: Ze's access
// concentrator authenticates a real pppd client with PAP, and answers a
// repeated Authenticate-Request after authentication completed.
// RFC: rfc/short/rfc1334.md -- Section 2.2.1: "the authenticator MUST allow
// repeated Authenticate-Request packets after completing the Authentication
// phase", returning the same reply Code.
// Related: check_ac.go -- the general Ze access-concentrator checker this one
// shares its dial, session-wait, LCP/IPCP and teardown helpers with.
// Related: check_padr_replay.go -- waitFixed and the replay round-trip bound.
// Related: check_ipv6cp.go -- the session-stage capture helpers this one reuses.

//go:build ze_l2tp

package pppoe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// PPPoE session-stage frame layout, as tcpdump records it on eth0:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-------------------------------+-------------------------------+
//	| Ethernet destination, source (12 octets), EtherType 0x8864    |
//	+-------+-------+---------------+-------------------------------+
//	|  VER  | TYPE  |  CODE (0x00)  |          SESSION_ID           |  offset 14
//	+-------+-------+---------------+-------------------------------+
//	|            LENGTH             |     PPP Protocol (0xc023)     |  offset 18
//	+---------------+---------------+-------------------------------+
//	|   PAP Code    |  Identifier   |            Length             |  offset 22
//	+---------------+---------------+-------------------------------+
//
// RFC 2516 Section 4 places the PPP protocol field directly after the six
// PPPoE header octets; RFC 1334 Section 2.2 places Code and Identifier first
// in the PAP packet.
const (
	etherTypeOffset       = 12
	etherTypePPPoESession = 0x8864
	pppoeCodeOffset       = 15
	pppoeSessionIDOffset  = 16
	pppProtocolOffset     = 20
	papCodeOffset         = 22
	papIdentifierOffset   = 23
	papFrameOctetsMin     = 26
	pppProtocolPAP        = 0xc023
	papCodeAuthRequest    = 1
	papCodeAuthAck        = 2
	papCodeAuthNak        = 3
)

// papObservation is what one capture window's frames say about the PAP
// exchange it carried. request holds the bytes of the first
// Authenticate-Request, so the checker can replay that frame. requestSession
// and ackSession are the PPPoE SESSION_IDs the first request and the last Ack
// travelled on, so a checker can tie both to the session Ze reports.
type papObservation struct {
	requests       int
	acks           int
	naks           int
	request        []byte
	requestID      byte
	requestSession uint16
	ackID          byte
	ackSession     uint16
}

// checkZeAccessConcentratorPAP proves, against a real pppd client that refuses
// CHAP, that Ze's AC negotiates PAP, acknowledges the client's credential,
// completes IPCP and carries ICMP. It then replays the client's own captured
// Authenticate-Request after authentication completed, with its Identifier
// changed as a conformant retransmission changes it, and requires Ze to put
// exactly one Authenticate-Ack carrying the new Identifier on the same PPPoE
// session, with that session still in Ze's table, before tearing down with
// PADT.
func checkZeAccessConcentratorPAP(
	ctx context.Context,
	check *interoplab.CheckContext,
) (err error) {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE PAP checker has no lab")
	}
	defer func() {
		err = appendPPPDLog(ctx, check.Lab, err)
		err = appendDiagnostics(ctx, check.Lab, err, zeImageName, clientImageName)
	}()

	if err := waitZePPPoEConfigured(ctx, check.Lab); err != nil {
		return fmt.Errorf("ze PPPoE AC did not bind its access interface: %w", err)
	}
	if err := waitZeRESTReady(ctx, check.Lab); err != nil {
		return err
	}

	first, sessionID, err := dialPAPSession(ctx, check.Lab)
	if err != nil {
		return err
	}
	if err := checkPAPReanswer(ctx, check.Lab, first, sessionID, replayFrameInClient); err != nil {
		return err
	}
	return checkTeardown(ctx, check.Lab)
}

// dialPAPSession dials pppd with CHAP refused, waits until the session carries
// ICMP, and returns what the capture saw of the original PAP exchange together
// with the PPPoE SESSION_ID of the one session Ze reports.
func dialPAPSession(ctx context.Context, lab interoplab.CheckerLab) (papObservation, int, error) {
	if err := startSessionCapture(ctx, lab); err != nil {
		return papObservation{}, 0, err
	}
	if err := pppdDialRefusing(ctx, lab, pppdPassword, pppoeService, refuseCHAP); err != nil {
		return papObservation{}, 0, err
	}
	sessions, err := waitZeSession(ctx, lab, 45*time.Second)
	if err != nil {
		return papObservation{}, 0, err
	}
	if err := checkDiscoverySession(sessions); err != nil {
		return papObservation{}, 0, err
	}
	iface, err := checkLCPIPCPWithAuth(ctx, lab, papAuthEvidence)
	if err != nil {
		return papObservation{}, 0, err
	}
	ping, err := exec(ctx, lab, clientImageName, []string{"ping", "-c", "3", "-W", "3", "-I", iface, zeGateway})
	if err != nil {
		return papObservation{}, 0, fmt.Errorf("data: ping Ze gateway %s: %w", zeGateway, err)
	}
	if ping.ExitCode != 0 {
		return papObservation{}, 0, fmt.Errorf("data: ICMP did not cross the PAP session to %s", zeGateway)
	}

	capture, err := stopSessionCapture(ctx, lab)
	if err != nil {
		return papObservation{}, 0, err
	}
	observed, err := observePAPFrames(capture)
	if err != nil {
		return papObservation{}, 0, err
	}
	if observed.requests != 1 {
		return papObservation{}, 0, fmt.Errorf("original dial's capture carries %d PAP Authenticate-Requests, expected 1", observed.requests)
	}
	if observed.acks != 1 {
		return papObservation{}, 0, fmt.Errorf("original dial's capture carries %d PAP Authenticate-Acks, expected 1", observed.acks)
	}
	if observed.naks != 0 {
		return papObservation{}, 0, fmt.Errorf("original dial's capture carries %d PAP Authenticate-Naks, expected none", observed.naks)
	}
	if observed.ackID != observed.requestID {
		return papObservation{}, 0, fmt.Errorf(
			"PAP Authenticate-Ack carries Identifier %d, the request carried %d",
			observed.ackID, observed.requestID,
		)
	}
	if err := checkPAPSession(observed, sessions[0].SID); err != nil {
		return papObservation{}, 0, err
	}
	return observed, sessions[0].SID, nil
}

// checkPAPSession requires the request and the Ack in observed to have
// travelled on the PPPoE session Ze reports, so an Ack on another session
// cannot stand in for this one's.
func checkPAPSession(observed papObservation, sessionID int) error {
	if int(observed.requestSession) != sessionID {
		return fmt.Errorf("PAP Authenticate-Request travelled on PPPoE session %d, Ze reports session %d", observed.requestSession, sessionID)
	}
	if int(observed.ackSession) != sessionID {
		return fmt.Errorf("PAP Authenticate-Ack travelled on PPPoE session %d, Ze reports session %d", observed.ackSession, sessionID)
	}
	return nil
}

// checkPAPReanswer replays the client's original Authenticate-Request from
// inside the client container, after authentication completed and IPCP
// opened, with its Identifier advanced by one, and requires Ze to answer it on
// the same PPPoE session with the Code it answered the first time and the new
// Identifier. Replaying the bytes unchanged could not tell an Ack that copies
// the request's Identifier from a cached Ack resent as it was.
func checkPAPReanswer(
	ctx context.Context,
	lab interoplab.CheckerLab,
	first papObservation,
	sessionID int,
	send clientFrameSender,
) error {
	// RFC 1334 Section 2.2.1: "The Identifier field MUST be changed each time
	// an Authenticate-Request packet is issued."
	replay := append([]byte(nil), first.request...)
	replayID := first.requestID + 1
	replay[papIdentifierOffset] = replayID

	if err := startSessionCapture(ctx, lab); err != nil {
		return err
	}
	if err := send(ctx, lab, replay); err != nil {
		return fmt.Errorf("replay the PAP Authenticate-Request: %w", err)
	}
	if err := waitFixed(ctx, replayRoundTripBound); err != nil {
		return err
	}
	capture, err := stopSessionCapture(ctx, lab)
	if err != nil {
		return err
	}
	replayed, err := observePAPFrames(capture)
	if err != nil {
		return err
	}
	if replayed.requests != 1 {
		return fmt.Errorf("replay capture carries %d PAP Authenticate-Requests, expected exactly the one replayed", replayed.requests)
	}
	if replayed.requestID != replayID {
		return fmt.Errorf("replay capture's Authenticate-Request carries Identifier %d, the replay sent %d", replayed.requestID, replayID)
	}
	// RFC 1334 Section 2.2.1: "the authenticator MUST allow repeated
	// Authenticate-Request packets after completing the Authentication phase."
	if replayed.acks != 1 {
		return fmt.Errorf(
			"RFC 1334 Section 2.2.1: Ze answered the repeated Authenticate-Request with %d Acks and %d Naks, want exactly one Ack",
			replayed.acks, replayed.naks,
		)
	}
	if replayed.naks != 0 {
		return fmt.Errorf("RFC 1334 Section 2.2.1: Ze answered the repeated Authenticate-Request with %d Naks", replayed.naks)
	}
	// RFC 1334 Section 2.2.2: "The Identifier field MUST be copied from the
	// Identifier field of the Authenticate-Request which caused this reply."
	if replayed.ackID != replayID {
		return fmt.Errorf(
			"RFC 1334 Section 2.2.2: repeated Authenticate-Request carried Identifier %d, Ze's Ack carries %d",
			replayID, replayed.ackID,
		)
	}
	if err := checkPAPSession(replayed, sessionID); err != nil {
		return fmt.Errorf("repeated Authenticate-Request: %w", err)
	}

	sessions, err := zeSessions(ctx, lab)
	if err != nil {
		return err
	}
	if len(sessions) != 1 {
		return fmt.Errorf("the repeated Authenticate-Request changed Ze's session table: %+v, want one session", sessions)
	}
	if sessions[0].SID != sessionID {
		return fmt.Errorf("the repeated Authenticate-Request replaced Ze's session %d with session %d", sessionID, sessions[0].SID)
	}
	links, err := pppLinks(ctx, lab, clientImageName)
	if err != nil {
		return err
	}
	if len(links) != 1 {
		return fmt.Errorf("the repeated Authenticate-Request left %d client PPP interfaces, want 1", len(links))
	}
	return nil
}

// replayPCAPPath is where the one-frame replay capture is written inside the
// client container, beside pppd's own log.
const replayPCAPPath = "/var/log/ppp/pap-replay.pcap"

// clientFrameSender puts frame on the client container's wire. The type
// exists so TestCheckPAPReanswerJudgesTheReply can substitute a fake for the
// Docker exec and script Ze's reply to the frame it was handed.
type clientFrameSender func(ctx context.Context, lab interoplab.CheckerLab, frame []byte) error

// replayFrameInClient writes frame as a one-record pcap inside the client
// container and sends it on eth0 with tcpreplay. The send runs inside the
// container, so it needs no host privilege to enter the container's network
// namespace (interoplab.SendFrameInNamespace does).
func replayFrameInClient(ctx context.Context, lab interoplab.CheckerLab, frame []byte) error {
	var capture bytes.Buffer
	if err := pcap.WriteFileHeader(&capture, uint32(len(frame)), pcap.LinkTypeEthernet); err != nil { //nolint:gosec // one captured Ethernet frame, far below 4 GiB
		return fmt.Errorf("build replay capture: %w", err)
	}
	if err := pcap.WriteRecord(&capture, time.Unix(0, 0), frame, len(frame)); err != nil {
		return fmt.Errorf("build replay capture: %w", err)
	}
	var tb textbuf.Buffer
	shell := tb.Str("echo ").Str(base64.StdEncoding.EncodeToString(capture.Bytes())).
		Str(" | base64 -d > ").Str(replayPCAPPath).
		Str(" && tcpreplay -q -i eth0 ").Str(replayPCAPPath).String()
	result, err := exec(ctx, lab, clientImageName, []string{"sh", "-c", shell})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("tcpreplay exited %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// papAuthEvidence proves from pppd's trace that Ze demanded PAP, that the
// client sent its named credential, and that Ze acknowledged it. CHAP never
// appearing proves the method was PAP and not a CHAP fallback.
func papAuthEvidence(log string) error {
	if !logLineWith(log, "rcvd [LCP ConfReq", "<auth pap>") {
		return errors.New("LCP: Ze's Configure-Request did not demand PAP")
	}
	if !logLineWith(log, "sent [PAP AuthReq", pppdUsername) {
		return errors.New("auth: the client sent no named PAP Authenticate-Request")
	}
	if !strings.Contains(log, "rcvd [PAP AuthAck") {
		return errors.New("auth: Ze did not acknowledge the PAP Authenticate-Request")
	}
	if strings.Contains(log, "CHAP") {
		return errors.New("auth: CHAP appeared in a session the client restricted to PAP")
	}
	return nil
}

// observePAPFrames reads every frame in capture and counts the PAP packets
// among the PPPoE session-stage frames. LCP echoes and IPCP share the capture
// filter, so a frame carrying another PPP protocol is skipped, while a frame
// too short to hold the PAP header is a corrupt capture.
func observePAPFrames(capture []byte) (papObservation, error) {
	reader, err := pcap.NewReader(bytes.NewReader(capture))
	if err != nil {
		return papObservation{}, fmt.Errorf("session capture: %w", err)
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return papObservation{}, fmt.Errorf("session capture link type %d, expected Ethernet", reader.LinkType())
	}

	var observed papObservation
	var record pcap.Record
	for {
		if err := reader.Next(&record); err != nil {
			if errors.Is(err, io.EOF) {
				return observed, nil
			}
			return papObservation{}, fmt.Errorf("session capture: %w", err)
		}
		frame := record.Data
		if len(frame) < pppProtocolOffset+2 {
			return papObservation{}, fmt.Errorf("session capture: %d-octet frame is shorter than a PPPoE session header", len(frame))
		}
		if binary.BigEndian.Uint16(frame[etherTypeOffset:]) != etherTypePPPoESession {
			continue
		}
		if frame[pppoeCodeOffset] != 0 {
			continue
		}
		if binary.BigEndian.Uint16(frame[pppProtocolOffset:]) != pppProtocolPAP {
			continue
		}
		if len(frame) < papFrameOctetsMin {
			return papObservation{}, fmt.Errorf("session capture: %d-octet PAP frame is shorter than its header", len(frame))
		}
		identifier := frame[papIdentifierOffset]
		session := binary.BigEndian.Uint16(frame[pppoeSessionIDOffset:])
		switch frame[papCodeOffset] {
		case papCodeAuthRequest:
			observed.requests++
			if observed.request == nil {
				observed.request = append([]byte(nil), frame...)
				observed.requestID = identifier
				observed.requestSession = session
			}
		case papCodeAuthAck:
			observed.acks++
			observed.ackID = identifier
			observed.ackSession = session
		case papCodeAuthNak:
			observed.naks++
		default:
			return papObservation{}, fmt.Errorf("session capture: PAP frame carries unknown Code %d", frame[papCodeOffset])
		}
	}
}
