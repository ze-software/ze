// Design: docs/architecture/provisioning/dhcp-server.md -- Reply delivery to a client with no address

//go:build linux

package dhcpserver

import (
	"testing"

	"github.com/vishvananda/netlink"
)

// TestOperatorHeldNeighborStates proves readNeighbor's state test keeps both
// entries only an operator writes, PERMANENT and NOARP, and lets ze replace
// every state the kernel or ze itself produces. Method: ask operatorHeld for
// each NUD state netlink names.
// PREVENTS: a DHCP reply overwriting an operator's nud noarp entry with ze's
// reachable one.
func TestOperatorHeldNeighborStates(t *testing.T) {
	cases := []struct {
		name  string
		state int
		want  bool
	}{
		{"PERMANENT", netlink.NUD_PERMANENT, true},
		{"NOARP", netlink.NUD_NOARP, true},
		{"REACHABLE", netlink.NUD_REACHABLE, false},
		{"STALE", netlink.NUD_STALE, false},
		{"DELAY", netlink.NUD_DELAY, false},
		{"PROBE", netlink.NUD_PROBE, false},
		{"INCOMPLETE", netlink.NUD_INCOMPLETE, false},
		{"FAILED", netlink.NUD_FAILED, false},
		{"NONE", netlink.NUD_NONE, false},
	}
	for _, c := range cases {
		if got := operatorHeld(c.state); got != c.want {
			t.Errorf("operatorHeld(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
