// Design: docs/guide/l2tp.md -- accepted proxy LCP is subject to the local transmit cap.
// Related: tunnel_fixture_l2tp_ppp.go -- shared tunnel, proxy AVPs and CHAP/IPCP peer.

package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register("l2tp/proxy-mru-transmit-cap", tunnelL2TPProxyMRUTransmitCap)
}

func tunnelL2TPProxyMRUTransmitCap(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return errors.New("l2tp/proxy-mru-transmit-cap requires <port> <peer-mru>; use 0 for an omitted MRU option")
	}
	peerMRU, err := strconv.ParseUint(args[1], 10, 16)
	if err != nil || (peerMRU != 0 && peerMRU < 68) {
		return fmt.Errorf("invalid peer MRU %q; use 0 or an integer from 68 through 65535", args[1])
	}
	wantMTU := 1492
	if peerMRU != 0 && peerMRU < uint64(wantMTU) {
		wantMTU = int(peerMRU)
	}
	peer, err := tunnelL2TPOpenProxySession(ctx, port, 1492, uint16(peerMRU))
	if err != nil {
		return err
	}
	// The shared setup transferred the socket; this fixture owns its close.
	defer peer.conn.Close() //nolint:errcheck // fixture teardown
	if err := peer.negotiateIPCP(ctx); err != nil {
		return err
	}
	gateway := net.IPv4(10, 99, 8, 1).To4()
	link, err := tunnelProxyPPPLink(ctx, gateway, wantMTU)
	if err != nil {
		return err
	}
	if _, err := fmt.Printf("OK: proxy peer MRU %d installed kernel MTU %d on %s\n", peerMRU, link.MTU, link.Name); err != nil {
		return err
	}
	if err := tunnelProxyICMPRoundTrip(ctx, peer, gateway, wantMTU); err != nil {
		return err
	}
	if _, err := fmt.Printf("OK: proxy peer MRU %d passed an ICMP round trip with %d Information-field octets\n", peerMRU, wantMTU); err != nil {
		return err
	}
	stop := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(4)), tunnelL2TPAVP(true, 9, tunnelL2TPU16(0x0321))...)
	stop = append(stop, tunnelL2TPAVP(true, 1, append(tunnelL2TPU16(1), tunnelL2TPU16(0)...))...)
	peer.sendControl(stop, 0)
	if err := tunnelProxyPPPRemoved(ctx, link.Index); err != nil {
		return err
	}
	_, err = fmt.Println("OK: proxy PPP interface removed after StopCCN")
	return err
}

// Poll the installed state: IPCP's Ack can precede the transport's assignment
// acknowledgment and the MTU/AdminUp writes. A fixed settling sleep would not
// prove which state the session reached.
func tunnelProxyPPPLink(ctx context.Context, gateway net.IP, wantMTU int) (net.Interface, error) {
	var observed net.Interface
	seen := false
	for range 100 {
		links, err := net.Interfaces()
		if err != nil {
			return net.Interface{}, fmt.Errorf("read kernel interfaces: %w", err)
		}
		for _, link := range links {
			if !strings.HasPrefix(link.Name, "ppp") || link.Flags&net.FlagPointToPoint == 0 {
				continue
			}
			addresses, err := link.Addrs()
			if err != nil {
				return net.Interface{}, fmt.Errorf("read %s addresses: %w", link.Name, err)
			}
			for _, address := range addresses {
				ip, _, err := net.ParseCIDR(address.String())
				if err != nil || !ip.Equal(gateway) {
					continue
				}
				observed = link
				seen = true
				if link.MTU == wantMTU && link.Flags&net.FlagUp != 0 {
					return link, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return net.Interface{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return net.Interface{}, fmt.Errorf("proxy did not install an up PPP interface at %s with MTU %d; observed=%+v present=%t", gateway, wantMTU, observed, seen)
}

func tunnelProxyPPPRemoved(ctx context.Context, index int) error {
	for range 100 {
		links, err := net.Interfaces()
		if err != nil {
			return fmt.Errorf("read kernel interfaces after StopCCN: %w", err)
		}
		present := false
		for _, link := range links {
			if link.Index == index {
				present = true
			}
		}
		if !present {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return fmt.Errorf("PPP interface index %d remains after the proxy's StopCCN", index)
}

func tunnelProxyICMPRoundTrip(ctx context.Context, peer *tunnelAccountingPeer, gateway net.IP, size int) error {
	request := tunnelProxyICMPPacket(peer.negotiatedAddr, gateway, size)
	for range 32 {
		if err := ctx.Err(); err != nil {
			return err
		}
		peer.sendPPP(0x0021, request)
		packet, err := peer.readPacket(250 * time.Millisecond)
		if err != nil {
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				continue
			}
			return fmt.Errorf("read proxy ICMP reply: %w", err)
		}
		protocol, payload, ok := tunnelL2TPPPPData(packet)
		if !ok || protocol != 0x0021 {
			if err := peer.handleAccountingPacket(packet); err != nil {
				return err
			}
			continue
		}
		if len(payload) > size {
			return fmt.Errorf("proxy transmitted %d Information-field octets, exceeding %d", len(payload), size)
		}
		if len(payload) < 28 || payload[0]>>4 != 4 || payload[9] != 1 {
			continue
		}
		header := int(payload[0]&0x0f) * 4
		length := int(binary.BigEndian.Uint16(payload[2:4]))
		if header < 20 || length > len(payload) || header+8 > length {
			return errors.New("proxy returned a malformed IPv4 ICMP packet")
		}
		icmp := payload[header:length]
		if icmp[0] != 0 || icmp[1] != 0 || !bytes.Equal(payload[12:16], gateway) || !bytes.Equal(payload[16:20], peer.negotiatedAddr.To4()) {
			continue
		}
		if len(payload) != size || !bytes.Equal(icmp[4:], request[24:]) || tunnelProxyChecksum(icmp) != 0 {
			return errors.New("proxy ICMP reply lost the request identifier, sequence, checksum or payload")
		}
		return nil
	}
	return errors.New("accepted proxy session did not return the full-sized ICMP Echo-Reply")
}

func tunnelProxyICMPPacket(source, destination net.IP, size int) []byte {
	packet := make([]byte, size)
	packet[0] = 0x45
	binary.BigEndian.PutUint16(packet[2:4], uint16(size))
	binary.BigEndian.PutUint16(packet[4:6], 0x16b2)
	packet[8], packet[9] = 64, 1
	copy(packet[12:16], source.To4())
	copy(packet[16:20], destination.To4())
	binary.BigEndian.PutUint16(packet[10:12], tunnelProxyChecksum(packet[:20]))
	icmp := packet[20:]
	icmp[0] = 8
	binary.BigEndian.PutUint16(icmp[4:6], 0x16b2)
	binary.BigEndian.PutUint16(icmp[6:8], 1)
	for i := 8; i < len(icmp); i++ {
		icmp[i] = byte(i)
	}
	binary.BigEndian.PutUint16(icmp[2:4], tunnelProxyChecksum(icmp))
	return packet
}

func tunnelProxyChecksum(data []byte) uint16 {
	var sum uint32
	for len(data) >= 2 {
		sum += uint32(binary.BigEndian.Uint16(data[:2]))
		data = data[2:]
	}
	if len(data) != 0 {
		sum += uint32(data[0]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
