//go:build linux

// Design: ai/rules/repo-maintenance.md -- kernel macvlan capability probe
// Overview: ifacenetlink.go -- package hub
//
// probeMacvlanCapability proves the kernel can create a bridge-mode macvlan by
// actually doing it: create a throwaway dummy parent, create a bridge macvlan
// on it, then delete both. Only a real RTM_NEWLINK proves the kind works --
// builtin modules are invisible in /proc/modules, so inspection cannot (Key
// Design Decision). EPERM (no CAP_NET_ADMIN) is reported distinctly from an
// unsupported kind so the doctor check can give the right advice. Probe devices
// use fixed reserved names and are pre-cleaned + deferred-cleaned so a crashed
// prior run cannot leave the next probe misreporting EEXIST as unsupported.

package ifacenetlink

import (
	"errors"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	macvlanProbeParent = "zedoc-mvl-p"
	macvlanProbeChild  = "zedoc-mvl-m"
)

func probeMacvlanCapability() macvlanProbeResult {
	cleanupProbeLink(macvlanProbeChild)
	cleanupProbeLink(macvlanProbeParent)
	defer cleanupProbeLink(macvlanProbeChild)
	defer cleanupProbeLink(macvlanProbeParent)

	dummy := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: macvlanProbeParent}}
	if err := netlink.LinkAdd(dummy); err != nil {
		if errors.Is(err, unix.EPERM) {
			return macvlanProbeNoPrivilege
		}
		// Cannot even create a dummy parent -- the probe cannot run, so it
		// cannot vouch for macvlan support.
		return macvlanProbeUnsupported
	}
	parent, err := netlink.LinkByName(macvlanProbeParent)
	if err != nil {
		return macvlanProbeUnsupported
	}
	mv := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{Name: macvlanProbeChild, ParentIndex: parent.Attrs().Index},
		Mode:      netlink.MACVLAN_MODE_BRIDGE,
	}
	if err := netlink.LinkAdd(mv); err != nil {
		if errors.Is(err, unix.EPERM) {
			return macvlanProbeNoPrivilege
		}
		return macvlanProbeUnsupported
	}
	return macvlanProbeOK
}

// cleanupProbeLink best-effort deletes a probe link by name; a missing link is
// not an error worth surfacing.
func cleanupProbeLink(name string) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return
	}
	if delErr := netlink.LinkDel(link); delErr != nil {
		logger().Warn("iface: cleanup macvlan probe link", "name", name, "err", delErr)
	}
}
