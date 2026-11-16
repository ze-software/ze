// Design: docs/labs/pppoe-interop.md -- independent CPE RS and DHCPv6 stimulus.
// Related: product_ipv6_wire.go -- every accepted answer comes from received bytes.
package testvpppppoe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"golang.org/x/net/ipv6"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

func answerProductIPv6Peer(args leaction.Arguments) (any, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := productIPv6Peer(ctx, args.One("interface"), args.One("local"))
	return productIPv6Answer(result, err)
}

func productIPv6Peer(ctx context.Context, device, local string) (result productIPv6WireResult, err error) {
	link, err := net.InterfaceByName(device)
	if err != nil {
		return result, err
	}
	address, err := netip.ParseAddr(local)
	if err != nil || !address.IsLinkLocalUnicast() {
		return result, errors.New("CPE needs the negotiated link-local address")
	}
	result.Device, result.Local = device, address
	// Both sockets live in the independent client container and bind its pppN.
	// Ze is reached exclusively through that negotiated peer's PPPoE session.
	var listen net.ListenConfig
	raw, err := listen.ListenPacket(ctx, "ip6:ipv6-icmp", address.String()+"%"+device)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, raw.Close()) }()
	icmp := ipv6.NewPacketConn(raw)
	if err := icmp.SetControlMessage(ipv6.FlagInterface|ipv6.FlagHopLimit, true); err != nil {
		return result, err
	}
	if err := icmp.SetMulticastInterface(link); err != nil {
		return result, err
	}
	if err := icmp.SetMulticastHopLimit(255); err != nil {
		return result, err
	}
	if _, err := icmp.WriteTo([]byte{133, 0, 0, 0, 0, 0, 0, 0}, &ipv6.ControlMessage{IfIndex: link.Index, HopLimit: 255}, &net.IPAddr{IP: net.ParseIP("ff02::2"), Zone: device}); err != nil {
		return result, err
	}
	if err := raw.SetReadDeadline(time.Now().Add(12 * time.Second)); err != nil {
		return result, err
	}
	var buffer [2048]byte
	for {
		n, cm, source, readErr := icmp.ReadFrom(buffer[:])
		if readErr != nil {
			return result, fmt.Errorf("receive RA after RS: %w", readErr)
		}
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if n == 0 || buffer[0] != 134 {
			continue
		}
		peer, ok := source.(*net.IPAddr)
		if !ok {
			return result, errors.New("RA has no IP source")
		}
		a, ok := netip.AddrFromSlice(peer.IP)
		if !ok || cm == nil || cm.IfIndex != link.Index {
			return result, errors.New("RA is not on the negotiated CPE link")
		}
		a = a.Unmap()
		if err := productIPv6RA(buffer[:n], a, cm.HopLimit); err != nil {
			return result, err
		}
		result.RA = append([]byte(nil), buffer[:n]...)
		result.Router, result.RAHop = a, cm.HopLimit
		break
	}
	udp, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.ParseIP(address.String()), Port: 546, Zone: device})
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, udp.Close()) }()
	if err := ipv6.NewPacketConn(udp).SetMulticastInterface(link); err != nil {
		return result, err
	}
	ethernet, err := net.InterfaceByName(accessDevice)
	if err != nil {
		return result, err
	}
	if len(ethernet.HardwareAddr) != 6 {
		return result, errors.New("CPE has no Ethernet DUID source")
	}
	result.ClientID = append([]byte{0, 3, 0, 1}, ethernet.HardwareAddr...)
	send := func(wire []byte) error {
		_, err := udp.WriteToUDP(wire, &net.UDPAddr{IP: net.ParseIP("ff02::1:2"), Port: 547, Zone: device})
		return err
	}
	receive := func(kind byte, xid [3]byte, wait time.Duration) ([]byte, error) {
		if err := udp.SetReadDeadline(time.Now().Add(wait)); err != nil {
			return nil, err
		}
		// A finite byte/packet budget prevents an unrelated flood extending a probe.
		for range 64 {
			n, source, err := udp.ReadFromUDP(buffer[:])
			if err != nil {
				return nil, err
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if n < 4 || buffer[0] != kind || !bytes.Equal(buffer[1:4], xid[:]) {
				continue
			}
			a, ok := netip.AddrFromSlice(source.IP)
			if !ok || a.Unmap() != result.Router || source.Port != 547 || source.Zone != device {
				return nil, errors.New("DHCPv6 answer has another server/link")
			}
			return append([]byte(nil), buffer[:n]...), nil
		}
		return nil, errors.New("DHCPv6 receive packet budget exhausted")
	}
	solicit := productIPv6Request(1, [3]byte{1, 2, 3}, result.ClientID, nil)
	if err := send(solicit); err != nil {
		return result, err
	}
	result.Advertise, err = receive(2, [3]byte{1, 2, 3}, 8*time.Second)
	if err != nil {
		return result, err
	}
	offered, err := productIPv6Lease(result.Advertise, 2, [3]byte{1, 2, 3}, result.ClientID, nil)
	if err != nil {
		return result, err
	}
	options, err := productIPv6Options(result.Advertise[4:])
	if err != nil {
		return result, err
	}
	result.ServerID = append([]byte(nil), options[2]...)
	wrong := append([]byte(nil), result.ServerID...)
	wrong[len(wrong)-1] ^= 0xff
	if err := send(productIPv6Request(3, [3]byte{7, 8, 9}, result.ClientID, wrong)); err != nil {
		return result, err
	}
	_, err = receive(7, [3]byte{7, 8, 9}, 2*time.Second)
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		return result, fmt.Errorf("wrong Server-ID Request was answered or probe failed: %w", err)
	}
	result.WrongServerIgnored = true
	// An option header declaring ten bytes with one available is malformed.
	if err := send([]byte{1, 10, 11, 12, 0, 1, 0, 10, 0}); err != nil {
		return result, err
	}
	_, err = receive(2, [3]byte{10, 11, 12}, 2*time.Second)
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		return result, fmt.Errorf("malformed Solicit was answered or probe failed: %w", err)
	}
	result.MalformedIgnored = true
	request := productIPv6Request(3, [3]byte{4, 5, 6}, result.ClientID, result.ServerID)
	if err := send(request); err != nil {
		return result, err
	}
	result.Reply, err = receive(7, [3]byte{4, 5, 6}, 8*time.Second)
	if err != nil {
		return result, err
	}
	result.Binding, err = productIPv6Lease(result.Reply, 7, [3]byte{4, 5, 6}, result.ClientID, result.ServerID)
	if err != nil {
		return result, err
	}
	if result.Binding != offered {
		return result, errors.New("request changed offered IA_NA/IA_PD binding")
	}
	return result, nil
}

func productIPv6Request(kind byte, xid [3]byte, client, server []byte) []byte {
	wire := []byte{kind, xid[0], xid[1], xid[2]}
	wire = productIPv6Option(wire, 1, client)
	if len(server) > 0 {
		wire = productIPv6Option(wire, 2, server)
	}
	wire = productIPv6Option(wire, 3, []byte{0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0})
	wire = productIPv6Option(wire, 25, []byte{0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0})
	return wire
}
