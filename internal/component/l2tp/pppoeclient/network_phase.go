// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- network phase: keepalive and CHAP re-challenge
// Related: session.go -- negotiation that hands its frame channel to keepaliveLoop
//
// RFC: rfc/short/rfc1994.md

package pppoeclient

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// networkPhaseCHAP answers the authenticator's CHAP packets once the
// session has reached the network phase. It holds the credentials and the
// Identifier of the last Response written, so only the result for that
// Response is acted on. Not safe for concurrent use: negotiateIPCP owns it
// while IPCP negotiates, then hands it to keepaliveLoop.
type networkPhaseCHAP struct {
	cfg     sessionConfig
	id      uint8
	pending bool
}

// networkPhaseCHAPFor returns the CHAP state keepaliveLoop needs, or nil
// when LCP negotiated an authentication protocol other than CHAP.
func networkPhaseCHAPFor(authProto uint16, cfg sessionConfig) *networkPhaseCHAP {
	if authProto != ppp.ProtoCHAP {
		return nil
	}
	return &networkPhaseCHAP{cfg: cfg}
}

// handle answers one CHAP packet received in the network phase. It returns
// an error when the session must end: a Response that could not be
// written, or a Failure for the Response last written. A Success for that
// Response clears it, and a result for any other Identifier is discarded.
func (c *networkPhaseCHAP) handle(w io.Writer, buf []byte, pkt ppp.LCPPacket, logger *slog.Logger) error {
	switch pkt.Code {
	case 1: // Challenge
		resp := buildCHAPResponse(pkt, c.cfg)
		if resp == nil {
			// A malformed Challenge is discarded, as runClientAuth does.
			return nil
		}
		// RFC 1994 Section 4.1: "Whenever a Challenge packet is received,
		// the peer MUST transmit a CHAP packet with the Code field set to 2
		// (Response)."
		off := ppp.WriteFrame(buf, 0, ppp.ProtoCHAP, resp)
		if err := writeClientPacket(w, buf[:off]); err != nil {
			return fmt.Errorf("pppoe-client: CHAP re-challenge Response: %w", err)
		}
		c.id = pkt.Identifier
		c.pending = true
		logger.Info("pppoe-client: CHAP re-challenge answered", "id", pkt.Identifier)
	case 3: // Success
		if !c.pending {
			return nil
		}
		if pkt.Identifier != c.id {
			return nil
		}
		c.pending = false
		logger.Info("pppoe-client: CHAP re-authentication succeeded")
	case 4: // Failure
		if !c.pending {
			return nil
		}
		if pkt.Identifier != c.id {
			return nil
		}
		// RFC 1994 Section 4.2: the authenticator "MUST transmit a CHAP
		// packet with the Code field set to 4 (Failure), and SHOULD take
		// action to terminate the link." The client stops using a link that
		// failed re-authentication rather than wait for that termination.
		return errors.New("pppoe-client: CHAP re-authentication failed")
	}
	return nil
}

// keepaliveLoop owns the constructed connected session until it closes done.
// It MUST stop echo timing before restart and end the session on restart error.
// The supervisor MUST close the transport after it returns. Existing readers
// serve every phase, so restart never creates another reader on either fd.
//
// current.chap is nil when LCP did not negotiate CHAP. That nil is a guard: a
// CHAP frame is then dropped like any other non-LCP frame, because no
// Challenge is expected on a link that authenticated with PAP or not at all.
func keepaliveLoop(session *connectedSession, done chan<- struct{}, stopCh <-chan struct{}, logger *slog.Logger) {
	defer close(done)
	if session == nil {
		logger.Error("pppoe-client: missing connected session; restart the PPPoE client")
		return
	}
	if session.transport == nil {
		logger.Error("pppoe-client: uninitialized connected session; restart the PPPoE client")
		return
	}

	echoTicker := time.NewTicker(echoInterval)
	defer echoTicker.Stop()

	var (
		echoID    uint8
		echoFails int
		frameBuf  [ppp.MaxFrameBufLen]byte
	)

	for {
		select {
		case <-stopCh:
			return
		case <-echoTicker.C:
			echoID++
			off := ppp.WriteFrame(frameBuf[:], 0, ppp.ProtoLCP, nil)
			off += ppp.WriteLCPEcho(frameBuf[:], off, ppp.LCPEchoRequest, echoID, session.current.magic, nil)
			if err := writeClientPacket(session.transport, frameBuf[:off]); err != nil {
				logger.Warn("pppoe-client: echo write failed", "error", err)
				return
			}
			echoFails++
			if echoFails >= echoMaxFailures {
				logger.Warn("pppoe-client: echo timeout", "failures", echoFails)
				return
			}
		case frame, ok := <-session.reader.next():
			session.reader.received()
			if !ok {
				return
			}
			if frame.err != nil {
				session.warnReceiveFailure(frame.err, stopCh, logger)
				return
			}
			proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
			if parseErr != nil {
				continue
			}
			if proto != ppp.ProtoLCP {
				if proto != ppp.ProtoCHAP {
					if proto != ppp.ProtoIPCP {
						continue
					}
				}
			}
			pkt, pktErr := ppp.ParseLCPPacket(payload)
			if pktErr != nil {
				continue
			}
			if proto == ppp.ProtoIPCP {
				// RFC 1332 Section 2: connected IPCP uses the same ordered source.
				if !answerOpenedIPCP(session.transport, frameBuf[:], pkt, &session.ipcpCodeRejectID, session.current.peerMRU, logger) {
					return
				}
				continue
			}
			if proto == ppp.ProtoCHAP {
				if session.current.chap == nil {
					continue
				}
				// RFC 1994 Section 4.1 (re-challenge) and Section 4.2 (result).
				if err := session.current.chap.handle(session.transport, frameBuf[:], pkt, logger); err != nil {
					logger.Warn("pppoe-client: network-phase CHAP ended the session", "error", err)
					return
				}
				continue
			}
			switch pkt.Code {
			case ppp.LCPConfigureRequest:
				// RFC 1661 Sections 3.4 and 6: discard before phase reentry.
				reentry, admitted := requestLCPReentry(pkt)
				if !admitted {
					continue
				}
				echoTicker.Stop()
				// RFC 1661 Sections 3.4-3.6: DROP precedes scr and sca/scn.
				if err := session.restart(reentry, stopCh, logger); err != nil {
					logger.Warn("pppoe-client: LCP restart failed; the client will reconnect", "error", err)
					return
				}
				// RFC 1661 Section 5.8: "On transmission, the Identifier field MUST
				// be changed whenever the content of the Data field changes, and
				// whenever a valid reply has been received for a previous request."
				// Retain the sequence across restart, including an acknowledged Echo.
				echoFails = 0
				echoTicker.Reset(echoInterval)
			case ppp.LCPEchoReply:
				echoFails = 0
			case ppp.LCPEchoRequest:
				sendEchoReply(session.transport, frameBuf[:], pkt, session.current.magic)
			case ppp.LCPTerminateRequest:
				sendTerminateAck(session.transport, frameBuf[:], pkt)
				logger.Info("pppoe-client: server sent LCP Terminate-Request")
				return
			}
		}
	}
}

// warnReceiveFailure records active-session failures, including EOF from a zero
// read. Close publishes stop before a shutdown read can return. The separate
// boundary also lets tests check stop after delivery without a scheduler race.
func (session *connectedSession) warnReceiveFailure(err error, stopCh <-chan struct{}, logger *slog.Logger) {
	select {
	case <-stopCh:
		return
	default:
		logger.Warn("pppoe-client: cannot receive session packets; the client will reconnect",
			"name", session.cfg.name, "error", err)
	}
}

// answerOpenedIPCP answers one IPCP packet received once IPCP is Opened, and
// reports whether the session goes on. Only an LCP request restarts the phase
// owner. An independent IPCP packet that takes IPCP out of Opened
// (RFC 1661 Section 4.1: RCR, RCA, RCN, RTR,
// RTA and RXJ- all leave Opened through tld) ends the session, and the dialer
// brings up a new one.
func answerOpenedIPCP(w io.Writer, buf []byte, pkt ppp.LCPPacket, codeRejectID *uint8, peerMRU uint16,
	logger *slog.Logger) bool {
	// RFC 1332 Section 2: "Other Codes should be treated as unrecognized and
	// should result in Code-Rejects."
	if ipcpCodeUnrecognized(pkt.Code) {
		// RFC 1661 Section 5.6: "The Identifier field MUST be changed for
		// each Code-Reject sent."
		*codeRejectID++
		if err := sendIPCPCodeReject(w, buf, pkt, *codeRejectID, peerMRU); err != nil {
			logger.Warn("pppoe-client: IPCP Code-Reject write failed", "error", err)
			return false
		}
		return true
	}
	if pkt.Code == ppp.LCPTerminateRequest {
		// RFC 1661 Section 5.5: "Upon reception of a Terminate-Request, a
		// Terminate-Ack MUST be transmitted."
		off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, nil)
		off += ppp.WriteLCPPacket(buf, off, ppp.LCPTerminateAck, pkt.Identifier, nil)
		if err := writeClientPacket(w, buf[:off]); err != nil {
			logger.Warn("pppoe-client: IPCP Terminate-Ack write failed", "error", err)
		}
		logger.Info("pppoe-client: server sent IPCP Terminate-Request")
		return false
	}
	logger.Info("pppoe-client: IPCP left Opened, ending the session", "code", ppp.LCPCodeName(pkt.Code))
	return false
}

// ipcpCodeUnrecognized reports whether code is none of the seven IPCP uses.
// RFC 1332 Section 2: "Only Codes 1 through 7 (Configure-Request,
// Configure-Ack, Configure-Nak, Configure-Reject, Terminate-Request,
// Terminate-Ack and Code-Reject) are used." Code 0 is outside that range as
// much as Code 8 is.
func ipcpCodeUnrecognized(code uint8) bool {
	if code == 0 {
		return true
	}
	return code > ppp.LCPCodeReject
}
