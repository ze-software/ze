//go:build linux

// Design: docs/labs/pppoe-interop.md -- bounded exact-session family probes.
// Related: product_ipv6_check.go -- normal pppd owns discovery, auth and NCP.
package testvpppppoe

import (
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

type productFamilyWire struct {
	Request, Response []byte
	Received          bool
}

func productFamilyProbe(device, remote, sidText, operation, source, target string) (result productFamilyWire, err error) {
	link, err := net.InterfaceByName(device)
	if err != nil {
		return result, err
	}
	peer, err := net.ParseMAC(remote)
	if err != nil || len(peer) != 6 {
		return result, errors.New("family probe has no Ethernet AC")
	}
	sid, err := strconv.ParseUint(sidText, 10, 16)
	if err != nil || sid == 0 {
		return result, errors.New("family probe has no negotiated SID")
	}
	marker := []byte("ze-product-family-probe")
	var protocol uint16
	payload := make([]byte, 0, 48+len(marker))
	switch operation {
	case "close4":
		protocol = 0x8021
		payload = append(payload, 5, 0x77, 0, 4)
	case "close6":
		protocol = 0x8057
		payload = append(payload, 5, 0x77, 0, 4)
	case "probe4", "probe6":
		src, e1 := netip.ParseAddr(source)
		dst, e2 := netip.ParseAddr(target)
		if e1 != nil || e2 != nil || src.Is4() != dst.Is4() {
			return result, errors.New("family probe addresses invalid")
		}
		echo := append([]byte{8, 0, 0, 0, 0x70, 0x70, 0, 1}, marker...)
		if operation == "probe4" {
			if !src.Is4() {
				return result, errors.New("IPv4 probe requires IPv4")
			}
			protocol = 0x21
			binary.BigEndian.PutUint16(echo[2:4], productFamilyChecksum(echo))
			payload = payload[:20]
			payload[0], payload[8], payload[9] = 0x45, 64, 1
			binary.BigEndian.PutUint16(payload[2:4], uint16(20+len(echo)))
			a, b := src.As4(), dst.As4()
			copy(payload[12:16], a[:])
			copy(payload[16:20], b[:])
			binary.BigEndian.PutUint16(payload[10:12], productFamilyChecksum(payload))
		} else {
			if !src.Is6() || src.Is4In6() {
				return result, errors.New("IPv6 probe requires native IPv6")
			}
			protocol = 0x57
			echo[0] = 128
			payload = payload[:40]
			payload[0], payload[6], payload[7] = 0x60, 58, 64
			binary.BigEndian.PutUint16(payload[4:6], uint16(len(echo)))
			a, b := src.As16(), dst.As16()
			copy(payload[8:24], a[:])
			copy(payload[24:40], b[:])
			pseudo := append([]byte(nil), payload[8:40]...)
			pseudo = append(pseudo, 0, 0, 0, byte(len(echo)), 0, 0, 0, 58)
			pseudo = append(pseudo, echo...)
			binary.BigEndian.PutUint16(echo[2:4], productFamilyChecksum(pseudo))
		}
		payload = append(payload, echo...)
	default:
		return result, errors.New("family probe operation is unknown")
	}
	frame := make([]byte, 22, 22+len(payload))
	copy(frame[:6], peer)
	copy(frame[6:12], link.HardwareAddr)
	binary.BigEndian.PutUint16(frame[12:14], 0x8864)
	frame[14] = 0x11
	binary.BigEndian.PutUint16(frame[16:18], uint16(sid))
	binary.BigEndian.PutUint16(frame[18:20], uint16(2+len(payload)))
	binary.BigEndian.PutUint16(frame[20:22], protocol)
	frame = append(frame, payload...)
	result.Request = frame
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(etherTypeAllNetwork))
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, unix.Close(fd)) }()
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: link.Index, Protocol: etherTypeAllNetwork}); err != nil {
		return result, err
	}
	var mac [8]byte
	copy(mac[:], peer)
	if err := unix.Sendto(fd, frame, 0, &unix.SockaddrLinklayer{Ifindex: link.Index, Halen: 6, Addr: mac}); err != nil {
		return result, err
	}
	deadline := time.Now().Add(4 * time.Second)
	var buffer [2048]byte
	reply, observeErr := productFamilyObserve(time.Now, deadline, func(wait time.Duration) ([]byte, error) {
		milliseconds := int((wait + time.Millisecond - 1) / time.Millisecond)
		polled, err := unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, milliseconds)
		if err != nil {
			return nil, err
		}
		if polled == 0 {
			return nil, nil
		}
		n, _, err := unix.Recvfrom(fd, buffer[:], 0)
		if errors.Is(err, unix.EAGAIN) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return buffer[:n], nil
	}, frame, 1024)
	if observeErr != nil {
		return result, observeErr
	}
	if len(reply) > 0 {
		size := int(binary.BigEndian.Uint16(reply[18:20]))
		result.Response = reply[:20+size]
		result.Received = true
	}
	// Only a completed healthy receive window can establish response absence.
	// Packet-budget exhaustion and native read failures remain explicit errors.
	return result, nil
}
