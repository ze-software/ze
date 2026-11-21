//go:build !linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the IKE bypass on Ze's own IKE sockets
// Related: ike_socket_bypass_linux.go -- the platform that sets it

package dataplane

import "net"

// BypassIKESocket has nothing to set off Linux. The only backend that programs a
// kernel SPD is the XFRM one, which is Linux only, so no policy exists on this
// platform that could capture an IKE socket's traffic, and no per-socket IPsec
// policy option exists to set.
func BypassIKESocket(*net.UDPConn) error {
	return nil
}
