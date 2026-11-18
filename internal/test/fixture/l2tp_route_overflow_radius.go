// Design: docs/guide/l2tp.md -- RADIUS framed-route overflow reaches the daemon.
// Related: tunnel_fixture_l2tp_radius.go -- existing signed RADIUS wire answer.
// Related: l2tp_route_overflow_fixture.go -- two genuine PPP admissions.
package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"
)

// routeOverflowRadius is an independent synthetic RADIUS server. It gives the
// first real access port a useful64-route profile and the second65 routes.
// NAS-Port alone is only the wire SID and can collide across tunnels; use the
// explicitly configured NAS-Port-Id as opaque server input, never parse a Birth
// or manufacture an admission. Retransmissions retain their original profile.
func routeOverflowRadius(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return errors.New("route-overflow RADIUS takes <port> <shared-secret>")
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // owned fixture socket
	profiles := make(map[string]int, 2)
	buffer := make([]byte, 4096)
	for ctx.Err() == nil {
		if err := conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
			return err
		}
		count, remote, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if timeout, ok := errors.AsType[net.Error](err); ok {
				if timeout.Timeout() {
					continue
				}
			}
			return err
		}
		if count < 20 {
			return errors.New("RADIUS request has no complete header")
		}
		length := int(binary.BigEndian.Uint16(buffer[2:4]))
		if length != count {
			return errors.New("RADIUS request length differs from its datagram")
		}
		request := buffer[:count]
		code := byte(5)
		var attributes []byte
		switch request[0] {
		case 1:
			key, found := tunnelRadiusValue(tunnelRadiusDescribe(request), "NAS-Port-Id")
			if !found {
				return errors.New("real Access-Request has no configured NAS-Port-Id")
			}
			if key == "" {
				return errors.New("real Access-Request has an empty NAS-Port-Id")
			}
			profile, exists := profiles[key]
			if !exists {
				if len(profiles) == 2 {
					return errors.New("RADIUS fixture saw more than its two real access ports")
				}
				profile = len(profiles)
				profiles[key] = profile
			}
			code = 2
			attributes = routeOverflowAttributes(profile)
		case 4:
			// Accounting is answered but does not decide this test's verdict.
		default:
			return fmt.Errorf("unexpected RADIUS code %d", request[0])
		}
		response := tunnelRadiusAnswer(code, request, attributes, []byte(args[1]))
		if _, err := conn.WriteToUDP(response, remote); err != nil {
			return err
		}
	}
	return nil
}

func routeOverflowAttributes(profile int) []byte {
	address := net.IPv4(10, 99, 9, byte(10+profile)).To4()
	attributes := tunnelRadiusAttr(8, address)
	attributes = append(attributes, tunnelRadiusUint32Attr(6, 2)...)
	attributes = append(attributes, tunnelRadiusUint32Attr(7, 1)...)
	for index := range 64 + profile {
		attributes = append(attributes, tunnelRadiusAttr(22, []byte(routeOverflowPrefix(profile, index).String()+" 0.0.0.0 7"))...)
	}
	return attributes
}

// profile and index come only from this fixture's two bounded declarations.
func routeOverflowPrefix(profile, index int) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(64 + profile), byte(index), 0}), 24)
}
