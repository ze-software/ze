// Design: docs/architecture/core-design.md -- TCP MD5 authentication (RFC 2385)
// Overview: network.go -- network abstraction layer

//go:build !linux && !freebsd && !darwin

package network

import (
	"errors"
	"net"
)

var errTcpMd5AuthenticationRfc2385Is = errors.New("TCP MD5 authentication (RFC 2385) is not supported on this platform")

// setTCPMD5Sig returns an error on unsupported platforms.
func setTCPMD5Sig(_ int, _ net.IP, _ string) error {
	return errTcpMd5AuthenticationRfc2385Is
}

// tcpMD5Supported reports whether TCP MD5 is supported on this platform.
func tcpMD5Supported() bool { return false }
