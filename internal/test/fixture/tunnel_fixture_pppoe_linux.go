//go:build linux

package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func init() {
	Register("pppoe/basic", tunnelPPPoEBasic)
	Register("pppoe/concurrent-l2tp", tunnelPPPoEConcurrent)
	Register("pppoe/vlan", tunnelPPPoEVLAN)
}

type tunnelPPPoEWire struct {
	fd      int
	ifindex int
	mac     net.HardwareAddr
}

func tunnelPPPoEOpen(interfaceName string) (*tunnelPPPoEWire, error) {
	link, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, err
	}
	protocol := tunnelNetwork16(tunnelPPPoEEtherType)
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(protocol))
	if err != nil {
		return nil, fmt.Errorf("open PPPoE packet socket: %w", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: protocol, Ifindex: link.Index}); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("bind PPPoE packet socket to %s: %w", interfaceName, err)
	}
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 1}); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return &tunnelPPPoEWire{fd: fd, ifindex: link.Index, mac: append(net.HardwareAddr(nil), link.HardwareAddr...)}, nil
}

func tunnelNetwork16(value uint16) uint16 { return value<<8 | value>>8 }

func (wire *tunnelPPPoEWire) close() { _ = unix.Close(wire.fd) }

func (wire *tunnelPPPoEWire) send(destination net.HardwareAddr, payload []byte) error {
	frame := make([]byte, 14+len(payload))
	copy(frame[:6], destination)
	copy(frame[6:12], wire.mac)
	binary.BigEndian.PutUint16(frame[12:14], tunnelPPPoEEtherType)
	copy(frame[14:], payload)
	var address [8]byte
	copy(address[:], destination)
	return unix.Sendto(wire.fd, frame, 0, &unix.SockaddrLinklayer{
		Protocol: tunnelNetwork16(tunnelPPPoEEtherType), Ifindex: wire.ifindex,
		Halen: 6, Addr: address,
	})
}

func (wire *tunnelPPPoEWire) read() (net.HardwareAddr, byte, uint16, map[uint16][]byte, error) {
	buffer := make([]byte, 1518)
	for {
		n, _, err := unix.Recvfrom(wire.fd, buffer, 0)
		if err != nil {
			return nil, 0, 0, nil, err
		}
		frame := buffer[:n]
		if len(frame) < 20 || bytes.Equal(frame[6:12], wire.mac) {
			continue
		}
		payload := frame[14:]
		length := int(binary.BigEndian.Uint16(payload[4:6]))
		if 6+length > len(payload) {
			continue
		}
		return append(net.HardwareAddr(nil), frame[6:12]...), payload[1], binary.BigEndian.Uint16(payload[2:4]), tunnelPPPoEParseTags(payload[6 : 6+length]), nil
	}
}

func (wire *tunnelPPPoEWire) exchange(ctx context.Context, destination net.HardwareAddr, payload []byte, wanted byte, attempts int) (net.HardwareAddr, uint16, map[uint16][]byte, error) {
	for range attempts {
		if err := ctx.Err(); err != nil {
			return nil, 0, nil, err
		}
		if err := wire.send(destination, payload); err != nil {
			return nil, 0, nil, err
		}
		source, code, sid, tags, err := wire.read()
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				continue
			}
			return nil, 0, nil, err
		}
		if code == wanted {
			return source, sid, tags, nil
		}
		fmt.Fprintf(os.Stderr, "ignoring unexpected code 0x%02x\n", code)
	}
	return nil, 0, nil, errors.New("exchange attempt budget exhausted")
}

func tunnelPPPoEDiscover(ctx context.Context, wire *tunnelPPPoEWire, hostUniq []byte) (net.HardwareAddr, map[uint16][]byte, error) {
	broadcast := net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	server, _, tags, err := wire.exchange(ctx, broadcast, tunnelPPPoEPacket(tunnelPPPoEPADI, nil, hostUniq), tunnelPPPoEPADO, 15)
	return server, tags, err
}

func tunnelPPPoEBasic(ctx context.Context, _ []string) error {
	interfaceName := os.Getenv("TEST_IFACE")
	if interfaceName == "" {
		interfaceName = "veth-sub"
	}
	wire, err := tunnelPPPoEOpen(interfaceName)
	if err != nil {
		return err
	}
	defer wire.close()
	server, tags, err := tunnelPPPoEDiscover(ctx, wire, []byte{0x42, 0x42})
	if err != nil {
		return errors.New("no PADO received")
	}
	if tags[tunnelPPPoEACName] == nil {
		return errors.New("PADO missing AC-Name")
	}
	if tags[tunnelPPPoEACCookie] == nil {
		return errors.New("PADO missing AC-Cookie")
	}
	fmt.Printf("OK: PADO received, AC-Name=%s\n", tags[tunnelPPPoEACName])
	var sid uint16
	for range 6 {
		_, sid, _, err = wire.exchange(ctx, server, tunnelPPPoEPacket(tunnelPPPoEPADR, tags[tunnelPPPoEACCookie], []byte{0x42, 0x42}), tunnelPPPoEPADS, 2)
		if err == nil {
			break
		}
		server, tags, err = tunnelPPPoEDiscover(ctx, wire, []byte{0x42, 0x42})
		if err != nil {
			return errors.New("AC stopped answering PADI while retrying PADR")
		}
	}
	if err != nil {
		return errors.New("no PADS received for valid AC-Cookie")
	}
	if sid == 0 {
		return errors.New("PADS session ID is 0")
	}
	fmt.Printf("OK: PADS received, session-id=%d\n", sid)
	forged := tunnelPPPoEPacket(tunnelPPPoEPADR, make([]byte, 32), []byte{0x43, 0x43})
	if err := wire.send(server, forged); err != nil {
		return err
	}
	_, code, badSID, _, readErr := wire.read()
	if readErr == nil {
		return fmt.Errorf("forged cookie answered with code 0x%02x sid=%d", code, badSID)
	}
	if !errors.Is(readErr, unix.EAGAIN) && !errors.Is(readErr, unix.EWOULDBLOCK) {
		return readErr
	}
	fmt.Println("OK: forged AC-Cookie PADR silently dropped")
	return nil
}

func tunnelPPPoEVLAN(ctx context.Context, _ []string) error {
	interfaceName := os.Getenv("TEST_IFACE")
	if interfaceName == "" {
		interfaceName = "veth-sub.100"
	}
	wire, err := tunnelPPPoEOpen(interfaceName)
	if err != nil {
		return err
	}
	defer wire.close()
	server, tags, err := tunnelPPPoEDiscover(ctx, wire, []byte{0x44, 0x44})
	if err != nil {
		return fmt.Errorf("no PADO received on %s", interfaceName)
	}
	if tags[tunnelPPPoEACCookie] == nil {
		return errors.New("PADO missing AC-Cookie")
	}
	fmt.Println("OK: PADO received on VLAN interface")
	var sid uint16
	for range 6 {
		_, sid, _, err = wire.exchange(ctx, server, tunnelPPPoEPacket(tunnelPPPoEPADR, tags[tunnelPPPoEACCookie], []byte{0x44, 0x44}), tunnelPPPoEPADS, 2)
		if err == nil {
			break
		}
		server, tags, err = tunnelPPPoEDiscover(ctx, wire, []byte{0x44, 0x44})
		if err != nil {
			return errors.New("AC stopped answering PADI while retrying PADR")
		}
	}
	if err != nil {
		return errors.New("no PADS received on VLAN interface")
	}
	if sid == 0 {
		return errors.New("PADS session ID is 0")
	}
	fmt.Printf("OK: PADS received on VLAN, session-id=%d\n", sid)
	return nil
}

func tunnelPPPoEConcurrent(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return err
	}
	reply, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(0x0abc, "concurrent-test", nil), 15, time.Second)
	_ = conn.Close()
	if err != nil {
		return errors.New("no L2TP SCCRP received")
	}
	avps, err := tunnelL2TPParseAVPs(reply)
	if err != nil || tunnelL2TPMessageType(avps) != 2 {
		return errors.New("no L2TP SCCRP received")
	}
	fmt.Println("OK: L2TP SCCRP received")
	interfaceName := os.Getenv("TEST_IFACE")
	if interfaceName == "" {
		interfaceName = "veth-sub"
	}
	wire, err := tunnelPPPoEOpen(interfaceName)
	if err != nil {
		return err
	}
	defer wire.close()
	if _, _, err := tunnelPPPoEDiscover(ctx, wire, []byte{0x45, 0x45}); err != nil {
		return errors.New("no PPPoE PADO received")
	}
	fmt.Println("OK: PPPoE PADO received")
	fmt.Println("OK: both subsystems active concurrently")
	return nil
}
