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

// keepaliveLoop handles LCP echo on the frames channel created during
// negotiation, and IPCP on the unit's unitFrames. Closes done when the
// session ends (echo timeout, terminate, read error, IPCP leaving Opened).
// Uses the existing reader goroutines to avoid a second concurrent Read on
// either fd.
//
// result.chap is nil when LCP did not negotiate CHAP. That nil is a guard: a
// CHAP frame is then dropped like any other non-LCP frame, because no
// Challenge is expected on a link that authenticated with PAP or not at all.
func keepaliveLoop(chanFile io.Writer, result *sessionResult, done chan<- struct{}, stopCh <-chan struct{}, logger *slog.Logger) {
	defer close(done)
	frames, magic, chap := result.frames, result.magic, result.chap
	ipcpCodeRejectID := result.ipcpCodeRejectID

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
			off += ppp.WriteLCPEcho(frameBuf[:], off, ppp.LCPEchoRequest, echoID, magic, nil)
			if _, err := chanFile.Write(frameBuf[:off]); err != nil {
				logger.Warn("pppoe-client: echo write failed", "error", err)
				return
			}
			echoFails++
			if echoFails >= echoMaxFailures {
				logger.Warn("pppoe-client: echo timeout", "failures", echoFails)
				return
			}
		case frame, ok := <-result.unitFrames:
			if !ok || frame.err != nil {
				// The unit's read ends when the transport closes, at Cleanup or
				// on loss, which ends the channel's read in the same moment, so
				// it ends the loop as quietly as the channel's does.
				return
			}
			proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
			if parseErr != nil {
				continue
			}
			if proto != ppp.ProtoIPCP {
				continue
			}
			pkt, pktErr := ppp.ParseLCPPacket(payload)
			if pktErr != nil {
				continue
			}
			if !answerOpenedIPCP(chanFile, frameBuf[:], pkt, &ipcpCodeRejectID, result.peerMRU, logger) {
				return
			}
		case frame, ok := <-frames:
			if !ok || frame.err != nil {
				return
			}
			proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
			if parseErr != nil {
				continue
			}
			if proto != ppp.ProtoLCP && proto != ppp.ProtoCHAP {
				continue
			}
			pkt, pktErr := ppp.ParseLCPPacket(payload)
			if pktErr != nil {
				continue
			}
			if proto == ppp.ProtoCHAP {
				if chap == nil {
					continue
				}
				// RFC 1994 Section 4.1 (re-challenge) and Section 4.2 (result).
				if err := chap.handle(chanFile, frameBuf[:], pkt, logger); err != nil {
					logger.Warn("pppoe-client: network-phase CHAP ended the session", "error", err)
					return
				}
				continue
			}
			switch pkt.Code {
			case ppp.LCPEchoReply:
				echoFails = 0
			case ppp.LCPEchoRequest:
				sendEchoReply(chanFile, frameBuf[:], pkt, magic)
			case ppp.LCPTerminateRequest:
				sendTerminateAck(chanFile, frameBuf[:], pkt)
				logger.Info("pppoe-client: server sent LCP Terminate-Request")
				return
			}
		}
	}
}

// answerOpenedIPCP answers one IPCP packet received once IPCP is Opened, and
// reports whether the session goes on. The client negotiates IPCP once, before
// PPPIOCCONNECT, and has no address to offer a renegotiation, so every packet
// that takes IPCP out of Opened (RFC 1661 Section 4.1: RCR, RCA, RCN, RTR,
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
