// Design: docs/architecture/provisioning/tftp-server.md -- Non-Linux TFTP socket fallback

//go:build !linux

package tftpserver

import (
	"fmt"
	"net"
)

func listenTFTP(ifaceName string) (*net.UDPConn, error) {
	addr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 69,
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("bind %s:69: %w", ifaceName, err)
	}
	return conn, nil
}
