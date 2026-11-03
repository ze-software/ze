//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- scripted kernel peer.
// Related: internal/component/l2tp/pppoeclient/unit_reservation_integration_linux_test.go.
// RFC 2516 Sections 5 and 6 -- see rfc/short/rfc2516.md.

// Package pppoeac supplies the wire peer for PPPoE client kernel tests.
package pppoeac

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

// Peer borrows a bound AF_PACKET socket. Its zero value is uninitialized.
// The caller MUST cancel and join ServeNoAuth before closing the socket.
// One ServeNoAuth call may run concurrently with SendFrame or SendControl.
// Peer starts no goroutine and never closes the borrowed socket.
type Peer struct {
	fd, ifindex int
	mac         [pppoe.EthALen]byte
	sid         uint16
	initialized bool
}

// New borrows fd, already bound to ifindex with a finite receive timeout.
// The caller MUST keep fd open until ServeNoAuth and all sends have returned.
func New(fd, ifindex int, mac [pppoe.EthALen]byte, sid uint16) (*Peer, error) {
	if fd < 0 {
		return nil, errors.New("wire peer needs an open socket")
	}
	if ifindex <= 0 {
		return nil, errors.New("wire peer needs a positive interface index")
	}
	if sid == 0 {
		return nil, errors.New("wire peer needs a nonzero session ID")
	}
	if sid == 0xffff {
		return nil, errors.New("wire peer cannot use reserved session ID 65535")
	}
	return &Peer{fd: fd, ifindex: ifindex, mac: mac, sid: sid, initialized: true}, nil
}

// NetworkOrder converts the AF_PACKET protocol argument to network byte order.
func NetworkOrder(v uint16) uint16 { return v<<8 | v>>8 }

// SendFrame sends a complete Ethernet frame without changing its contents.
// RFC 2516 Section 4: "The ETHER_TYPE is set to either 0x8863 (Discovery Stage) or 0x8864 (PPP Session Stage)."
func (p *Peer) SendFrame(frame []byte) error {
	if p == nil {
		return errors.New("wire peer is nil")
	}
	if !p.initialized {
		return errors.New("wire peer is uninitialized")
	}
	if len(frame) < 14 {
		return errors.New("wire peer needs a complete Ethernet header")
	}
	var destination [8]byte
	copy(destination[:], frame[:pppoe.EthALen])
	return unix.Sendto(p.fd, frame, 0, &unix.SockaddrLinklayer{
		Ifindex: p.ifindex, Halen: pppoe.EthALen, Addr: destination,
		Protocol: NetworkOrder(binary.BigEndian.Uint16(frame[12:14])),
	})
}

// SendControl sends one PPP control packet on the borrowed transport.
// RFC 2516 Section 6: "The frame begins with the PPP Protocol-ID."
// Wire offsets (octets):
//
//	0..5 dst | 6..11 src | 12..13 EtherType | 14 ver/type | 15 code
//	16..17 SID | 18..19 length | 20..21 protocol | 22 code | 23 ID
//	24..25 control length | 26.. data
func (p *Peer) SendControl(remote []byte, proto uint16, code, id uint8, data []byte) error {
	if p == nil {
		return errors.New("wire peer is nil")
	}
	if !p.initialized {
		return errors.New("wire peer is uninitialized")
	}
	if len(remote) != pppoe.EthALen {
		return errors.New("wire peer needs a six-octet destination")
	}
	if len(data) > pppoe.EthMaxLen-26 {
		return errors.New("wire peer control packet exceeds the Ethernet buffer")
	}
	var payload [pppoe.EthMaxLen - 22]byte
	// RFC 1661 Section 5.
	n := ppp.WriteLCPPacket(payload[:], 0, code, id, data)
	// RFC 2516 Section 6.
	return p.sendPacket(remote, proto, payload[:n])
}

// sendPacket is the single session envelope encoder for control and IP data.
// RFC 2516 Section 6: "The frame begins with the PPP Protocol-ID."
// Wire offsets: 0..13 Ethernet, 14..19 PPPoE, 20..21 protocol, 22.. payload.
func (p *Peer) sendPacket(remote []byte, proto uint16, payload []byte) error {
	if p == nil {
		return errors.New("wire peer is nil")
	}
	if !p.initialized {
		return errors.New("wire peer is uninitialized")
	}
	if len(remote) != pppoe.EthALen {
		return errors.New("wire peer needs a six-octet destination")
	}
	if len(payload) > pppoe.EthMaxLen-22 {
		return errors.New("wire peer packet exceeds the Ethernet buffer")
	}
	var buf [pppoe.EthMaxLen]byte
	copy(buf[:6], remote)
	copy(buf[6:12], p.mac[:])
	binary.BigEndian.PutUint16(buf[12:14], unix.ETH_P_PPP_SES)
	buf[14] = 0x11
	binary.BigEndian.PutUint16(buf[16:18], p.sid)
	// RFC 2516 Section 6.
	off := 20 + ppp.WriteFrame(buf[:], 20, proto, payload)
	binary.BigEndian.PutUint16(buf[18:20], uint16(off-20))
	// RFC 2516 Section 6.
	return p.SendFrame(buf[:off])
}

// ServeNoAuth preserves the unit-ownership fixture's no-auth IPv4 exchange.
// The caller MUST cancel ctx and join this call before closing the socket.
// observed receives owned copies of non-Configure-Request IPCP packets.
// A nil observed disables observation. Cancellation interrupts delivery.
// RFC 1332 Section 2: "IPCP uses the same packet exchange machanism as the Link Control Protocol (LCP)."
func (p *Peer) ServeNoAuth(ctx context.Context, observed chan<- ppp.LCPPacket) error {
	if p == nil {
		return errors.New("wire peer is nil")
	}
	if !p.initialized {
		return errors.New("wire peer is uninitialized")
	}
	var incoming [pppoe.EthMaxLen]byte
	// The caller's context bounds this socket worker's lifetime.
	for {
		n, err := p.receiveFrame(ctx, incoming[:])
		if err != nil {
			if ctx.Err() != nil {
				return nil //nolint:nilerr // ServeNoAuth treats caller cancellation as successful worker shutdown, matching canceled observation delivery below.
			}
			return err
		}
		switch binary.BigEndian.Uint16(incoming[12:14]) {
		case unix.ETH_P_PPP_DISC:
			// RFC 2516 Section 5.
			_, err := p.replyDiscovery(incoming[:n])
			if err != nil {
				return err
			}
		case unix.ETH_P_PPP_SES:
			if !bytes.Equal(incoming[:6], p.mac[:]) {
				continue
			}
			if binary.BigEndian.Uint16(incoming[16:18]) != p.sid {
				continue
			}
			end := 20 + int(binary.BigEndian.Uint16(incoming[18:20]))
			if end > n {
				return errors.New("truncated PPPoE session frame")
			}
			// RFC 2516 Section 6.
			proto, payload, _, err := ppp.ParseFrame(incoming[20:end])
			if err != nil {
				return err
			}
			// RFC 1661 Section 5.
			pkt, err := ppp.ParseLCPPacket(payload)
			if err != nil {
				return err
			}
			if observed != nil {
				if proto == ppp.ProtoIPCP {
					if pkt.Code != ppp.LCPConfigureRequest {
						pkt.Data = bytes.Clone(pkt.Data)
						select {
						case observed <- pkt:
						case <-ctx.Done():
							return nil
						}
					}
				}
			}
			if pkt.Code != ppp.LCPConfigureRequest {
				continue
			}
			switch proto {
			case ppp.ProtoLCP:
				// RFC 1661 Section 5.2: "The acknowledged Configuration Options MUST NOT be reordered or modified in any way."
				if err := p.SendControl(incoming[6:12], proto, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data); err != nil {
					return err
				}
				// RFC 1661 Section 5.1.
				if err := p.SendControl(incoming[6:12], proto, ppp.LCPConfigureRequest, 77, nil); err != nil {
					return err
				}
			case ppp.ProtoIPCP:
				address := []byte{3, 6, 192, 0, 2, 2}
				if !bytes.Equal(pkt.Data, address) {
					// RFC 1332 Section 3.3: "The peer can provide this information by NAKing the option, and returning a valid IP-address."
					if err := p.SendControl(incoming[6:12], proto, ppp.LCPConfigureNak, pkt.Identifier, address); err != nil {
						return err
					}
					continue
				}
				// RFC 1332 Section 2, RFC 1661 Section 5.2.
				if err := p.SendControl(incoming[6:12], proto, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data); err != nil {
					return err
				}
				// RFC 1332 Section 3.3.
				if err := p.SendControl(incoming[6:12], proto, ppp.LCPConfigureRequest, 78, []byte{3, 6, 192, 0, 2, 1}); err != nil {
					return err
				}
			}
		}
	}
}

// receiveFrame filters the socket's outgoing copies and unrelated interfaces.
// The finite socket timeout makes cancellation observable without another worker.
func (p *Peer) receiveFrame(ctx context.Context, incoming []byte) (int, error) {
	if p == nil {
		return 0, errors.New("wire peer is nil")
	}
	if !p.initialized {
		return 0, errors.New("wire peer is uninitialized")
	}
	// The caller's context bounds this receive loop.
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		n, from, err := unix.Recvfrom(p.fd, incoming, 0)
		if errors.Is(err, unix.EAGAIN) {
			continue
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		link, ok := from.(*unix.SockaddrLinklayer)
		if !ok {
			continue
		}
		if link.Ifindex != p.ifindex {
			continue
		}
		if link.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		if n < 20 {
			continue
		}
		return n, nil
	}
}

// replyDiscovery returns the received code after answering PADI or PADR.
// Other codes have no response in this scripted discovery responder.
// RFC 2516 Section 5.2: "When the Access Concentrator receives a PADI that it can serve, it replies by sending a PADO packet."
func (p *Peer) replyDiscovery(frame []byte) (uint8, error) {
	// RFC 2516 Section 5.
	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		return 0, err
	}
	var outgoing [pppoe.EthMaxLen]byte
	var reply []byte
	//exhaustive:ignore // This fixture serves only discovery requests.
	switch pkt.Code {
	case pppoe.CodePADI:
		// RFC 2516 Section 5.2.
		reply = pppoe.BuildPADO(outgoing[:], p.mac, &pkt, "unit-owner-peer", nil, nil)
	case pppoe.CodePADR:
		// RFC 2516 Section 5.4.
		reply = pppoe.BuildPADS(outgoing[:], p.mac, &pkt, "unit-owner-peer", p.sid)
	// refuse-by-default: guards Peer.Accept -- return the received code to the caller without replying to a non-request.
	default:
		return pkt.Code, nil
	}
	// RFC 2516 Section 5.
	return pkt.Code, p.SendFrame(reply)
}
