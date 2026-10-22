// Design: docs/labs/pppoe-interop.md -- ipv6cp-zero-identifier and
// ipv6cp-missing-option: RFC 5072 Section 4.1's zero-identifier Nak and its
// missing-option one-shot Nak, proved against a real pppd client.
// RFC: rfc/short/rfc5072.md -- Section 4.1: "If the two interface
// identifiers are different but the received interface identifier is zero,
// a Configure-Nak is sent with a non-zero interface-identifier value
// suggested for use by the remote peer"; a Configure-Request carrying no
// Interface-Identifier option is Naked once, then Acked on the peer's next
// tagless request.
// Related: check_padr_replay.go -- the wire-capture-and-replay shape both
// checkers here follow (interoplab.SendFrameInContainer),
// extended from the PPPoE discovery ethertype (0x8863) to the session
// ethertype (0x8864) that carries PPP once a session is up.
// Related: check_ac.go -- the general Ze access-concentrator checker both
// share their dial and session-wait helpers with.
//
// BLOCKED on plan/spec-l2tp-ipv6-subscriber.md, confirmed by reading both
// producers: poolPlugin.handle (internal/component/l2tp/plugins/pool/register.go)
// answers every EventIPRequest whose Family != AddressFamilyIPv4 with
// Accept: false, unconditionally; runNCPPhase
// (internal/component/l2tp/ppp/ncp.go) reads that decline BEFORE the
// session reads a single client frame and sets disableIPv6CP, so
// startNCP(AddressFamilyIPv6) never runs and evalIPv6CPRequest is
// unreachable in a shipped daemon today, on every configuration, regardless
// of what a client offers. Both checkers below wait for wire evidence Ze
// engaged IPv6CP at all and return a citing error there until that spec
// changes the pool decline. Written and registered now so
// interoplab.Discover picks them up the moment it does; the assertions
// after that wait are the real target behavior, not weakened to pass today.

//go:build ze_l2tp

package pppoe

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	discovery "github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// sessionCaptureFile is the pcap path tcpdump writes inside the client
// container while it captures PPPoE session-stage (0x8864) frames -- the
// ethertype that carries PPP, and inside it IPv6CP, once discovery has
// finished. Distinct from captureFile (check_service_name.go), which
// captures discovery-stage (0x8863) frames only.
const sessionCaptureFile = "/tmp/session.pcap"

// ipv6cpExchangeWindow is how long a checker gives the client and Ze to
// exchange IPv6CP frames before it stops a capture and reads what arrived.
// Generous: on a working dependency this is a same-host loopback exchange
// with no negotiation depth beyond a handful of round trips; on today's
// tree nothing arrives at all and the checker waits out the whole window
// before reporting that plainly.
const ipv6cpExchangeWindow = 15 * time.Second

// pppSessionOverhead is the PPPoE session-stage encapsulation this package
// strips before handing bytes to ppp.ParseFrame: an Ethernet header plus a
// PPPoE session header (RFC 2516 Section 4).
const pppSessionOverhead = discovery.EthHdrLen + discovery.PPPoEHdrLen

// startSessionCapture is startDiscoveryCapture's sibling for PPPoE
// session-stage (0x8864) frames (check_service_name.go's own pair captures
// discovery-stage 0x8863 only). Both call startCapture/stopCapture
// (check_service_name.go), which do the waiting.
func startSessionCapture(ctx context.Context, lab interoplab.CheckerLab) error {
	return startCapture(ctx, lab, sessionCaptureFile, "ether proto 0x8864")
}

// stopSessionCapture is stopDiscoveryCapture's sibling for sessionCaptureFile.
func stopSessionCapture(ctx context.Context, lab interoplab.CheckerLab) ([]byte, error) {
	return stopCapture(ctx, lab, sessionCaptureFile)
}

// ipv6cpFrame is one IPv6CP packet read off a session-stage capture: the LCP
// header's own code and 1-byte Identifier (RFC 1661 Section 5's req/ack
// pairing tag, distinct from the option below), the value from its (at
// most one) Interface-Identifier option, and the raw session-stage frame
// bytes it arrived in, kept for replay.
type ipv6cpFrame struct {
	code        uint8
	lcpID       uint8
	hasOption   bool
	interfaceID [8]byte
	raw         []byte
}

// parseIPv6CPFrames reads every session-stage Ethernet frame in capture and
// returns the ones that parse as PPP protocol 0x8057 (IPv6CP, RFC 5072
// Section 4). A frame on a different protocol (LCP, CHAP, IPCP) is skipped
// rather than treated as corrupt: the capture filter narrows to PPPoE
// session frames only, which carry every PPP protocol in flight, not IPv6CP
// alone.
func parseIPv6CPFrames(capture []byte) ([]ipv6cpFrame, error) {
	reader, err := pcap.NewReader(bytes.NewReader(capture))
	if err != nil {
		return nil, fmt.Errorf("session capture: %w", err)
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return nil, fmt.Errorf("session capture link type %d, expected Ethernet", reader.LinkType())
	}

	var frames []ipv6cpFrame
	var record pcap.Record
	for {
		if err := reader.Next(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("session capture: %w", err)
		}
		frame, ok, parseErr := parseOneIPv6CPFrame(record.Data)
		if parseErr != nil {
			return nil, fmt.Errorf("session capture: frame matching the PPPoE session filter did not parse: %w", parseErr)
		}
		if ok {
			frames = append(frames, frame)
		}
	}
	return frames, nil
}

// parseOneIPv6CPFrame strips the Ethernet and PPPoE session headers RFC
// 2516 Section 4 defines (discovery.EthHdrLen, discovery.PPPoEHdrLen -- the
// same fixed sizes discovery.ParseDiscovery uses for the discovery stage)
// and, when what remains is PPP protocol 0x8057, parses it as an LCP-shaped
// IPv6CP packet (RFC 1661 Section 5 reused by RFC 5072 Section 4). Returns
// ok=false for a session frame carrying a different PPP protocol -- not an
// error, since a session-stage capture carries every PPP control protocol.
func parseOneIPv6CPFrame(raw []byte) (ipv6cpFrame, bool, error) {
	if len(raw) < pppSessionOverhead+2 {
		return ipv6cpFrame{}, false, nil
	}
	ethertype := binary.BigEndian.Uint16(raw[12:14])
	if ethertype != discovery.EthPPPSes {
		return ipv6cpFrame{}, false, nil
	}
	proto, payload, _, err := ppp.ParseFrame(raw[pppSessionOverhead:])
	if err != nil {
		return ipv6cpFrame{}, false, fmt.Errorf("PPP frame: %w", err)
	}
	if proto != ppp.ProtoIPv6CP {
		return ipv6cpFrame{}, false, nil
	}
	pkt, err := ppp.ParseLCPPacket(payload)
	if err != nil {
		return ipv6cpFrame{}, false, fmt.Errorf("IPv6CP packet: %w", err)
	}
	frame := ipv6cpFrame{code: pkt.Code, lcpID: pkt.Identifier, raw: append([]byte(nil), raw...)}
	if len(pkt.Data) >= 2+8 && pkt.Data[0] == ppp.IPv6CPOptInterfaceID && pkt.Data[1] == 2+8 {
		frame.hasOption = true
		copy(frame.interfaceID[:], pkt.Data[2:2+8])
	}
	return frame, true, nil
}

// isUsableSuggestedIdentifier applies the same two checks
// isValidIPv6CPInterfaceID (internal/component/l2tp/ppp/ipv6cp.go) applies
// on receive, so a scenario proving AC-9 does not have to export that
// validator for a test lab to call it.
//
// Only the first of the two comes from the RFC. RFC 5072 Section 4.1: "If
// the two interface identifiers are different but the received interface
// identifier is zero, a Configure-Nak is sent with a non-zero
// interface-identifier value suggested for use by the remote peer". The
// all-ones exclusion is Ze's own, stated in isValidIPv6CPInterfaceID's own
// doc comment; RFC 5072 says nothing about that value.
func isUsableSuggestedIdentifier(id [8]byte) bool {
	var zero, allOnes [8]byte
	for i := range allOnes {
		allOnes[i] = 0xff
	}
	return id != zero && id != allOnes
}

// interfaceIDOption is the one IPv6CP option RFC 5072 Section 4.1 defines,
// Interface-Identifier, encoded as Type 1, Length 10, then the 8-byte value.
func interfaceIDOption(id [8]byte) []byte {
	option := make([]byte, 2+len(id))
	option[0] = ppp.IPv6CPOptInterfaceID
	option[1] = byte(len(option))
	copy(option[2:], id[:])
	return option
}

// rebuildIPv6CPConfigureRequest returns a session-stage frame byte-for-byte
// identical to template's Ethernet header and the first four bytes of its
// PPPoE header (destination/source MAC, PPPoE Ver/Type, Code, Session-ID --
// none of that changes when only the LCP-layer payload does), but carrying
// an IPv6CP Configure-Request whose option list is options. A nil options
// is the request with NO Interface-Identifier option, which RFC 5072
// Section 4.1 requires exactly one of; interfaceIDOption of the zero value
// is the request a pppd client cannot be made to send. The PPPoE header's
// own Length field (RFC 2516 Section 4: "the length of the PPPoE payload,
// not including the Ethernet or PPPoE headers") is recomputed for the new
// payload.
func rebuildIPv6CPConfigureRequest(template []byte, identifier uint8, options []byte) ([]byte, error) {
	if len(template) < pppSessionOverhead {
		return nil, fmt.Errorf("template frame too short: %d bytes", len(template))
	}
	pppPayload := make([]byte, 2+4+len(options)) // protocol field + LCP header + options
	n := ppp.WriteLCPPacket(pppPayload, 2, ppp.LCPConfigureRequest, identifier, options)
	ppp.WriteFrame(pppPayload, 0, ppp.ProtoIPv6CP, pppPayload[2:2+n])

	out := make([]byte, pppSessionOverhead+len(pppPayload))
	copy(out, template[:discovery.EthHdrLen+4]) // Ethernet header + PPPoE Ver/Type+Code+Session-ID
	binary.BigEndian.PutUint16(out[discovery.EthHdrLen+4:pppSessionOverhead], uint16(len(pppPayload)))
	copy(out[pppSessionOverhead:], pppPayload)
	return out, nil
}

// pppdDialIPv6 is pppdDial (check_ac.go) with IPv6CP requested. pppd draws
// its own non-zero Interface-Identifier; a checker that needs another value
// replays a rebuilt request (rebuildIPv6CPConfigureRequest), because pppd's
// `ipv6` option refuses the zero identifier.
func pppdDialIPv6(
	ctx context.Context,
	lab interoplab.CheckerLab,
	username string,
	password string,
	service string,
) error {
	if _, err := exec(ctx, lab, clientImageName, []string{"sh", "-c", "rm -f " + pppdLogPath}); err != nil {
		return fmt.Errorf("clear pppd log: %w", err)
	}
	arguments := []string{
		pppdExecutable,
		"plugin", "pppoe.so", "nic-eth0",
		"user", username,
		"password", password,
		"noauth", "refuse-pap", "refuse-eap", "refuse-mschap", "refuse-mschap-v2",
		"noipdefault", "nodefaultroute", "noaccomp", "nopcomp",
		"mtu", "1492", "mru", "1492",
		"lcp-echo-interval", "10", "lcp-echo-failure", "5",
		"maxfail", "1", "nodetach", "debug",
		"+ipv6",
	}
	if service != "" {
		arguments = append(arguments, "rp_pppoe_service", service)
	}
	shell := "exec " + strings.Join(arguments, " ") + " >" + pppdLogPath + " 2>&1"
	if err := lab.ExecDetached(ctx, clientImageName, []string{"sh", "-c", shell}, nil); err != nil {
		return fmt.Errorf("start pppd: %w", err)
	}
	return nil
}

// waitIPv6CPFrame gives the exchange ipv6cpExchangeWindow, then returns the
// IPv6CP frames a session-stage capture observed. capture must already be
// running (startSessionCapture); this stops it and reads it back.
func captureIPv6CPFrames(ctx context.Context, lab interoplab.CheckerLab) ([]ipv6cpFrame, error) {
	if err := waitFixed(ctx, ipv6cpExchangeWindow); err != nil {
		return nil, err
	}
	capture, err := stopSessionCapture(ctx, lab)
	if err != nil {
		return nil, err
	}
	return parseIPv6CPFrames(capture)
}

// errIPv6CPNeverEngaged names the two producers a reader checks first: see
// this file's own header comment for the full citation.
var errIPv6CPNeverEngaged = errors.New(
	"ze sent no IPv6CP frame of any kind: poolPlugin.handle declines every " +
		"non-IPv4 EventIPRequest and runNCPPhase reads that decline before " +
		"the session reads a client frame, so IPv6CP's FSM never starts " +
		"(plan/spec-l2tp-ipv6-subscriber.md; see this file's header comment)")

// checkZeAccessConcentratorIPv6CPZeroIdentifier proves RFC 5072 Section
// 4.1's zero-identifier rule on a real pppd 2.5.4 session: a
// Configure-Request whose Interface-Identifier is all-zero is answered
// with a Configure-Nak carrying a non-zero, non-all-ones suggestion, and
// the exchange converges once the request incorporates it (RFC 1661
// Section 5.3: a Configure-Nak's suggested value is what the peer's next
// Configure-Request offers).
//
// pppd cannot be made to offer the zero identifier: setifaceid (pppd/ipv6cp.c,
// the same in 2.5.1 and 2.5.4) refuses `ipv6 ::,` as "Illegal interface
// identifier (local)" through its VALIDID check, and with no `ipv6` option it
// draws a non-zero random one. So the two requests are built here, the
// missing-option checker's way: pppd's own Configure-Request is the template
// (same MACs, same session), replayed first with the zero identifier, then
// with the identifier ze's Configure-Nak suggested.
func checkZeAccessConcentratorIPv6CPZeroIdentifier(
	ctx context.Context,
	check *interoplab.CheckContext,
) error {
	return checkZeAccessConcentratorIPv6CPZeroIdentifierWith(ctx, check, interoplab.SendFrameInContainer)
}

// checkZeAccessConcentratorIPv6CPZeroIdentifierWith is
// checkZeAccessConcentratorIPv6CPZeroIdentifier with the frame-replay
// mechanics injected, as checkZeAccessConcentratorIPv6CPMissingOptionWith.
func checkZeAccessConcentratorIPv6CPZeroIdentifierWith(
	ctx context.Context,
	check *interoplab.CheckContext,
	send interoplab.FrameSender,
) (err error) {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE IPv6CP zero-identifier checker has no lab")
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
	if err := startSessionCapture(ctx, check.Lab); err != nil {
		return err
	}
	if err := pppdDialIPv6(ctx, check.Lab, pppdUsername, pppdPassword, pppoeService); err != nil {
		return err
	}

	frames, err := captureIPv6CPFrames(ctx, check.Lab)
	if err != nil {
		return err
	}
	template, ok := firstClientRequest(frames)
	if !ok {
		return errors.New("the client's own IPv6CP Configure-Request never appeared in the capture")
	}
	if !sawAnyResponse(frames) {
		return errIPv6CPNeverEngaged
	}

	// RFC 5072 Section 4.1: "If the two interface identifiers are different
	// but the received interface identifier is zero, a Configure-Nak is sent
	// with a non-zero interface-identifier value suggested for use by the
	// remote peer". A fresh LCP Identifier per replay (RFC 1661 Section 4.6)
	// keeps each one a new Configure-Request.
	zeroID := template.lcpID + 1
	answer, err := replayIPv6CPRequest(ctx, check.Lab, send, template.raw, zeroID, interfaceIDOption([8]byte{}))
	if err != nil {
		return fmt.Errorf("zero-identifier Configure-Request: %w", err)
	}
	if _, acked := zeAnswer(answer, template.raw, zeroID, ppp.LCPConfigureAck); acked {
		return errors.New("ze Configure-Acked a Configure-Request carrying a zero interface identifier " +
			"(RFC 5072 Section 4.1 requires a Configure-Nak with a non-zero suggestion)")
	}
	nak, naked := zeAnswer(answer, template.raw, zeroID, ppp.LCPConfigureNak)
	if !naked {
		return fmt.Errorf("zero-identifier Configure-Request: ze's response codes were %v, want a Configure-Nak", codes(answer))
	}
	if !nak.hasOption {
		return errors.New("ze's Configure-Nak to the zero identifier carried no Interface-Identifier suggestion")
	}
	if nak.interfaceID == ([8]byte{}) {
		return errors.New("ze's Configure-Nak suggested the zero identifier it just refused on receive")
	}
	if !isUsableSuggestedIdentifier(nak.interfaceID) {
		return fmt.Errorf("ze's Configure-Nak suggestion %x fails isValidIPv6CPInterfaceID: "+
			"RFC 5072 Section 4.1 requires a non-zero suggestion, and ze additionally refuses all-ones (AC-9)", nak.interfaceID)
	}

	// RFC 1661 Section 5.3: the next Configure-Request carries the suggested
	// value, and ze must Ack it, or the exchange never converges.
	suggestedID := template.lcpID + 2
	answer, err = replayIPv6CPRequest(ctx, check.Lab, send, template.raw, suggestedID, interfaceIDOption(nak.interfaceID))
	if err != nil {
		return fmt.Errorf("suggested-identifier Configure-Request: %w", err)
	}
	if _, acked := zeAnswer(answer, template.raw, suggestedID, ppp.LCPConfigureAck); !acked {
		return fmt.Errorf("ze's Configure-Nak was observed, but the request carrying its own suggestion %x "+
			"drew response codes %v, not a Configure-Ack", nak.interfaceID, codes(answer))
	}
	return nil
}

// replayIPv6CPRequest puts one rebuilt IPv6CP Configure-Request on the
// client's wire and returns the IPv6CP frames a fresh capture saw after it.
func replayIPv6CPRequest(
	ctx context.Context,
	lab interoplab.CheckerLab,
	send interoplab.FrameSender,
	template []byte,
	identifier uint8,
	options []byte,
) ([]ipv6cpFrame, error) {
	frame, err := rebuildIPv6CPConfigureRequest(template, identifier, options)
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	if err := startSessionCapture(ctx, lab); err != nil {
		return nil, err
	}
	if err := send(ctx, lab, clientImageName, replayInterface, frame); err != nil {
		return nil, fmt.Errorf("replay: %w", err)
	}
	return captureIPv6CPFrames(ctx, lab)
}

// firstClientRequest returns the first Configure-Request in frames that
// carries an Interface-Identifier option. The capture starts before pppd
// dials and ze sends no Configure-Request of its own before pppd's, so the
// first such request is pppd's.
func firstClientRequest(frames []ipv6cpFrame) (ipv6cpFrame, bool) {
	for _, frame := range frames {
		if frame.code != ppp.LCPConfigureRequest {
			continue
		}
		if frame.hasOption {
			return frame, true
		}
	}
	return ipv6cpFrame{}, false
}

// sawAnyResponse reports whether frames hold any Configure-Nak, -Ack or
// -Reject, the evidence that ze's IPv6CP FSM started at all.
func sawAnyResponse(frames []ipv6cpFrame) bool {
	if hasCode(frames, ppp.LCPConfigureNak) {
		return true
	}
	if hasCode(frames, ppp.LCPConfigureAck) {
		return true
	}
	return hasCode(frames, ppp.LCPConfigureReject)
}

// zeAnswer returns ze's frame of the given code answering the request
// with LCP Identifier identifier. RFC 1661 Section 5.3 has the answer copy
// the request's Identifier, but pppd's own answers to ze's renegotiation
// share the capture and the one-byte Identifier space, so the answer must
// also come from ze: its source MAC is the template request's destination
// MAC.
func zeAnswer(frames []ipv6cpFrame, template []byte, identifier, code uint8) (ipv6cpFrame, bool) {
	if len(template) < discovery.EthHdrLen {
		return ipv6cpFrame{}, false
	}
	zeMAC := template[0:6]
	for _, frame := range frames {
		if frame.code != code {
			continue
		}
		if frame.lcpID != identifier {
			continue
		}
		if bytes.Equal(frame.raw[6:12], zeMAC) {
			return frame, true
		}
	}
	return ipv6cpFrame{}, false
}

// checkZeAccessConcentratorIPv6CPMissingOption proves RFC 5072 Section
// 4.1's missing-option rule against a real pppd 2.5.4 client: a
// Configure-Request carrying no Interface-Identifier option is Naked
// once, and the peer's next tagless request is Acked rather than Naked
// again (AC-10's one-shot rule; pppd(8)'s own ipv6cp_reqci applies the
// identical guard, per this spec's Required Reading).
//
// pppd always sends the option in its own negotiation, so the two
// requests this checker needs are built here rather than dialed: a
// genuine client Configure-Request is captured first and used as the
// template (same MACs, same session, same PPPoE header), then replayed
// twice with the option stripped (rebuildIPv6CPConfigureRequest, nil options).
func checkZeAccessConcentratorIPv6CPMissingOption(
	ctx context.Context,
	check *interoplab.CheckContext,
) error {
	return checkZeAccessConcentratorIPv6CPMissingOptionWith(ctx, check, interoplab.SendFrameInContainer)
}

// checkZeAccessConcentratorIPv6CPMissingOptionWith is
// checkZeAccessConcentratorIPv6CPMissingOption with the frame-replay
// mechanics injected (the interoplab.FrameSender indirection
// check_padr_replay.go also takes), so a unit test can substitute a fake for
// the Docker exec that puts the frame on the client's wire.
func checkZeAccessConcentratorIPv6CPMissingOptionWith(
	ctx context.Context,
	check *interoplab.CheckContext,
	send interoplab.FrameSender,
) (err error) {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE IPv6CP missing-option checker has no lab")
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
	if err := startSessionCapture(ctx, check.Lab); err != nil {
		return err
	}
	if err := pppdDialIPv6(ctx, check.Lab, pppdUsername, pppdPassword, pppoeService); err != nil {
		return err
	}

	frames, err := captureIPv6CPFrames(ctx, check.Lab)
	if err != nil {
		return err
	}

	var template ipv6cpFrame
	var sawZeResponse bool
	for _, frame := range frames {
		if frame.code == ppp.LCPConfigureRequest && frame.hasOption && template.raw == nil {
			template = frame
		}
		if frame.code == ppp.LCPConfigureNak || frame.code == ppp.LCPConfigureAck || frame.code == ppp.LCPConfigureReject {
			sawZeResponse = true
		}
	}
	if template.raw == nil {
		return errors.New("the client's own IPv6CP Configure-Request never appeared in the capture")
	}
	if !sawZeResponse {
		return errIPv6CPNeverEngaged
	}

	// First tagless request: RFC 5072 Section 4.1 requires exactly one
	// Interface-Identifier option, so ze must Nak this rather than Ack it.
	// A fresh LCP Identifier byte per replay (RFC 1661 Section 4.6) keeps
	// each one a new Configure-Request rather than a retransmission of the
	// template's own.
	firstMissing, err := rebuildIPv6CPConfigureRequest(template.raw, template.lcpID+1, nil)
	if err != nil {
		return fmt.Errorf("build the missing-option Configure-Request (first): %w", err)
	}
	if err := startSessionCapture(ctx, check.Lab); err != nil {
		return err
	}
	if err := send(ctx, check.Lab, clientImageName, replayInterface, firstMissing); err != nil {
		return fmt.Errorf("replay the missing-option Configure-Request (first): %w", err)
	}
	first, err := captureIPv6CPFrames(ctx, check.Lab)
	if err != nil {
		return err
	}
	if !hasCode(first, ppp.LCPConfigureNak) {
		return fmt.Errorf("first tagless Configure-Request: ze's response codes were %v, want a Configure-Nak", codes(first))
	}

	// Second tagless request, same session: AC-10's one-shot rule -- ze
	// must Ack rather than Nak again, or the negotiation never terminates.
	secondMissing, err := rebuildIPv6CPConfigureRequest(template.raw, template.lcpID+2, nil)
	if err != nil {
		return fmt.Errorf("build the missing-option Configure-Request (second): %w", err)
	}
	if err := startSessionCapture(ctx, check.Lab); err != nil {
		return err
	}
	if err := send(ctx, check.Lab, clientImageName, replayInterface, secondMissing); err != nil {
		return fmt.Errorf("replay the missing-option Configure-Request (second): %w", err)
	}
	second, err := captureIPv6CPFrames(ctx, check.Lab)
	if err != nil {
		return err
	}
	if !hasCode(second, ppp.LCPConfigureAck) {
		return fmt.Errorf("second tagless Configure-Request: ze's response codes were %v, want a Configure-Ack (AC-10 one-shot)", codes(second))
	}
	return nil
}

func hasCode(frames []ipv6cpFrame, code uint8) bool {
	for _, frame := range frames {
		if frame.code == code {
			return true
		}
	}
	return false
}

func codes(frames []ipv6cpFrame) []uint8 {
	out := make([]uint8, len(frames))
	for i, frame := range frames {
		out[i] = frame.code
	}
	return out
}
