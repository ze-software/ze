// Design: docs/architecture/testing/interop.md -- NCP lifetime against pppd on Ze's IPv4-only pool.
// Related: ppp_ipcp_reopen.go -- the IPCP withdrawal and reopen helpers the redistribute scenario reuses.
// Related: avprelay/pppinject.go -- the relay that injects the IPCP Terminate-Request and the CCP Configure-Request.
package l2tp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The values below restate what each scenario's ze.conf and the relay set,
// because the relay is a separate program built alone in its own image and a
// checker reads each frame off the wire rather than trusting either side.
const (
	protocolIPv6CP = 0x8057
	protocolCCP    = 0x80fd

	// codeProtocolReject is the LCP Protocol-Reject Code (RFC 1661 Section 5.7).
	codeProtocolReject = 8

	// ccpMarker and ccpIdentifier are the relay's CCP Configure-Request
	// marker and the Identifier the injected request carries
	// (avprelay/pppinject.go, ccpMarker and ccpIdentifier).
	ccpMarker     = "/run/ccp-configure-request"
	ccpIdentifier = 0x3c

	// cdnMessageType is the L2TP Call-Disconnect-Notify Message Type (RFC
	// 2661 Section 3.2).
	cdnMessageType = 14

	// logSessionUpLine is what the reactor logs on EventSessionUp
	// (reactor_kernel.go), and logNoNCPBoundExpiry what the session logs
	// when the no-NCP bound expires in the NCP phase (ncp.go, runNCPPhase).
	logSessionUpLine    = "PPP session up"
	logNoNCPBoundExpiry = " with no NCP opened"

	// ipv6cpRejectedNCPTimeout is `ncp timeout 8` in
	// l2tp-lns-pppd-ipv6cp-rejected-by-lns/ze.conf, and
	// ipv6cpRejectedRestartTimer the default `lcp restart-timer` it keeps.
	ipv6cpRejectedNCPTimeout   = 8 * time.Second
	ipv6cpRejectedRestartTimer = 3 * time.Second

	// noIPNCPTimeout is `ncp timeout 8` in
	// l2tp-lns-pppd-noip-rejects-ipcp/ze.conf, beside its `lcp restart-timer
	// 2000`. The bound exceeds several Restart times, so a Ze that ignored
	// pppd's Protocol-Reject would retransmit its IPCP Configure-Request
	// before the session ends: the revert of the dispatch can show its red.
	noIPNCPTimeout = 8 * time.Second

	// redistributeNCPTimeout and redistributeRestartTimer are `ncp timeout
	// 20` and `lcp restart-timer 10000` in
	// l2tp-lns-pppd-ipcp-terminate-redistribute/ze.conf. Ze holds IPCP in
	// Stopping for one Restart time after the Terminate-Request and answers
	// no Configure-Request there, so the reopen waits for pppd's next
	// Configure-Request, up to pppd's 3 s IPCP retransmission later
	// (DEFTIMEOUT, pppd/fsm.h): the bound exceeds both together, so the
	// reopen cancels it. The Restart time is the window in which FRR is
	// polled for the withdrawal.
	redistributeNCPTimeout   = 20 * time.Second
	redistributeRestartTimer = 10 * time.Second

	// frrWithdrawalPoll is how often the withdrawal window is polled in FRR.
	frrWithdrawalPoll = 250 * time.Millisecond
)

// checkIPv6CPRejectedByLNS runs stock pppd 2.5.5, which offers IPv6CP,
// against Ze's IPv4-only pool. Ze runs no IPv6CP and Protocol-Rejects
// pppd's; pppd parks IPv6CP and keeps the link, and IPv4 carries traffic past
// `ncp timeout`. Then the relay injects one CCP Configure-Request on the live
// session, and Ze answers with an LCP Protocol-Reject naming 0x80fd.
//
// pppd 2.5.5 source the checks rely on (read at the v2.5.5 tag the LAC image
// pins). IPv6CP is on by default: ipv6cp.c initializes ipv6cp_protent with
// enabled_flag 1 (the `1,` before "IPV6CP"). On Ze's Protocol-Reject,
// lcp.c lcp_rprotrej upcalls the enabled protocol:
//
//	if (protp->protocol == prot && protp->enabled_flag) {
//	    ...
//	    (*protp->protrej)(f->unit);
//
// and ipv6cp.c ipv6cp_protrej is `fsm_lowerdown(&ipv6cp_fsm[unit]);`, whose
// REQSENT, ACKRCVD and ACKSENT arms (fsm.c) set `f->state = STARTING;` and
// `UNTIMEOUT(fsm_timeout, f);`: no retransmission follows, and nothing calls
// np_finished, so `lcp_close(0, "No network protocols running")` never runs.
//
// Goal evidence for the home-line case: it passes before and after the NCP
// lifetime work, and owes no revert walk.
func checkIPv6CPRejectedByLNS(ctx context.Context, lab labOperations, _ time.Duration) error {
	const name = "l2tp-lns-pppd-ipv6cp-rejected-by-lns"
	fail := func(assertion int, cause error) error {
		return fmt.Errorf("scenario %s assertion %d: %w", name, assertion, cause)
	}

	// Assertion 1. IPCP opens and IPv4 crosses the session.
	if err := waitPeerLog(ctx, lab, peerZe, logSessionIPLine, 60*time.Second); err != nil {
		return fail(1, err)
	}
	upAt := time.Now()
	zeLink, lacLink, err := sessionLinks(ctx, lab)
	if err != nil {
		return fail(1, err)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(1, err)
	}

	// Assertion 2. pppd offered IPv6CP and Ze Protocol-Rejected it, sending
	// no IPv6CP of its own.
	if err := waitCapture(ctx, lab, "ze's LCP Protocol-Reject of pppd's IPv6CP", 30*time.Second,
		func(frames []pppFrame) error {
			_, rejectErr := ipv6cpRejected(frames)
			return rejectErr
		}); err != nil {
		return fail(2, err)
	}

	// Assertion 3. Past `ncp timeout` and one Restart time beyond it, the
	// session still carries IPv4 on the same interfaces, pppd sent no
	// IPv6CP after Ze's Protocol-Reject, and nobody ended LCP or the call.
	if err := sleepContext(ctx, time.Until(upAt.Add(ipv6cpRejectedNCPTimeout+ipv6cpRejectedRestartTimer))); err != nil {
		return fail(3, err)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(3, err)
	}
	if err := requireSameLinks(ctx, lab, zeLink, lacLink); err != nil {
		return fail(3, err)
	}
	text, frames, err := readSessionCapture(ctx, lab)
	if err != nil {
		return fail(3, err)
	}
	if err := requireIPv6CPParked(frames); err != nil {
		return fail(3, interoplab.WithSearchedOutput(err, text))
	}
	if err := requireSessionNotEnded(text, frames); err != nil {
		return fail(3, err)
	}

	// Assertion 4. RFC 1661 Section 5.7: a CCP Configure-Request the relay
	// injects on the live session draws an LCP Protocol-Reject naming 0x80fd
	// that carries the injected packet, and the session stays up.
	if _, err := lab.Exec(ctx, peerAVPRelay, []string{commandTouch, ccpMarker}, nil); err != nil {
		return fail(4, err)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(4, err)
	}
	if err := waitCapture(ctx, lab, "the injected CCP Configure-Request and ze's Protocol-Reject naming 0x80fd",
		30*time.Second, ccpProtocolRejected); err != nil {
		return fail(4, err)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(4, err)
	}
	if err := requireSameLinks(ctx, lab, zeLink, lacLink); err != nil {
		return fail(4, err)
	}
	text, frames, err = readSessionCapture(ctx, lab)
	if err != nil {
		return fail(4, err)
	}
	if err := requireSessionNotEnded(text, frames); err != nil {
		return fail(4, err)
	}
	// Retain the wire evidence in the native job log before Suite removes the
	// relay container, including on a successful run.
	var out textbuf.Buffer
	return out.Str("L2TP-PPP-IPV6CP-REJECTED-WIRE pppd's IPv6CP Protocol-Rejected and parked, ").
		Str("IPv4 kept past ncp timeout, injected CCP Protocol-Rejected\n").Str(text).Byte('\n').StdOut()
}

// checkNoIPRejectsIPCP runs pppd 2.5.5 with `noip` against Ze's IPv4-only
// pool. pppd Protocol-Rejects Ze's IPCP, Ze stops sending IPCP, Ze
// Protocol-Rejects pppd's IPv6CP, no NCP opens, Ze emits no session-up, and
// at the no-NCP bound Ze ends the call with a CDN.
//
// pppd 2.5.5 source the checks rely on. ipcp.c binds the option to the
// protocol's enabled flag:
//
//	{ "noip", o_bool, &ipcp_protent.enabled_flag,
//	  "Disable IP and IPCP" },
//
// so main.c get_input matches no protent for Ze's IPCP
// (`if (protp->protocol == protocol && protp->enabled_flag)`) and falls
// through to `lcp_sprotrej(0, p - PPP_HDRLEN, len + PPP_HDRLEN);`, which
// carries Ze's Configure-Request as the Rejected-Information. auth.c
// continue_networks still opens IPv6CP, the one enabled NCP, and counts it in
// `++num_np_open;`, so `if (num_np_open == 0)` does not close the link; Ze's
// Protocol-Reject of that IPv6CP runs ipv6cp_protrej, `fsm_lowerdown`, which
// calls no np_finished, so pppd holds LCP until Ze ends the call.
//
// The revert walk removes Ze's Protocol-Reject dispatch (receiveProtocolReject,
// internal/component/l2tp/ppp/session_run.go): assertion 3 then goes red with
// Ze's IPCP Configure-Requests after pppd's Protocol-Reject.
func checkNoIPRejectsIPCP(ctx context.Context, lab labOperations, _ time.Duration) error {
	const name = "l2tp-lns-pppd-noip-rejects-ipcp"
	fail := func(assertion int, cause error) error {
		return fmt.Errorf("scenario %s assertion %d: %w", name, assertion, cause)
	}

	// Assertion 1. pppd answers Ze's IPCP Configure-Request with an LCP
	// Protocol-Reject naming 0x8021.
	if err := waitCapture(ctx, lab, "pppd's LCP Protocol-Reject of ze's IPCP", 60*time.Second,
		func(frames []pppFrame) error {
			_, rejectErr := ipcpRejectedByPeer(frames)
			return rejectErr
		}); err != nil {
		return fail(1, err)
	}

	// Assertion 2. With no NCP Opened, the no-NCP bound ends the call: the
	// relay sees Ze's CDN, and Ze's log names the bound.
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: noIPNCPTimeout + 30*time.Second, Interval: time.Second, Description: "ze's CDN at the no-NCP bound",
	}, func(probeCtx context.Context) (int, error) {
		logs, readErr := lab.Logs(probeCtx, peerAVPRelay, pppCaptureLines)
		if readErr != nil {
			return 0, readErr
		}
		return zeCallDisconnects(logs.Text)
	}, func(count int) bool { return count > 0 }); err != nil {
		return fail(2, err)
	}
	var tb textbuf.Buffer
	expiry := tb.Str("ncp: timeout after ").Str(noIPNCPTimeout.String()).Str(logNoNCPBoundExpiry).String()
	if err := requirePeerLog(ctx, lab, peerZe, expiry); err != nil {
		return fail(2, err)
	}

	// Assertion 3. RFC 1661 Section 5.7: "Upon reception of a
	// Protocol-Reject, the implementation MUST stop sending packets of the
	// indicated protocol at the earliest opportunity." The capture runs from
	// the first data datagram to past Ze's CDN.
	text, frames, err := readSessionCapture(ctx, lab)
	if err != nil {
		return fail(3, err)
	}
	reject, err := ipcpRejectedByPeer(frames)
	if err != nil {
		return fail(3, err)
	}
	if err := requireNoPacketsFrom(frames, reject+1, relayFromZe, protocolIPCP,
		"pppd's LCP Protocol-Reject naming 0x8021"); err != nil {
		return fail(3, interoplab.WithSearchedOutput(err, text))
	}

	// Assertion 4. Ze Protocol-Rejected pppd's IPv6CP, never came up, and
	// ended the call with no LCP Terminate-Request: the NCP phase fails
	// through the negotiation timeout path.
	if _, err := ipv6cpRejected(frames); err != nil {
		return fail(4, interoplab.WithSearchedOutput(err, text))
	}
	if err := requireNoPacketsWithCode(frames, relayFromZe, protocolLCP, codeTerminateRequest); err != nil {
		return fail(4, interoplab.WithSearchedOutput(err, text))
	}
	if err := requirePeerLogAbsent(ctx, lab, peerZe, logSessionUpLine); err != nil {
		return fail(4, err)
	}
	var out textbuf.Buffer
	return out.Str("L2TP-PPP-NOIP-WIRE pppd Protocol-Rejected IPCP, ze sent no IPCP after it, ").
		Str("ze Protocol-Rejected IPv6CP, no session-up, CDN at the no-NCP bound\n").Str(text).Byte('\n').StdOut()
}

// checkIPCPTerminateRedistribute redistributes the subscriber /32 to FRR,
// then has the relay inject one IPCP Terminate-Request. Phase A: Ze answers
// with a Terminate-Ack carrying its Identifier, the IPv4 address leaves Ze's
// pppN and FRR loses the /32. Phase B: pppd renegotiates IPCP, the address
// and the /32 return, traffic crosses the session, and the no-NCP bound armed
// when IPCP left Opened was canceled, so no CDN follows.
//
// pppd 2.5.5 source phase B relies on: fsm.c fsm_rtermack, shared by every
// pppd NCP, answers a Terminate-Ack it did not ask for in Opened with a
// renegotiation:
//
//	case OPENED:
//	    if (f->callbacks->down)
//	        (*f->callbacks->down)(f);	/* Inform upper layers */
//	    fsm_sconfreq(f, 0);
//	    f->state = REQSENT;
//
// The revert walk removes the release event (publishNCPRelease,
// internal/component/l2tp/ppp/ncp.go): phase A's FRR assertion then goes red,
// the /32 staying in FRR although the address has left pppN.
func checkIPCPTerminateRedistribute(ctx context.Context, lab labOperations, timeout time.Duration) error {
	const name = "l2tp-lns-pppd-ipcp-terminate-redistribute"
	fail := func(assertion int, cause error) error {
		return fmt.Errorf("scenario %s assertion %d: %w", name, assertion, cause)
	}

	// Assertion 1. BGP is up, IPCP opens, traffic crosses the session and
	// FRR holds the redistributed /32.
	if err := waitFRRSession(ctx, lab, zeAddress, timeout); err != nil {
		return fail(1, err)
	}
	if err := waitPeerLog(ctx, lab, peerZe, logSessionIPLine, 60*time.Second); err != nil {
		return fail(1, err)
	}
	zeLink, lacLink, err := sessionLinks(ctx, lab)
	if err != nil {
		return fail(1, err)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(1, err)
	}
	if err := waitFRRRoute(ctx, lab, true); err != nil {
		return fail(1, err)
	}

	// Assertion 2, phase A. RFC 1661 Section 4.1, Opened RTR: tld, zrc, sta.
	// The address leaves pppN and the /32 leaves FRR, both polled inside the
	// one Restart time Ze holds IPCP in Stopping, never read after a sleep.
	if err := triggerIPCPTerminate(ctx, lab); err != nil {
		return fail(2, err)
	}
	triggeredAt := time.Now()
	if err := waitIPv4Withdrawn(ctx, lab, zeLink); err != nil {
		return fail(2, err)
	}
	window := time.Until(triggeredAt.Add(redistributeRestartTimer))
	if window <= 0 {
		return fail(2, errors.New("the IPv4 address took longer than one Restart time to leave pppN: "+
			"the IPCP Stopping window ended before FRR could be read"))
	}
	var tb textbuf.Buffer
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: window, Interval: frrWithdrawalPoll,
		Description: tb.Str("FRR without ").Str(peerPrefix).Str(" inside the IPCP Stopping window").String(),
	}, func(probeCtx context.Context) (bool, error) {
		return frrHasRoute(probeCtx, lab)
	}, func(held bool) bool { return !held }); err != nil {
		return fail(2, err)
	}
	if err := waitCapture(ctx, lab, "the injected IPCP Terminate-Request and ze's Terminate-Ack", 30*time.Second,
		zeTerminateAck); err != nil {
		return fail(2, err)
	}
	_, frames, err := readSessionCapture(ctx, lab)
	if err != nil {
		return fail(2, err)
	}
	if err := injectedTerminateAlone(frames); err != nil {
		return fail(2, err)
	}

	// Assertion 3, phase B. pppd renegotiates IPCP, Ze reinstalls the family
	// on the same pppN (RFC 1661 Section 3.6: "Each NCP MAY be Opened and
	// Closed at any time."), FRR holds the /32 again and traffic crosses.
	if err := waitCapture(ctx, lab, "an IPCP Configure-Ack from each side after ze's Terminate-Ack", 30*time.Second,
		ipcpReopened); err != nil {
		return fail(3, err)
	}
	if err := waitIPv4Reinstalled(ctx, lab, zeLink); err != nil {
		return fail(3, err)
	}
	if err := waitFRRRoute(ctx, lab, true); err != nil {
		return fail(3, err)
	}
	if err := requireSameLinks(ctx, lab, zeLink, lacLink); err != nil {
		return fail(3, err)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(3, err)
	}

	// Assertion 4. Past the no-NCP bound measured from IPCP leaving Opened,
	// plus one Restart time, the session, its interfaces, the /32 and the
	// BGP session all stay, and nobody ended LCP or the call: the reopen
	// canceled the bound.
	if err := sleepContext(ctx, time.Until(triggeredAt.Add(redistributeNCPTimeout+redistributeRestartTimer))); err != nil {
		return fail(4, err)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fail(4, err)
	}
	if err := requireSameLinks(ctx, lab, zeLink, lacLink); err != nil {
		return fail(4, err)
	}
	held, err := frrHasRoute(ctx, lab)
	if err != nil {
		return fail(4, err)
	}
	if !held {
		return fail(4, errors.New("FRR lost "+peerPrefix+" after the IPCP reopen"))
	}
	if err := requireFRRSession(ctx, lab, zeAddress); err != nil {
		return fail(4, err)
	}
	text, frames, err := readSessionCapture(ctx, lab)
	if err != nil {
		return fail(4, err)
	}
	if err := requireSessionNotEnded(text, frames); err != nil {
		return fail(4, err)
	}
	var out textbuf.Buffer
	return out.Str("L2TP-PPP-IPCP-TERMINATE-REDISTRIBUTE-WIRE IPCP Terminate-Ack, IPv4 and the FRR /32 withdrawn, ").
		Str("IPCP reopened, IPv4 and the /32 reinstalled, no CDN past ncp timeout\n").Str(text).Byte('\n').StdOut()
}

// sessionLinks answers the one PPP interface each peer holds once IPCP is up.
func sessionLinks(ctx context.Context, lab labOperations) (string, string, error) {
	zeLink, err := waitPPPInterface(ctx, lab, peerZe, 30*time.Second)
	if err != nil {
		return "", "", err
	}
	lacLink, err := waitPPPInterface(ctx, lab, peerLAC, 30*time.Second)
	if err != nil {
		return "", "", err
	}
	return zeLink, lacLink, nil
}

// requireSameLinks requires each peer to hold exactly the PPP interface it
// held when the session came up.
func requireSameLinks(ctx context.Context, lab labOperations, zeLink, lacLink string) error {
	for _, link := range []struct{ peer, before string }{{peerZe, zeLink}, {peerLAC, lacLink}} {
		links, err := pppLinks(ctx, lab, link.peer)
		if err != nil {
			return err
		}
		if len(links) != 1 {
			return errors.New(link.peer + " holds " + strconv.Itoa(len(links)) + " PPP interfaces, not one")
		}
		if links[0] != link.before {
			return errors.New(link.peer + " replaced its PPP interface " + link.before + " with " + links[0])
		}
	}
	return nil
}

// requirePeerLogAbsent requires peer's log to hold no needle. It reads as many
// lines as the relay capture does, not the 1000 a presence check reads,
// because an absence read from a tail would miss a line logged early under
// debug logging.
func requirePeerLogAbsent(ctx context.Context, lab labOperations, peer, needle string) error {
	result, err := lab.Logs(ctx, peer, pppCaptureLines)
	if err != nil {
		return err
	}
	if !result.Available {
		return errors.New(peer + " logs unavailable")
	}
	if strings.Contains(result.Text, needle) {
		return interoplab.WithSearchedOutput(fmt.Errorf("%s log holds %q", peer, needle), result.Text)
	}
	return nil
}

// waitCapture polls the relay's capture until match answers nil. A capture
// the checker cannot read counts as a failed probe, and a later
// readSessionCapture refuses it for good.
func waitCapture(ctx context.Context, lab labOperations, description string, timeout time.Duration,
	match func([]pppFrame) error) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: time.Second, Description: description,
	}, func(probeCtx context.Context) ([]pppFrame, error) {
		logs, readErr := lab.Logs(probeCtx, peerAVPRelay, pppCaptureLines)
		if readErr != nil {
			return nil, readErr
		}
		return sessionPPPFrames(logs.Text)
	}, func(frames []pppFrame) bool { return match(frames) == nil })
	return err
}

// readSessionCapture reads the relay's whole capture once.
func readSessionCapture(ctx context.Context, lab labOperations) (string, []pppFrame, error) {
	logs, err := lab.Logs(ctx, peerAVPRelay, pppCaptureLines)
	if err != nil {
		return "", nil, err
	}
	frames, err := sessionPPPFrames(logs.Text)
	if err != nil {
		return "", nil, err
	}
	return logs.Text, frames, nil
}

// sessionPPPFrames decodes every PPP frame of the relay's capture, IP
// included, in the order the relay logged them. The relay numbers the normal
// data datagrams of both directions from one (avprelay/pppframing.go,
// observe), and this refuses a gap or a missing prefix, so a truncated log
// cannot hide a frame an absence assertion reads. It skips the L2TP control
// messages, takes the relay's own injections as they come, and refuses a row
// it cannot read.
func sessionPPPFrames(text string) ([]pppFrame, error) {
	if len(text) > 16<<20 {
		return nil, errors.New("PPP capture exceeds 16 MiB")
	}
	frames := make([]pppFrame, 0, 64)
	ordinal := 0
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event struct {
			avpWireObservation
			DataOrdinal int `json:"data_ordinal"`
		}
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
		switch event.Direction {
		case relayFromZe, relayFromLAC:
			ordinal++
			if event.DataOrdinal != ordinal {
				return nil, fmt.Errorf("PPP capture has data ordinal %d, want %d from the session's first datagram",
					event.DataOrdinal, ordinal)
			}
		case relayInjected:
		default:
			return nil, errors.New("PPP capture row has the unknown direction " + strconv.Quote(event.Direction))
		}
		protocol, packet, err := pppInformation(datagram)
		if err != nil {
			return nil, fmt.Errorf("%s datagram: %w", event.Direction, err)
		}
		frames = append(frames, pppFrame{direction: event.Direction, protocol: protocol, packet: packet})
	}
	if ordinal == 0 {
		return nil, errors.New("PPP capture holds no normal data datagram")
	}
	return frames, nil
}

// zeCallDisconnects counts the L2TP Call-Disconnect-Notify messages Ze sent
// through the relay. A ZLB acknowledgement carries no Message Type and is not
// counted; a control message the checker cannot read is refused.
func zeCallDisconnects(text string) (int, error) {
	count := 0
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event avpWireObservation
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return 0, fmt.Errorf("decode relay observation: %w", err)
		}
		if event.Direction != relayFromZe {
			continue
		}
		datagram, err := hex.DecodeString(event.Forwarded)
		if err != nil {
			return 0, fmt.Errorf("decode relayed datagram: %w", err)
		}
		if len(datagram) < 2 {
			return 0, errors.New("relayed datagram shorter than its L2TP flags")
		}
		if datagram[0]&0x80 == 0 {
			continue
		}
		found, carried, err := controlMessageType(datagram)
		if err != nil {
			return 0, err
		}
		if !carried {
			continue
		}
		if found == cdnMessageType {
			count++
		}
	}
	return count, nil
}

// controlMessageType reads the Message Type of an L2TP control message.
//
// RFC 2661 Section 3.1 control header, then the first AVP (Section 4.4.1):
//
//	octet 0-1   flags 0xc802 (T, L, S set, version 2)
//	octet 2-3   Length
//	octet 4-7   Tunnel ID, Session ID
//	octet 8-11  Ns, Nr
//	octet 12-17 Message Type AVP header: M bit and Length 8, Vendor 0, Attribute 0
//	octet 18-19 Message Type
//
// A 12-octet message is a ZLB acknowledgement, which carries no AVP, so
// carried is false for it.
func controlMessageType(packet []byte) (messageType uint16, carried bool, err error) {
	if len(packet) < 12 {
		return 0, false, errors.New("short L2TP control message")
	}
	if binary.BigEndian.Uint16(packet) != 0xc802 {
		return 0, false, errors.New("L2TP control message with unexpected header flags " + hex.EncodeToString(packet[:2]))
	}
	if len(packet) == 12 {
		return 0, false, nil
	}
	if len(packet) < 20 {
		return 0, false, errors.New("L2TP control message too short for its Message Type AVP")
	}
	if !bytes.Equal(packet[12:18], []byte{0x80, 8, 0, 0, 0, 0}) {
		return 0, false, errors.New("L2TP control message does not open with its Message Type AVP")
	}
	return binary.BigEndian.Uint16(packet[18:20]), true, nil
}

// protocolRejectNaming answers the Rejected-Information of frame when it is
// an LCP Protocol-Reject naming protocol.
//
// RFC 1661 Section 5.7: Code 8 (1), Identifier (1), Length (2),
// Rejected-Protocol (2), Rejected-Information.
func protocolRejectNaming(frame pppFrame, protocol uint16) ([]byte, bool) {
	if frame.protocol != protocolLCP {
		return nil, false
	}
	if len(frame.packet) < 6 {
		return nil, false
	}
	if frame.packet[0] != codeProtocolReject {
		return nil, false
	}
	if binary.BigEndian.Uint16(frame.packet[4:6]) != protocol {
		return nil, false
	}
	return frame.packet[6:], true
}

// firstProtocolReject answers the index of the first LCP Protocol-Reject sent
// in direction naming protocol whose Rejected-Information is a
// Configure-Request, or -1 when the capture holds none.
func firstProtocolReject(frames []pppFrame, direction string, protocol uint16) int {
	for index, frame := range frames {
		if frame.direction != direction {
			continue
		}
		information, named := protocolRejectNaming(frame, protocol)
		if !named {
			continue
		}
		if len(information) == 0 {
			continue
		}
		if information[0] == codeConfigureRequest {
			return index
		}
	}
	return -1
}

// ipv6cpRejected requires pppd to have sent an IPv6CP Configure-Request, Ze
// to have answered one with an LCP Protocol-Reject naming 0x8057, and Ze to
// have sent no IPv6CP packet of its own. It answers the index of Ze's first
// such Protocol-Reject.
func ipv6cpRejected(frames []pppFrame) (int, error) {
	offered := false
	for _, frame := range frames {
		if frame.protocol != protocolIPv6CP {
			continue
		}
		if frame.direction == relayFromZe {
			return -1, errors.New("ze sent an IPv6CP packet: its IPv6CP ran on an IPv4-only pool")
		}
		if frame.direction != relayFromLAC {
			continue
		}
		if len(frame.packet) > 0 && frame.packet[0] == codeConfigureRequest {
			offered = true
		}
	}
	if !offered {
		return -1, errors.New("pppd sent no IPv6CP Configure-Request: the LAC's pppd does not offer IPv6CP")
	}
	reject := firstProtocolReject(frames, relayFromZe, protocolIPv6CP)
	if reject < 0 {
		return -1, errors.New("ze sent no LCP Protocol-Reject naming 0x8057 for pppd's IPv6CP Configure-Request")
	}
	return reject, nil
}

// requireIPv6CPParked requires pppd to have sent no IPv6CP packet after Ze's
// first Protocol-Reject naming 0x8057 (pppd ipv6cp_protrej, fsm_lowerdown).
func requireIPv6CPParked(frames []pppFrame) error {
	reject, err := ipv6cpRejected(frames)
	if err != nil {
		return err
	}
	return requireNoPacketsFrom(frames, reject+1, relayFromLAC, protocolIPv6CP,
		"ze's LCP Protocol-Reject naming 0x8057")
}

// ipcpRejectedByPeer requires pppd to have answered one of Ze's IPCP
// Configure-Requests with an LCP Protocol-Reject naming 0x8021, and Ze to have
// sent an IPCP Configure-Request before it. It answers the index of pppd's
// first such Protocol-Reject.
func ipcpRejectedByPeer(frames []pppFrame) (int, error) {
	reject := firstProtocolReject(frames, relayFromLAC, protocolIPCP)
	if reject < 0 {
		return -1, errors.New("pppd sent no LCP Protocol-Reject naming 0x8021 for ze's IPCP Configure-Request")
	}
	for _, frame := range frames[:reject] {
		if frame.direction != relayFromZe {
			continue
		}
		if frame.protocol != protocolIPCP {
			continue
		}
		if len(frame.packet) > 0 && frame.packet[0] == codeConfigureRequest {
			return reject, nil
		}
	}
	return -1, errors.New("pppd's Protocol-Reject naming 0x8021 precedes every IPCP Configure-Request from ze")
}

// requireNoPacketsFrom requires no frame from index from on to have been sent
// in direction on protocol. after names the frame the absence is measured
// from, for the error.
func requireNoPacketsFrom(frames []pppFrame, from int, direction string, protocol uint16, after string) error {
	sent := 0
	for _, frame := range frames[from:] {
		if frame.direction != direction {
			continue
		}
		if frame.protocol == protocol {
			sent++
		}
	}
	if sent == 0 {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str(direction).Str(" sent ").Str(strconv.Itoa(sent)).Str(" packets of protocol 0x").
		Hex([]byte{byte(protocol >> 8), byte(protocol)}).Str(" after ").Str(after).String())
}

// requireNoPacketsWithCode requires no frame sent in direction on protocol to
// carry code.
func requireNoPacketsWithCode(frames []pppFrame, direction string, protocol uint16, code byte) error {
	for _, frame := range frames {
		if frame.direction != direction {
			continue
		}
		if frame.protocol != protocol {
			continue
		}
		if len(frame.packet) > 0 && frame.packet[0] == code {
			var tb textbuf.Buffer
			return errors.New(tb.Str(direction).Str(" sent a packet of protocol 0x").
				Hex([]byte{byte(protocol >> 8), byte(protocol)}).Str(" with Code ").Str(strconv.Itoa(int(code))).String())
		}
	}
	return nil
}

// requireSessionNotEnded requires the capture to hold no CDN from Ze and no
// LCP Terminate-Request from either side.
func requireSessionNotEnded(text string, frames []pppFrame) error {
	disconnects, err := zeCallDisconnects(text)
	if err != nil {
		return err
	}
	if disconnects != 0 {
		return interoplab.WithSearchedOutput(errors.New("ze sent a CDN: the session ended"), text)
	}
	for _, direction := range []string{relayFromZe, relayFromLAC} {
		if err := requireNoPacketsWithCode(frames, direction, protocolLCP, codeTerminateRequest); err != nil {
			return interoplab.WithSearchedOutput(err, text)
		}
	}
	return nil
}

// ccpProtocolRejected requires the relay to have injected exactly one frame,
// the CCP Configure-Request with ccpIdentifier and no option, and Ze to have
// answered it after with an LCP Protocol-Reject naming 0x80fd whose
// Rejected-Information is that whole packet, while sending no CCP of its own.
func ccpProtocolRejected(frames []pppFrame) error {
	want := []byte{codeConfigureRequest, ccpIdentifier, 0, 4}
	injected := -1
	for index, frame := range frames {
		if frame.direction != relayInjected {
			continue
		}
		if injected >= 0 {
			return errors.New("the relay injected more than one frame")
		}
		if frame.protocol != protocolCCP {
			return errors.New("the relay's injected frame is not CCP")
		}
		if !bytes.Equal(frame.packet, want) {
			return errors.New("the relay's injected frame is not the CCP Configure-Request: " + hex.EncodeToString(frame.packet))
		}
		injected = index
	}
	if injected < 0 {
		return errors.New("the relay injected no CCP Configure-Request")
	}
	if err := requireNoPacketsFrom(frames, 0, relayFromZe, protocolCCP, "the session started"); err != nil {
		return err
	}
	for _, frame := range frames[injected+1:] {
		if frame.direction != relayFromZe {
			continue
		}
		information, named := protocolRejectNaming(frame, protocolCCP)
		if !named {
			continue
		}
		if !bytes.Equal(information, want) {
			continue
		}
		if int(binary.BigEndian.Uint16(frame.packet[2:4])) != len(frame.packet) {
			return errors.New("ze's Protocol-Reject naming 0x80fd has a Length that is not its own")
		}
		return nil
	}
	return errors.New("ze sent no LCP Protocol-Reject naming 0x80fd carrying the injected CCP Configure-Request")
}

// injectedTerminateAlone requires the relay to have injected exactly one
// frame, the IPCP Terminate-Request with terminateIdentifier and no Data.
func injectedTerminateAlone(frames []pppFrame) error {
	injected := 0
	for _, frame := range frames {
		if frame.direction != relayInjected {
			continue
		}
		injected++
		if frame.protocol != protocolIPCP {
			return errors.New("the relay injected a frame that is not IPCP")
		}
		if !bytes.Equal(frame.packet, []byte{codeTerminateRequest, terminateIdentifier, 0, 4}) {
			return errors.New("the relay's injected frame is not the IPCP Terminate-Request: " + hex.EncodeToString(frame.packet))
		}
	}
	if injected != 1 {
		return errors.New("the relay injected " + strconv.Itoa(injected) + " frames, not one IPCP Terminate-Request")
	}
	return nil
}
