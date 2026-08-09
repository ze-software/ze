// Design: docs/architecture/provisioning/dhcp-server.md -- Non-Linux DHCP socket fallback

//go:build !linux

package dhcpserver

import (
	"fmt"
	"net"
)

func listenDHCP(ifaceName string) (*net.UDPConn, error) {
	addr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 67,
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("bind %s:67: %w", ifaceName, err)
	}
	return conn, nil
}
