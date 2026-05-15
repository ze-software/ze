// Design: docs/architecture/core-design.md -- TCP MD5 authentication (RFC 2385)
// Overview: network.go -- network abstraction layer

//go:build darwin

package network

import (
	"errors"
	"net"
)

var errTcpMd5AuthenticationRfc2385Is = errors.New("TCP MD5 authentication (RFC 2385) is not supported on macOS")

// setTCPMD5Sig returns an error on macOS.
// Darwin's kernel does not implement TCP_MD5SIG (RFC 2385).
func setTCPMD5Sig(_ int, _ net.IP, _ string) error {
	return errTcpMd5AuthenticationRfc2385Is
}

// tcpMD5Supported reports whether TCP MD5 is supported on this platform.
func tcpMD5Supported() bool { return false }
