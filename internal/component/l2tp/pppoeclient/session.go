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
	echoInterval          = 10 * time.Second
	echoMaxFailures       = 3
	papRetryInterval      = 3 * time.Second
	papRequestsMax        = 5
)

// sessionConfig carries parameters for negotiateSession.
type sessionConfig struct {
	mtu      uint16
	username string
	password string
	chanFD   int
}

// sessionResult holds the outcome of a successful PPP negotiation.
type sessionResult struct {
	localIP netip.Addr
	peerIP  netip.Addr
	negMTU  uint16
	magic   uint32
	frames  <-chan readFrame // reused by keepaliveLoop to avoid double-reader
}

// readFrame is a frame delivered from the reader goroutine.
type readFrame struct {
	data []byte
	err  error
}

// startReader owns a bounded frame-delivery queue until stopCh closes or Read
// fails. The caller MUST close stopCh and r to release both delivery and Read.
// The returned channel closes when the reader exits.
func startReader(r io.Reader, stopCh <-chan struct{}) <-chan readFrame {
	ch := make(chan readFrame, 4)
	go func() {
		defer close(ch)
		for {
			select {
			case <-stopCh:
				return
			default:
			}
			buf := make([]byte, ppp.MaxFrameBufLen)
			n, err := r.Read(buf)
			if err != nil {
				select {
				case ch <- readFrame{err: err}:
				case <-stopCh:
				}
				return
			}
			if n < 2 {
				continue
			}
			select {
			case ch <- readFrame{data: buf[:n]}:
			case <-stopCh:
				return
			}
		}
	}()
	return ch
}

// negotiateSession drives LCP, authentication, and IPCP on the PPP
// channel fd. Returns the negotiated addresses and MTU.
func negotiateSession(chanFile io.ReadWriteCloser, frames <-chan readFrame, unitFD, unitNum int, cfg sessionConfig, stopCh <-chan struct{}, logger *slog.Logger) (sessionResult, error) {
	magic, err := generateMagic()
	if err != nil {
		return sessionResult{}, err
	}

	var frameBuf [ppp.MaxFrameBufLen]byte

	// Phase 1: LCP (AC-3).
	lcpResult, err := negotiateLCP(chanFile, frames, frameBuf[:], cfg, magic, stopCh, logger)
	if err != nil {
		return sessionResult{}, err
	}
	magic = lcpResult.localMagic
	logger.Info("pppoe-client: LCP opened", "peer-mru", lcpResult.peerMRU, "auth-proto", lcpResult.authProto)

	// Phase 2: Authentication (AC-4).
	if lcpResult.authProto != 0 {
		if err := runClientAuth(chanFile, frames, frameBuf[:], lcpResult, cfg, magic, stopCh, logger); err != nil {
			return sessionResult{}, err
		}
	}

	// Phase 3: IPCP (AC-5).
	ipcpResult, err := negotiateIPCP(chanFile, frames, frameBuf[:], magic, stopCh, logger)
	if err != nil {
		return sessionResult{}, err
	}
	logger.Info("pppoe-client: IPCP opened", "local-ip", ipcpResult.localIP, "peer-ip", ipcpResult.peerIP)

	// Phase 4: PPPIOCCONNECT + PPPIOCSMRU.
	// The PADT watcher and Dial cleanup MUST close descriptors under this
	// same lock, so these ioctls cannot target a reused descriptor.
	if link, ok := chanFile.(*sessionLink); ok {
		link.mu.Lock()
		defer link.mu.Unlock()
		if link.closed {
			return sessionResult{}, io.ErrClosedPipe
		}
	}
	if err := ppp.Connect(cfg.chanFD, unitNum); err != nil {
		return sessionResult{}, errors.New("pppoe-client: PPPIOCCONNECT: " + err.Error())
	}
	negMTU := lcpResult.peerMRU
	if negMTU == 0 {
		negMTU = ppp.MaxFrameLen
	}
	negMTU = min(negMTU, cfg.mtu)
	// RFC 1661 Section 6.1 requires reception of the full 1500-octet
	// Information field even after requesting a smaller MRU.
	if err := ppp.SetMRU(unitFD, ppp.MaxFrameLen); err != nil {
		return sessionResult{}, errors.New("pppoe-client: PPPIOCSMRU: " + err.Error())
	}

	return sessionResult{
		localIP: ipcpResult.localIP,
		peerIP:  ipcpResult.peerIP,
		negMTU:  negMTU,
		magic:   magic,
		frames:  frames,
	}, nil
}

type lcpResult struct {
	peerMRU    uint16
	authProto  uint16
	authData   []byte
	localMagic uint32
}

func negotiateLCP(w io.Writer, frames <-chan readFrame, buf []byte, cfg sessionConfig, magic uint32, stopCh <-chan struct{}, logger *slog.Logger) (lcpResult, error) {
	var (
		result lcpResult
		lcpID  uint8 = 1
		state        = ppp.LCPStateReqSent
	)

	// Retain the sent packet separately: replies and Echo responses reuse buf.
	// RFC 1661 Section 5.2: "Additionally, the Configuration Options in a
	// Configure-Ack MUST exactly match those of the last transmitted
	// Configure-Request."
	var request [ppp.MaxFrameBufLen]byte
	requestLen, err := sendLCPConfigRequest(w, request[:], lcpID, cfg.mtu, magic, logger)
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
				lcpID++
				request[3] = lcpID
				state = ppp.LCPStateReqSent
			}
			if err := writeClientLCPRequest(w, request[:requestLen]); err != nil {
				return lcpResult{}, err
			}
		case frame, ok := <-frames:
			if !ok || frame.err != nil {
				return lcpResult{}, errors.New("pppoe-client: channel closed during LCP")
			}
			proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
			if parseErr != nil || proto != ppp.ProtoLCP {
				continue
			}
			pkt, pktErr := ppp.ParseLCPPacket(payload)
			if pktErr != nil {
				continue
			}
			switch pkt.Code {
			case ppp.LCPConfigureAck, ppp.LCPConfigureNak, ppp.LCPConfigureReject:
				if !ppp.ValidateLCPReply(pkt, lcpID, request[6:requestLen]) {
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
				switch state { //nolint:exhaustive // Only negotiation states are reachable.
				case ppp.LCPStateReqSent:
					state = ppp.LCPStateAckRcvd
				case ppp.LCPStateAckSent:
					state = ppp.LCPStateOpened
				case ppp.LCPStateAckRcvd:
					// RFC 1661 Section 4.1: a duplicate RCA in Ack-Rcvd
					// sends a fresh Configure-Request and returns to Req-Sent.
					lcpID++
					request[3] = lcpID
					if err := writeClientLCPRequest(w, request[:requestLen]); err != nil {
						return lcpResult{}, err
					}
					state = ppp.LCPStateReqSent
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
							if mruRejected || len(opt.Data) != 2 {
								continue
							}
							mru := binary.BigEndian.Uint16(opt.Data)
							if mru >= ppp.MinFrameLen && mru <= cfg.mtu {
								localMRU = mru
							}
						case ppp.LCPOptMagic:
							if magicRejected || len(opt.Data) != 4 {
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
				lcpID++
				requestLen, err = sendLCPConfigRequest(w, request[:], lcpID, localMRU, localMagic, logger)
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
	}
	result.localMagic = localMagic
	return result, nil
}

func runClientAuth(w io.ReadWriteCloser, frames <-chan readFrame, buf []byte, lcp lcpResult, cfg sessionConfig, magic uint32, stopCh <-chan struct{}, logger *slog.Logger) error {
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
		papRequest = buildPAPAuthRequest(1, cfg.username, cfg.password)
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
			papRequest[1]++
			if err := writeClientPAPRequest(w, buf, papRequest); err != nil {
				return err
			}
			papRequests++
		case frame, ok := <-frames:
			if !ok || frame.err != nil {
				return errors.New("pppoe-client: channel closed during auth")
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
}

func negotiateIPCP(w io.Writer, frames <-chan readFrame, buf []byte, magic uint32, stopCh <-chan struct{}, _ *slog.Logger) (ipcpResult, error) {
	var result ipcpResult
	requestedIP := netip.IPv4Unspecified()
	ipcpID := uint8(1)
	state := ppp.LCPStateReqSent
	var request [12]byte
	requestLen, err := sendIPCPRequest(w, request[:], ipcpID, requestedIP)
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
		select {
		case <-stopCh:
			return ipcpResult{}, errors.New("pppoe-client: stopped during IPCP")
		case <-deadline.C:
			return ipcpResult{}, errors.New("pppoe-client: IPCP timeout")
		case <-restart.C:
			if state == ppp.LCPStateAckRcvd {
				ipcpID++
				request[3] = ipcpID
				state = ppp.LCPStateReqSent
			}
			if err := writeClientPacket(w, request[:requestLen]); err != nil {
				return ipcpResult{}, err
			}
		case frame, ok := <-frames:
			if !ok {
				return ipcpResult{}, errors.New("pppoe-client: channel closed during IPCP")
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
				case ppp.LCPEchoRequest:
					sendEchoReply(w, buf, pkt, magic)
				case ppp.LCPTerminateRequest:
					sendTerminateAck(w, buf, pkt)
					return ipcpResult{}, errors.New("pppoe-client: server terminated during IPCP")
				}
				continue
			}
			if proto != ppp.ProtoIPCP {
				continue
			}
			switch pkt.Code {
			case ppp.LCPConfigureAck, ppp.LCPConfigureReject:
				if !ppp.ValidateLCPReply(pkt, ipcpID, request[6:requestLen]) {
					continue
				}
			case ppp.LCPConfigureNak:
				// Nak value widths are IPCP-specific, unlike Ack/Reject.
				if pkt.Identifier != ipcpID {
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
				result.localIP = requestedIP
				switch state {
				case ppp.LCPStateReqSent:
					state = ppp.LCPStateAckRcvd
				case ppp.LCPStateAckSent:
					state = ppp.LCPStateOpened
				case ppp.LCPStateAckRcvd:
					ipcpID++
					request[3] = ipcpID
					if err := writeClientPacket(w, request[:requestLen]); err != nil {
						return ipcpResult{}, err
					}
					state = ppp.LCPStateReqSent
				}
			case ppp.LCPConfigureNak:
				options, err := ppp.ParseIPCPOptions(pkt.Data)
				if err != nil {
					continue
				}
				if options.HasIPAddress {
					requestedIP = options.IPAddress
				}
				ipcpID++
				requestLen, err = sendIPCPRequest(w, request[:], ipcpID, requestedIP)
				if err != nil {
					return ipcpResult{}, err
				}
				if state != ppp.LCPStateAckSent {
					state = ppp.LCPStateReqSent
				}
			case ppp.LCPConfigureReject:
				if len(pkt.Data) != 0 {
					return ipcpResult{}, errors.New("pppoe-client: server rejected IPCP IP-Address option")
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

// keepaliveLoop handles LCP echo on the frames channel created during
// negotiation. Closes done when the session ends (echo timeout,
// terminate, read error). Uses the existing reader goroutine to avoid
// a second concurrent Read on the same fd.
func keepaliveLoop(chanFile io.Writer, frames <-chan readFrame, magic uint32, done chan<- struct{}, stopCh <-chan struct{}, logger *slog.Logger) {
	defer close(done)

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
		case frame, ok := <-frames:
			if !ok || frame.err != nil {
				return
			}
			proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
			if parseErr != nil || proto != ppp.ProtoLCP {
				continue
			}
			pkt, pktErr := ppp.ParseLCPPacket(payload)
			if pktErr != nil {
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

func sendIPCPRequest(w io.Writer, buf []byte, id uint8, addr netip.Addr) (int, error) {
	ipcpPkt := buildIPCPRequest(id, addr)
	off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, ipcpPkt)
	if err := writeClientPacket(w, buf[:off]); err != nil {
		return 0, fmt.Errorf("pppoe-client: IPCP Configure-Request: %w", err)
	}
	return off, nil
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
