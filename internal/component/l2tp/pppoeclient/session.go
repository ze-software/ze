// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- client-mode PPP session negotiation
// Related: dialer.go -- discovery phase that precedes this

package pppoeclient

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
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

// startReader launches a goroutine that reads PPP frames from r and
// sends them on the returned channel. Exits when r returns an error
// or when the channel is no longer consumed.
func startReader(r io.Reader) <-chan readFrame {
	ch := make(chan readFrame, 4)
	go func() {
		defer close(ch)
		for {
			buf := make([]byte, ppp.MaxFrameLen)
			n, err := r.Read(buf)
			if err != nil {
				ch <- readFrame{err: err}
				return
			}
			if n < 2 {
				continue
			}
			ch <- readFrame{data: buf[:n]}
		}
	}()
	return ch
}

// negotiateSession drives LCP, authentication, and IPCP on the PPP
// channel fd. Returns the negotiated addresses and MTU.
func negotiateSession(chanFile io.ReadWriteCloser, unitFD, unitNum int, cfg sessionConfig, stopCh <-chan struct{}, logger *slog.Logger) (sessionResult, error) {
	magic, err := generateMagic()
	if err != nil {
		return sessionResult{}, err
	}

	frames := startReader(chanFile)
	var frameBuf [ppp.MaxFrameLen]byte

	// Phase 1: LCP (AC-3).
	lcpResult, err := negotiateLCP(chanFile, frames, frameBuf[:], cfg, magic, stopCh, logger)
	if err != nil {
		return sessionResult{}, err
	}
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
	if err := ppp.Connect(cfg.chanFD, unitNum); err != nil {
		return sessionResult{}, errors.New("pppoe-client: PPPIOCCONNECT: " + err.Error())
	}
	negMTU := lcpResult.peerMRU
	if negMTU == 0 {
		negMTU = ppp.MaxFrameLen
	}
	if err := ppp.SetMRU(unitFD, negMTU); err != nil {
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
	peerMRU   uint16
	authProto uint16
	authData  []byte
}

func negotiateLCP(w io.Writer, frames <-chan readFrame, buf []byte, cfg sessionConfig, magic uint32, stopCh <-chan struct{}, _ *slog.Logger) (lcpResult, error) {
	var (
		result    lcpResult
		lcpID     uint8 = 1
		state           = ppp.LCPStateReqSent
		ackedPeer bool
	)

	// RFC 1661: client sends CONFREQ with MRU + Magic (no Auth-Protocol).
	sendLCPConfigRequest(w, buf, lcpID, cfg.mtu, magic)

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
			if state == ppp.LCPStateReqSent || state == ppp.LCPStateAckSent {
				lcpID++
				sendLCPConfigRequest(w, buf, lcpID, cfg.mtu, magic)
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
			case ppp.LCPConfigureRequest:
				opts, oErr := ppp.ParseLCPOptions(pkt.Data)
				if oErr != nil {
					sendLCPReject(w, buf, pkt)
					continue
				}
				authProto, authData, mru := extractServerOptions(opts)
				if mru > 0 {
					result.peerMRU = mru
				}
				result.authProto = authProto
				result.authData = authData
				sendLCPAck(w, buf, pkt)
				ackedPeer = true
				if state == ppp.LCPStateAckRcvd {
					state = ppp.LCPStateOpened
				} else {
					state = ppp.LCPStateAckSent
				}

			case ppp.LCPConfigureAck:
				switch state { //nolint:exhaustive // only ReqSent/AckSent are reachable here
				case ppp.LCPStateReqSent:
					state = ppp.LCPStateAckRcvd
				case ppp.LCPStateAckSent:
					state = ppp.LCPStateOpened
				}

			case ppp.LCPConfigureNak:
				lcpID++
				sendLCPConfigRequest(w, buf, lcpID, cfg.mtu, magic)

			case ppp.LCPConfigureReject:
				lcpID++
				sendLCPConfigRequestMinimal(w, buf, lcpID, magic)

			case ppp.LCPEchoRequest:
				sendEchoReply(w, buf, pkt, magic)

			case ppp.LCPTerminateRequest:
				sendTerminateAck(w, buf, pkt)
				return lcpResult{}, errors.New("pppoe-client: server terminated LCP")
			}

			if ackedPeer && state == ppp.LCPStateAckRcvd {
				state = ppp.LCPStateOpened
			}
		}
	}
	return result, nil
}

func runClientAuth(w io.ReadWriteCloser, frames <-chan readFrame, buf []byte, lcp lcpResult, cfg sessionConfig, magic uint32, stopCh <-chan struct{}, logger *slog.Logger) error {
	deadline := time.NewTimer(authTimeout)
	defer deadline.Stop()

	// RFC 1334 PAP: client initiates with Authenticate-Request.
	if lcp.authProto == ppp.ProtoPAP {
		pkt := buildPAPAuthRequest(1, cfg.username, cfg.password)
		off := ppp.WriteFrame(buf, 0, ppp.ProtoPAP, pkt)
		w.Write(buf[:off]) //nolint:errcheck // best effort
		logger.Info("pppoe-client: PAP auth-request sent")
	}

	for {
		select {
		case <-stopCh:
			return errors.New("pppoe-client: stopped during auth")
		case <-deadline.C:
			return errors.New("pppoe-client: auth timeout")
		case frame, ok := <-frames:
			if !ok || frame.err != nil {
				return errors.New("pppoe-client: channel closed during auth")
			}
			proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
			if parseErr != nil {
				continue
			}
			if proto == ppp.ProtoLCP {
				pkt, _ := ppp.ParseLCPPacket(payload)
				if pkt.Code == ppp.LCPEchoRequest {
					sendEchoReply(w, buf, pkt, magic)
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
					off := ppp.WriteFrame(buf, 0, ppp.ProtoCHAP, resp)
					w.Write(buf[:off]) //nolint:errcheck // best effort
					logger.Info("pppoe-client: CHAP response sent")
				case 3: // Success
					logger.Info("pppoe-client: CHAP auth success")
					return nil
				case 4: // Failure
					return errors.New("pppoe-client: CHAP auth failed")
				}
				continue
			}

			// PAP response.
			if proto == ppp.ProtoPAP && lcp.authProto == ppp.ProtoPAP {
				pkt, pktErr := ppp.ParseLCPPacket(payload)
				if pktErr != nil {
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

type ipcpResult struct {
	localIP netip.Addr
	peerIP  netip.Addr
}

func negotiateIPCP(w io.Writer, frames <-chan readFrame, buf []byte, magic uint32, stopCh <-chan struct{}, _ *slog.Logger) (ipcpResult, error) {
	var (
		result      ipcpResult
		ipcpID      uint8 = 1
		requestedIP       = netip.IPv4Unspecified()
		gotOurAck   bool
		gotPeerCR   bool
	)

	// RFC 1332: client sends CONFREQ with IP=0.0.0.0 to request assignment.
	sendIPCPRequest(w, buf, ipcpID, requestedIP)

	deadline := time.NewTimer(ncpTimeout)
	defer deadline.Stop()

	for !gotOurAck || !gotPeerCR {
		select {
		case <-stopCh:
			return ipcpResult{}, errors.New("pppoe-client: stopped during IPCP")
		case <-deadline.C:
			return ipcpResult{}, errors.New("pppoe-client: IPCP timeout")
		case frame, ok := <-frames:
			if !ok || frame.err != nil {
				return ipcpResult{}, errors.New("pppoe-client: channel closed during IPCP")
			}
			proto, payload, _, parseErr := ppp.ParseFrame(frame.data)
			if parseErr != nil {
				continue
			}

			if proto == ppp.ProtoLCP {
				pkt, _ := ppp.ParseLCPPacket(payload)
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

			pkt, pktErr := ppp.ParseLCPPacket(payload)
			if pktErr != nil {
				continue
			}

			switch pkt.Code {
			case ppp.LCPConfigureRequest:
				serverIP := parseIPCPNakAddress(pkt.Data)
				if serverIP.IsValid() {
					result.peerIP = serverIP
				}
				off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, nil)
				off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data)
				w.Write(buf[:off]) //nolint:errcheck // best effort
				gotPeerCR = true

			case ppp.LCPConfigureAck:
				result.localIP = requestedIP
				gotOurAck = true

			case ppp.LCPConfigureNak:
				assigned := parseIPCPNakAddress(pkt.Data)
				if assigned.IsValid() {
					requestedIP = assigned
					ipcpID++
					sendIPCPRequest(w, buf, ipcpID, requestedIP)
				}

			case ppp.LCPConfigureReject:
				return ipcpResult{}, errors.New("pppoe-client: server rejected IPCP IP-Address option")
			}
		}
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
		frameBuf  [ppp.MaxFrameLen]byte
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

func sendLCPConfigRequest(w io.Writer, buf []byte, id uint8, mtu uint16, magic uint32) {
	opts := ppp.BuildLocalConfigRequest(ppp.LCPOptions{
		MRU:   mtu,
		Magic: magic,
	})
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	dataOff := off + 4 // lcpHeaderLen
	dataLen := ppp.WriteLCPOptions(buf, dataOff, opts)
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureRequest, id, buf[dataOff:dataOff+dataLen])
	w.Write(buf[:off]) //nolint:errcheck // best effort
}

func sendLCPConfigRequestMinimal(w io.Writer, buf []byte, id uint8, magic uint32) {
	opts := ppp.BuildLocalConfigRequest(ppp.LCPOptions{
		Magic: magic,
	})
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	dataOff := off + 4
	dataLen := ppp.WriteLCPOptions(buf, dataOff, opts)
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureRequest, id, buf[dataOff:dataOff+dataLen])
	w.Write(buf[:off]) //nolint:errcheck // best effort
}

func sendLCPAck(w io.Writer, buf []byte, req ppp.LCPPacket) {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureAck, req.Identifier, req.Data)
	w.Write(buf[:off]) //nolint:errcheck // best effort
}

func sendLCPReject(w io.Writer, buf []byte, req ppp.LCPPacket) {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureReject, req.Identifier, req.Data)
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

func sendIPCPRequest(w io.Writer, buf []byte, id uint8, addr netip.Addr) {
	ipcpPkt := buildIPCPRequest(id, addr)
	off := ppp.WriteFrame(buf, 0, ppp.ProtoIPCP, ipcpPkt)
	w.Write(buf[:off]) //nolint:errcheck // best effort
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
