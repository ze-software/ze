// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Path MTU of a Child SA
// Related: rfc4301_pmtu_integration_linux_test.go -- the topology and the IPv4-outer legs
//
// VALIDATES: RFC 4301 Section 8.2.2 over an IPv6 outer pair: the PMTU a learns for
// the SA's outer path from an ICMPv6 Packet Too Big about an ESP packet (esp6_err,
// net/ipv6/esp6.c, calls ip6_update_pmtu; rt6_do_update_pmtu, net/ipv6/route.c,
// stamps the exception with net.ipv6.route.mtu_expires) expires after that
// period, the SA's PMTU is the first-hop link MTU again, and the next oversized
// packet relearns it.
// PREVENTS: an IPv6-outer SA whose learned PMTU never ages, which would keep the
// SA below a path MTU that has grown back.
//
// WHY integration && linux. net.ipv6.route.mtu_expires is per network namespace,
// but the kernel exports the IPv6 route table to a namespace another user
// namespace owns with max_size alone (ipv6_route_sysctl_table_size, net/ipv6/route.c:
// "Don't export sysctls to unprivileged users"), so userns.Enter cannot shorten the
// period and the test would wait out the 600-second default. A namespace the
// initial user namespace owns can, which takes root there: the QEMU guest
// (`./le test qemu all`) or `sudo ./le test integration`.

//go:build integration && linux

package dataplane

import (
	"os"
	"runtime"
	"testing"

	"github.com/vishvananda/netns"
)

// VALIDATES: RFC4301-8.2.2-1 and RFC4301-7-2 over an IPv6 outer pair.
// RFC requirement: RFC4301-8.2.2-1 positive -- the PMTU learned for an IPv6-outer SA's outer path is aged out after the configured period, and the next oversized packet relearns it.
// RFC requirement: RFC4301-7-2 positive -- after the configured period an IPv6-outer SA's PMTU is the first-hop link MTU again and normal PMTU discovery lowers it anew.
func TestXFRMTunnelPMTUAgesBackToTheLinkMTUOverIPv6(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root in the initial user namespace: net.ipv6.route.mtu_expires is not exported to a user-owned network namespace; run `./le test qemu all`")
	}
	// The thread moves between namespaces for the rest of the test and is never
	// unlocked, so the runtime ends it with this goroutine rather than reuse it.
	runtime.LockOSThread()
	origin := pmtuCurrent(t)
	base, err := netns.New()
	if err != nil {
		t.Fatalf("create the base namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := netns.Set(origin); err != nil {
			t.Errorf("return to the original namespace: %v", err)
		}
		_ = base.Close()
	})
	pmtuUp(t, "lo")
	n := pmtuBuild(t, false, true)
	pmtuAgeAndRelearn(t, n, "net/ipv6/route/mtu_expires", pmtuInner6)
}
