// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- client-mode PPP session negotiation
// Related: dialer.go -- discovery phase that precedes this

package pppoeclient

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

const (
	lcpNegotiationTimeout = 30 * time.Second
	authTimeout           = 30 * time.Second
	ncpTimeout            = 30 * time.Second
	papRetryInterval      = 3 * time.Second
	papRequestsMax        = 5
)

// sessionConfig carries parameters for negotiateSession and the connected
// session. keepaliveInterval and keepaliveFailures come from the pppoe-client
// keepalive leaves; newConnectedSession refuses a zero of either, so no zero
// interval reaches the echo ticker.
type sessionConfig struct {
	name              string
	mtu               uint16
	username          string
	password          string
	chanFD            int
	keepaliveInterval time.Duration
	keepaliveFailures int
}

// sessionResult is Dial's stable initial outcome and receive owner.
// newConnectedSession copies negotiated values for the worker; it MUST NOT mutate
// this record, because Dial snapshots it before starting that worker.
type sessionResult struct {
	localIP netip.Addr
	peerIP  netip.Addr
	// primaryDNS and secondaryDNS are the RFC 1877 servers IPCP learned; an
	// invalid address means the server supplied none for that option.
	primaryDNS   netip.Addr
	secondaryDNS netip.Addr
	negMTU       uint16
	magic        uint32
	reader       *frameReader // Shared by initial negotiation and the connected owner.
	// peerMRU bounds the Rejected-Packet of a network-phase Code-Reject, and
	// ipcpCodeRejectID is the Identifier of the last IPCP Code-Reject sent.
	peerMRU          uint16
	ipcpCodeRejectID uint8
	ipcpIdentifier   uint8
	lcpIdentifier    uint8
	papIdentifier    uint8
	// chap is non-nil when LCP negotiated CHAP, so keepaliveLoop answers a
	// Challenge the authenticator sends in the network phase.
	chap *networkPhaseCHAP
}

// readFrame is a frame delivered from the reader goroutine.
type readFrame struct {
	data []byte
	err  error
}

// lcpReentry is an admitted Configure-Request, constructed by requestLCPReentry.
// Its packet borrows the reader-owned frame, which frameReader never reuses.
// The caller MUST transfer the frame without further mutation and consume this
// handoff before resuming authentication or IPCP. Nil means no pending request.
// The zero value is uninitialized and negotiateLCP rejects it before any write.
// Callers MUST NOT return a typed nil as an error.
type lcpReentry struct {
	request ppp.LCPPacket
}

func (*lcpReentry) Error() string {
	return "pppoe-client: LCP renegotiation requested"
}

// requestLCPReentry admits a parsed request before the current phase can end.
// RFC 1661 Section 3.4: "The receipt of the LCP Configure-Request causes a return
// to the Link Establishment phase from the Network-Layer Protocol phase or
// Authentication phase."
// The caller MUST retain its phase on false. On true it MUST transfer the
// immutable frame to the phase owner through the returned handoff.
// RFC 1661 Section 6 option offsets within pkt.Data:
//
//	0 Type | 1 Length (including header) | 2..Length-1 Data
func requestLCPReentry(pkt ppp.LCPPacket) (*lcpReentry, bool) {
	if pkt.Code != ppp.LCPConfigureRequest {
		return nil, false
	}
	// RFC 1661 Section 6: "When the Data field is indicated by the Length to
	// extend beyond the end of the Information field, the entire packet is
	// silently discarded without affecting the automaton."
	if ppp.WalkLCPOptions(pkt.Data).Fault.Discards() {
		return nil, false
	}
	return &lcpReentry{request: pkt}, true
}

// negotiateSession drives LCP, authentication, and IPCP on the PPP
// channel fd. Returns the negotiated addresses and MTU.
func negotiateSession(chanFile io.ReadWriteCloser, reader *frameReader, unitFD, unitNum int, cfg sessionConfig, stopCh <-chan struct{}, logger *slog.Logger) (sessionResult, error) {
	// Admission precedes the first LCP write, including for a typed-nil link.
	link, ok := chanFile.(*sessionLink)
	if !ok {
		return sessionResult{}, errors.New("pppoe-client: PPPIOCCONNECT requires sessionLink lifecycle ownership")
	}
	if link == nil {
		return sessionResult{}, errors.New("pppoe-client: PPPIOCCONNECT requires a non-nil sessionLink")
	}
	var ipcpIdentifier, lcpIdentifier, papIdentifier, ipcpCodeRejectID uint8
	// RFC 1661 Sections 3.4-3.6: the same owner drives initial and connected negotiation.
	state, err := negotiatePhases(chanFile, reader, cfg, 0, nil, &ipcpIdentifier, stopCh, logger, &lcpIdentifier, &papIdentifier, &ipcpCodeRejectID)
	if err != nil {
		return sessionResult{}, err
	}

	// Phase 4: PPPIOCCONNECT + PPPIOCSMRU.
	// The PADT watcher and Dial cleanup MUST close descriptors under this
	// same lock, so these ioctls cannot target a reused descriptor.
	link.mu.Lock()
	defer link.mu.Unlock()
	if link.closed {
		return sessionResult{}, io.ErrClosedPipe
	}
	if err := ppp.Connect(cfg.chanFD, unitNum); err != nil {
		return sessionResult{}, errors.New("pppoe-client: PPPIOCCONNECT: " + err.Error())
	}
	// RFC 1661 Section 6.1 requires reception of the full 1500-octet
	// Information field even after requesting a smaller MRU.
	if err := ppp.SetMRU(unitFD, ppp.MaxFrameLen); err != nil {
		return sessionResult{}, errors.New("pppoe-client: PPPIOCSMRU: " + err.Error())
	}

	// The unit now receives IPCP. Enable it under the same lifecycle lock
	// before authorizing another read through the session's existing reader.
	link.unitConnected = true
	return sessionResult{
		localIP:          state.localIP,
		peerIP:           state.peerIP,
		negMTU:           state.negMTU,
		magic:            state.magic,
		reader:           reader,
		peerMRU:          state.peerMRU,
		ipcpCodeRejectID: ipcpCodeRejectID,
		ipcpIdentifier:   ipcpIdentifier,
		lcpIdentifier:    lcpIdentifier,
		papIdentifier:    papIdentifier,
		chap:             state.chap,
	}, nil
}

type lcpResult struct {
	peerMRU    uint16
	authProto  uint16
	authData   []byte
	localMagic uint32
}

// The caller MUST supply the existing reader so non-LCP traffic consumed in
// this phase is discarded before another descriptor read is authorized.
// The caller MUST retain identifier for the session lifetime, including restarts.
func negotiateLCP(w io.Writer, reader *frameReader, buf []byte, cfg sessionConfig, magic uint32, pending *lcpReentry, stopCh <-chan struct{}, logger *slog.Logger, identifier *uint8) (lcpResult, error) {
	if pending != nil {
		if pending.request.Code != ppp.LCPConfigureRequest {
			return lcpResult{}, errors.New("pppoe-client: uninitialized LCP reentry")
		}
	}
	var (
		result lcpResult
		state  = ppp.LCPStateReqSent
	)
	// RFC 1661 Section 5.1: "The Identifier field MUST be changed whenever the
	// contents of the Options field changes, and whenever a valid reply has been
	// received for a previous request."
	*identifier++

	// Retain the sent packet separately: replies and Echo responses reuse buf.
	// RFC 1661 Section 5.2: "Additionally, the Configuration Options in a
	// Configure-Ack MUST exactly match those of the last transmitted
	// Configure-Request."
	var request [ppp.MaxFrameBufLen]byte
	requestLen, err := sendLCPConfigRequest(w, request[:], *identifier, cfg.mtu, magic, logger)
	if err != nil {
		return lcpResult{}, err
	}
	localMRU := cfg.mtu
	localMagic := magic
	var mruRejected, magicRejected bool

	deadline := time.NewTimer(lcpNegotiationTimeout)
	defer deadline.Stop()
	restart := time.NewTicker(3 * time.Second)
	defer restart.Stop()

	for state != ppp.LCPStateOpened {
		var pkt ppp.LCPPacket
		if pending != nil {
			// RFC 1661 Section 4.1, Opened/RCR: scr precedes sca/scn.
			// The initial request above is on the wire before this first event.
			pkt = pending.request
			pending = nil
		} else {
			select {
			case <-stopCh:
				return lcpResult{}, errors.New("pppoe-client: stopped during LCP")
			case <-deadline.C:
				return lcpResult{}, errors.New("pppoe-client: LCP timeout")
			case <-restart.C:
				// RFC 1661 Section 5.1: "The Identifier field MUST be changed
				// whenever the contents of the Options field changes, and whenever
				// a valid reply has been received for a previous request."
				if state == ppp.LCPStateAckRcvd {
					*identifier++
					request[3] = *identifier
					state = ppp.LCPStateReqSent
				}
				if err := writeClientLCPRequest(w, request[:requestLen]); err != nil {
					return lcpResult{}, err
				}
				continue
			case frame, ok := <-reader.next():
				reader.received()
				if !ok {
					return lcpResult{}, io.EOF
				}
				if frame.err != nil {
					return lcpResult{}, frame.err
				}
				proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
				if parseErr != nil {
					continue
				}
				if proto != ppp.ProtoLCP {
					// RFC 1661 Section 3.4: "Any non-LCP packets received during this phase MUST be silently discarded."
					continue
				}
				parsed, pktErr := ppp.ParseLCPPacket(payload)
				if pktErr != nil {
					continue
				}
				pkt = parsed
			}
		}
		switch pkt.Code {
		case ppp.LCPConfigureAck, ppp.LCPConfigureNak, ppp.LCPConfigureReject:
			if !ppp.ValidateLCPReply(pkt, *identifier, request[6:requestLen]) {
				continue
			}
		}

		switch pkt.Code {
		case ppp.LCPConfigureRequest:
			walk := ppp.WalkLCPOptions(pkt.Data)
			if walk.Fault.Discards() {
				// RFC 1661 Section 6: "When the Data field is
				// indicated by the Length to extend beyond the end
				// of the Information field, the entire packet is
				// silently discarded without affecting the
				// automaton."
				//
				// The silence is owed twice. Any reply to a packet
				// an off-path sender can forge makes ze a
				// reflector, and echoing the sender's own octets
				// back is the reflection it would perform.
				logger.Debug("pppoe-client: LCP Configure-Request silently discarded",
					"id", pkt.Identifier,
					"len", len(pkt.Data))
				continue
			}
			// RFC 1661 Section 4.1: RCR- in Ack-Sent returns to
			// Req-Sent. A newer peer proposal invalidates its prior Ack.
			if state == ppp.LCPStateAckSent {
				state = ppp.LCPStateReqSent
			}
			if walk.Fault == ppp.LCPOptionsBadLength {
				// The options are contained in the packet, so the
				// fault is one option's own Length. RFC 1661
				// Section 6 answers that with a Configure-Nak
				// carrying the desired option, and Section 5.4 with
				// a Configure-Reject for a Type ze holds no value
				// for. ppp.LCPNakOrReject picks between them, so
				// the client and the LNS answer alike.
				code, opts, reply := ppp.LCPNakOrReject(walk, clientLCPPolicy(cfg, magic))
				if !reply {
					// Nothing to name in the answer. RFC 1661
					// Section 5.3 fills the Options field with
					// "only the unacceptable Configuration
					// Options from the Configure-Request", so an
					// empty Configure-Nak is a packet the section
					// does not describe. The client sends nothing
					// and says so, and the server's own
					// retransmission drives the negotiation.
					logger.Warn("pppoe-client: LCP reply suppressed, no option to carry in it",
						"id", pkt.Identifier,
						"len", len(pkt.Data))
					continue
				}
				sendLCPOptionReply(w, buf, code, pkt.Identifier, opts, logger)
				continue
			}
			// The options are contained in the packet and each
			// one's own Length is valid, so the remaining question
			// is whether their VALUES are acceptable. RFC 1661
			// Section 5.3: "If every instance of the received
			// Configuration Options is recognizable, but some
			// values are not acceptable, then the implementation
			// MUST transmit a Configure-Nak." Section 5.4 does the
			// same for an option that is "not recognizable or
			// not acceptable for negotiation", and takes
			// precedence: a reply is a Configure-Reject or a
			// Configure-Nak, never both.
			//
			// The client runs the SAME negotiator the LNS side runs
			// (ppp.NegotiatePeerOptions), so one peer gets one
			// answer from ze whichever role ze is in. Until this
			// branch existed the client Acked every parseable
			// Configure-Request unread, which acknowledged a
			// Magic-Number of zero that RFC 1661 Section 6.4 says
			// "MUST always be Nak'd, if it is not Rejected
			// outright", and the LNS side Nak'd it.
			_, naks, rejects := ppp.NegotiatePeerOptions(walk.Options, clientLCPPolicy(cfg, magic))
			if len(rejects) > 0 {
				sendLCPOptionReply(w, buf, ppp.LCPConfigureReject, pkt.Identifier, rejects, logger)
				continue
			}
			if len(naks) > 0 {
				sendLCPOptionReply(w, buf, ppp.LCPConfigureNak, pkt.Identifier, naks, logger)
				continue
			}

			authProto, authData, mru := extractServerOptions(walk.Options)
			// The shared negotiator checks the option shape. This client
			// implements PAP and CHAP-MD5; other methods need a new offer,
			// never an Ack followed by a different authentication algorithm.
			authSupported := true
			for _, opt := range walk.Options {
				if opt.Type != ppp.LCPOptAuthProto {
					continue
				}
				authSupported = false
				if authProto == ppp.ProtoPAP {
					authSupported = len(opt.Data) == 2
				}
				if authProto == ppp.ProtoCHAP {
					if len(authData) == 1 {
						authSupported = authData[0] == 5
					}
				}
			}
			if !authSupported {
				// RFC 1661 Section 6.2 permits a Nak containing a
				// supported Authentication-Protocol as an alternative.
				sendLCPOptionReply(w, buf, ppp.LCPConfigureNak, pkt.Identifier,
					[]ppp.LCPOption{{Type: ppp.LCPOptAuthProto, Data: []byte{0xc2, 0x23, 5}}}, logger)
				continue
			}
			result.peerMRU = mru
			result.authProto = authProto
			result.authData = authData
			if err := sendLCPAck(w, buf, pkt); err != nil {
				return lcpResult{}, err
			}
			if state == ppp.LCPStateAckRcvd {
				state = ppp.LCPStateOpened
			} else {
				state = ppp.LCPStateAckSent
			}

		case ppp.LCPConfigureAck:
			switch state {
			case ppp.LCPStateReqSent:
				state = ppp.LCPStateAckRcvd
			case ppp.LCPStateAckSent:
				state = ppp.LCPStateOpened
			case ppp.LCPStateAckRcvd:
				// RFC 1661 Section 4.1: a duplicate RCA in Ack-Rcvd
				// sends a fresh Configure-Request and returns to Req-Sent.
				*identifier++
				request[3] = *identifier
				if err := writeClientLCPRequest(w, request[:requestLen]); err != nil {
					return lcpResult{}, err
				}
				state = ppp.LCPStateReqSent
			case ppp.LCPStateInitial, ppp.LCPStateStarting, ppp.LCPStateClosed,
				ppp.LCPStateStopped, ppp.LCPStateClosing, ppp.LCPStateStopping, ppp.LCPStateOpened:
				// The other named states do not participate in this negotiation.
			default:
				panic("BUG: invalid LCP negotiation state")
			}

		case ppp.LCPConfigureNak, ppp.LCPConfigureReject:
			if pkt.Code == ppp.LCPConfigureReject {
				// RFC 1661 Section 5.4: "Reception of a valid
				// Configure-Reject indicates that when a new
				// Configure-Request is sent, it MUST NOT include any of the
				// Configuration Options listed in the Configure-Reject."
				for _, opt := range ppp.WalkLCPOptions(pkt.Data).Options {
					switch opt.Type {
					case ppp.LCPOptMRU:
						localMRU = 0
						mruRejected = true
					case ppp.LCPOptMagic:
						localMagic = 0
						magicRejected = true
					}
				}
			}
			if pkt.Code == ppp.LCPConfigureNak {
				for _, opt := range ppp.WalkLCPOptions(pkt.Data).Options {
					switch opt.Type {
					case ppp.LCPOptMRU:
						if mruRejected {
							continue
						}
						if len(opt.Data) != 2 {
							continue
						}
						mru := binary.BigEndian.Uint16(opt.Data)
						if mru >= ppp.MinFrameLen {
							if mru <= cfg.mtu {
								localMRU = mru
							}
						}
					case ppp.LCPOptMagic:
						if magicRejected {
							continue
						}
						if len(opt.Data) != 4 {
							continue
						}
						// RFC 1661 Section 6.4: "If the Magic-Number is
						// equal to the one sent in the last Configure-Nak,
						// the possibility of a looped-back link is
						// increased, and a new Magic-Number MUST be chosen."
						localMagic, err = generateDifferentMagic(localMagic)
						if err != nil {
							return lcpResult{}, err
						}
					}
				}
			}
			magic = localMagic
			*identifier++
			requestLen, err = sendLCPConfigRequest(w, request[:], *identifier, localMRU, localMagic, logger)
			if err != nil {
				return lcpResult{}, err
			}
			if state != ppp.LCPStateAckSent {
				state = ppp.LCPStateReqSent
			}

		case ppp.LCPEchoRequest:
			// RFC 1661 Section 5.8: "Echo-Request and Echo-Reply
			// packets MUST only be sent in the LCP Opened state."
			continue

		case ppp.LCPTerminateRequest:
			sendTerminateAck(w, buf, pkt)
			return lcpResult{}, errors.New("pppoe-client: server terminated LCP")
		}
	}
	result.localMagic = localMagic
	return result, nil
}

// The reader has the same exclusive-consumer contract as negotiateLCP.
// The caller MUST retain identifier for every PAP request in the session.
func runClientAuth(w io.ReadWriteCloser, reader *frameReader, buf []byte, lcp lcpResult, cfg sessionConfig, magic uint32, stopCh <-chan struct{}, logger *slog.Logger, identifier *uint8) error {
	deadline := time.NewTimer(authTimeout)
	defer deadline.Stop()

	var papRequest []byte
	var papRestart <-chan time.Time
	papRequests := 0
	var chapID uint8
	chapResponseSent := false
	if lcp.authProto == ppp.ProtoPAP {
		// RFC 1334 Section 2.2.1: "The link peer MUST transmit a PAP packet
		// with the Code field set to 1 (Authenticate-Request) during the
		// Authentication phase."
		// RFC 1334 Section 2.2.1: "The Identifier field MUST be changed each
		// time an Authenticate-Request packet is issued."
		*identifier++
		papRequest = buildPAPAuthRequest(*identifier, cfg.username, cfg.password)
		if err := writeClientPAPRequest(w, buf, papRequest); err != nil {
			return err
		}
		papRequests++
		restart := time.NewTicker(papRetryInterval)
		defer restart.Stop()
		papRestart = restart.C
		logger.Info("pppoe-client: PAP auth-request sent")
	}

	for {
		select {
		case <-stopCh:
			return errors.New("pppoe-client: stopped during auth")
		case <-deadline.C:
			return errors.New("pppoe-client: auth timeout")
		case <-papRestart:
			// RFC 1334 Section 2.2.1: "The Authenticate-Request packet MUST be
			// repeated until a valid reply packet is received, or an optional
			// retry counter expires."
			if papRequests == papRequestsMax {
				return errors.New("pppoe-client: PAP retry limit reached")
			}
			// RFC 1334 Section 2.2.1: "The Identifier field MUST be changed
			// each time an Authenticate-Request packet is issued."
			*identifier++
			papRequest[1] = *identifier
			if err := writeClientPAPRequest(w, buf, papRequest); err != nil {
				return err
			}
			papRequests++
		case frame, ok := <-reader.next():
			reader.received()
			if !ok {
				return io.EOF
			}
			if frame.err != nil {
				return frame.err
			}
			proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
			if parseErr != nil {
				continue
			}
			if proto == ppp.ProtoLCP {
				pkt, err := ppp.ParseLCPPacket(payload)
				if err != nil {
					continue
				}
				switch pkt.Code {
				case ppp.LCPConfigureRequest:
					// RFC 1661 Sections 3.4 and 6: discard before phase reentry.
					reentry, admitted := requestLCPReentry(pkt)
					if !admitted {
						continue
					}
					return reentry
				case ppp.LCPEchoRequest:
					sendEchoReply(w, buf, pkt, magic)
				case ppp.LCPTerminateRequest:
					// RFC 1334 Section 2.2.1: LCP termination is an
					// alternative failure indication when a PAP Nak is lost.
					sendTerminateAck(w, buf, pkt)
					return errors.New("pppoe-client: server terminated during auth")
				}
				continue
			}

			// RFC 1994 CHAP: server sends Challenge, client responds.
			if proto == ppp.ProtoCHAP && lcp.authProto == ppp.ProtoCHAP {
				pkt, pktErr := ppp.ParseLCPPacket(payload)
				if pktErr != nil {
					continue
				}
				switch pkt.Code {
				case 1: // Challenge
					resp := buildCHAPResponse(pkt, cfg)
					if resp == nil {
						continue
					}
					off := ppp.WriteFrame(buf, 0, ppp.ProtoCHAP, resp)
					if err := writeClientPacket(w, buf[:off]); err != nil {
						return fmt.Errorf("pppoe-client: CHAP Response: %w", err)
					}
					chapID = pkt.Identifier
					chapResponseSent = true
					logger.Info("pppoe-client: CHAP response sent")
				case 3, 4: // Success or Failure
					// RFC 1994 Section 4.2: the Identifier MUST be copied
					// from the Response which caused this reply.
					if !chapResponseSent {
						continue
					}
					if pkt.Identifier != chapID {
						continue
					}
					if pkt.Code == 4 {
						return errors.New("pppoe-client: CHAP auth failed")
					}
					logger.Info("pppoe-client: CHAP auth success")
					return nil
				}
				continue
			}

			// PAP response.
			if proto == ppp.ProtoPAP && lcp.authProto == ppp.ProtoPAP {
				pkt, pktErr := ppp.ParseLCPPacket(payload)
				if pktErr != nil {
					continue
				}
				// RFC 1334 Section 2.2.2: "The Identifier field MUST be copied
				// from the Identifier field of the Authenticate-Request which
				// caused this reply."
				if pkt.Identifier != papRequest[1] {
					continue
				}
				// RFC 1334 Section 2.2.2: Msg-Length counts the Message
				// octets. ParseLCPPacket already removes link-layer padding.
				if len(pkt.Data) == 0 {
					continue
				}
				if int(pkt.Data[0]) != len(pkt.Data)-1 {
					continue
				}
				switch pkt.Code {
				case 2: // Authenticate-Ack
					logger.Info("pppoe-client: PAP auth success")
					return nil
				case 3: // Authenticate-Nak
					return errors.New("pppoe-client: PAP auth rejected")
				}
			}
		}
	}
}

// writeClientPAPRequest writes the complete PAP request or reports failure.
// RFC 1334 Section 2.2.1, offsets inside request:
//
//	0 Code | 1 Identifier | 2..3 Length | 4 Peer-ID-Length
//	5.. Peer-ID | 5+Peer-ID-Length Passwd-Length | 6+Peer-ID-Length.. Password
func writeClientPAPRequest(w io.Writer, buf, request []byte) error {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoPAP, request)
	n, err := w.Write(buf[:off])
	if err != nil {
		return fmt.Errorf("pppoe-client: PAP Authenticate-Request: %w", err)
	}
	if n != off {
		return fmt.Errorf("pppoe-client: PAP Authenticate-Request: %w", io.ErrShortWrite)
	}
	return nil
}

type ipcpResult struct {
	localIP netip.Addr
	peerIP  netip.Addr
	// primaryDNS and secondaryDNS are invalid when the server Rejected the
	// option or Acknowledged the 0.0.0.0 request (learnedDNS).
	primaryDNS   netip.Addr
	secondaryDNS netip.Addr
}

// ipcpRequestLenMax is the largest IPCP Configure-Request frame this client
// sends: the PPP protocol field, the packet header and the three 6-octet
// options IP-Address, Primary-DNS-Address and Secondary-DNS-Address.
const ipcpRequestLenMax = 2 + 4 + 3*6

// ipcpInitialRequest is the option set of the first IPCP Configure-Request:
// IP-Address, Primary-DNS-Address and Secondary-DNS-Address, each 0.0.0.0,
// in that order.
//
// The client wants an address assigned, so IP-Address also carries 0.0.0.0.
//
// RFC 1877 Section 1.1: "If all four octets are set to zero, it indicates an
// explicit request that the peer provide the address information in a
// Config-Nak packet." Section 1.3 states the same for the secondary server.
func ipcpInitialRequest() ppp.IPCPOptions {
	return ppp.IPCPOptions{
		IPAddress: netip.IPv4Unspecified(), HasIPAddress: true,
		PrimaryDNS: netip.IPv4Unspecified(), HasPrimary: true,
		SecondaryDNS: netip.IPv4Unspecified(), HasSecondary: true,
	}
}

// adoptNakDNS answers the server value a Configure-Nak carried for one DNS
// option, or current when that value is not a usable server. requested says
// the outstanding request still carries the option: a Rejected option is
// never requested again, whatever a later Nak names. A 0.0.0.0 or other
// non-unicast value is not adopted, so the request keeps asking and the IPCP
// timeout bounds a server that never names one.
//
// RFC 1877 Section 1.1: "If local peer requests an invalid server address
// (which it will typically do intentionally) the remote peer specifies the
// address by NAKing this option, and returning the IP address of a valid DNS
// server."
func adoptNakDNS(current netip.Addr, requested, naked bool, value netip.Addr) netip.Addr {
	if !requested {
		return current
	}
	if !naked {
		return current
	}
	if !value.IsGlobalUnicast() {
		return current
	}
	return value
}

// learnedDNS answers the server an Acknowledged request names for one DNS
// option: the invalid address when the option was not requested, or when the
// server Acknowledged the 0.0.0.0 request and so named no server.
func learnedDNS(requested bool, value netip.Addr) netip.Addr {
	if !requested {
		return netip.Addr{}
	}
	if !value.IsGlobalUnicast() {
		return netip.Addr{}
	}
	return value
}

// negotiateIPCP opens IPCP. chap is nil when LCP did not negotiate CHAP, and
// a CHAP frame is then dropped; otherwise a Challenge is answered, because
// IPCP negotiation is already the Network-Layer Protocol phase.
// peerMRU is the MRU the server negotiated in LCP, zero when it sent none,
// and bounds the Rejected-Packet of a Code-Reject.
// identifier MUST point to the session-owned last Configure-Request Identifier.
// The caller MUST retain it across interrupted negotiations; zero starts at 1.
// codeRejectID MUST point to the session-owned last Code-Reject Identifier.
// The caller MUST retain it across interrupted negotiations and network handling.
// The caller MUST supply the existing ordered reader, which enables the unit
// only after PPPIOCCONNECT and dispatches channel control before unit input.
func negotiateIPCP(w io.Writer, reader *frameReader, buf []byte, magic uint32, peerMRU uint16, chap *networkPhaseCHAP, identifier *uint8, stopCh <-chan struct{}, logger *slog.Logger, codeRejectID *uint8) (ipcpResult, error) {
	var result ipcpResult
	requested := ipcpInitialRequest()
	// RFC 1332 Section 2 uses the LCP exchange mechanism. A new exchange
	// advances the session sequence so an interrupted exchange cannot supply
	// the reply to this request (RFC 1661 Section 5.1).
	*identifier++
	state := ppp.LCPStateReqSent
	var request [ipcpRequestLenMax]byte
	requestLen, err := sendIPCPRequest(w, request[:], *identifier, requested)
	if err != nil {
		return ipcpResult{}, err
	}

	deadline := time.NewTimer(ncpTimeout)
	defer deadline.Stop()
	restart := time.NewTicker(3 * time.Second)
	defer restart.Stop()

	// RFC 1332 Section 2 uses the LCP exchange mechanism. Keep the exact
	// outstanding request until a valid reply changes it (RFC 1661 Section 5).
	for state != ppp.LCPStateOpened {
		var frame readFrame
		var ok bool
		select {
		case <-stopCh:
			return ipcpResult{}, errors.New("pppoe-client: stopped during IPCP")
		case <-deadline.C:
			return ipcpResult{}, errors.New("pppoe-client: IPCP timeout")
		case <-restart.C:
			if state == ppp.LCPStateAckRcvd {
				*identifier++
				request[3] = *identifier
				state = ppp.LCPStateReqSent
			}
			if err := writeClientPacket(w, request[:requestLen]); err != nil {
				return ipcpResult{}, err
			}
			continue
		case frame, ok = <-reader.next():
			reader.received()
		}
		if !ok {
			return ipcpResult{}, fmt.Errorf("pppoe-client: PPP input closed during IPCP: %w", io.EOF)
		}
		if frame.err != nil {
			return ipcpResult{}, frame.err
		}
		proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
		if parseErr != nil {
			continue
		}
		pkt, pktErr := ppp.ParseLCPPacket(payload)
		if pktErr != nil {
			continue
		}
		if proto == ppp.ProtoLCP {
			switch pkt.Code {
			case ppp.LCPConfigureRequest:
				// RFC 1661 Sections 3.4 and 6: discard before phase reentry.
				reentry, admitted := requestLCPReentry(pkt)
				if !admitted {
					continue
				}
				return ipcpResult{}, reentry
			case ppp.LCPEchoRequest:
				sendEchoReply(w, buf, pkt, magic)
			case ppp.LCPTerminateRequest:
				sendTerminateAck(w, buf, pkt)
				return ipcpResult{}, errors.New("pppoe-client: server terminated during IPCP")
			}
			continue
		}
		if proto == ppp.ProtoCHAP {
			if chap == nil {
				continue
			}
			// RFC 1994 Section 4.1 (re-challenge) and Section 4.2 (result).
			if err := chap.handle(w, buf, pkt, logger); err != nil {
				return ipcpResult{}, err
			}
			continue
		}
		if proto != ppp.ProtoIPCP {
			continue
		}
		// RFC 1332 Section 2: "Other Codes should be treated as
		// unrecognized and should result in Code-Rejects."
		if ipcpCodeUnrecognized(pkt.Code) {
			// RFC 1661 Section 5.6: "The Identifier field MUST be changed for
			// each Code-Reject sent."
			*codeRejectID++
			if err := sendIPCPCodeReject(w, buf, pkt, *codeRejectID, peerMRU); err != nil {
				return ipcpResult{}, err
			}
			continue
		}
		switch pkt.Code {
		case ppp.LCPConfigureAck, ppp.LCPConfigureReject:
			if !ppp.ValidateLCPReply(pkt, *identifier, request[6:requestLen]) {
				continue
			}
		case ppp.LCPConfigureNak:
			// Nak value widths are IPCP-specific, unlike Ack/Reject.
			if pkt.Identifier != *identifier {
				continue
			}
		}

		switch pkt.Code {
		case ppp.LCPConfigureRequest:
			options, err := ppp.ParseIPCPOptions(pkt.Data)
			if err != nil {
				continue
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
					return ipcpResult{}, err
				}
				if state == ppp.LCPStateAckSent {
					state = ppp.LCPStateReqSent
				}
				continue
			}
			result.peerIP = options.IPAddress
			off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, nil)
			off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data)
			if err := writeClientPacket(w, buf[:off]); err != nil {
				return ipcpResult{}, err
			}
			if state == ppp.LCPStateAckRcvd {
				state = ppp.LCPStateOpened
			} else {
				state = ppp.LCPStateAckSent
			}
		case ppp.LCPConfigureAck:
			// ValidateLCPReply matched the Ack to the outstanding request
			// byte for byte, so its values are the request's.
			result.localIP = requested.IPAddress
			result.primaryDNS = learnedDNS(requested.HasPrimary, requested.PrimaryDNS)
			result.secondaryDNS = learnedDNS(requested.HasSecondary, requested.SecondaryDNS)
			switch state {
			case ppp.LCPStateReqSent:
				state = ppp.LCPStateAckRcvd
			case ppp.LCPStateAckSent:
				state = ppp.LCPStateOpened
			case ppp.LCPStateAckRcvd:
				*identifier++
				request[3] = *identifier
				if err := writeClientPacket(w, request[:requestLen]); err != nil {
					return ipcpResult{}, err
				}
				state = ppp.LCPStateReqSent
			case ppp.LCPStateInitial, ppp.LCPStateStarting, ppp.LCPStateClosed,
				ppp.LCPStateStopped, ppp.LCPStateClosing, ppp.LCPStateStopping, ppp.LCPStateOpened:
				// The other named states do not participate in this negotiation.
			default:
				panic("BUG: invalid IPCP negotiation state")
			}
		case ppp.LCPConfigureNak:
			options, err := ppp.ParseIPCPOptions(pkt.Data)
			if err != nil {
				continue
			}
			if options.HasIPAddress {
				requested.IPAddress = options.IPAddress
			}
			// RFC 1877 Section 1.1 and Section 1.3: the server names each DNS
			// server by NAKing the 0.0.0.0 request.
			requested.PrimaryDNS = adoptNakDNS(requested.PrimaryDNS, requested.HasPrimary, options.HasPrimary, options.PrimaryDNS)
			requested.SecondaryDNS = adoptNakDNS(requested.SecondaryDNS, requested.HasSecondary, options.HasSecondary, options.SecondaryDNS)
			*identifier++
			requestLen, err = sendIPCPRequest(w, request[:], *identifier, requested)
			if err != nil {
				return ipcpResult{}, err
			}
			if state != ppp.LCPStateAckSent {
				state = ppp.LCPStateReqSent
			}
		case ppp.LCPConfigureReject:
			if len(pkt.Data) == 0 {
				continue
			}
			// ValidateLCPReply proved the Reject an ordered subset of the
			// outstanding request, so it lists only options this client sent.
			rejected, err := ppp.ParseIPCPOptions(pkt.Data)
			if err != nil {
				continue
			}
			if rejected.HasIPAddress {
				return ipcpResult{}, errors.New("pppoe-client: the access concentrator rejected the IPCP IP-Address option, so no IPv4 address can be assigned; configure an address pool for PPPoE sessions on the access concentrator")
			}
			// RFC 1661 Section 5.4: "Reception of a valid Configure-Reject indicates that when a new Configure-Request is sent, it MUST NOT include any of the Configuration Options listed in the Configure-Reject."
			// A server with no DNS to offer Rejects 129 and 131; the session
			// continues without learned servers.
			if rejected.HasPrimary {
				requested.HasPrimary = false
			}
			if rejected.HasSecondary {
				requested.HasSecondary = false
			}
			*identifier++
			requestLen, err = sendIPCPRequest(w, request[:], *identifier, requested)
			if err != nil {
				return ipcpResult{}, err
			}
			if state != ppp.LCPStateAckSent {
				state = ppp.LCPStateReqSent
			}
		case ppp.LCPTerminateRequest:
			off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, nil)
			off += ppp.WriteLCPPacket(buf, off, ppp.LCPTerminateAck, pkt.Identifier, nil)
			if err := writeClientPacket(w, buf[:off]); err != nil {
				return ipcpResult{}, err
			}
			return ipcpResult{}, errors.New("pppoe-client: server terminated IPCP")
		}
	}
	if !result.localIP.IsGlobalUnicast() {
		return ipcpResult{}, errors.New("pppoe-client: IPCP did not assign a usable local address")
	}
	if !result.peerIP.IsGlobalUnicast() {
		return ipcpResult{}, errors.New("pppoe-client: IPCP did not supply a usable peer address")
	}
	return result, nil
}

// buildCHAPResponse builds a CHAP Response for a received Challenge.
// RFC 1994 Section 4.1.
func buildCHAPResponse(challenge ppp.LCPPacket, cfg sessionConfig) []byte {
	if len(challenge.Data) < 1 {
		return nil
	}
	valueSize := int(challenge.Data[0])
	if valueSize == 0 {
		return nil
	}
	if len(challenge.Data) < 1+valueSize {
		return nil
	}
	challengeValue := challenge.Data[1 : 1+valueSize]

	digest := chapMD5Response(challenge.Identifier, cfg.password, challengeValue)
	nameBytes := []byte(cfg.username)
	respLen := 4 + 1 + len(digest) + len(nameBytes)
	resp := make([]byte, respLen)
	resp[0] = 2 // Response
	resp[1] = challenge.Identifier
	binary.BigEndian.PutUint16(resp[2:4], uint16(respLen)) //nolint:gosec // respLen bounded
	resp[4] = byte(len(digest))
	copy(resp[5:5+len(digest)], digest[:])
	copy(resp[5+len(digest):], nameBytes)
	return resp
}

func extractServerOptions(opts []ppp.LCPOption) (authProto uint16, authData []byte, mru uint16) {
	for _, opt := range opts {
		switch opt.Type {
		case ppp.LCPOptAuthProto:
			if len(opt.Data) >= 2 {
				authProto = binary.BigEndian.Uint16(opt.Data[:2])
				if len(opt.Data) > 2 {
					authData = opt.Data[2:]
				}
			}
		case ppp.LCPOptMRU:
			if len(opt.Data) == 2 {
				mru = binary.BigEndian.Uint16(opt.Data)
			}
		}
	}
	return
}

func sendLCPConfigRequest(w io.Writer, buf []byte, id uint8, mtu uint16, magic uint32, logger *slog.Logger) (int, error) {
	opts := ppp.BuildLocalConfigRequest(ppp.LCPOptions{
		MRU:   mtu,
		Magic: magic,
	})
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	dataOff := off + 4 // lcpHeaderLen
	// Rejected options may be omitted, but a request must never contain a
	// truncated option list. Report failure before writing any packet.
	dataLen, fits := ppp.WriteLCPOptions(buf, dataOff, opts)
	if !fits {
		logger.Warn("pppoe-client: LCP Configure-Request not sent, its options do not fit a frame",
			"id", id,
			"options", len(opts))
		return 0, errors.New("pppoe-client: LCP Configure-Request does not fit")
	}
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureRequest, id, buf[dataOff:dataOff+dataLen])
	if err := writeClientLCPRequest(w, buf[:off]); err != nil {
		return 0, err
	}
	return off, nil
}

// writeClientLCPRequest sends a retained Configure-Request, including its
// original Identifier on timeout retransmissions. RFC 1661 Section 5.1:
//
//	0..1 PPP Protocol | 2 Code | 3 Identifier | 4..5 Length | 6.. Options
func writeClientLCPRequest(w io.Writer, frame []byte) error {
	if err := writeClientPacket(w, frame); err != nil {
		return fmt.Errorf("pppoe-client: LCP Configure-Request: %w", err)
	}
	return nil
}

// writeClientPacket refuses a short write before any negotiation state advances.
func writeClientPacket(w io.Writer, frame []byte) error {
	n, err := w.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}

func sendLCPAck(w io.Writer, buf []byte, req ppp.LCPPacket) error {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureAck, req.Identifier, req.Data)
	return writeClientPacket(w, buf[:off])
}

// clientLCPPolicy validates both option lengths and values. Authentication is
// negotiated here because the peer chooses the method; negotiateLCP limits the
// accepted methods to those runClientAuth implements. MaxMRU bounds the peer's
// receive offer to the PPPoE transport MTU.
func clientLCPPolicy(cfg sessionConfig, magic uint32) ppp.LCPNegPolicy {
	return ppp.LCPNegPolicy{
		MinMRU:          sessionMTUMin,
		MaxMRU:          cfg.mtu,
		AcceptAuthProto: true,
		LocalMagic:      magic,
		PPPoE:           true,
	}
}

// sendLCPOptionReply writes a Configure-Nak or Configure-Reject carrying
// opts. The reply carries only the options that earned it, never the
// request's own Data: RFC 1661 Section 5.3 fills a Nak with the values ze
// wants instead, and Section 5.4 fills a Reject with the refused options
// alone. Echoing the whole request back would reflect octets an off-path
// sender chose.
//
// The server sizes this reply, so it can be larger than the request that
// produced it and larger than buf. RFC 1661 Section 5.4 fills the Options
// field with "only the unacceptable Configuration Options from the
// Configure-Request", so the prefix that fits is a different packet: the
// client writes nothing and says so, and the server's own retransmission
// drives the negotiation.
func sendLCPOptionReply(w io.Writer, buf []byte, code, id uint8, opts []ppp.LCPOption, logger *slog.Logger) {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	dataOff := off + 4 // lcpHeaderLen
	dataLen, fits := ppp.WriteLCPOptions(buf, dataOff, opts)
	if !fits {
		logger.Warn("pppoe-client: LCP reply suppressed, its options do not fit a frame",
			"code", ppp.LCPCodeName(code),
			"id", id,
			"options", len(opts))
		return
	}
	off += ppp.WriteLCPPacket(buf, off, code, id, buf[dataOff:dataOff+dataLen])
	w.Write(buf[:off]) //nolint:errcheck // best effort
}

func sendTerminateAck(w io.Writer, buf []byte, req ppp.LCPPacket) {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPTerminateAck, req.Identifier, nil)
	w.Write(buf[:off]) //nolint:errcheck // best effort
}

func sendEchoReply(w io.Writer, buf []byte, req ppp.LCPPacket, magic uint32) {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	off += ppp.BuildLCPEchoReply(buf, off, req.Identifier, magic, req.Data)
	w.Write(buf[:off]) //nolint:errcheck // best effort
}

// sendIPCPRequest writes the IPCP Configure-Request carrying options into buf
// and sends it, answering the frame length. buf MUST hold ipcpRequestLenMax
// octets; the caller keeps it as the outstanding request that
// ppp.ValidateLCPReply compares an Ack or Reject with.
//
// Wire format (RFC 1661 Section 5.1, RFC 1332 Section 3.3, RFC 1877
// Sections 1.1 and 1.3), offsets from the start of buf:
//
//	0       2    3    4        6
//	+-------+----+----+--------+------------------------------+
//	| 80 21 | 01 | ID | Length | options, 6 octets each:      |
//	+-------+----+----+--------+ type(1) length=6(1) addr(4) |
//	                           +------------------------------+
func sendIPCPRequest(w io.Writer, buf []byte, id uint8, options ppp.IPCPOptions) (int, error) {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, nil)
	optionsLen := ppp.WriteIPCPOptions(buf, off+4, options)
	// WriteLCPPacket copies the options onto themselves: they are already in place.
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureRequest, id, buf[off+4:off+4+optionsLen])
	if err := writeClientPacket(w, buf[:off]); err != nil {
		return 0, fmt.Errorf("pppoe-client: IPCP Configure-Request: %w", err)
	}
	return off, nil
}

// sendIPCPCodeReject answers an IPCP packet whose Code is not one IPCP uses.
//
// RFC 1332 Section 2: "Other Codes should be treated as unrecognized and
// should result in Code-Rejects."
//
// Wire format (RFC 1661 Section 5.6), offsets from the start of the IPCP
// packet, which itself starts 2 octets into the frame:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Code = 7  |  Identifier   |            Length             |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|    Rejected-Packet ...
//	+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Octet 0 Code, 1 Identifier, 2-3 Length, 4 onward the rejected packet's own
// Code, Identifier, Length and Data.
//
// RFC 1661 Section 5.6: "The Rejected-Packet MUST be truncated to comply with
// the peer's established MRU." The copy takes the smaller of that MRU (1500,
// the RFC 1661 Section 6.1 default, when none was negotiated) and buf, less
// the 2-octet Protocol field and the 4-octet Code-Reject header.
//
// RFC 1661 Section 5.6: "The Identifier field MUST be changed for each
// Code-Reject sent." The caller owns the counter and passes the next value.
func sendIPCPCodeReject(w io.Writer, buf []byte, rejected ppp.LCPPacket, id uint8, peerMRU uint16) error {
	mru := int(peerMRU)
	if mru == 0 {
		mru = ppp.MaxFrameLen
	}
	room := min(mru+ppp.ProtoFieldLen, len(buf)) - ppp.ProtoFieldLen - 4
	if room < 4 {
		return errors.New("pppoe-client: IPCP Code-Reject: no room for the Rejected-Packet header")
	}

	off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, nil)
	// The Rejected-Packet is written where WriteLCPPacket needs its Data
	// field, so the copy that call makes is onto itself.
	data := buf[off+4:]
	data[0] = rejected.Code
	data[1] = rejected.Identifier
	binary.BigEndian.PutUint16(data[2:4], uint16(4+len(rejected.Data))) //nolint:gosec // ParseLCPPacket bounded Data by a 16-bit Length
	n := 4 + copy(data[4:room], rejected.Data)
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPCodeReject, id, data[:n])
	if err := writeClientPacket(w, buf[:off]); err != nil {
		return fmt.Errorf("pppoe-client: IPCP Code-Reject: %w", err)
	}
	return nil
}

func generateMagic() (uint32, error) {
	var b [4]byte
	for range 8 {
		if _, err := rand.Read(b[:]); err != nil {
			return 0, err
		}
		v := binary.BigEndian.Uint32(b[:])
		if v != 0 {
			return v, nil
		}
	}
	return 0, errors.New("pppoe-client: crypto/rand returned zero 8 times")
}

// generateDifferentMagic bounds repeated entropy draws when a peer Naks Magic.
// Zero and the last proposal cannot escape as the replacement value.
func generateDifferentMagic(previous uint32) (uint32, error) {
	for range 8 {
		magic, err := generateMagic()
		if err != nil {
			return 0, fmt.Errorf("pppoe-client: Magic-Number after Configure-Nak: %w", err)
		}
		if magic != previous {
			return magic, nil
		}
	}
	return 0, errors.New("pppoe-client: crypto/rand repeated the previous Magic-Number 8 times")
}
