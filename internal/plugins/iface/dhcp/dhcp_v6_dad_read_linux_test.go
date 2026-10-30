// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: rfc9915_dhcp_v6_lease_linux_test.go -- the rig these tests drive
// Related: dhcp_v6_attach_linux.go -- the DAD read judged here

package ifacedhcp

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestDHCPv6AddressGoneBehindAStaleUpReadNotDeclined holds an address that
// is tentative for 0.5 s; at 0.5 s the device is set down and the kernel
// deletes the address between the backend's link read and its address read,
// so the first read that finds the address gone still says up. Every later
// read says down until 5 s, and no link event reaches the worker until the up
// at 5 s.
//
// VALIDATES: an address found gone with the link up is judged against a
// second read, whose link state is taken after the first read's addresses:
// that read says down, so the address went with the link, no Decline goes
// out, and the up installs it again (valid 295 s, preferred 195 s).
// PREVENTS: a unique address declined because the backend reads the link
// before the addresses (GetInterface: RTM_GETLINK, then RTM_GETADDR).
func TestDHCPv6AddressGoneBehindAStaleUpReadNotDeclined(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(leaseLongPD, false, false)
	r.backend.addrsAt = func(at time.Duration) []iface.AddrInfo {
		switch {
		case at < 500*time.Millisecond:
			return v6TestAddrInfo(true, false)
		case at < seconds(5):
			return nil // deleted with the down
		default:
			return v6TestAddrInfo(false, false) // installed again, DAD complete
		}
	}
	var downReads atomic.Int32
	r.backend.stateAt = func(at time.Duration) string {
		if at < 500*time.Millisecond || at >= seconds(5) {
			return iface.LinkStateUp
		}
		if downReads.Add(1) == 1 {
			return iface.LinkStateUp // the link read that preceded the down
		}
		return iface.LinkStateDown
	}
	r.startWorker()
	r.runUntil(seconds(5))

	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Fatalf("%d DECLINE for an address deleted with the link: %s", len(got), r.describeSent())
	}
	r.clk.Set(r.start.Add(seconds(5)))
	r.link(iface.LinkUp)
	r.awaitRebind()
	r.runUntil(seconds(8))

	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Errorf("%d DECLINE after the link came back: %s", len(got), r.describeSent())
	}
	if got := r.countOps("replace " + loopKernelDevice + " " + v6TestCIDR + " 295 195"); got != 1 {
		t.Errorf("address reinstalled with 295/195 %d times after the up, want 1", got)
	}
}

// TestDHCPv6AddressesTheBackendDoesNotReportNotDeclined runs the client over
// a backend that answers every read without the device's addresses, as the
// VPP backend does.
//
// VALIDATES: no address is declined, and one Warn names the interface and
// says the DAD outcome cannot be read, however many leases are installed.
// PREVENTS: every leased address read as gone, so declined at the first DAD
// poll, on a backend that never reports addresses.
func TestDHCPv6AddressesTheBackendDoesNotReportNotDeclined(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(leaseLongPD, false, false)
	r.backend.addressesUnread = true
	r.startWorker()
	r.runUntil(seconds(3))

	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Fatalf("%d DECLINE on a backend that reports no address: %s", len(got), r.describeSent())
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("address removed %d times, want 0", got)
	}
	if got := r.logs.countWith("does not report", "iface", r.backend.logical); got != 1 {
		t.Errorf("%d Warn naming %s for the unreported addresses, want 1", got, r.backend.logical)
	}
}

// TestDHCPv6AddressesWithoutDADOutcomeNotReadAsUnique runs the client over a
// backend that answers the device's addresses but no DAD outcome, as the VPP
// backend does: the leased address is present and carries neither Tentative
// nor DADFailed.
//
// VALIDATES: the client reads the DAD outcome as unknown, not as unique: one
// Warn names the interface and says the outcome cannot be read, and nothing
// is declined or removed.
// PREVENTS: an address whose DAD nobody reported taken as DAD-complete and
// unique because it carries no flag (RFC 9915 Section 18.2.10.1).
func TestDHCPv6AddressesWithoutDADOutcomeNotReadAsUnique(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(leaseLongPD, false, false)
	r.backend.dadUnreported = true
	r.startWorker()
	r.runUntil(seconds(3))

	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Fatalf("%d DECLINE on a backend that reports no DAD outcome: %s", len(got), r.describeSent())
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("address removed %d times, want 0", got)
	}
	if got := r.logs.countWith("duplicate address detection outcome", "iface", r.backend.logical); got != 1 {
		t.Errorf("%d Warn naming %s for the unreported DAD outcome, want 1", got, r.backend.logical)
	}
}

// TestDHCPv6FailedAddressReadDeclinesNothing fails every backend read from
// 0.4 s, while the leased address is still tentative.
//
// VALIDATES: a read that cannot answer declines nothing, and removes nothing.
// PREVENTS: a failed read taken as a device holding no address.
func TestDHCPv6FailedAddressReadDeclinesNothing(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(leaseLongPD, false, false)
	r.backend.addrsAt = func(time.Duration) []iface.AddrInfo { return v6TestAddrInfo(true, false) }
	r.startWorker()
	r.runUntil(400 * time.Millisecond)
	r.backend.mu.Lock()
	r.backend.readErr = errors.New("address dump failed")
	r.backend.mu.Unlock()
	r.runUntil(seconds(3))

	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Fatalf("%d DECLINE after a failed read: %s", len(got), r.describeSent())
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("address removed %d times, want 0", got)
	}
}
