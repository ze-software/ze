// Design: docs/features/interfaces.md -- Interface management via netlink
// Detail: addr_primary_linux.go -- netlink adapter that feeds this policy
// Related: manage_linux.go -- RemoveAddress applies this policy before AddrDel

package ifacenetlink

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Linux IPv4 primary/secondary address semantics.
//
// Adding an IPv4 address to a device that already carries an address in the
// SAME subnet makes the newcomer a SECONDARY (the kernel sets IFA_F_SECONDARY
// in inet_insert_ifa). Deleting the PRIMARY address of that subnet then deletes
// every secondary in it as well, unless net.ipv4.conf.<dev>.promote_secondaries
// is 1, in which case the kernel promotes one secondary to primary and keeps
// the others (Linux net/ipv4/devinet.c, __inet_del_ifa: "Deleting primary
// ifaddr forces deletion all secondaries unless alias promotion is set").
// IPv6 has no primary/secondary distinction, so none of this applies to it.
//
// Ze's address reconcilers are deliberately make-before-break: they ADD the new
// address and only then REMOVE the old one -- see the constraint rule
// iface-add-address-before-remove-same-interface in
// internal/component/iface/operation.go and the add-loop-before-remove-loop
// order in internal/component/iface/config_apply.go
// (reconcileOnReadyWithJournal). For a same-subnet renumber such as
// 10.0.0.1/24 -> 10.0.0.2/24 that ordering makes the NEW address a secondary of
// the OLD one, so removing the old address silently takes the new one with it
// and the interface is left with no address at all -- while the journal, the
// reconcile, and the reload all report success.
//
// Which path reaches the hazard depends on one other thing. The transaction
// path gates the ADD on the settlement rule iface-add-address-settles-addr-added
// (internal/component/iface/operation.go:57-65), and the executor rolls the
// whole transaction back when that times out
// (internal/component/config/transaction/executor.go:140-143). Until the
// netlink monitor seeded its link-index cache (seedLinkNames,
// monitor_linux.go), the addr-added event never arrived, so the transaction
// path timed out and rolled back rather than reaching the REMOVE at all. The
// reconcile path has no settlement gate, so it always reached the hazard --
// which is why it showed up first as
// TestIntegrationApplyConfigVLANUnitAddressReconcile.
//
// Backend.RemoveAddress promises to remove ONE address. The functions below
// make the netlink implementation keep that promise: before deleting a primary
// IPv4 address that has same-subnet secondaries, enable promote_secondaries on
// the device so the kernel promotes instead of flushing. The knob is left
// enabled afterwards on purpose -- restoring it would re-arm the same hazard
// for the next removal, and its only effect is to make address deletion
// non-destructive.

// procNetRoot is the sysctl tree root; a var so tests can point it at a
// temporary directory instead of a live /proc.
var procNetRoot = "/proc/sys/net"

// promoteSecondariesKnob returns the per-device promote_secondaries path. The
// procfs directory is the literal interface name (dots and all, e.g. eth0.100),
// unlike the dotted sysctl key form, so no escaping is needed.
func promoteSecondariesKnob(dev string) string {
	return filepath.Join(procNetRoot, "ipv4", "conf", dev, "promote_secondaries")
}

// Seams so the policy is unit-testable on any OS without a live /proc.
var (
	addrSysctlRead  = osAddrSysctlRead
	addrSysctlWrite = osAddrSysctlWrite
)

func osAddrSysctlRead(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path built from a validated device name
	if err != nil {
		return "", err
	}
	return trimAddrSysctl(string(b)), nil
}

func osAddrSysctlWrite(path, value string) error {
	return os.WriteFile(path, []byte(value), 0o644) //nolint:gosec // procfs knob, not a secret
}

// trimAddrSysctl strips the trailing newline procfs adds to a scalar knob.
func trimAddrSysctl(s string) string {
	for s != "" && (s[len(s)-1] == '\n' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// deviceAddress is one address configured on a device, carrying the kernel's
// IFA_F_SECONDARY classification.
type deviceAddress struct {
	Prefix    netip.Prefix
	Secondary bool
}

// flushedByDelete returns the addresses the kernel would delete along with
// target if target were removed from a device currently holding existing,
// assuming promote_secondaries is 0.
//
// The kernel cascades only when the address being deleted is a PRIMARY: it then
// unlinks every SECONDARY with the same mask length whose network matches
// (__inet_del_ifa's ifa_mask comparison plus inet_ifa_match). Deleting a
// secondary never touches anything else, and IPv6 has no such rule.
//
// It deliberately does NOT require a sibling to be flagged secondary, even
// though the kernel does. Secondary is derived from netlink's IFA_FLAGS
// ATTRIBUTE (vendor/github.com/vishvananda/netlink/addr_linux.go:264); the
// parser never reads the ifa_flags header byte where IFA_F_SECONDARY also
// lives, so an address whose flags did not arrive reads as Secondary=false. If
// the sibling test required the flag, that zero value would silently look like
// "no hazard" and the guard would fail OPEN -- exactly the trap
// ai/rules/fail-closed-guards.md names. Any other same-subnet IPv4 address is
// therefore treated as at risk: a device only ever holds one primary per
// subnet, so a sibling in the same subnet IS the secondary whether or not its
// flag survived. The cost of the conservative read is at most one extra
// procfs write.
//
// target.Secondary stays as a skip, because it is a POSITIVE signal: it only
// short-circuits when the kernel explicitly said this address is a secondary.
// Missing flags leave it false, which keeps the guard armed.
func flushedByDelete(target deviceAddress, existing []deviceAddress) []netip.Prefix {
	if target.Secondary || !target.Prefix.IsValid() || !target.Prefix.Addr().Is4() {
		return nil
	}
	network := target.Prefix.Masked()
	var doomed []netip.Prefix
	for _, addr := range existing {
		if !addr.Prefix.IsValid() || !addr.Prefix.Addr().Is4() {
			continue
		}
		if addr.Prefix == target.Prefix {
			continue
		}
		// Prefix equality covers both address and bits, and Masked preserves
		// bits, so this single test is the kernel's ifa_mask comparison and
		// inet_ifa_match together.
		if addr.Prefix.Masked() != network {
			continue
		}
		doomed = append(doomed, addr.Prefix)
	}
	return doomed
}

// addrRemover is the seam the guarded removal uses to reach the kernel (the
// fakeOps pattern, ai/rules/testing.md). Keeping it OS-independent is what lets
// removeAddressGuarded -- and therefore the guard's WIRING, not just its
// helpers -- be driven by a fake on any host, so the coverage runs under
// `make ze-verify` rather than only under the QEMU integration suite.
type addrRemover interface {
	// List returns the addresses currently configured on dev.
	List(dev string) ([]deviceAddress, error)
	// Delete removes exactly one address from dev.
	Delete(dev string, target deviceAddress) error
}

// removeAddressGuarded removes target from dev, first making sure the kernel
// will not cascade the delete to same-subnet siblings.
//
// The guard and the delete live in ONE function on purpose: a caller cannot
// reach the delete without passing the guard, so the wiring cannot rot into a
// silently-unguarded AddrDel the way a separate pre-call could.
func removeAddressGuarded(ops addrRemover, dev string, target deviceAddress) error {
	existing, err := ops.List(dev)
	if err != nil {
		return err
	}
	// A target that is not on the device cannot cascade, because the delete
	// that follows will fail. Leave that failure to the kernel rather than
	// flipping a knob for it.
	if resolved, present := selectDeleteTarget(target, existing); present {
		if err := ensureDeleteIsolated(dev, resolved, existing); err != nil {
			return err
		}
	}
	return ops.Delete(dev, target)
}

// selectDeleteTarget locates target among the device's live addresses and
// returns it carrying the kernel's own IFA_F_SECONDARY classification, which a
// caller-built address (parsed from a CIDR string) does not have.
//
// Reports false when the address is not on the device: there is nothing to
// delete, so no policy is needed and the delete should be left to fail with the
// kernel's own error rather than flipping a knob for a removal that cannot
// cascade because it cannot happen.
func selectDeleteTarget(target deviceAddress, existing []deviceAddress) (deviceAddress, bool) {
	for _, addr := range existing {
		if addr.Prefix == target.Prefix {
			target.Secondary = addr.Secondary
			return target, true
		}
	}
	return target, false
}

// ensureDeleteIsolated makes a subsequent delete of target from dev remove ONLY
// target. It is a no-op when the delete is already isolated (target is IPv6,
// target is itself a secondary, or no same-subnet secondary exists). Otherwise
// it enables promote_secondaries on dev so the kernel promotes a secondary
// instead of flushing the subnet.
//
// Returns an error naming the addresses that would be destroyed when the knob
// can neither be read as already-enabled nor written: removing one address and
// silently taking others with it is exactly the exact-or-reject failure the
// caller must surface rather than paper over.
func ensureDeleteIsolated(dev string, target deviceAddress, existing []deviceAddress) error {
	doomed := flushedByDelete(target, existing)
	if len(doomed) == 0 {
		return nil
	}
	// The kernel's effective value is IN_DEV_ORCONF: all.promote_secondaries
	// OR <dev>.promote_secondaries. Check both so a host that already promotes
	// globally costs no write at all. A read error is NOT read as "enabled":
	// it falls through to the write, which is the fail-closed direction.
	for _, knob := range []string{promoteSecondariesKnob("all"), promoteSecondariesKnob(dev)} {
		if value, err := addrSysctlRead(knob); err == nil && value == "1" {
			return nil
		}
	}
	knob := promoteSecondariesKnob(dev)
	if err := addrSysctlWrite(knob, "1"); err != nil {
		return fmt.Errorf("iface: remove address %s on %s: the kernel would also delete %s; enabling %s failed: %w",
			target.Prefix, dev, joinPrefixes(doomed), knob, err)
	}
	loggerPtr.Load().Info("iface: enabled promote_secondaries so address removal keeps same-subnet addresses",
		"iface", dev, "removing", target.Prefix.String(), "preserving", joinPrefixes(doomed))
	return nil
}

// joinPrefixes renders a prefix list for a log field or error message.
func joinPrefixes(prefixes []netip.Prefix) string {
	var b textbuf.Buffer
	for i, prefix := range prefixes {
		if i > 0 {
			b.Str(", ")
		}
		b.Prefix(prefix)
	}
	return b.String()
}
