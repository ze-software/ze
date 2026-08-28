package fixture

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // L2TP CHAP and RADIUS require MD5 on the wire
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

func tunnelL2TPRadiusAccountingPeer(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	challenge, _ := tunnelHex("0f1e2d3c4b5a69788796a5b4c3d2e1f0")
	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return err
	}
	defer conn.Close()
	reply, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(0x0321, "py-lac", challenge), 40, 250*time.Millisecond)
	if err != nil {
		return errors.New("no SCCRP received")
	}
	avps, err := tunnelL2TPParseAVPs(reply)
	if err != nil || len(avps[9]) != 2 || len(avps[11]) == 0 {
		return errors.New("SCCRP missing Assigned Tunnel ID or Challenge")
	}
	state := &tunnelAccountingPeer{conn: conn, target: target, localTID: binary.BigEndian.Uint16(avps[9]), ns: 1, nr: 1}
	digest := md5.Sum(append(append([]byte{3}, tunnelL2TPTestSecret()...), avps[11]...))
	state.sendControl(append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(3)), tunnelL2TPAVP(true, 13, digest[:])...), 0)
	icrq := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(10)), tunnelL2TPAVP(true, 14, tunnelL2TPU16(700))...)
	icrq = append(icrq, tunnelL2TPAVP(true, 15, tunnelL2TPU32(42))...)
	state.sendControl(icrq, 0)
	deadline := time.Now().Add(10 * time.Second)
	for state.zeSID == 0 && time.Now().Before(deadline) {
		packet, readErr := state.readPacket(time.Second)
		if readErr != nil {
			continue
		}
		parsed, parseErr := tunnelL2TPParseAVPs(packet)
		if parseErr == nil && tunnelL2TPMessageType(parsed) == 11 && len(parsed[14]) == 2 {
			state.zeSID = binary.BigEndian.Uint16(parsed[14])
		}
		if len(packet) >= 12 && int(binary.BigEndian.Uint16(packet[2:4])) > 12 {
			ns := binary.BigEndian.Uint16(packet[8:10])
			if ns == state.nr {
				state.nr = ns + 1
			}
		}
	}
	if state.zeSID == 0 {
		return errors.New("no ICRP received")
	}
	fmt.Printf("OK: ICRP received, ze session id %d on tunnel %d\n", state.zeSID, state.localTID)
	peerOptions := append([]byte{1, 4, 0x05, 0xdc}, []byte{5, 6, 0x11, 0x22, 0x33, 0x44}...)
	lacOptions := append([]byte{1, 4, 0x05, 0xdc}, []byte{5, 6, 0x55, 0x66, 0x77, 0x88}...)
	iccn := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(12)), tunnelL2TPAVP(true, 24, tunnelL2TPU32(10000000))...)
	iccn = append(iccn, tunnelL2TPAVP(true, 19, tunnelL2TPU32(2))...)
	iccn = append(iccn, tunnelL2TPAVP(false, 26, peerOptions)...)
	iccn = append(iccn, tunnelL2TPAVP(false, 27, lacOptions)...)
	iccn = append(iccn, tunnelL2TPAVP(false, 28, peerOptions)...)
	state.sendControl(iccn, state.zeSID)
	fmt.Println("OK: ICCN sent with proxy LCP AVPs 26, 27 and 28")
	if err := state.negotiateIPCP(ctx); err != nil {
		return err
	}
	return state.checkRadius(ctx)
}

type tunnelAccountingPeer struct {
	conn           *net.UDPConn
	target         *net.UDPAddr
	localTID       uint16
	zeSID          uint16
	ns             uint16
	nr             uint16
	zeConfReqAcked bool
	ourIPCPID      byte
	ourRequestAddr net.IP
	ourIPCPAcked   bool
	negotiatedAddr net.IP
}

func (peer *tunnelAccountingPeer) sendControl(body []byte, sid uint16) {
	_, _ = peer.conn.WriteToUDP(tunnelL2TPControl(peer.localTID, sid, peer.ns, peer.nr, body), peer.target)
	peer.ns++
}

func (peer *tunnelAccountingPeer) readPacket(timeout time.Duration) ([]byte, error) {
	buffer := make([]byte, 2048)
	_ = peer.conn.SetReadDeadline(time.Now().Add(timeout))
	n, _, err := peer.conn.ReadFromUDP(buffer)
	return append([]byte(nil), buffer[:n]...), err
}

func (peer *tunnelAccountingPeer) sendPPP(protocol uint16, payload []byte) {
	packet := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint16(packet[0:2], 2)
	binary.BigEndian.PutUint16(packet[2:4], peer.localTID)
	binary.BigEndian.PutUint16(packet[4:6], peer.zeSID)
	binary.BigEndian.PutUint16(packet[6:8], protocol)
	copy(packet[8:], payload)
	_, _ = peer.conn.WriteToUDP(packet, peer.target)
}

func tunnelPPPControl(code, identifier byte, body []byte) []byte {
	packet := make([]byte, 4+len(body))
	packet[0], packet[1] = code, identifier
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	copy(packet[4:], body)
	return packet
}

func (peer *tunnelAccountingPeer) negotiateIPCP(ctx context.Context) error {
	peer.ourRequestAddr = net.IPv4zero.To4()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if peer.zeConfReqAcked && peer.ourIPCPAcked {
			fmt.Printf("OK: IPCP opened, ze assigned %s\n", peer.negotiatedAddr)
			return nil
		}
		packet, err := peer.readPacket(250 * time.Millisecond)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		if err := peer.handleAccountingPacket(packet); err != nil {
			return err
		}
	}
	return errors.New("IPCP did not open within 30s")
}

func (peer *tunnelAccountingPeer) handleAccountingPacket(packet []byte) error {
	if len(packet) < 6 {
		return nil
	}
	flags := binary.BigEndian.Uint16(packet[:2])
	if flags&0x8000 != 0 {
		if len(packet) >= 12 {
			length := int(binary.BigEndian.Uint16(packet[2:4]))
			if length > 12 && length <= len(packet) {
				ns := binary.BigEndian.Uint16(packet[8:10])
				if ns == peer.nr {
					peer.nr = ns + 1
				}
				avps, _ := tunnelL2TPParseAVPs(packet)
				switch tunnelL2TPMessageType(avps) {
				case 4:
					return errors.New("ze tore the tunnel down")
				case 14:
					return errors.New("ze disconnected the call")
				default:
					_, _ = peer.conn.WriteToUDP(tunnelL2TPControl(peer.localTID, 0, peer.ns, peer.nr, nil), peer.target)
				}
			}
		}
		return nil
	}
	offset := 2
	if flags&0x4000 != 0 {
		offset += 2
	}
	offset += 4
	if flags&0x0800 != 0 {
		offset += 4
	}
	if flags&0x0200 != 0 {
		if len(packet) < offset+2 {
			return nil
		}
		offset += 2 + int(binary.BigEndian.Uint16(packet[offset:offset+2]))
	}
	if len(packet) < offset+6 {
		return nil
	}
	payload := packet[offset:]
	if bytes.HasPrefix(payload, []byte{0xff, 0x03}) {
		payload = payload[2:]
	}
	if len(payload) < 6 {
		return nil
	}
	protocol := binary.BigEndian.Uint16(payload[:2])
	code, identifier := payload[2], payload[3]
	length := int(binary.BigEndian.Uint16(payload[4:6]))
	if length < 4 || 2+length > len(payload) {
		return nil
	}
	body := payload[6 : 2+length]
	if protocol == 0xc021 {
		if code == 9 {
			peer.sendPPP(0xc021, tunnelPPPControl(10, identifier, []byte{0x11, 0x22, 0x33, 0x44}))
		} else if code == 5 {
			peer.sendPPP(0xc021, tunnelPPPControl(6, identifier, nil))
			return errors.New("ze terminated LCP")
		}
		return nil
	}
	if protocol != 0x8021 {
		return nil
	}
	switch code {
	case 1:
		peer.sendPPP(0x8021, tunnelPPPControl(2, identifier, body))
		if !peer.zeConfReqAcked {
			peer.zeConfReqAcked = true
			peer.sendIPCPRequest()
		}
	case 3:
		address := tunnelIPCPAddress(body)
		if address == nil {
			return errors.New("IPCP Configure-Nak without IP-Address option")
		}
		if address.Equal(peer.ourRequestAddr) {
			return fmt.Errorf("ze re-Naked suggested address %s", address)
		}
		peer.ourRequestAddr = address
		peer.sendIPCPRequest()
	case 2:
		if identifier == peer.ourIPCPID && !peer.ourIPCPAcked {
			peer.ourIPCPAcked = true
			peer.negotiatedAddr = append(net.IP(nil), peer.ourRequestAddr...)
		}
	case 4:
		return errors.New("ze rejected IPCP IP-Address option")
	}
	return nil
}

func (peer *tunnelAccountingPeer) sendIPCPRequest() {
	peer.ourIPCPID++
	option := append([]byte{3, 6}, peer.ourRequestAddr.To4()...)
	peer.sendPPP(0x8021, tunnelPPPControl(1, peer.ourIPCPID, option))
}

func tunnelIPCPAddress(options []byte) net.IP {
	for offset := 0; offset+2 <= len(options); {
		length := int(options[offset+1])
		if length < 2 || offset+length > len(options) {
			return nil
		}
		if options[offset] == 3 && length == 6 {
			return net.IP(append([]byte(nil), options[offset+2:offset+6]...))
		}
		offset += length
	}
	return nil
}

func (peer *tunnelAccountingPeer) checkRadius(ctx context.Context) error {
	access, err := peer.waitRadius(ctx, func(line string) bool { return strings.HasPrefix(line, "RADIUS-RX Access-Request ") })
	if err != nil {
		return err
	}
	nasID := os.Getenv("TEST_NAS_ID")
	if nasID == "" {
		nasID = "lns1"
	}
	portPattern := regexp.MustCompile(`NAS-Port-Id=(\S+)`)
	match := portPattern.FindStringSubmatch(access)
	if len(match) != 2 {
		return fmt.Errorf("Access-Request carried no NAS-Port-Id: %s", access)
	}
	authPortID := match[1]
	validPortID := regexp.MustCompile(`^` + regexp.QuoteMeta(nasID) + `:\d+\.\d+$`)
	if !validPortID.MatchString(authPortID) {
		return fmt.Errorf("Access-Request NAS-Port-Id %q does not match template", authPortID)
	}
	fmt.Printf("OK: Access-Request NAS-Port-Id resolved from the template as %s\n", authPortID)
	accounting, err := peer.waitRadius(ctx, func(line string) bool {
		return strings.HasPrefix(line, "RADIUS-RX Accounting-Request ") && strings.Contains(line, "Acct-Status-Type=Start")
	})
	if err != nil {
		return err
	}
	addressPattern := regexp.MustCompile(`Framed-IP-Address=(\S+)`)
	address := addressPattern.FindStringSubmatch(accounting)
	if len(address) != 2 || address[1] != peer.negotiatedAddr.String() {
		return fmt.Errorf("Accounting-Start address does not match IPCP: %s", accounting)
	}
	fmt.Printf("OK: Accounting-Start Framed-IP-Address matches the IPCP-negotiated address %s\n", address[1])
	accountingPort := portPattern.FindStringSubmatch(accounting)
	if len(accountingPort) != 2 || accountingPort[1] != authPortID {
		return fmt.Errorf("NAS-Port-Id is not stable across auth and accounting: %s", accounting)
	}
	fmt.Printf("OK: Accounting-Start carries the same NAS-Port-Id %s\n", authPortID)
	return nil
}

func (peer *tunnelAccountingPeer) waitRadius(ctx context.Context, predicate func(string) bool) (string, error) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		contents, _ := os.ReadFile("radius-rx.log")
		for _, line := range strings.Split(string(contents), "\n") {
			if predicate(line) {
				return line, nil
			}
		}
		packet, err := peer.readPacket(250 * time.Millisecond)
		if err == nil {
			if err := peer.handleAccountingPacket(packet); err != nil {
				return "", err
			}
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	return "", errors.New("RADIUS packet did not arrive")
}
