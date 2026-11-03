//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- synchronous wire scripts.
// Related: peer_linux.go -- shared transport and legacy no-auth exchange.
// RFC 2516 Sections 5 and 6 -- see rfc/short/rfc2516.md.

package pppoeac

import (
	"bytes"
	"context"
	"crypto/md5" // #nosec G501 -- RFC 1994 Section 3 requires MD5 for the negotiated CHAP algorithm 5 under test.
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

// OpenVeth creates a test-owned pair and socket in the caller's namespace.
// The top-level test MUST enter userns first and join all peer users before
// cleanup. Cleanup closes the socket before removing the pair.
func OpenVeth(t testing.TB, client, concentrator string, sid uint16) *Peer {
	t.Helper()
	pair := &netlink.Veth{Name: client, PeerName: concentrator}
	if err := netlink.LinkAdd(pair); err != nil {
		t.Fatalf("create wire-peer veth: %v", err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(pair); err != nil {
			t.Errorf("remove wire-peer veth: %v", err)
		}
	})
	for _, name := range []string{client, concentrator} {
		link, err := netlink.LinkByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatal(err)
		}
	}
	link, err := netlink.LinkByName(concentrator)
	if err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(NetworkOrder(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Errorf("close wire-peer socket: %v", err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: link.Attrs().Index, Protocol: NetworkOrder(unix.ETH_P_ALL)}); err != nil {
		t.Fatal(err)
	}
	if err := pppoe.SetRecvTimeout(fd, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	var mac [pppoe.EthALen]byte
	copy(mac[:], link.Attrs().HardwareAddr)
	peer, err := New(fd, link.Attrs().Index, mac, sid)
	if err != nil {
		t.Fatal(err)
	}
	return peer
}

// Session pins the host MAC learned during discovery. It starts no goroutine.
// Only one reader may use it. Sends may run concurrently with that reader.
// Its nil and zero values are uninitialized and return errors.
type Session struct {
	peer   *Peer
	remote [pppoe.EthALen]byte
	echo   echoState
}

// Accept answers discovery until PADS establishes a session.
// RFC 2516 Section 5: "When the Access Concentrator sends the Confirmation packet, it may proceed to the PPP Session Stage."
func (p *Peer) Accept(ctx context.Context) (*Session, error) {
	var incoming [pppoe.EthMaxLen]byte
	// Discovery ends at PADS or the caller's deadline.
	for {
		n, err := p.receiveFrame(ctx, incoming[:])
		if err != nil {
			return nil, err
		}
		if binary.BigEndian.Uint16(incoming[12:14]) != unix.ETH_P_PPP_DISC {
			continue
		}
		// RFC 2516 Section 5.
		code, err := p.replyDiscovery(incoming[:n])
		if err != nil {
			return nil, err
		}
		if code == pppoe.CodePADR {
			s := &Session{peer: p}
			copy(s.remote[:], incoming[6:12])
			return s, nil
		}
	}
}

// SendControl keeps the established session identity for every script step.
// RFC 2516 Section 6: "The SESSION_ID MUST NOT change for that PPPoE session and MUST be the value assigned in the Discovery stage."
func (s *Session) SendControl(proto uint16, code, id uint8, data []byte) error {
	if s == nil {
		return errors.New("script session is nil")
	}
	// RFC 2516 Section 6. Serialize successful LCP sends with their observation.
	s.echo.mu.Lock()
	defer s.echo.mu.Unlock()
	if err := s.peer.SendControl(s.remote[:], proto, code, id, data); err != nil {
		return err
	}
	if proto == ppp.ProtoLCP {
		// RFC 1661 Sections 5.2 and 6.4.
		s.echo.observe(true, ppp.LCPPacket{Code: code, Identifier: id, Data: data})
	}
	return nil
}

// ReadControl returns an owned packet, rejecting truncated session data.
// RFC 2516 Section 6: "The PPPoE payload contains a PPP frame."
func (s *Session) ReadControl(ctx context.Context) (uint16, ppp.LCPPacket, error) {
	// RFC 2516 Section 6.
	proto, payload, err := s.ReadPacket(ctx)
	if err != nil {
		return 0, ppp.LCPPacket{}, err
	}
	// RFC 1661 Section 5. ReadPacket transfers ownership of the backing bytes.
	pkt, err := ppp.ParseLCPPacket(payload)
	return proto, pkt, err
}

// SendPacket sends one network or control payload on this established session.
// RFC 2516 Section 6: "The PPPoE payload contains a PPP frame."
func (s *Session) SendPacket(proto uint16, payload []byte) error {
	if s == nil {
		return errors.New("script session is nil")
	}
	// RFC 2516 Section 6.
	s.echo.mu.Lock()
	defer s.echo.mu.Unlock()
	if err := s.peer.sendPacket(s.remote[:], proto, payload); err != nil {
		return err
	}
	if proto == ppp.ProtoLCP {
		// RFC 1661 Sections 5 and 6.4. Raw malformed sends remain observable.
		if packet, err := ppp.ParseLCPPacket(payload); err == nil {
			s.echo.observe(true, packet)
		}
	}
	return nil
}

// ReadPacket returns an owned PPP payload and verifies the established SID.
// The caller MUST remain the sole reader until return. The context bounds it.
// RFC 2516 Section 6: "The PPPoE payload contains a PPP frame."
func (s *Session) ReadPacket(ctx context.Context) (uint16, []byte, error) {
	if s == nil {
		return 0, nil, errors.New("script session is nil")
	}
	var incoming [pppoe.EthMaxLen]byte
	// Unrelated Ethernet traffic is bounded by the caller's deadline.
	for {
		n, err := s.peer.receiveFrame(ctx, incoming[:])
		if err != nil {
			return 0, nil, err
		}
		if binary.BigEndian.Uint16(incoming[12:14]) != unix.ETH_P_PPP_SES {
			continue
		}
		if !bytes.Equal(incoming[:6], s.peer.mac[:]) {
			continue
		}
		if !bytes.Equal(incoming[6:12], s.remote[:]) {
			continue
		}
		if binary.BigEndian.Uint16(incoming[16:18]) != s.peer.sid {
			return 0, nil, errors.New("client changed PPPoE session ID")
		}
		if incoming[14] != 0x11 {
			return 0, nil, errors.New("invalid PPPoE version/type")
		}
		if incoming[15] != 0 {
			return 0, nil, errors.New("invalid PPPoE session code")
		}
		end := 20 + int(binary.BigEndian.Uint16(incoming[18:20]))
		if end > n {
			return 0, nil, errors.New("truncated PPPoE session frame")
		}
		// RFC 2516 Section 6.
		proto, payload, _, err := ppp.ParseFrame(incoming[20:end])
		if err != nil {
			return 0, nil, err
		}
		if proto == ppp.ProtoLCP {
			// RFC 1661 Sections 5 and 6.4. Observation does not consume packets.
			if packet, err := ppp.ParseLCPPacket(payload); err == nil {
				s.echo.mu.Lock()
				s.echo.observe(false, packet)
				s.echo.mu.Unlock()
			}
		}
		return proto, bytes.Clone(payload), nil
	}
}

// LCP completes both Configure exchanges. restart sends the request first.
// initial setup waits for the host's request so its PPPoX socket is connected.
// RFC 1661 Section 5.1: "An implementation wishing to open a connection MUST transmit a Configure-Request."
func (s *Session) LCP(ctx context.Context, id uint8, options []byte, restart bool) error {
	sent, acknowledged, accepted := false, false, false
	if restart {
		// RFC 1661 Sections 4.1 and 5.1.
		if err := s.SendControl(ppp.ProtoLCP, ppp.LCPConfigureRequest, id, options); err != nil {
			return err
		}
		sent = true
	}
	// Both directions must agree before this bounded-by-context exchange ends.
	for !acknowledged || !accepted {
		proto, pkt, err := s.ReadExpectedControl(ctx)
		if err != nil {
			return err
		}
		if proto != ppp.ProtoLCP {
			return fmt.Errorf("expected LCP, received protocol %x", proto)
		}
		switch pkt.Code {
		case ppp.LCPConfigureRequest:
			if !sent {
				// RFC 1661 Section 5.1.
				if err := s.SendControl(proto, ppp.LCPConfigureRequest, id, options); err != nil {
					return err
				}
				sent = true
			}
			// RFC 1661 Section 5.2: "The acknowledged Configuration Options MUST NOT be reordered or modified in any way."
			if err := s.SendControl(proto, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data); err != nil {
				return err
			}
			accepted = true
		case ppp.LCPConfigureAck:
			if pkt.Identifier != id {
				return errors.New("LCP Ack identifier differs from request")
			}
			if !bytes.Equal(pkt.Data, options) {
				return fmt.Errorf("LCP Ack options %x differ from %x", pkt.Data, options)
			}
			acknowledged = true
		default:
			return fmt.Errorf("unexpected LCP code %d", pkt.Code)
		}
	}
	return nil
}

// CHAP verifies the actual Response digest and identity before sending Success.
// RFC 1994 Section 4.1: "Whenever a Response packet is received, the authenticator compares the Response Value with its own calculation of the expected value."
func (s *Session) CHAP(ctx context.Context, id uint8, challenge []byte, username, secret string) error {
	if len(challenge) == 0 {
		return errors.New("CHAP challenge is empty")
	}
	if len(challenge) > 255 {
		return errors.New("CHAP challenge exceeds one-octet Value-Size")
	}
	var data [263]byte
	data[0] = byte(len(challenge))
	copy(data[1:], challenge)
	// RFC 1994 Section 4.1: "The Name field is one or more octets representing the identification of the system transmitting the packet."
	nameEnd := 1 + len(challenge) + copy(data[1+len(challenge):], "pppoeac")
	// RFC 1994 Section 4.1.
	if err := s.SendControl(ppp.ProtoCHAP, 1, id, data[:nameEnd]); err != nil {
		return err
	}
	proto, pkt, err := s.ReadControl(ctx)
	if err != nil {
		return err
	}
	if proto != ppp.ProtoCHAP {
		return fmt.Errorf("expected CHAP, received protocol %x", proto)
	}
	if pkt.Code != 2 {
		return fmt.Errorf("expected CHAP Response, received code %d", pkt.Code)
	}
	if pkt.Identifier != id {
		return errors.New("CHAP Response identifier differs from Challenge")
	}
	// RFC 1994 Section 4.1: "The Response Value is the one-way hash calculated over a stream of octets consisting of the Identifier, followed by (concatenated with) the "secret", followed by (concatenated with) the Challenge Value."
	hash := md5.New() // #nosec G401 -- RFC 1994 Sections 3 and 4.1 require this CHAP-MD5 Response calculation.
	// hash.Hash.Write cannot return an error.
	_, _ = hash.Write([]byte{id})     //nolint:errcheck // hash.Hash.Write cannot fail.
	_, _ = hash.Write([]byte(secret)) //nolint:errcheck // hash.Hash.Write cannot fail.
	_, _ = hash.Write(challenge)      //nolint:errcheck // hash.Hash.Write cannot fail.
	expected := hash.Sum(nil)
	if len(pkt.Data) != 17+len(username) {
		return errors.New("CHAP Response has unexpected length")
	}
	if pkt.Data[0] != 16 {
		return errors.New("CHAP Response Value-Size is not 16")
	}
	if !bytes.Equal(pkt.Data[1:17], expected) {
		return errors.New("CHAP Response digest differs from challenge calculation")
	}
	if string(pkt.Data[17:]) != username {
		return errors.New("CHAP Response name differs from configured username")
	}
	// RFC 1994 Section 4.2: "If the Value received in a Response is equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 3 (Success)."
	return s.SendControl(ppp.ProtoCHAP, 3, id, nil)
}

// IPCP assigns the host address and requires an exact Ack of the peer address.
// RFC 1332 Section 3.3: "The peer can provide this information by NAKing the option, and returning a valid IP-address."
func (s *Session) IPCP(ctx context.Context, id uint8, local, remote netip.Addr) error {
	if !local.Is4() {
		return errors.New("IPCP local address must be IPv4")
	}
	if !remote.Is4() {
		return errors.New("IPCP remote address must be IPv4")
	}
	ours, theirs := [6]byte{3, 6}, [6]byte{3, 6}
	localBytes, remoteBytes := local.As4(), remote.As4()
	copy(ours[2:], localBytes[:])
	copy(theirs[2:], remoteBytes[:])
	// Retries are bounded by the script's context.
	for {
		proto, pkt, err := s.ReadExpectedControl(ctx)
		if err != nil {
			return err
		}
		if proto != ppp.ProtoIPCP {
			return fmt.Errorf("expected IPCP, received protocol %x", proto)
		}
		if pkt.Code != ppp.LCPConfigureRequest {
			return fmt.Errorf("expected IPCP Request, received code %d", pkt.Code)
		}
		if !bytes.Equal(pkt.Data, theirs[:]) {
			// RFC 1332 Section 3.3.
			if err := s.SendControl(proto, ppp.LCPConfigureNak, pkt.Identifier, theirs[:]); err != nil {
				return err
			}
			continue
		}
		// RFC 1332 Section 2, RFC 1661 Section 5.2.
		if err := s.SendControl(proto, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data); err != nil {
			return err
		}
		// RFC 1332 Section 3.3.
		if err := s.SendControl(proto, ppp.LCPConfigureRequest, id, ours[:]); err != nil {
			return err
		}
		proto, pkt, err = s.ReadExpectedControl(ctx)
		if err != nil {
			return err
		}
		if proto != ppp.ProtoIPCP {
			return errors.New("expected IPCP Configure-Ack protocol")
		}
		if pkt.Code != ppp.LCPConfigureAck {
			return errors.New("expected IPCP Configure-Ack code")
		}
		if pkt.Identifier != id {
			return errors.New("IPCP Ack identifier differs from request")
		}
		if !bytes.Equal(pkt.Data, ours[:]) {
			return fmt.Errorf("IPCP Ack options %x differ from %x", pkt.Data, ours)
		}
		return nil
	}
}
