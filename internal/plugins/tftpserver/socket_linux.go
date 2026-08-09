// Design: docs/architecture/provisioning/tftp-server.md -- Linux SO_BINDTODEVICE for interface-specific TFTP

//go:build linux

package tftpserver

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

func listenTFTP(ifaceName string) (*net.UDPConn, error) {
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

	pc, err := lc.ListenPacket(context.Background(), "udp4", ":69")
	if err != nil {
		return nil, fmt.Errorf("bind %s:69: %w", ifaceName, err)
	}

	conn, ok := pc.(*net.UDPConn)
	if !ok {
		if closeErr := pc.Close(); closeErr != nil {
			return nil, fmt.Errorf("bind %s:69: unexpected conn type, close: %w", ifaceName, closeErr)
		}
		return nil, fmt.Errorf("bind %s:69: unexpected conn type", ifaceName)
	}
	return conn, nil
}
