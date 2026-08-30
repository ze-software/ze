//go:build linux

package ifacenetlink

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

// addrPayloadFor unmarshals the JSON payload of the first addr-added event in
// the bus (the monitor emits payloads as JSON strings via emit()).
func addrPayloadFor(t *testing.T, bus *collectingEventBus) *addrEventPayload {
	t.Helper()
	for _, ev := range bus.snapshot() {
		if ev.EventType != ifaceevents.EventAddrAdded {
			continue
		}
		s, ok := ev.Payload.(string)
		if !ok {
			continue
		}
		var p addrEventPayload
		if json.Unmarshal([]byte(s), &p) == nil {
			return &p
		}
	}
	return nil
}

// VALIDATES: AC-6 -- address origin classification distinguishes SLAAC/RA
// addresses (finite-lifetime IPv6), privacy/temporary addresses, static
// (permanent) addresses, and dynamic IPv4 (DHCP) from the kernel IFA_F_* flags.
func TestAddrOrigin(t *testing.T) {
	cases := []struct {
		name   string
		isIPv6 bool
		flags  int
		want   string
	}{
		{"permanent ipv4 static", false, unix.IFA_F_PERMANENT, "static"},
		{"permanent ipv6 static", true, unix.IFA_F_PERMANENT, "static"},
		{"ipv6 slaac (finite lifetime)", true, 0, "slaac"},
		{"ipv6 slaac noprefixroute", true, unix.IFA_F_NOPREFIXROUTE, "slaac"},
		{"ipv6 temporary (RFC 4941)", true, unix.IFA_F_TEMPORARY, "temporary"},
		{"ipv4 dynamic (dhcp)", false, 0, "dynamic"},
		// A permanent temporary flag combination still reads permanent-first.
		{"permanent wins over temporary", true, unix.IFA_F_PERMANENT | unix.IFA_F_TEMPORARY, "static"},
	}
	for _, tc := range cases {
		if got := addrOrigin(tc.isIPv6, tc.flags); got != tc.want {
			t.Errorf("%s: addrOrigin(%v, %#x) = %q, want %q", tc.name, tc.isIPv6, tc.flags, got, tc.want)
		}
	}
}

// VALIDATES: AC-6 -- lifetime normalization maps infinite/forever and negative
// values to 0 (omitted), and passes finite RA lease times through.
func TestNormalizeLifetime(t *testing.T) {
	cases := []struct {
		in   int
		want uint32
	}{
		{0xFFFFFFFF, 0}, // forever -> omit
		{-1, 0},         // invalid -> omit
		{0, 0},          // none -> omit
		{86400, 86400},  // a one-day RA lifetime
		{7200, 7200},
	}
	for _, tc := range cases {
		if got := normalizeLifetime(tc.in); got != tc.want {
			t.Errorf("normalizeLifetime(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// VALIDATES: AC-6 -- the monitor observes a kernel SLAAC address via a netlink
// addr update and emits an addr-added event carrying Origin "slaac". Uses the
// handler seam directly (synthetic netlink.AddrUpdate), so it needs no
// CAP_NET_ADMIN.
func TestHandleAddrUpdate_SlaacOrigin(t *testing.T) {
	m, bus := newTestMonitor()

	// Register the link name for index 7 so the addr handler can resolve it.
	lu := netlink.LinkUpdate{Link: &netlink.Dummy{Name: "eth0", Index: 7, MTU: 1500}}
	lu.Header = unix.NlMsghdr{Type: unix.RTM_NEWLINK}
	m.handleLinkUpdate(lu)

	// A SLAAC IPv6 address: non-permanent, finite RA lifetimes.
	au := netlink.AddrUpdate{
		LinkIndex:   7,
		LinkAddress: net.IPNet{IP: net.ParseIP("2001:db8::abcd"), Mask: net.CIDRMask(64, 128)},
		Flags:       unix.IFA_F_NOPREFIXROUTE,
		ValidLft:    86400,
		PreferedLft: 3600,
		NewAddr:     true,
	}
	m.handleAddrUpdate(au)

	found := addrPayloadFor(t, bus)
	if found == nil {
		t.Fatal("no addr-added event emitted for the SLAAC address")
	}
	if found.Origin != "slaac" {
		t.Fatalf("origin = %q, want slaac", found.Origin)
	}
	if found.Address != "2001:db8::abcd" {
		t.Fatalf("address = %q, want 2001:db8::abcd", found.Address)
	}
}

// VALIDATES: AC-6 -- a permanent (static) address is classified "static" in the
// emitted event, not slaac.
func TestHandleAddrUpdate_StaticOrigin(t *testing.T) {
	m, bus := newTestMonitor()
	lu := netlink.LinkUpdate{Link: &netlink.Dummy{Name: "eth0", Index: 8, MTU: 1500}}
	lu.Header = unix.NlMsghdr{Type: unix.RTM_NEWLINK}
	m.handleLinkUpdate(lu)

	au := netlink.AddrUpdate{
		LinkIndex:   8,
		LinkAddress: net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
		Flags:       unix.IFA_F_PERMANENT,
		NewAddr:     true,
	}
	m.handleAddrUpdate(au)

	found := addrPayloadFor(t, bus)
	if found == nil {
		t.Fatal("no addr-added event emitted for the static address")
	}
	if found.Origin != "static" {
		t.Fatalf("permanent address origin = %q, want static", found.Origin)
	}
}
