// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- network phase: keepalive, CHAP re-challenge, IPv6CP
// Related: session.go -- negotiation that hands its frame channel to keepaliveLoop
// Related: ipv6cp_client.go -- the IPv6CP automaton keepaliveLoop feeds
// Related: ipcp_independent.go -- connected IPCP, which closes and reopens on its own
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

	// RFC 1661 Section 5.8: "Echo-Request and Echo-Reply packets MUST only be
	// sent in the LCP Opened state." This loop runs only after LCP opened.
	// The interval and the failure count come from the pppoe-client
	// keepalive leaves through sessionConfig.
	echoInterval := session.cfg.keepaliveInterval
	echoTicker := time.NewTicker(echoInterval)
	defer echoTicker.Stop()

	var (
		echoID    uint8
		echoFails int
		frameBuf  [ppp.MaxFrameBufLen]byte
	)

	// RFC 7084 Section 4.2 WLL-3: IPv6CP runs independently of IPCP. It
	// started beside IPCP when the network phase was reached (negotiateIPCP),
	// and this loop carries on its negotiation where that left it.
	// ipv6Restart stays nil, so its case never fires, when session.ipv6 is
	// nil (no ipv6 configured).
	var ipv6Restart <-chan time.Time
	if session.ipv6 != nil {
		restart := time.NewTicker(ipv6cpRestartInterval)
		defer restart.Stop()
		ipv6Restart = restart.C
	}

	// reenter returns the session to Link Establishment for an admitted LCP
	// Configure-Request, met here or inside an IPCP renegotiation, and
	// reports whether the session goes on.
	reenter := func(reentry *lcpReentry) bool {
		echoTicker.Stop()
		if session.ipv6 != nil {
			// RFC 1661 Section 3.4: leaving the network phase takes
			// every NCP down, IPv6CP included.
			if err := session.applyIPv6CP(session.ipv6.lowerDown(), logger); err != nil {
				logger.Warn("pppoe-client: IPv6CP could not be taken down for the LCP restart; the client will reconnect", "error", err)
				return false
			}
		}
		// RFC 1661 Sections 3.4-3.6: DROP precedes scr and sca/scn.
		// restart starts IPv6CP again beside IPCP and applies the
		// state it reached.
		if err := session.restart(reentry, stopCh, logger); err != nil {
			logger.Warn("pppoe-client: LCP restart failed; the client will reconnect", "error", err)
			return false
		}
		// RFC 1661 Section 5.8: "On transmission, the Identifier field MUST
		// be changed whenever the content of the Data field changes, and
		// whenever a valid reply has been received for a previous request."
		// Retain the sequence across restart, including an acknowledged Echo.
		echoFails = 0
		echoTicker.Reset(echoInterval)
		return true
	}

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
			// The session ends on the Nth unanswered request, not the one after.
			if echoFails >= session.cfg.keepaliveFailures {
				logger.Warn("pppoe-client: echo timeout", "failures", echoFails)
				return
			}
		case <-ipv6Restart:
			if err := session.ipv6.tick(session.transport, frameBuf[:], logger); err != nil {
				logger.Warn("pppoe-client: IPv6CP write failed", "error", err)
				return
			}
			if err := session.ncpsRemaining(); err != nil {
				logger.Warn("pppoe-client: IPv6CP gave up while IPCP is down", "error", err)
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
			if proto == ppp.ProtoIPv6CP {
				if session.ipv6 == nil {
					continue
				}
			} else if proto != ppp.ProtoLCP {
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
			if proto == ppp.ProtoIPv6CP {
				// RFC 7084 Section 4.2 WLL-3: an IPv6CP outcome never ends IPCP.
				event, err := session.ipv6.handle(session.transport, frameBuf[:], pkt, logger)
				if err != nil {
					logger.Warn("pppoe-client: IPv6CP write failed", "error", err)
					return
				}
				if err := session.applyIPv6CP(event, logger); err != nil {
					logger.Warn("pppoe-client: IPv6CP state could not be applied; the client will reconnect", "error", err)
					return
				}
				if err := session.ncpsRemaining(); err != nil {
					logger.Warn("pppoe-client: IPv6CP closed while IPCP is down", "error", err)
					return
				}
				continue
			}
			if proto == ppp.ProtoIPCP {
				// RFC 1332 Section 2: connected IPCP uses the same ordered source.
				// RFC 7084 Section 4.2 WLL-3: IPCP terminates and opens again
				// on its own, with LCP and IPv6CP up (handleIPCP).
				err := session.handleIPCP(frameBuf[:], pkt, stopCh, logger)
				if reentry, ok := errors.AsType[*lcpReentry](err); ok {
					if !reenter(reentry) {
						return
					}
					continue
				}
				if err != nil {
					logger.Warn("pppoe-client: IPCP ended the session", "error", err)
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
				if !reenter(reentry) {
					return
				}
			case ppp.LCPEchoReply:
				echoFails = 0
			case ppp.LCPEchoRequest:
				sendEchoReply(session.transport, frameBuf[:], pkt, session.current.magic)
			case ppp.LCPTerminateRequest:
				sendTerminateAck(session.transport, frameBuf[:], pkt)
				logger.Info("pppoe-client: server sent LCP Terminate-Request")
				return
			case ppp.LCPProtocolReject:
				if session.ipv6 == nil {
					continue
				}
				if !rejectsIPv6CP(pkt) {
					continue
				}
				logger.Info("pppoe-client: the access concentrator rejected IPv6CP; IPv4 continues")
				if err := session.applyIPv6CP(session.ipv6.protocolRejected(), logger); err != nil {
					logger.Warn("pppoe-client: IPv6CP state could not be applied; the client will reconnect", "error", err)
					return
				}
				if err := session.ncpsRemaining(); err != nil {
					logger.Warn("pppoe-client: IPv6CP rejected while IPCP is down", "error", err)
					return
				}
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
