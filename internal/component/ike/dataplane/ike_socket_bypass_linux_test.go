//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the IKE bypass on Ze's own IKE sockets
// Related: ike_socket_bypass_linux.go -- ikeSocketPolicy and BypassIKESocket, the producers

package dataplane

import (
	"testing"

	"golang.org/x/sys/unix"
)

// TestIKESocketPolicyIsATemplateFreeAllowOverEveryFlow checks the policy each IKE
// socket carries, as built, before the kernel sees it.
//
// VALIDATES: both directions are set, each an XFRM_POLICY_ALLOW over a zero selector
// of the socket's family, marked with PriorityIKESocket. The serialized form is the
// bare struct xfrm_userpolicy_info, so no template follows it: xfrm_compile_policy
// counts templates from the bytes after the struct.
// PREVENTS: a BLOCK that drops ze's own IKE, a selector narrowed to a port or address
// that stops matching after a NAT rebinding, a family the kernel refuses for an IPv4
// socket, and a mark the listing would name socket-override.
func TestIKESocketPolicyIsATemplateFreeAllowOverEveryFlow(t *testing.T) {
	seen := map[uint8]bool{}
	for _, dir := range ikeSocketPolicyDirections {
		p := ikeSocketPolicy(unix.AF_INET, dir)
		seen[p.Dir] = true
		if p.Action != xfrmPolicyActionAllow {
			t.Errorf("dir %d: action %d, want XFRM_POLICY_ALLOW", dir, p.Action)
		}
		if p.Sel.Family != unix.AF_INET {
			t.Errorf("dir %d: selector family %d, want AF_INET", dir, p.Sel.Family)
		}
		if p.Sel.PrefixlenS != 0 || p.Sel.PrefixlenD != 0 || p.Sel.SportMask != 0 || p.Sel.DportMask != 0 || p.Sel.Proto != 0 {
			t.Errorf("dir %d: selector %+v is narrowed; the socket's own traffic must match whatever its peer address", dir, p.Sel)
		}
		if p.Priority != PriorityIKESocket {
			t.Errorf("dir %d: priority %d, want the mark %d", dir, p.Priority, uint32(PriorityIKESocket))
		}
		if got := len(p.Serialize()); got != p.Len() {
			t.Errorf("dir %d: serialized %d octets, want the bare struct of %d (no template)", dir, got, p.Len())
		}
	}
	if !seen[xfrmPolicyDirIn] || !seen[xfrmPolicyDirOut] {
		t.Errorf("directions set = %v, want both in and out", seen)
	}
}
