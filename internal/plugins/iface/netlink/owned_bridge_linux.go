// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Overview: ifacenetlink.go -- package hub
// Related: owned_link_linux.go -- the links that join an owned bridge
// Related: macvlan_linux.go -- the owned macvlan, the same create-then-mark sequence

//go:build linux

package ifacenetlink

import (
	"fmt"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/report"
)

// reportOwnedRollbackFailure logs and raises a half-made owned device the
// rollback of a failed create could not delete. The device holds a name its
// plugin asked for, so the next create of that name fails until it is gone.
// The warning is the interface component's (doctor_owned.go): its owned pass
// clears it once the device is absent.
func reportOwnedRollbackFailure(name, kind, stage string, delErr error) {
	logger().Warn("iface: rollback delete after create failure",
		"name", name, "kind", kind, "stage", stage, "err", delErr,
		"code", iface.CodeDoctorIfaceOwnedDeviceReconcile, "fix", "ip link delete "+name)
	report.RaiseWarningFix(iface.OwnedDeviceReportSource, iface.CodeDoctorIfaceOwnedDeviceReconcile,
		iface.OwnedDeviceSubjectDevice+name,
		report.FitMessage("half-made owned "+kind+" "+name+" left after a failed "+stage+": "+delErr.Error()),
		"ip link delete "+name)
}

// CreateOwnedBridge creates a bridge named spec.Name, sets its MTU, marks it
// with the owned-device alias spec.Alias and brings it admin-up.
//
// The MTU is set with its own call after the create, not in the create: the
// kernel then records it as set by the user, so a member added later does not
// pull the bridge's MTU down to its own. The alias is set after the create too,
// because the kernel ignores IFLA_IFALIAS in RTM_NEWLINK (Linux limitation
// LL-RTNL-NEWLINK-NO-ALIAS, docs/architecture/linux-limitations.md). On any
// failure after the create the bridge is deleted, so a completed call never
// leaves an unmarked device. A crash between the create and the alias leaves
// an unmarked bridge under an owned name, which the reconcile refuses to touch
// (fail closed): the operator deletes it.
func (b *netlinkBackend) CreateOwnedBridge(spec iface.OwnedBridgeSpec) error {
	if !qosMapMu.TryLock() {
		return iface.ErrNativeAccessPending
	}
	defer qosMapMu.Unlock()
	if err := b.nativeVLANs.legacyAdmissionLocked(); err != nil {
		return err
	}
	if err := iface.ValidateIfaceName(spec.Name); err != nil {
		return fmt.Errorf("create owned bridge %q: %w", spec.Name, err)
	}
	if err := validateMTU(spec.MTU); err != nil {
		return fmt.Errorf("create owned bridge %q: %w", spec.Name, err)
	}
	attrs := netlink.NewLinkAttrs()
	attrs.Name = spec.Name
	link := &netlink.Bridge{LinkAttrs: attrs}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create owned bridge %q: %w", spec.Name, err)
	}
	rollback := func(stage string, cause error) error {
		return fmt.Errorf("iface: %s owned bridge %q: %w", stage, spec.Name, b.rollbackNativeCreate(link, spec.Name, "bridge", stage, cause))
	}
	if err := netlink.LinkSetMTU(link, spec.MTU); err != nil {
		return rollback("set mtu on", err)
	}
	if err := netlink.LinkSetAlias(link, spec.Alias); err != nil {
		return rollback("set alias on", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return rollback("set up", err)
	}
	return nil
}
