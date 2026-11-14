//go:build linux

// Design: docs/features/interfaces.md -- Netlink interface monitor
// Related: monitor_linux.go -- resolveAddrLink, handleAddrUpdate.

package ifacenetlink

import (
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/report"
)

// gatewayAddr is the RTM_NEWADDR the rib-import daemon fixtures produce with
// `ip addr add 198.51.100.0/31 dev zeimpgw` on a dummy link created just before.
func gatewayAddr(index int) netlink.AddrUpdate {
	return netlink.AddrUpdate{
		LinkIndex:   index,
		LinkAddress: net.IPNet{IP: net.ParseIP("198.51.100.0").To4(), Mask: net.CIDRMask(31, 32)},
		Flags:       unix.IFA_F_PERMANENT,
		NewAddr:     true,
	}
}

// Goal: an address whose RTM_NEWADDR reaches the monitor before its link's
// RTM_NEWLINK is still announced, retained, and not re-announced when the link
// update follows. Method: drive handleAddrUpdate with an empty name cache and a
// kernel lookup that knows the link, which is the order run's random select
// produces when both channels are ready; then deliver the delayed RTM_NEWLINK.
// Before resolveAddrLink the address was dropped on the cache miss, and the
// connected route for 198.51.100.0/31 never reached the system RIB.
func TestHandleAddrUpdate_AddressBeforeItsLink(t *testing.T) {
	m, bus := newTestMonitor()
	var lookups []int
	m.linkByIndex = func(index int) (netlink.Link, error) {
		lookups = append(lookups, index)
		return &netlink.Dummy{Name: "zeimpgw", Index: index, MTU: 1500}, nil
	}

	m.handleAddrUpdate(gatewayAddr(9))

	events := bus.snapshot()
	if len(events) != 1 {
		t.Fatalf("address before its link: %d events %+v, want one addr-added", len(events), events)
	}
	if events[0].EventType != ifaceevents.EventAddrAdded {
		t.Fatalf("event = %q, want %q", events[0].EventType, ifaceevents.EventAddrAdded)
	}
	raw, ok := events[0].Payload.(string)
	if !ok {
		t.Fatalf("payload = %T, want string", events[0].Payload)
	}
	var got addrEventPayload
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	want := addrEventPayload{Name: "zeimpgw", Index: 9, Address: "198.51.100.0", PrefixLength: 31, Family: "ipv4", Origin: "static"}
	if got != want {
		t.Fatalf("announced %+v, want %+v", got, want)
	}
	if len(m.addresses) != 1 {
		t.Fatalf("retained %d addresses, want 1 so a replay republishes it", len(m.addresses))
	}

	link := netlink.LinkUpdate{Link: &netlink.Dummy{Name: "zeimpgw", Index: 9, MTU: 1500, OperState: netlink.OperUp}}
	link.Header = unix.NlMsghdr{Type: unix.RTM_NEWLINK}
	m.handleLinkUpdate(link)

	events = bus.snapshot()
	var after []string
	for _, ev := range events[1:] {
		after = append(after, ev.EventType)
	}
	if len(after) != 2 || after[0] != ifaceevents.EventCreated || after[1] != ifaceevents.EventUp {
		t.Fatalf("delayed RTM_NEWLINK emitted %v, want [%s %s] and no address churn", after, ifaceevents.EventCreated, ifaceevents.EventUp)
	}
	if len(lookups) != 1 || lookups[0] != 9 {
		t.Fatalf("kernel lookups %v, want exactly [9]", lookups)
	}
}

// Goal: an address event whose link the kernel cannot produce announces and
// retains nothing, for a deleted link and for a failed lookup alike, and only
// the failed lookup, which loses a live address, raises the
// doctor-iface-address-unattributed error with its subject and fix. Method:
// the same empty cache, with a lookup that answers not-found, then an error,
// reading the report bus after each.
func TestHandleAddrUpdate_AddressOfUnresolvableLink(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		raised bool
	}{
		{"deleted link", netlink.LinkNotFoundError{}, false},
		{"lookup failed", errors.New("netlink receive: no buffer space available"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			m, bus := newTestMonitor()
			m.linkByIndex = func(int) (netlink.Link, error) { return nil, tc.err }

			m.handleAddrUpdate(gatewayAddr(9))

			if events := bus.snapshot(); len(events) != 0 {
				t.Fatalf("unresolvable link emitted %+v, want nothing", events)
			}
			if len(m.addresses) != 0 {
				t.Fatalf("unresolvable link retained %d addresses, want 0", len(m.addresses))
			}
			if _, cached := m.linkNames.Load(9); cached {
				t.Fatal("unresolvable link was cached")
			}
			var issues []report.Issue
			for _, issue := range report.Errors(0) {
				if issue.Code == reportCodeIfaceAddressUnattributed {
					issues = append(issues, issue)
				}
			}
			if !tc.raised {
				if len(issues) != 0 {
					t.Fatalf("a deleted link raised %+v, want nothing: its address went with it", issues)
				}
				return
			}
			if len(issues) != 1 {
				t.Fatalf("a failed lookup raised %d %s errors, want one: %+v", len(issues), reportCodeIfaceAddressUnattributed, issues)
			}
			issue := issues[0]
			if issue.Source != "iface" {
				t.Errorf("source = %q, want iface", issue.Source)
			}
			if issue.Subject != "198.51.100.0/31 index 9" {
				t.Errorf("subject = %q, want %q", issue.Subject, "198.51.100.0/31 index 9")
			}
			wantFix := "ip -o addr show to 198.51.100.0/31; ip addr del 198.51.100.0/31 dev <device>; " +
				"ip addr add 198.51.100.0/31 dev <device>"
			if issue.Fix != wantFix {
				t.Errorf("fix = %q, want %q", issue.Fix, wantFix)
			}
			wantMessage := "Interface address 198.51.100.0/31 was not announced, so its connected route is missing: " +
				"the lookup of interface index 9 failed: netlink receive: no buffer space available"
			if issue.Message != wantMessage {
				t.Errorf("message = %q, want %q", issue.Message, wantMessage)
			}
		})
	}
}
