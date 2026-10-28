// Design: docs/architecture/provisioning/dhcp-server.md -- Reply delivery to a client with no address

//go:build linux

package dhcpserver

import (
	"testing"

	"github.com/vishvananda/netlink"
)

// TestOperatorHeldNeighborStates proves readNeighbor's state test keeps the two
// states ARP never learns, PERMANENT (an operator's) and NOARP (an
// operator's, or the kernel's for a multicast or broadcast address and on a
// device that does no ARP), and lets ze replace every state ARP or ze itself
// produces. Method: ask neighborHeld for each NUD state netlink names.
// PREVENTS: a DHCP reply overwriting a nud noarp entry with ze's reachable
// one.
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
		if got := neighborHeld(c.state); got != c.want {
			t.Errorf("neighborHeld(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
