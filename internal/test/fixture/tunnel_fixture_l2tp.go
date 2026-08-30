package fixture

import (
	"context"
	"crypto/md5" //nolint:gosec // L2TP CHAP and RADIUS require MD5 on the wire
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const tunnelL2TPSecret = "s3cr3t"

func tunnelL2TPTestSecret() []byte {
	secret := os.Getenv("TEST_SECRET")
	if secret == "" {
		secret = tunnelL2TPSecret
	}
	return []byte(secret)
}

func init() {
	Register("l2tp/auth-local-config", tunnelL2TPHandshakeDriver(0x0400, "py-auth", "aabbccddeeff00112233445566778899", "OK: tunnel established with auth-local config"))
	Register("l2tp/auth-radius-basic/radius", tunnelRadiusDriver(11812, 2, nil))
	Register("l2tp/auth-radius-basic", tunnelL2TPHandshakeDriver(0x0500, "py-radius", "aabbccddeeff00112233445566778899", "OK: tunnel established with RADIUS auth config"))
	Register("l2tp/auth-radius-reject/radius", tunnelRadiusDriver(11813, 3, tunnelRadiusAttr(18, []byte("bad credentials"))))
	Register("l2tp/auth-radius-reject", tunnelL2TPHandshakeDriver(0x0600, "py-reject", "aabbccddeeff00112233445566778899", "OK: tunnel established with RADIUS reject config"))
	Register("l2tp/bad-challenge-response", tunnelL2TPBadChallenge)
	Register("l2tp/handshake-full", tunnelL2TPHandshakeDriver(0x0123, "py-peer", "00112233445566778899aabbccddeeff", "OK: SCCCN sent"))
	Register("l2tp/handshake-sccrq", tunnelL2TPHandshakeSCCRQ)
	Register("l2tp/lns-outgoing-call", tunnelL2TPOutgoingDriver(0x0c0c, 0x0d0d, true))
	Register("l2tp/pool-basic", tunnelL2TPHandshakeDriver(0x0401, "py-pool", "11223344556677889900aabbccddeeff", "OK: tunnel established with pool config"))
	Register("l2tp/pool-minimal-range", tunnelL2TPHandshakeDriver(0x0402, "py-exhaust", "ffeeddccbbaa00998877665544332211", "OK: tunnel established with single-address pool"))
	Register("l2tp/radius-acct-wire/radius", tunnelRadiusAccounting)
	Register("l2tp/radius-acct-wire", tunnelL2TPRadiusAccountingPeer)
	Register("l2tp/radius-coa-listener", tunnelRadiusCoA)
	Register("l2tp/radius-filter-rate/radius", tunnelRadiusDriver(11812, 2, append(tunnelRadiusAttr(11, []byte("rate:20mbit/5mbit")), tunnelRadiusUint32Attr(85, 120)...)))
	Register("l2tp/radius-filter-rate", tunnelL2TPHandshakeDriver(0x0602, "py-filterrate", "aabbccddeeff00112233445566778899", "OK: tunnel established with RADIUS filter-rate config"))
	Register("l2tp/radius-framed-ip/radius", tunnelRadiusDriver(11812, 2, append(tunnelRadiusAttr(8, net.IPv4(10, 0, 0, 5).To4()), tunnelRadiusAttr(88, []byte("gold"))...)))
	Register("l2tp/radius-framed-ip", tunnelL2TPHandshakeDriver(0x0603, "py-framedip", "aabbccddeeff00112233445566778899", "OK: tunnel established with RADIUS framed-ip + named pool config"))
	Register("l2tp/radius-session-timeout/radius", tunnelRadiusDriver(11812, 2, append(tunnelRadiusUint32Attr(27, 60), tunnelRadiusUint32Attr(28, 120)...)))
	Register("l2tp/radius-session-timeout", tunnelL2TPHandshakeDriver(0x0601, "py-timeout", "aabbccddeeff00112233445566778899", "OK: tunnel established with RADIUS session-timeout config"))
	Register("l2tp/rfc2661-emitted-control-shape", tunnelL2TPEmittedShape)
	Register("l2tp/rfc2661-sccrq-tunnel-id-zero", tunnelL2TPZeroTunnelID)
	Register("l2tp/session-auth-pool", tunnelL2TPSessionDriver(0x0403, 600, "0011223344556677aabbccddeeff8899", 1, "auth-pool"))
	Register("l2tp/session-cdn-teardown", tunnelL2TPSessionDriver(0x0123, 500, "00112233445566778899aabbccddeeff", 1, "cdn"))
	Register("l2tp/session-incoming-lns", tunnelL2TPSessionDriver(0x0123, 500, "00112233445566778899aabbccddeeff", 1, "incoming"))
	Register("l2tp/session-stopccn-cascade", tunnelL2TPSessionDriver(0x0123, 500, "00112233445566778899aabbccddeeff", 2, "stopccn"))
	Register("l2tp/tunnel-initiate-sccrq", tunnelL2TPOutgoingDriver(0x0a0a, 0x0b0b, false))
}

func tunnelArgPort(args []string, index int) (int, error) {
	if len(args) <= index {
		return 0, errors.New("missing port argument")
	}
	port, err := strconv.Atoi(args[index])
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port %q", args[index])
	}
	return port, nil
}

func tunnelL2TPAVP(mandatory bool, attribute uint16, value []byte) []byte {
	word := uint16(6 + len(value))
	if mandatory {
		word |= 0x8000
	}
	result := make([]byte, 6+len(value))
	binary.BigEndian.PutUint16(result[0:2], word)
	binary.BigEndian.PutUint16(result[4:6], attribute)
	copy(result[6:], value)
	return result
}

func tunnelL2TPU16(value uint16) []byte {
	result := make([]byte, 2)
	binary.BigEndian.PutUint16(result, value)
	return result
}

func tunnelL2TPU32(value uint32) []byte {
	result := make([]byte, 4)
	binary.BigEndian.PutUint32(result, value)
	return result
}

func tunnelL2TPControl(tid, sid, ns, nr uint16, body []byte) []byte {
	packet := make([]byte, 12+len(body))
	binary.BigEndian.PutUint16(packet[0:2], 0xc802)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	binary.BigEndian.PutUint16(packet[4:6], tid)
	binary.BigEndian.PutUint16(packet[6:8], sid)
	binary.BigEndian.PutUint16(packet[8:10], ns)
	binary.BigEndian.PutUint16(packet[10:12], nr)
	copy(packet[12:], body)
	return packet
}

func tunnelL2TPParseAVPs(packet []byte) (map[uint16][]byte, error) {
	if len(packet) < 12 {
		return nil, fmt.Errorf("control datagram is %d bytes, want at least 12", len(packet))
	}
	length := int(binary.BigEndian.Uint16(packet[2:4]))
	if length < 12 || length > len(packet) {
		return nil, fmt.Errorf("invalid control length %d for %d-byte datagram", length, len(packet))
	}
	result := make(map[uint16][]byte)
	for offset := 12; offset+6 <= length; {
		word := binary.BigEndian.Uint16(packet[offset : offset+2])
		vendor := binary.BigEndian.Uint16(packet[offset+2 : offset+4])
		attribute := binary.BigEndian.Uint16(packet[offset+4 : offset+6])
		avpLength := int(word & 0x03ff)
		if avpLength < 6 || offset+avpLength > length {
			return nil, fmt.Errorf("invalid AVP length %d at offset %d", avpLength, offset)
		}
		if vendor == 0 {
			result[attribute] = append([]byte(nil), packet[offset+6:offset+avpLength]...)
		}
		offset += avpLength
	}
	return result, nil
}

func tunnelL2TPMessageType(avps map[uint16][]byte) uint16 {
	if len(avps[0]) < 2 {
		return 0
	}
	return binary.BigEndian.Uint16(avps[0])
}

func tunnelL2TPSCCRQ(peerTID uint16, hostname string, challenge []byte) []byte {
	body := make([]byte, 0, 128)
	body = append(body, tunnelL2TPAVP(true, 0, tunnelL2TPU16(1))...)
	body = append(body, tunnelL2TPAVP(true, 2, []byte{1, 0})...)
	body = append(body, tunnelL2TPAVP(true, 3, tunnelL2TPU32(3))...)
	body = append(body, tunnelL2TPAVP(true, 4, tunnelL2TPU32(0))...)
	body = append(body, tunnelL2TPAVP(true, 7, []byte(hostname))...)
	body = append(body, tunnelL2TPAVP(true, 9, tunnelL2TPU16(peerTID))...)
	body = append(body, tunnelL2TPAVP(true, 10, tunnelL2TPU16(8))...)
	if challenge != nil {
		body = append(body, tunnelL2TPAVP(true, 11, challenge)...)
	}
	return tunnelL2TPControl(0, 0, 0, 0, body)
}

func tunnelL2TPDial(port int) (*net.UDPConn, *net.UDPAddr, error) {
	local := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	target := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}
	conn, err := net.ListenUDP("udp4", local)
	return conn, target, err
}

func tunnelL2TPExchange(ctx context.Context, conn *net.UDPConn, target *net.UDPAddr, packet []byte, attempts int, timeout time.Duration) ([]byte, *net.UDPAddr, error) {
	buffer := make([]byte, 4096)
	for range attempts {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		_ = conn.SetWriteDeadline(time.Now().Add(timeout))
		if _, err := conn.WriteToUDP(packet, target); err != nil {
			continue
		}
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		n, address, err := conn.ReadFromUDP(buffer)
		if err == nil {
			return append([]byte(nil), buffer[:n]...), address, nil
		}
	}
	return nil, nil, fmt.Errorf("no reply after %d attempts", attempts)
}

func tunnelL2TPHandshakeDriver(peerTID uint16, hostname, challengeHex, success string) Driver {
	return func(ctx context.Context, args []string) error {
		port, err := tunnelArgPort(args, 0)
		if err != nil {
			return err
		}
		challenge, err := tunnelHex(challengeHex)
		if err != nil {
			return err
		}
		conn, target, err := tunnelL2TPDial(port)
		if err != nil {
			return err
		}
		defer conn.Close() //nolint:errcheck // fixture teardown
		reply, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(peerTID, hostname, challenge), 20, 250*time.Millisecond)
		if err != nil {
			return fmt.Errorf("no SCCRP received: %w", err)
		}
		avps, err := tunnelL2TPParseAVPs(reply)
		if err != nil {
			return err
		}
		if tunnelL2TPMessageType(avps) != 2 || len(avps[9]) != 2 || len(avps[11]) == 0 {
			return errors.New("SCCRP missing Challenge or Assigned Tunnel ID")
		}
		localTID := binary.BigEndian.Uint16(avps[9])
		digest := md5.Sum( //nolint:gosec // RFC 2661 Section 4.4.3 makes the Challenge Response a CHAP value, and RFC 1994 CHAP is MD5
			append(append([]byte{3}, tunnelL2TPTestSecret()...), avps[11]...))
		body := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(3)), tunnelL2TPAVP(true, 13, digest[:])...)
		_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
		if _, err := conn.WriteToUDP(tunnelL2TPControl(localTID, 0, 1, 1, body), target); err != nil {
			return err
		}
		tunnelL2TPDrain(conn, time.Second)
		fmt.Println(success)
		return nil
	}
}

func tunnelHex(value string) ([]byte, error) {
	if len(value)%2 != 0 {
		return nil, errors.New("odd-length hexadecimal fixture value")
	}
	result := make([]byte, len(value)/2)
	for index := range result {
		part, err := strconv.ParseUint(value[index*2:index*2+2], 16, 8)
		if err != nil {
			return nil, err
		}
		result[index] = byte(part)
	}
	return result, nil
}

func tunnelL2TPHandshakeSCCRQ(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture teardown
	reply, address, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(0x0101, "py-peer", nil), 20, 250*time.Millisecond)
	if err != nil {
		return fmt.Errorf("no SCCRP after 20 attempts: %w", err)
	}
	if len(reply) < 20 || binary.BigEndian.Uint16(reply[:2]) != 0xc802 {
		return fmt.Errorf("invalid SCCRP header in %d-byte response", len(reply))
	}
	avps, err := tunnelL2TPParseAVPs(reply)
	if err != nil || binary.BigEndian.Uint16(reply[16:18]) != 0 || binary.BigEndian.Uint16(reply[18:20]) != 2 || tunnelL2TPMessageType(avps) != 2 {
		return errors.New("first AVP is not SCCRP")
	}
	fmt.Printf("OK: SCCRP received (%d bytes) from %s\n", len(reply), address)
	return nil
}

func tunnelL2TPBadChallenge(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	challenge, _ := tunnelHex("ffeeddccbbaa99887766554433221100")
	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture teardown
	reply, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(0x0234, "py-bad", challenge), 20, 250*time.Millisecond)
	if err != nil {
		return errors.New("no SCCRP received")
	}
	avps, err := tunnelL2TPParseAVPs(reply)
	if err != nil || len(avps[9]) != 2 {
		return errors.New("SCCRP missing Assigned Tunnel ID")
	}
	body := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(3)), tunnelL2TPAVP(true, 13, make([]byte, 16))...)
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.WriteToUDP(tunnelL2TPControl(binary.BigEndian.Uint16(avps[9]), 0, 1, 1, body), target)
	if err != nil {
		return err
	}
	buffer := make([]byte, 1500)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		return fmt.Errorf("no StopCCN received: %w", err)
	}
	stop, err := tunnelL2TPParseAVPs(buffer[:n])
	if err != nil || tunnelL2TPMessageType(stop) != 4 || len(stop[1]) < 2 || binary.BigEndian.Uint16(stop[1]) != 4 {
		return errors.New("StopCCN did not carry Result Code 4")
	}
	fmt.Println("OK: StopCCN RC=4 received")
	return nil
}
