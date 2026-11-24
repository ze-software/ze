// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPCP terminates independently of IPv6CP
// Related: network_phase.go -- keepaliveLoop hands every connected IPCP packet here
// Related: session.go -- runIPCP, the IPCP negotiation a renegotiation reuses
// Related: ipv6cp_client.go -- the IPv6CP automaton whose state keeps the link up
//
// RFC: rfc/short/rfc7084.md, rfc/short/rfc1661.md

package pppoeclient

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// defaultNoNCPIdleTimeout bounds how long LCP stays Opened while no NCP is
// Opened (RFC 1661 Section 3.7). Matches ncpTimeout and ppp's defaultIPTimeout.
const defaultNoNCPIdleTimeout = 30 * time.Second

// errNoNCPIdleExpired reports that the no-NCP idle-period timer expired before
// any NCP reached Opened, so the worker closed LCP and ends the session.
var errNoNCPIdleExpired = errors.New("pppoe-client: no NCP opened within idle period")

// ipcpRun says which IPCP negotiation runIPCP performs. Its zero value, or
// one naming only ipv6, is the negotiation that enters the network phase:
// IPv6CP starts beside IPCP and its events wait for the caller.
// applyIPv6 is set only by a connected session renegotiating IPCP alone
// (connectedSession.renegotiateIPCP): IPv6CP is already running, so it is not
// started again, and each event it produces is applied at once, because IPv6
// may be passing while IPCP renegotiates (RFC 7084 Section 4.2 WLL-3).
// queued is a peer IPCP Configure-Request already read, which started the
// renegotiation; runIPCP answers it after its own request.
// lcpIdentifier points to the session's LCP Identifier sequence so Protocol-
// Rejects emitted during IPCP advance the same sequence.
// noNCPIdle returns the connected session's idle-period expiry channel when
// renegotiating on an established link; nil during initial negotiation.
type ipcpRun struct {
	ipv6          *clientIPv6CP
	applyIPv6     func(ipv6cpEvent) error
	queued        *ppp.LCPPacket
	lcpIdentifier *uint8
	noNCPIdle     func() <-chan time.Time
}

// idleChannel answers the current no-NCP idle expiry channel, or nil when no
// idle timer applies to this run.
func (run ipcpRun) idleChannel() <-chan time.Time {
	// refuse-by-default: guards runIPCP -- initial negotiation has no connected idle timer and uses ncpTimeout
	if run.noNCPIdle == nil {
		return nil
	}
	return run.noNCPIdle()
}

// apply carries one IPv6CP event to the unit during a connected
// renegotiation of IPCP.
func (run ipcpRun) apply(event ipv6cpEvent) error {
	// refuse-by-default: guards negotiatePhases -- the initial negotiation applies IPv6CP's final state itself
	if run.applyIPv6 == nil {
		return nil
	}
	return run.applyIPv6(event)
}

// ipcpStoppedError is an IPCP negotiation that ended while LCP stays up: the
// peer terminated IPCP, refused the address option, or never completed IPCP.
// The initial negotiation ends the session on it; a connected session keeps
// the link while IPv6CP still runs (connectedSession.renegotiateIPCP).
type ipcpStoppedError struct{ reason string }

func (e ipcpStoppedError) Error() string { return e.reason }

// answerIPCPRequest answers one peer IPCP Configure-Request during IPCP
// negotiation and moves state, the negotiation's RFC 1661 automaton state.
// A request whose options do not parse is discarded.
func answerIPCPRequest(w io.Writer, buf []byte, pkt ppp.LCPPacket, result *ipcpResult, state *ppp.LCPState) error {
	options, err := ppp.ParseIPCPOptions(pkt.Data)
	if err != nil {
		return nil //nolint:nilerr // RFC 1661 Section 5: a malformed request is silently discarded.
	}
	// This client negotiates IP-Address only. Preserve unsupported
	// options byte-for-byte in a Reject (RFC 1661 Section 5.4).
	rejected := 0
	for data := pkt.Data; len(data) != 0; {
		length := int(data[1]) // ParseIPCPOptions checked each boundary.
		if data[0] != ppp.IPCPOptIPAddress {
			rejected += copy(buf[6+rejected:], data[:length])
		}
		data = data[length:]
	}
	if rejected != 0 {
		off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, nil)
		off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureReject, pkt.Identifier, buf[6:6+rejected])
		if err := writeClientPacket(w, buf[:off]); err != nil {
			return err
		}
		if *state == ppp.LCPStateAckSent {
			*state = ppp.LCPStateReqSent
		}
		return nil
	}
	result.peerIP = options.IPAddress
	off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, nil)
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data)
	if err := writeClientPacket(w, buf[:off]); err != nil {
		return err
	}
	if *state == ppp.LCPStateAckRcvd {
		*state = ppp.LCPStateOpened
	} else {
		*state = ppp.LCPStateAckSent
	}
	return nil
}

// handleIPCP answers one IPCP packet received in the network phase once IPCP
// has first opened. An error ends the session; an *lcpReentry error asks the
// caller to return to Link Establishment (a renegotiation met an LCP
// Configure-Request).
//
// RFC 7084 Section 4.2 WLL-3: "If the WAN interface supports PPP
// encapsulation, in a dual-stack environment with IPCP and IPV6CP running
// over one PPP logical channel, the Network Control Protocols (NCPs) MUST be
// treated as independent of each other and start and terminate
// independently."
func (s *connectedSession) handleIPCP(buf []byte, pkt ppp.LCPPacket, stop <-chan struct{}, logger *slog.Logger) error {
	// RFC 1332 Section 2: "Other Codes should be treated as unrecognized and
	// should result in Code-Rejects."
	if ipcpCodeUnrecognized(pkt.Code) {
		// RFC 1661 Section 5.6: "The Identifier field MUST be changed for
		// each Code-Reject sent."
		s.ipcpCodeRejectID++
		if err := sendNCPCodeReject(s.transport, buf, ppp.ProtoIPCP, pkt, s.ipcpCodeRejectID, s.current.peerMRU); err != nil {
			return fmt.Errorf("pppoe-client: IPCP Code-Reject write failed: %w", err)
		}
		return nil
	}
	if s.ipcpStopped {
		return s.answerStoppedIPCP(buf, pkt, stop, logger)
	}
	return s.leaveOpenedIPCP(buf, pkt, stop, logger)
}

// leaveOpenedIPCP takes IPCP out of Opened for a recognized code. Every
// recognized code leaves Opened (RFC 1661 Section 4.1, the Opened column):
// RCR, RCA, RCN and RTA take tld then scr, and IPCP negotiates again; RTR
// takes tld, zrc, sta and RXJ- takes tld, irc, str, and IPCP then waits for
// the peer. tld discards and withdraws IPv4 only (ipcpDown); LCP and IPv6CP
// stay as they are.
func (s *connectedSession) leaveOpenedIPCP(buf []byte, pkt ppp.LCPPacket, stop <-chan struct{}, logger *slog.Logger) error {
	logger.Info("pppoe-client: IPCP left Opened; IPv4 stops while LCP and IPv6CP stay up",
		"code", ppp.LCPCodeName(pkt.Code))
	if err := s.ipcpDown(); err != nil {
		return err
	}
	switch pkt.Code {
	case ppp.LCPTerminateRequest:
		// RFC 1661 Section 5.5: "Upon reception of a Terminate-Request, a
		// Terminate-Ack MUST be transmitted."
		if err := s.sendIPCPTerminate(buf, ppp.LCPTerminateAck, pkt.Identifier); err != nil {
			return err
		}
		s.waitForIPCP()
		return nil
	case ppp.LCPCodeReject:
		// RXJ-: IPCP sends only the seven codes it needs, so a Code-Reject of
		// one of them is catastrophic and IPCP terminates (str).
		s.ipcpIdentifier++
		if err := s.sendIPCPTerminate(buf, ppp.LCPTerminateRequest, s.ipcpIdentifier); err != nil {
			return err
		}
		s.waitForIPCP()
		return nil
	case ppp.LCPConfigureRequest:
		s.publish()
		s.syncNoNCPIdle()
		return s.renegotiateIPCP(buf, &pkt, stop, logger)
	}
	// RCA, RCN (Configure-Nak or Configure-Reject) and RTA.
	s.publish()
	s.syncNoNCPIdle()
	return s.renegotiateIPCP(buf, nil, stop, logger)
}

// answerStoppedIPCP answers a recognized IPCP code while IPCP is Stopped
// (RFC 1661 Section 4.1, the Stopped column): RCR takes irc, scr, sca and
// IPCP negotiates again; RCA, RCN and RTR take sta; RTA and RXJ change
// nothing.
func (s *connectedSession) answerStoppedIPCP(buf []byte, pkt ppp.LCPPacket, stop <-chan struct{}, logger *slog.Logger) error {
	switch pkt.Code {
	case ppp.LCPConfigureRequest:
		return s.renegotiateIPCP(buf, &pkt, stop, logger)
	case ppp.LCPConfigureAck, ppp.LCPConfigureNak, ppp.LCPConfigureReject, ppp.LCPTerminateRequest:
		return s.sendIPCPTerminate(buf, ppp.LCPTerminateAck, pkt.Identifier)
	case ppp.LCPTerminateAck, ppp.LCPCodeReject:
		// RFC 1661 Section 4.1: RTA and RXJ leave Stopped as it is.
	}
	return nil
}

// sendIPCPTerminate writes an IPCP Terminate-Request or Terminate-Ack.
func (s *connectedSession) sendIPCPTerminate(buf []byte, code, identifier uint8) error {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, nil)
	off += ppp.WriteLCPPacket(buf, off, code, identifier, nil)
	if err := writeClientPacket(s.transport, buf[:off]); err != nil {
		return fmt.Errorf("pppoe-client: IPCP %s write failed: %w", ppp.LCPCodeName(code), err)
	}
	return nil
}

// ipcpDown is RFC 1661's tld for IPCP on the connected link: IPv4 is
// discarded on the unit, and the next publish withdraws the IPv4 addresses,
// the default route and the DNS servers IPCP learned.
func (s *connectedSession) ipcpDown() error {
	// RFC 1661 Section 3.6: "Any supported network-layer protocol packets
	// received when the corresponding NCP is not in the Opened state MUST be
	// silently discarded."
	if err := s.setIPv4Mode(pppkernel.NPModeDrop); err != nil {
		return fmt.Errorf("pppoe-client: cannot discard IPv4 after IPCP left Opened: %w", err)
	}
	s.ipcpStopped = true
	return nil
}

// waitForIPCP leaves IPCP Stopped, publishes the withdrawal of IPv4, and
// starts the no-NCP idle-period timer when no other NCP is Opened.
// RFC 1661 Section 3.7: closing the last Opened NCP does not terminate LCP.
func (s *connectedSession) waitForIPCP() {
	s.publish()
	s.syncNoNCPIdle()
}

// renegotiateIPCP runs IPCP again on the open link while LCP, and IPv6CP
// when it runs, stay up. IPv4 is already discarded (ipcpDown). Success
// cancels the no-NCP idle timer, publishes the new addresses, and passes IPv4
// again once the interface consumer has applied them (awaitApplied). An
// IPCP-only failure leaves IPCP Stopped and LCP up under the no-NCP idle timer.
func (s *connectedSession) renegotiateIPCP(buf []byte, queued *ppp.LCPPacket, stop <-chan struct{}, logger *slog.Logger) error {
	run := ipcpRun{
		ipv6:          s.ipv6,
		queued:        queued,
		lcpIdentifier: &s.lcpIdentifier,
		noNCPIdle:     s.noNCPIdleChannel,
		applyIPv6: func(event ipv6cpEvent) error {
			return s.applyIPv6CP(event, logger)
		},
	}
	result, err := runIPCP(s.transport, s.reader, buf, s.current.magic, s.current.peerMRU, s.current.chap,
		&s.ipcpIdentifier, stop, logger, &s.ipcpCodeRejectID, run)
	if errors.Is(err, errNoNCPIdleExpired) {
		return s.closeIdleLCP(buf, logger)
	}
	if stopped, ok := errors.AsType[ipcpStoppedError](err); ok {
		logger.Warn("pppoe-client: IPCP stopped, so IPv4 stays down until the access concentrator opens IPCP again; check its IPv4 address pool",
			"reason", stopped.reason)
		s.syncNoNCPIdle()
		return nil
	}
	if err != nil {
		return err
	}
	s.current.localIP = result.localIP
	s.current.peerIP = result.peerIP
	s.current.primaryDNS = result.primaryDNS
	s.current.secondaryDNS = result.secondaryDNS
	s.ipcpStopped = false
	s.syncNoNCPIdle()
	logger.Info("pppoe-client: IPCP reopened", "local-ip", result.localIP, "peer-ip", result.peerIP)
	// IPv4 stays discarded until the interface carries the new addresses, so
	// no packet leaves with the replaced source (connectedSession.restart).
	if err := s.awaitApplied(s.publish(), stop); err != nil {
		return err
	}
	// RFC 1661 Section 3.6: "After a NCP has reached the Opened state, PPP
	// will carry the corresponding network-layer protocol packets."
	if err := s.setIPv4Mode(pppkernel.NPModePass); err != nil {
		return fmt.Errorf("pppoe-client: cannot pass IPv4 after IPCP reopened: %w", err)
	}
	return nil
}

// anyNCPOpened reports whether IPCP or IPv6CP is currently in the Opened state.
func (s *connectedSession) anyNCPOpened() bool {
	if !s.ipcpStopped {
		return true
	}
	if s.ipv6 != nil {
		return s.ipv6.opened()
	}
	return false
}

// idleTimeout returns the configured no-NCP idle duration, defaulting to
// defaultNoNCPIdleTimeout when unset.
func (s *connectedSession) idleTimeout() time.Duration {
	if s.noNCPIdleTimeout > 0 {
		return s.noNCPIdleTimeout
	}
	return defaultNoNCPIdleTimeout
}

// syncNoNCPIdle keeps the no-NCP idle-period timer aligned with whether any
// NCP is Opened. When no NCP is Opened, it arms the timer once without
// resetting an already-armed timer, so repeated NCP events cannot extend the
// idle period. When any NCP opens, it cancels the timer.
//
// RFC 1661 Section 3.7: "Conversely, the fact that one NCP has Closed is not
// sufficient reason to cause the termination of the PPP link, even if that
// NCP was the only NCP currently in the Opened state."
// RFC 1661 Section 3.7: "This might happen because of the loss of carrier,
// authentication failure, link quality failure, the expiration of an
// idle-period timer, or the administrative closing of the link."
func (s *connectedSession) syncNoNCPIdle() {
	if s.anyNCPOpened() {
		s.cancelNoNCPIdle()
		return
	}
	if s.noNCPIdleArmed {
		return
	}
	timeout := s.idleTimeout()
	if s.noNCPIdle == nil {
		s.noNCPIdle = time.NewTimer(timeout)
	} else {
		s.noNCPIdle.Reset(timeout)
	}
	s.noNCPIdleArmed = true
}

// cancelNoNCPIdle stops the no-NCP idle-period timer if running. Safe before
// the first arm and safe to call repeatedly.
func (s *connectedSession) cancelNoNCPIdle() {
	if s.noNCPIdle != nil {
		s.noNCPIdle.Stop()
	}
	s.noNCPIdleArmed = false
}

// noNCPIdleChannel returns the idle-period expiry channel while armed, or nil
// when disabled so select ignores it.
func (s *connectedSession) noNCPIdleChannel() <-chan time.Time {
	if !s.noNCPIdleArmed {
		return nil
	}
	if s.noNCPIdle == nil {
		return nil
	}
	return s.noNCPIdle.C
}

// closeIdleLCP terminates LCP when the no-NCP idle-period timer expires and
// returns the session-ending error.
//
// RFC 1661 Section 3.7: "PPP can terminate the link at any time.  This might
// happen because of the loss of carrier, authentication failure, link quality
// failure, the expiration of an idle-period timer, or the administrative
// closing of the link."
// RFC 1661 Section 3.7: "LCP is used to close the link through an exchange of
// Terminate packets."
func (s *connectedSession) closeIdleLCP(buf []byte, logger *slog.Logger) error {
	s.cancelNoNCPIdle()
	s.lcpIdentifier++
	if err := sendLCPTerminateRequest(s.transport, buf, s.lcpIdentifier); err != nil {
		reportLCPWriteFailure(logger, "no-ncp-idle", "pppoe-client: no-NCP idle timer expired and LCP Terminate-Request write failed", err)
		return err
	}
	logger.Info("pppoe-client: no NCP opened within idle period; closing LCP", "timeout", s.idleTimeout())
	return errNoNCPIdleExpired
}
