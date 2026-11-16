//go:build linux

// Design: docs/labs/pppoe-interop.md -- independent wire inputs to the VPP mechanism.
package testvpppppoe

import (
	"encoding/binary"
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

func injectFrames(device, remote string, ipv6 bool) (err error) {
	link, err := net.InterfaceByName(device)
	if err != nil {
		return err
	}
	peer, err := net.ParseMAC(remote)
	if err != nil {
		return err
	}
	if len(peer) != 6 {
		return errors.New("mechanism requires an Ethernet peer MAC")
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(network16(unix.ETH_P_ALL)))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, unix.Close(fd)) }()
	var destination [8]byte
	copy(destination[:], peer)
	send := func(frame []byte) error {
		return unix.Sendto(fd, frame, 0, &unix.SockaddrLinklayer{Ifindex: link.Index, Halen: 6, Addr: destination})
	}
	if ipv6 {
		if err := send(ipv6Frame(link.HardwareAddr, peer, false)); err != nil {
			return err
		}
		return send(ipv6Frame(link.HardwareAddr, peer, true))
	}
	// The Ethernet bytes physically include the IPv6 header and transport
	// fields. Only the PPPoE declared length is truncated. The unguarded
	// adopted parser reads past it and punts both packets to the CP TAP.
	for _, dhcp := range []bool{false, true} {
		frame := ipv6Frame(link.HardwareAddr, peer, dhcp)
		binary.BigEndian.PutUint16(frame[18:20], 2)
		if err := send(frame); err != nil {
			return err
		}
	}
	marker := []byte("bounds-barrier")
	frame := make([]byte, 24+len(marker))
	copy(frame[:6], []byte{255, 255, 255, 255, 255, 255})
	copy(frame[6:12], link.HardwareAddr)
	binary.BigEndian.PutUint16(frame[12:14], 0x8863)
	frame[14] = 0x11
	frame[15] = 0x09
	binary.BigEndian.PutUint16(frame[18:20], uint16(4+len(marker)))
	binary.BigEndian.PutUint16(frame[20:22], 0x0101)
	binary.BigEndian.PutUint16(frame[22:24], uint16(len(marker)))
	copy(frame[24:], marker)
	return send(frame)
}

// ipv6Frame constructs two valid subscriber-control inputs. IPv6 begins at
// byte22, after Ethernet14, PPPoE6 and PPP2; the transport begins at byte62.
// RFC2516 Section6: "The PPPoE payload contains a PPP frame."
func ipv6Frame(source, target net.HardwareAddr, dhcp bool) []byte {
	payloadLength := 8
	if dhcp {
		payloadLength = 12
	}
	frame := make([]byte, 62+payloadLength)
	copy(frame[:6], target)
	copy(frame[6:12], source)
	binary.BigEndian.PutUint16(frame[12:14], 0x8864)
	frame[14] = 0x11
	binary.BigEndian.PutUint16(frame[16:18], sessionID)
	binary.BigEndian.PutUint16(frame[18:20], uint16(42+payloadLength))
	binary.BigEndian.PutUint16(frame[20:22], 0x0057)
	ip := frame[22:62]
	ip[0] = 0x60
	binary.BigEndian.PutUint16(ip[4:6], uint16(payloadLength))
	ip[6] = 58
	ip[7] = 255
	ip[8] = 0xfe
	ip[9] = 0x80
	ip[23] = 2
	ip[24] = 0xff
	ip[25] = 2
	ip[39] = 2
	transport := frame[62:]
	transport[0] = 133
	checksumOffset := 2
	if dhcp {
		ip[6] = 17
		ip[7] = 1
		ip[37] = 1
		ip[39] = 2
		binary.BigEndian.PutUint16(transport[:2], 546)
		binary.BigEndian.PutUint16(transport[2:4], 547)
		binary.BigEndian.PutUint16(transport[4:6], 12)
		transport[8] = 1
		transport[9] = 1
		transport[10] = 2
		transport[11] = 3
		checksumOffset = 6
	}
	var sum uint32
	for offset := 8; offset < 40; offset += 2 {
		sum += uint32(binary.BigEndian.Uint16(ip[offset : offset+2]))
	}
	sum += uint32(payloadLength) + uint32(ip[6])
	for offset := 0; offset < len(transport); offset += 2 {
		sum += uint32(binary.BigEndian.Uint16(transport[offset : offset+2]))
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	binary.BigEndian.PutUint16(transport[checksumOffset:checksumOffset+2], ^uint16(sum))
	return frame
}
