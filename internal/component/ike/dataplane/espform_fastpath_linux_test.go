// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: espform_fastpath_linux.go -- espFormLinkVariant, the selection under test
//
// VALIDATES: the fast path's link selection (AC-7 of
// spec-ipsec-dual-form-receive-spd-bypass): an Ethernet link gets the program built
// for a 14-octet L2 header, a TUN, PPP or raw-IP link the program built for none, lo
// and ze-espform are skipped without a refusal, and every other type is refused with
// a reason naming it.
// PREVENTS: the L3 program on an Ethernet link (it would find no ESP and the fast path
// would silently serve nothing there), an attach to lo or ze-espform, and an
// unsupported link left out of the doctor report.
// METHOD: a table over the hardware types the kernel reports in ifi_type, and the
// two skipped names. A pure function: no privilege needed.

//go:build linux

package dataplane

import (
	"net"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// TestESPFormLinkSelection runs espFormLinkVariant over each link type and name the
// fast path can meet.
func TestESPFormLinkSelection(t *testing.T) {
	cases := []struct {
		label     string
		name      string
		linkType  uint16
		encapType string
		flags     net.Flags
		want      espFormLinkChoice
	}{
		{"ethernet link", "eth0", unix.ARPHRD_ETHER, "ether", net.FlagUp, espFormLinkEthernet},
		{"veth or dummy, which are Ethernet", "veth0", unix.ARPHRD_ETHER, "ether", 0, espFormLinkEthernet},
		{"TUN device in IFF_TUN mode", "tun0", unix.ARPHRD_NONE, "none", net.FlagUp | net.FlagPointToPoint, espFormLinkL3},
		{"PPP link", "ppp0", unix.ARPHRD_PPP, "ppp", net.FlagUp | net.FlagPointToPoint, espFormLinkL3},
		{"raw-IP link, which netlink cannot name", "rmnet_data0", unix.ARPHRD_RAWIP, "unknown519", net.FlagUp, espFormLinkL3},
		{"lo", "lo", unix.ARPHRD_LOOPBACK, "loopback", net.FlagUp | net.FlagLoopback, espFormLinkSkipped},
		{"a loopback type without the loopback flag", "lo1", unix.ARPHRD_LOOPBACK, "loopback", net.FlagUp, espFormLinkSkipped},
		{"the loopback flag on an Ethernet type", "lo2", unix.ARPHRD_ETHER, "ether", net.FlagLoopback, espFormLinkSkipped},
		{"ze-espform", espFormTUNName, unix.ARPHRD_NONE, "none", net.FlagUp | net.FlagPointToPoint, espFormLinkSkipped},
		{"ze-espform skipped by name whatever its type", espFormTUNName, unix.ARPHRD_ETHER, "ether", net.FlagUp, espFormLinkSkipped},
		{"IPIP tunnel", "ipip0", unix.ARPHRD_TUNNEL, "ipip", net.FlagUp, espFormLinkRefused},
		{"SIT tunnel", "sit0", unix.ARPHRD_SIT, "sit", net.FlagUp, espFormLinkRefused},
		{"GRE tunnel", "gre0", unix.ARPHRD_IPGRE, "gre", net.FlagUp, espFormLinkRefused},
		{"IPv6 tunnel", "ip6tnl0", unix.ARPHRD_TUNNEL6, "tunnel6", net.FlagUp, espFormLinkRefused},
		{"InfiniBand", "ib0", unix.ARPHRD_INFINIBAND, "infiniband", net.FlagUp, espFormLinkRefused},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got, reason := espFormLinkVariant(tc.name, tc.linkType, tc.encapType, tc.flags)
			if got != tc.want {
				t.Fatalf("espFormLinkVariant(%q, %d, %q, %v) = %d, want %d", tc.name, tc.linkType, tc.encapType, tc.flags, got, tc.want)
			}
			if tc.want != espFormLinkRefused {
				if reason != "" {
					t.Errorf("a link that is not refused carries the reason %q, want none", reason)
				}
				return
			}
			if !strings.Contains(reason, tc.encapType) {
				t.Errorf("refusal reason %q does not name the link type %q", reason, tc.encapType)
			}
			if !strings.Contains(reason, strconv.Itoa(int(tc.linkType))) {
				t.Errorf("refusal reason %q does not name the hardware type %d", reason, tc.linkType)
			}
		})
	}
}
