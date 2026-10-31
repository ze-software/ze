// Design: docs/architecture/testing/interop.md -- RFC 1661 Code-Reject interoperability.
// Related: avprelay/pppinject.go -- the probes the relay sends Ze as the LAC.
package l2tp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The probe the relay sends, as this checker expects it. The relay is a
// separate program built alone in its own image, so these values are stated
// here again rather than imported, and the checker reads each probe off the
// wire rather than trusting the relay to have sent it.
const (
	// pppCodeRejectMRU is the MRU the scenario's pppd requests (`mru 128` in
	// test/interop-l2tp/scenarios/ppp-code-reject-xl2tpd/ppp-options).
	pppCodeRejectMRU = 128
	// probeIdentifier is the one Identifier all four probes carry.
	probeIdentifier = 0x42
	// probeOctets is a probe's whole length, header and Data: past the MRU.
	probeOctets = 204
	// probeCodeLCP is no LCP Code (RFC 1661 Section 5), and probeCodeIPCP is
	// none of IPCP's (RFC 1332 Section 2 allows Codes 1 through 7 only).
	probeCodeLCP  = 99
	probeCodeIPCP = 12

	protocolLCP  = 0xc021
	protocolIPCP = 0x8021

	codeConfigureRequest = 1
	codeTerminateAck     = 6
	codeCodeReject       = 7

	// commandPing is busybox ping in the LAC image.
	commandPing = "ping"

	// relayInjected is the relay's direction label for a probe it sent.
	relayInjected = "inject"
	// relayFromZe is the relay's direction label for a datagram Ze sent.
	relayFromZe = "ze"
)

// pppFrame is one PPP control-protocol frame the relay logged: who sent it,
// its PPP Protocol, and its Information field, which starts at the Code.
type pppFrame struct {
	direction string
	protocol  uint16
	packet    []byte
}

// checkPPPCodeReject has the relay send Ze, as the LAC and on the live L2TP
// session, two LCP packets with the unknown Code 99 and two IPCP packets with
// Code 12, all four with Identifier 0x42 and 204 octets long, then reads Ze's
// Code-Rejects off the wire.
//
// Only the LCP answers are asserted. Once IPCP is Opened, ze connects the PPP
// channel to its unit, and Linux then queues every IPCP frame on the unit
// descriptor, which ze never reads, so the IPCP probes draw no answer at all
// (plan/journal/frames-rerouted-to-a-descriptor-nobody-reads.md). The IPCP
// probes stay, so the session must survive them, and the fix of that defect
// adds their assertions.
//
// RFC requirement: RFC1661-5.6-2 positive -- over a live xl2tpd and pppd
// session, two LCP packets with the unknown Code 99 sharing Identifier 0x42 draw
// two LCP Code-Rejects from ze whose Identifiers differ.
// RFC requirement: RFC1661-5.6-3 positive -- with pppd requesting an MRU of 128,
// each of those two LCP Code-Rejects ze sends has an Information field of at
// most 128 octets, and its Rejected-Packet is the start of the 204-octet packet
// it rejects, that packet's Code, Identifier and Length unchanged.
//
// Each assertion is numbered, so a red run names which one failed and a
// discrimination record can cite it.
func checkPPPCodeReject(ctx context.Context, lab labOperations, _ time.Duration) error {
	const (
		// The grouped form is what `interopScenarioOf` (`internal/le/rfc`) reads to
		// learn which scenario a tagged checker drives.
		name = "ppp-code-reject-xl2tpd"
	)
	fail := func(assertion int, cause error) error {
		var tb textbuf.Buffer
		tb.Str("scenario ").Str(name).Str(" assertion ").Str(strconv.Itoa(assertion)).Str(": ").Err(cause)
		return errors.New(tb.String())
	}

	// Assertion 1. A PPP session carries traffic before any probe is sent, so
	// what follows happens on an Opened LCP and IPCP.
	if err := waitPeerLog(ctx, lab, peerZe, logSessionIPLine, 60*time.Second); err != nil {
		return fail(1, err)
	}
	zeLink, err := waitPPPInterface(ctx, lab, peerZe, 30*time.Second)
	if err != nil {
		return fail(1, err)
	}
	lacLink, err := waitPPPInterface(ctx, lab, peerLAC, 30*time.Second)
	if err != nil {
		return fail(1, err)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(1, err)
	}

	// Assertion 2. The relay sent the four probes, and Ze answered both LCP
	// probes with a Code-Reject. The next datagram the LAC sends after the
	// marker is the relay's trigger, and the ping supplies it.
	if _, err := lab.Exec(ctx, peerAVPRelay, []string{"touch", "/run/code-reject"}, nil); err != nil {
		return fail(2, err)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(2, err)
	}
	frames, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "four probes and two Ze LCP Code-Rejects",
	}, func(probeCtx context.Context) ([]pppFrame, error) {
		result, readErr := lab.Logs(probeCtx, peerAVPRelay, 1000)
		if readErr != nil {
			return nil, readErr
		}
		return relayPPPFrames(result.Text)
	}, func(frames []pppFrame) bool {
		return len(zeCodeRejects(frames, protocolLCP)) >= 2
	})
	if err != nil {
		return fail(2, err)
	}
	if err := injectedProbes(frames); err != nil {
		return fail(2, err)
	}

	// Assertion 3. RFC 1661 Section 5.6: "The Identifier field MUST be changed
	// for each Code-Reject sent." Both LCP probes carried Identifier 0x42.
	if err := distinctIdentifiers(zeCodeRejects(frames, protocolLCP)); err != nil {
		return fail(3, err)
	}
	// Assertion 4. RFC 1661 Section 5.6: "The Rejected-Packet MUST be truncated
	// to comply with the peer's established MRU."
	if err := rejectsWithinMRU(zeCodeRejects(frames, protocolLCP), probeCodeLCP); err != nil {
		return fail(4, err)
	}

	// Assertion 5. The session stays up: traffic still flows, both PPP
	// interfaces are the ones from before the probes, and neither side
	// renegotiated or terminated LCP or IPCP after them.
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(5, err)
	}
	for _, link := range []struct{ peer, before string }{{peerZe, zeLink}, {peerLAC, lacLink}} {
		now, linkErr := pppLinks(ctx, lab, link.peer)
		if linkErr != nil {
			return fail(5, linkErr)
		}
		var tb textbuf.Buffer
		changed := errors.New(tb.Str(link.peer).Str(" PPP interfaces are ").Join(now, ", ").
			Str(", not the ").Str(link.before).Str(" the probes found").String())
		if len(now) != 1 {
			return fail(5, changed)
		}
		if now[0] != link.before {
			return fail(5, changed)
		}
	}
	result, err := lab.Logs(ctx, peerAVPRelay, 1000)
	if err != nil {
		return fail(5, err)
	}
	after, err := relayPPPFrames(result.Text)
	if err != nil {
		return fail(5, err)
	}
	if err := sessionKept(after); err != nil {
		return fail(5, interoplab.WithSearchedOutput(err, result.Text))
	}
	// Retain the wire evidence in the native job log before Suite removes the
	// relay container, including on a successful run.
	_, err = os.Stdout.WriteString("L2TP-PPP-CODE-REJECT-WIRE four probes, two LCP Code-Rejects, session kept\n" + result.Text + "\n")
	return err
}

// pingThroughSession sends three echo requests from the LAC to Ze's end of the
// PPP link, so they cross the L2TP session and the relay.
func pingThroughSession(ctx context.Context, lab labOperations) error {
	_, err := lab.Exec(ctx, peerLAC, []string{commandPing, "-c", "3", "-W", "3", localPPPAddress}, nil)
	return err
}

// relayPPPFrames decodes every PPP control-protocol frame the relay logged.
// It skips the L2TP control messages, which carry no PPP, and the IP
// datagrams, whose PPP Protocol has its high bit clear, and it refuses any
// data datagram it cannot read, so a frame the log holds is never passed over
// unread.
func relayPPPFrames(text string) ([]pppFrame, error) {
	frames := make([]pppFrame, 0, 16)
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event avpWireObservation
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("decode relay observation: %w", err)
		}
		datagram, err := hex.DecodeString(event.Forwarded)
		if err != nil {
			return nil, fmt.Errorf("decode relayed datagram: %w", err)
		}
		if len(datagram) >= 2 && datagram[0]&0x80 != 0 {
			continue
		}
		protocol, packet, err := pppInformation(datagram)
		if err != nil {
			return nil, err
		}
		if protocol&0x8000 == 0 {
			continue
		}
		frames = append(frames, pppFrame{direction: event.Direction, protocol: protocol, packet: packet})
	}
	return frames, nil
}

// pppInformation reads an L2TP data message (RFC 2661 Section 3.1) and answers
// its PPP Protocol and Information field. The header's optional Length, Ns and
// Nr, and Offset fields follow the flags in that order, and the Linux PPPoL2TP
// path writes the PPP Address 0xff and Control 0x03 before the Protocol.
func pppInformation(datagram []byte) (uint16, []byte, error) {
	if len(datagram) < 6 {
		return 0, nil, errors.New("short L2TP data message")
	}
	flags := binary.BigEndian.Uint16(datagram)
	start := 2
	if flags&0x4000 != 0 {
		start += 2
	}
	start += 4
	if flags&0x0800 != 0 {
		start += 4
	}
	if flags&0x0200 != 0 {
		if len(datagram) < start+2 {
			return 0, nil, errors.New("short L2TP offset field")
		}
		start += 2 + int(binary.BigEndian.Uint16(datagram[start:]))
	}
	if len(datagram) < start+4 {
		return 0, nil, errors.New("short PPP frame")
	}
	if !bytes.Equal(datagram[start:start+2], []byte{0xff, 0x03}) {
		return 0, nil, errors.New("PPP frame has no Address and Control octets")
	}
	return binary.BigEndian.Uint16(datagram[start+2:]), datagram[start+4:], nil
}

// injectedProbes requires the relay's log to hold exactly the four probes:
// LCP Code 99 twice then IPCP Code 12 twice, each Identifier 0x42 and each
// 204 octets, Data octet i holding i.
func injectedProbes(frames []pppFrame) error {
	want := []struct {
		protocol uint16
		code     byte
	}{{protocolLCP, probeCodeLCP}, {protocolLCP, probeCodeLCP}, {protocolIPCP, probeCodeIPCP}, {protocolIPCP, probeCodeIPCP}}
	seen := 0
	for _, frame := range frames {
		if frame.direction != relayInjected {
			continue
		}
		if seen == len(want) {
			return errors.New("the relay sent more than four probes")
		}
		if frame.protocol != want[seen].protocol {
			return errors.New("probe " + strconv.Itoa(seen+1) + " has the wrong PPP Protocol")
		}
		if err := requireProbe(frame.packet, want[seen].code); err != nil {
			return errors.New("probe " + strconv.Itoa(seen+1) + ": " + err.Error())
		}
		seen++
	}
	if seen != len(want) {
		return errors.New("the relay sent " + strconv.Itoa(seen) + " probes, not four")
	}
	return nil
}

func requireProbe(packet []byte, code byte) error {
	if len(packet) != probeOctets {
		return errors.New("not " + strconv.Itoa(probeOctets) + " octets")
	}
	if packet[0] != code {
		return errors.New("wrong Code")
	}
	if packet[1] != probeIdentifier {
		return errors.New("wrong Identifier")
	}
	if int(binary.BigEndian.Uint16(packet[2:])) != probeOctets {
		return errors.New("wrong Length")
	}
	for index, octet := range packet[4:] {
		if octet != byte(index) {
			return errors.New("wrong Data")
		}
	}
	return nil
}

// zeCodeRejects answers the Information field of every Code-Reject Ze sent on
// protocol, in the order the relay forwarded them. Only LCP is asked for
// today. The IPCP assertions owed by
// plan/journal/frames-rerouted-to-a-descriptor-nobody-reads.md ask for
// protocolIPCP.
func zeCodeRejects(frames []pppFrame, protocol uint16) [][]byte { //nolint:unparam // the owed IPCP assertions pass protocolIPCP
	rejects := make([][]byte, 0, 2)
	for _, frame := range frames {
		if frame.direction != relayFromZe {
			continue
		}
		if frame.protocol != protocol {
			continue
		}
		if len(frame.packet) == 0 {
			continue
		}
		if frame.packet[0] != codeCodeReject {
			continue
		}
		rejects = append(rejects, frame.packet)
	}
	return rejects
}

// distinctIdentifiers requires exactly two Code-Rejects, one per probe, whose
// Identifiers differ.
func distinctIdentifiers(rejects [][]byte) error {
	if len(rejects) != 2 {
		return errors.New("ze sent " + strconv.Itoa(len(rejects)) + " Code-Rejects for two probes")
	}
	for _, reject := range rejects {
		if len(reject) < 2 {
			return errors.New("Code-Reject has no Identifier")
		}
	}
	if rejects[0][1] == rejects[1][1] {
		return errors.New("both Code-Rejects carry Identifier " + strconv.Itoa(int(rejects[0][1])))
	}
	return nil
}

// rejectsWithinMRU requires every Code-Reject to fit pppCodeRejectMRU and to
// carry a copy of the start of the probe it rejects, whose Code is code. A
// probe is longer than that MRU, so a copy that fits had to be cut, and the
// rejected packet's own Length still says how long it was (RFC 1661 Section
// 5.6: "The Rejected-Packet field contains a copy of the LCP packet which is
// being rejected.").
func rejectsWithinMRU(rejects [][]byte, code byte) error {
	if len(rejects) == 0 {
		return errors.New("no Code-Reject to measure")
	}
	for _, reject := range rejects {
		if len(reject) > pppCodeRejectMRU {
			return errors.New("a Code-Reject's Information field is " + strconv.Itoa(len(reject)) +
				" octets, past the peer's MRU of " + strconv.Itoa(pppCodeRejectMRU))
		}
		if len(reject) < 8 {
			return errors.New("a Code-Reject carries no rejected packet header")
		}
		if int(binary.BigEndian.Uint16(reject[2:])) != len(reject) {
			return errors.New("a Code-Reject's Length is not its Information field's")
		}
		rejected := reject[4:]
		if rejected[0] != code {
			return errors.New("the Rejected-Packet's Code is not the probe's")
		}
		if rejected[1] != probeIdentifier {
			return errors.New("the Rejected-Packet's Identifier is not the probe's")
		}
		if int(binary.BigEndian.Uint16(rejected[2:])) != probeOctets {
			return errors.New("the Rejected-Packet's Length is not the probe's")
		}
		for index, octet := range rejected[4:] {
			if octet != byte(index) {
				return errors.New("the Rejected-Packet's Data is not the probe's")
			}
		}
	}
	return nil
}

// sessionKept requires the relay's log to hold ze's two LCP Code-Rejects and
// no LCP or IPCP Configure or Terminate packet, Codes 1 through 6, from either
// peer. The relay logs from the moment the checker arms it, before the
// probes, so the Code-Rejects are the proof the log covered the time after.
func sessionKept(frames []pppFrame) error {
	if len(zeCodeRejects(frames, protocolLCP)) != 2 {
		return errors.New("the relay log does not hold ze's two LCP Code-Rejects")
	}
	for _, frame := range frames {
		if frame.direction == relayInjected {
			continue
		}
		if frame.protocol != protocolLCP && frame.protocol != protocolIPCP {
			continue
		}
		if len(frame.packet) == 0 {
			continue
		}
		code := frame.packet[0]
		if code >= codeConfigureRequest && code <= codeTerminateAck {
			var tb textbuf.Buffer
			return errors.New(tb.Str(frame.direction).Str(" sent a PPP packet with Code ").
				Str(strconv.Itoa(int(code))).Str(" after the probes: the session was renegotiated or ended").String())
		}
	}
	return nil
}
