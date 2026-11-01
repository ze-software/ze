//go:build !linux

// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- authenticated endpoint mobility
// Related: mobike.go -- mobikeRouteSource, the one caller
// Related: mobike_route_linux.go -- the Linux lookup, which must not cross the SPD

package engine

import (
	"fmt"
	"net"
)

// routeSourceTo returns the source address the routing table picks for remote.
// Off Linux no XFRM SPD exists to refuse the connect, so the connected socket's
// local address is the routing table's answer.
func routeSourceTo(remote net.IP) (net.IP, error) {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: remote, Port: 4500})
	if err != nil {
		return nil, err
	}
	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if err := conn.Close(); err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("ike: MOBIKE route has no UDP source")
	}
	return local.IP, nil
}
