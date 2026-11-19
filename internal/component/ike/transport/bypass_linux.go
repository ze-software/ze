//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the IKE bypass on Ze's own IKE sockets
// Overview: udp.go -- newTransport, which sets it on every IKE socket before the first datagram
// Related: bypass_other.go -- the platforms with no XFRM
// RFC: rfc/short/rfc4301.md -- RFC4301-5.2-4, the explicit BYPASS entry IKE traffic needs

package transport

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// The struct xfrm_userpolicy_info values the socket policy carries
// (include/uapi/linux/xfrm.h).
const (
	xfrmPolicyDirIn  = 0 // XFRM_POLICY_IN
	xfrmPolicyDirOut = 1 // XFRM_POLICY_OUT
	xfrmPolicyAllow  = 0 // XFRM_POLICY_ALLOW
)

// ikeSocketPolicyDirections are the two directions the bypass is set in, outbound
// first, the order strongSwan's add_socket_bypass uses.
var ikeSocketPolicyDirections = [...]uint8{xfrmPolicyDirOut, xfrmPolicyDirIn}

// bypassIKESocket exempts one IKE socket, and only that socket, from IPsec.
//
// RFC 4301 Section 5.2: "(This implies that IKE traffic MUST have an explicit BYPASS
// entry in the SPD.)"
//
// The entry is a per-socket policy, set with IP_XFRM_POLICY in each direction: an
// allow action with no template, which is the BYPASS disposition, and a zero selector
// of the socket's family, which matches every datagram the socket sends or receives.
// It is strongSwan's add_socket_bypass (kernel_netlink_ipsec.c), which charon's
// socket-default plugin runs on every IKE socket it opens.
//
// WHY THE SOCKET AND NOT THE SPD. The kernel consults a socket's own policy before
// any SPD entry and, when it matches, never reaches the SPD: xfrm_lookup_with_ifid
// takes sk->sk_policy[XFRM_POLICY_OUT] for an outbound datagram, and
// __xfrm_policy_check calls xfrm_policy_lookup only when the socket policy did not
// match (net/xfrm/xfrm_policy.c). UDP's inbound check runs with the socket known
// (udp_queue_rcv_one_skb, before the encapsulation handler). So the exemption
// covers exactly the datagrams that terminate on Ze's IKE sockets. A node-wide SPD
// entry on UDP source port 500 or 4500 also matched a protected host's transit
// datagram from those ports, which then left in clear where the tunnel should have
// carried it.
//
// The policy lives and dies with the socket, so nothing removes it at shutdown. It
// carries IKESocketPolicyPriority as a mark: xfrm_sk_policy_lookup matches no
// priority, and the policy listing names the row ike-bypass by that mark.
//
// The lifetime stays zero, as add_socket_bypass leaves it: xfrm_policy_timer expires a
// policy only on its add and use seconds, and zero there means never.
//
// The transport listens on "udp4", so the option is the IPv4 one; xfrm_compile_policy
// refuses IP_XFRM_POLICY on a socket of another family.
func bypassIKESocket(conn *net.UDPConn) error {
	sc, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("transport: IKE socket bypass: %w", err)
	}
	for _, dir := range ikeSocketPolicyDirections {
		policy := ikeSocketPolicy(unix.AF_INET, dir)
		var setErr error
		if ctlErr := sc.Control(func(fd uintptr) {
			setErr = unix.SetsockoptString(int(fd), unix.IPPROTO_IP, unix.IP_XFRM_POLICY, string(policy.Serialize()))
		}); ctlErr != nil {
			return fmt.Errorf("transport: IKE socket bypass: %w", ctlErr)
		}
		if setErr != nil {
			return fmt.Errorf("transport: setsockopt IP_XFRM_POLICY, direction %d: %w", dir, setErr)
		}
	}
	return nil
}

// ikeSocketPolicy is the policy bypassIKESocket sets in one direction: allow, no
// template, every address and port of family.
func ikeSocketPolicy(family uint16, dir uint8) nl.XfrmUserpolicyInfo {
	return nl.XfrmUserpolicyInfo{
		Sel:      nl.XfrmSelector{Family: family},
		Priority: IKESocketPolicyPriority,
		Dir:      dir,
		Action:   xfrmPolicyAllow,
	}
}
