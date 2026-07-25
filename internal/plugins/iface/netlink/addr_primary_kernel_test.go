package ifacenetlink

import (
	"errors"
	"net/netip"
	"path/filepath"
	"sort"
	"testing"
)

// kernelAddrTable models the IPv4 address table of a Linux device, and is the
// addrRemover the guarded removal path is driven against. It reimplements the
// rules from net/ipv4/devinet.c INDEPENDENTLY of the production code, so it can
// disagree with it:
//
//   - inet_insert_ifa marks an address IFA_F_SECONDARY when the device already
//     holds an address with the same mask length in the same network.
//   - __inet_del_ifa deletes every same-subnet secondary along with the primary
//     ("Deleting primary ifaddr forces deletion all secondaries"), unless
//     promote_secondaries is set, in which case the first secondary is promoted
//     and the rest are kept.
//
// It deliberately does NOT call flushedByDelete: an oracle that computes the
// hazard with the function under test can never catch that function being
// wrong. The only production code these tests exercise is the real
// removeAddressGuarded -> ensureDeleteIsolated path.
type kernelAddrTable struct {
	dev   string
	addrs []deviceAddress
	knob  string
}

// sameSubnet is the kernel's ifa_mask + inet_ifa_match test, written out here
// rather than borrowed from the production helper.
func sameSubnet(a, b netip.Prefix) bool {
	if a.Bits() != b.Bits() {
		return false
	}
	if !a.Addr().Is4() || !b.Addr().Is4() {
		return false
	}
	return a.Masked() == b.Masked()
}

// add is inet_insert_ifa: a newcomer sharing a subnet with any existing address
// becomes a secondary.
func (k *kernelAddrTable) add(t *testing.T, cidr string) {
	t.Helper()
	prefix := mustPrefix(t, cidr)
	secondary := false
	for _, existing := range k.addrs {
		if sameSubnet(existing.Prefix, prefix) {
			secondary = true
			break
		}
	}
	k.addrs = append(k.addrs, deviceAddress{Prefix: prefix, Secondary: secondary})
}

func (k *kernelAddrTable) List(_ string) ([]deviceAddress, error) {
	out := make([]deviceAddress, len(k.addrs))
	copy(out, k.addrs)
	return out, nil
}

// Delete is __inet_del_ifa. It consults promote_secondaries through the same
// procfs seam the kernel would consult in_dev->cnf.promote_secondaries, so a
// policy that failed to enable it loses exactly what a real kernel loses.
func (k *kernelAddrTable) Delete(_ string, target deviceAddress) error {
	index := -1
	for i := range k.addrs {
		if k.addrs[i].Prefix == target.Prefix {
			index = i
			break
		}
	}
	if index < 0 {
		return errors.New("cannot assign requested address")
	}
	victim := k.addrs[index]
	promote := k.promotes()

	kept := make([]deviceAddress, 0, len(k.addrs))
	promoted := false
	for i, addr := range k.addrs {
		if i == index {
			continue
		}
		if victim.Secondary || !sameSubnet(addr.Prefix, victim.Prefix) || !addr.Secondary {
			kept = append(kept, addr)
			continue
		}
		if !promote {
			continue // cascaded: the kernel unlinks it with the primary
		}
		if !promoted {
			addr.Secondary = false
			promoted = true
		}
		kept = append(kept, addr)
	}
	k.addrs = kept
	return nil
}

func (k *kernelAddrTable) promotes() bool {
	for _, knob := range []string{promoteSecondariesKnob("all"), promoteSecondariesKnob(k.dev)} {
		if value, err := addrSysctlRead(knob); err == nil && value == "1" {
			return true
		}
	}
	return false
}

func (k *kernelAddrTable) list() []string {
	out := make([]string, 0, len(k.addrs))
	for _, addr := range k.addrs {
		out = append(out, addr.Prefix.String())
	}
	sort.Strings(out)
	return out
}

// remove drives the REAL production path: removeAddressGuarded runs the guard
// and then asks this modeled kernel to perform the delete.
func (k *kernelAddrTable) remove(t *testing.T, cidr string) error {
	t.Helper()
	return removeAddressGuarded(k, k.dev, deviceAddress{Prefix: mustPrefix(t, cidr)})
}

func newKernelAddrTable(t *testing.T, dev string) *kernelAddrTable {
	t.Helper()
	root := withTempProcRoot(t)
	knob := filepath.Join(root, "ipv4", "conf", dev, "promote_secondaries")
	fake := &fakeSysctl{values: map[string]string{
		knob: "0",
		filepath.Join(root, "ipv4", "conf", "all", "promote_secondaries"): "0",
	}}
	fake.install(t)
	return &kernelAddrTable{dev: dev, knob: knob}
}

func requireAddrList(t *testing.T, k *kernelAddrTable, want ...string) {
	t.Helper()
	got := k.list()
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("addresses = %v, want %v", got, want)
		}
	}
}

// TestSameSubnetRenumberKeepsNewAddress replays the exact sequence ze emits for
// a same-subnet address change on reload -- ADD the new address, then REMOVE
// the old one (the make-before-break order pinned by
// TestIfaceSameSubnetSwapOrdersAddBeforeRemove) -- through the real guarded
// removal path against the Linux primary/secondary rules.
//
// Without the guard the kernel takes the new address down with the old one and
// the interface ends up bare, which is the SIGHUP reload that reported success
// and left zdiag0 with no address.
//
// VALIDATES: after a make-before-break same-subnet renumber, only the new address remains.
// PREVENTS: an interface losing every address on an address change that reports success.
func TestSameSubnetRenumberKeepsNewAddress(t *testing.T) {
	kernel := newKernelAddrTable(t, "zdiag0")

	kernel.add(t, "10.77.0.1/24")
	requireAddrList(t, kernel, "10.77.0.1/24")

	// Reload: ze adds the new address first, then removes the old one.
	kernel.add(t, "10.77.0.2/24")
	if err := kernel.remove(t, "10.77.0.1/24"); err != nil {
		t.Fatalf("remove old address: %v", err)
	}

	requireAddrList(t, kernel, "10.77.0.2/24")
}

// TestSameSubnetRenumberWithUnflaggedSecondary is the fail-closed case: the
// device's addresses arrive with NO IFA_F_SECONDARY flag (netlink fills Flags
// only from the optional IFA_FLAGS attribute), so the sibling looks like a
// primary. The kernel still cascades, so the guard must still fire.
//
// VALIDATES: the guard protects a same-subnet sibling whose secondary flag never arrived.
// PREVENTS: a missing netlink attribute silently disarming the guard and re-opening the bug.
func TestSameSubnetRenumberWithUnflaggedSecondary(t *testing.T) {
	kernel := newKernelAddrTable(t, "zdiag0")

	kernel.add(t, "10.77.0.1/24")
	kernel.add(t, "10.77.0.2/24")
	// Simulate flags that never reached userspace: everything reads primary.
	for i := range kernel.addrs {
		kernel.addrs[i].Secondary = false
	}

	if err := kernel.remove(t, "10.77.0.1/24"); err != nil {
		t.Fatalf("remove old address: %v", err)
	}
	if !kernel.promotes() {
		t.Fatal("promote_secondaries was not enabled for an unflagged same-subnet sibling")
	}
}

// TestSameSubnetRenumberSurvivesRepeatedReloads verifies the second renumber
// works too: after the first swap the surviving address must be usable as a
// primary, so 10.77.0.2 -> 10.77.0.3 behaves like the first change.
//
// VALIDATES: consecutive same-subnet renumbers each leave exactly the new address.
// PREVENTS: a fix that works once because promotion left the table in a state the next pass mishandles.
func TestSameSubnetRenumberSurvivesRepeatedReloads(t *testing.T) {
	kernel := newKernelAddrTable(t, "zdiag0")

	kernel.add(t, "10.77.0.1/24")
	kernel.add(t, "10.77.0.2/24")
	if err := kernel.remove(t, "10.77.0.1/24"); err != nil {
		t.Fatalf("first renumber: %v", err)
	}
	requireAddrList(t, kernel, "10.77.0.2/24")

	kernel.add(t, "10.77.0.3/24")
	if err := kernel.remove(t, "10.77.0.2/24"); err != nil {
		t.Fatalf("second renumber: %v", err)
	}
	requireAddrList(t, kernel, "10.77.0.3/24")
}

// TestSameSubnetRemovalKeepsEveryOtherAddress covers the multi-address subnet:
// a primary plus two secondaries must leave BOTH secondaries behind.
//
// VALIDATES: removing a primary from a subnet holding several secondaries keeps all of them.
// PREVENTS: promotion rescuing only the first secondary while the rest are flushed.
func TestSameSubnetRemovalKeepsEveryOtherAddress(t *testing.T) {
	kernel := newKernelAddrTable(t, "zdiag0")

	kernel.add(t, "10.77.0.1/24")
	kernel.add(t, "10.77.0.2/24")
	kernel.add(t, "10.77.0.3/24")

	if err := kernel.remove(t, "10.77.0.1/24"); err != nil {
		t.Fatalf("remove primary: %v", err)
	}

	requireAddrList(t, kernel, "10.77.0.2/24", "10.77.0.3/24")
}

// TestPointToPointRenumberKeepsNewAddress covers the /31 boundary, the smallest
// prefix that still has a same-subnet sibling and therefore still cascades.
// /31 point-to-point links are a real ze deployment shape.
//
// VALIDATES: a /31 same-subnet renumber leaves exactly the new address.
// PREVENTS: the smallest multi-address prefix falling outside the guard.
func TestPointToPointRenumberKeepsNewAddress(t *testing.T) {
	kernel := newKernelAddrTable(t, "parent0.200")

	kernel.add(t, "10.77.0.0/31")
	kernel.add(t, "10.77.0.1/31")
	if err := kernel.remove(t, "10.77.0.0/31"); err != nil {
		t.Fatalf("remove old address: %v", err)
	}

	requireAddrList(t, kernel, "10.77.0.1/31")
}

// TestCrossSubnetRenumberKeepsNewAddress verifies the ordinary case is
// unaffected: a renumber into a DIFFERENT subnet never made the new address a
// secondary, so it must survive without the guard doing anything.
//
// VALIDATES: a cross-subnet renumber leaves exactly the new address and writes no sysctl.
// PREVENTS: the primary/secondary fix regressing or perturbing the common renumber path.
func TestCrossSubnetRenumberKeepsNewAddress(t *testing.T) {
	kernel := newKernelAddrTable(t, "zdiag0")

	kernel.add(t, "10.77.0.1/24")
	kernel.add(t, "10.88.0.1/24")
	if err := kernel.remove(t, "10.77.0.1/24"); err != nil {
		t.Fatalf("remove old address: %v", err)
	}

	requireAddrList(t, kernel, "10.88.0.1/24")
	if kernel.promotes() {
		t.Fatal("promote_secondaries was enabled for a removal that could not cascade")
	}
}

// TestHostAlreadyPromotingNeedsNoWrite covers the host that already promotes
// globally (net.ipv4.conf.all.promote_secondaries=1): the addresses survive and
// ze must not touch the per-device knob at all.
//
// VALIDATES: a globally promoting host keeps both addresses with no per-device sysctl write.
// PREVENTS: ze rewriting operator kernel state that is already correct.
func TestHostAlreadyPromotingNeedsNoWrite(t *testing.T) {
	root := withTempProcRoot(t)
	allKnob := filepath.Join(root, "ipv4", "conf", "all", "promote_secondaries")
	devKnob := filepath.Join(root, "ipv4", "conf", "zdiag0", "promote_secondaries")
	fake := &fakeSysctl{values: map[string]string{allKnob: "1", devKnob: "0"}}
	fake.install(t)
	kernel := &kernelAddrTable{dev: "zdiag0", knob: devKnob}

	kernel.add(t, "10.77.0.1/24")
	kernel.add(t, "10.77.0.2/24")
	if err := kernel.remove(t, "10.77.0.1/24"); err != nil {
		t.Fatalf("remove old address: %v", err)
	}

	requireAddrList(t, kernel, "10.77.0.2/24")
	if fake.writeCall != 0 {
		t.Fatalf("writeCall = %d, want 0 on a host that already promotes globally", fake.writeCall)
	}
}

// TestRemoveAbsentAddressTouchesNoKernelState verifies a removal of an address
// that is not on the device fails from the kernel, not from the guard, and
// flips no knob on the way.
//
// VALIDATES: removing an absent address returns the kernel's error and writes no sysctl.
// PREVENTS: the guard mutating kernel state on the error path of a doomed removal.
func TestRemoveAbsentAddressTouchesNoKernelState(t *testing.T) {
	kernel := newKernelAddrTable(t, "zdiag0")
	kernel.add(t, "10.77.0.1/24")
	kernel.add(t, "10.77.0.2/24")

	if err := kernel.remove(t, "10.77.0.9/24"); err == nil {
		t.Fatal("removing an absent address returned nil")
	}
	if kernel.promotes() {
		t.Fatal("promote_secondaries was enabled for an address that is not on the device")
	}
	requireAddrList(t, kernel, "10.77.0.1/24", "10.77.0.2/24")
}
