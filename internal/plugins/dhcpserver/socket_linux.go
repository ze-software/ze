// Design: docs/architecture/provisioning/dhcp-server.md -- Linux SO_BINDTODEVICE for interface-specific DHCP

//go:build linux

package dhcpserver

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

func listenDHCP(ifaceName string) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var sErr error
			err := c.Control(func(fd uintptr) {
				sErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, ifaceName)
			})
			if err != nil {
				return err
			}
			return sErr
		},
	}

	pc, err := lc.ListenPacket(context.Background(), "udp4", ":67")
	if err != nil {
		return nil, fmt.Errorf("bind %s:67: %w", ifaceName, err)
	}

	conn, ok := pc.(*net.UDPConn)
	if !ok {
		if closeErr := pc.Close(); closeErr != nil {
			return nil, fmt.Errorf("bind %s:67: unexpected conn type, close: %w", ifaceName, closeErr)
		}
		return nil, fmt.Errorf("bind %s:67: unexpected conn type", ifaceName)
	}
	return conn, nil
}
