// Design: docs/features/interfaces.md -- Interface listing via netlink
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
)

// linkListRetries is how many times listLinks re-issues an interrupted dump.
// Three is enough for a table that is merely churning; a table changing faster
// than that for three consecutive dumps is a real condition worth reporting.
const linkListRetries = 3

// listLinks dumps the link table, retrying while the kernel reports the dump was
// interrupted.
//
// NLM_F_DUMP_INTR (netlink.ErrDumpInterrupted, "results may be incomplete or
// inconsistent") is not a failure: it means the link table CHANGED while the
// kernel was walking it, so the snapshot may be torn. The correct response is to
// dump again -- the vendored netlink even keeps EINTR compatibility for it
// (vendor/github.com/vishvananda/netlink/nl/nl_linux.go:48-66).
//
// Treating it as fatal was operator-visible: the caller in
// internal/component/iface/config_apply.go:875 turns it into a config-apply
// error, which aborts the interface plugin at its Config stage and takes the
// whole daemon down. Ze's own target workloads churn the link table constantly
// (PPPoE/L2TP session interfaces, tunnels), so a boot or reload could fail purely
// because a session came up at the wrong microsecond. Observed in the QEMU reload
// suite, where concurrent tests create and delete devices in one netns.
func listLinks() ([]netlink.Link, error) {
	var links []netlink.Link
	var err error
	for attempt := 0; attempt <= linkListRetries; attempt++ {
		links, err = netlink.LinkList()
		if !errors.Is(err, netlink.ErrDumpInterrupted) {
			return links, err
		}
	}
	return nil, fmt.Errorf("link table still changing after %d dumps: %w", linkListRetries+1, err)
}

func (b *netlinkBackend) ListInterfaces() ([]iface.InterfaceInfo, error) {
	links, err := listLinks()
	if err != nil {
		return nil, fmt.Errorf("iface: list interfaces: %w", err)
	}
	result := make([]iface.InterfaceInfo, 0, len(links))
	for _, link := range links {
		info := linkToInfo(link)
		info.Addresses = addrList(link)
		// Populate raw kernel counters: the rate tracker and flow-export
		// counter snapshot both consume Stats from ListInterfaces. Without
		// this they see nil Stats and skip every interface.
		if s := link.Attrs().Statistics; s != nil {
			info.Stats = &iface.InterfaceStats{
				RxBytes:     s.RxBytes,
				RxPackets:   s.RxPackets,
				RxErrors:    s.RxErrors,
				RxDropped:   s.RxDropped,
				RxMulticast: s.Multicast,
				TxBytes:     s.TxBytes,
				TxPackets:   s.TxPackets,
				TxErrors:    s.TxErrors,
				TxDropped:   s.TxDropped,
			}
		}
		result = append(result, info)
	}
	return result, nil
}

func (b *netlinkBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	if err := iface.ValidateIfaceName(name); err != nil {
		return nil, err
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("iface: get %q: %w", name, err)
	}
	info := linkToInfo(link)
	info.Addresses = addrList(link)
	s := link.Attrs().Statistics
	if s != nil {
		info.Stats = &iface.InterfaceStats{
			RxBytes:     s.RxBytes,
			RxPackets:   s.RxPackets,
			RxErrors:    s.RxErrors,
			RxDropped:   s.RxDropped,
			RxMulticast: s.Multicast,
			TxBytes:     s.TxBytes,
			TxPackets:   s.TxPackets,
			TxErrors:    s.TxErrors,
			TxDropped:   s.TxDropped,
		}
	}
	return &info, nil
}

func linkToInfo(link netlink.Link) iface.InterfaceInfo {
	attrs := link.Attrs()
	state := "down"
	if attrs.OperState == netlink.OperUp {
		state = "up"
	} else if attrs.OperState == netlink.OperUnknown && (attrs.Flags&net.FlagUp != 0) {
		state = "up"
	}
	info := iface.InterfaceInfo{
		Name:    attrs.Name,
		OsName:  attrs.Name,
		Index:   attrs.Index,
		Type:    link.Type(),
		State:   state,
		MTU:     attrs.MTU,
		Promisc: attrs.Promisc > 0,
	}
	if len(attrs.HardwareAddr) > 0 {
		info.MAC = attrs.HardwareAddr.String()
	}
	// PermHWAddr is the factory/permanent address (IFLA_PERM_ADDRESS); empty
	// for virtual kinds and for NICs whose driver does not report one.
	if len(attrs.PermHWAddr) > 0 {
		info.PermanentMAC = attrs.PermHWAddr.String()
	}
	// IFLA_IFALIAS carries the owned-device ownership marker ("ze:owned:<owner>")
	// for plugin-owned macvlans; the reconcile orphan scan reads it back.
	if attrs.Alias != "" {
		info.Alias = attrs.Alias
	}
	if vlan, ok := link.(*netlink.Vlan); ok {
		info.VlanID = vlan.VlanId
		info.ParentIndex = attrs.ParentIndex
		info.IngressQoSMap = vlan.IngressQosMap
		info.EgressQoSMap = vlan.EgressQosMap
	}
	// Owned macvlans carry their parent's index; expose it so the reconcile
	// drift check can detect a re-parented device and operators can see which
	// parent an owned device rides (show interface).
	if mv, ok := link.(*netlink.Macvlan); ok {
		info.ParentIndex = attrs.ParentIndex
		info.MacvlanMode = macvlanModeName(mv.Mode)
	}
	return info
}

// macvlanModeName maps the kernel macvlan mode to the canonical string used by
// iface.MacvlanMode.String(), so the reconcile drift check compares like for
// like. Only the two modes ze creates are named; anything else reads back as
// "other", which never equals a desired mode and so triggers a re-create
// (fail safe).
func macvlanModeName(mode netlink.MacvlanMode) string {
	switch mode {
	case netlink.MACVLAN_MODE_PRIVATE:
		return "private"
	case netlink.MACVLAN_MODE_BRIDGE:
		return "bridge"
	default:
		return "other"
	}
}

// LinkSpeedDuplex reads the link speed (Mbit/s) and duplex from sysfs, the
// ethtool-backed values the kernel exposes without an ioctl. Both files are
// absent or unreadable for virtual devices and report a negative speed /
// "unknown" duplex for a down link; in every such case this returns the zero
// value (0, "").
//
// This is deliberately NOT called from linkToInfo: that would put two sysfs
// reads per interface on every ListInterfaces call (the 1Hz rate-tracker tick,
// every show/web/health caller), even when nothing consumes the values. Only
// the flow-export counter snapshot needs ifSpeed/ifDirection, so only it calls
// this -- and only when flow-export is configured.
func (b *netlinkBackend) LinkSpeedDuplex(name string) (int, string) {
	speedRaw := ""
	if data, err := os.ReadFile("/sys/class/net/" + name + "/speed"); err == nil { //nolint:gosec // fixed /sys/class/net base plus a kernel interface name; read-only sysfs
		speedRaw = string(data)
	}
	duplexRaw := ""
	if data, err := os.ReadFile("/sys/class/net/" + name + "/duplex"); err == nil { //nolint:gosec // fixed /sys/class/net base plus a kernel interface name; read-only sysfs
		duplexRaw = string(data)
	}
	return parseLinkSpeedDuplex(speedRaw, duplexRaw)
}

// parseLinkSpeedDuplex turns the raw sysfs speed/duplex file contents into a
// sanitized (Mbit/s, duplex) pair. A non-positive or unparseable speed becomes
// 0; only "full" and "half" are accepted as duplex, anything else (including
// the kernel's "unknown") becomes "". Split out from the file reads so the
// value handling is unit-testable without touching /sys.
func parseLinkSpeedDuplex(speedRaw, duplexRaw string) (int, string) {
	speed := 0
	if v, err := strconv.Atoi(strings.TrimSpace(speedRaw)); err == nil && v > 0 {
		speed = v
	}
	duplex := ""
	switch d := strings.TrimSpace(duplexRaw); d {
	case "full", "half":
		duplex = d
	}
	return speed, duplex
}

// ifaFlagTentative is the kernel IFA_F_TENTATIVE address flag: the IPv6 address is still
// completing Duplicate Address Detection and is not yet a usable source.
const ifaFlagTentative = 0x40

func addrList(link netlink.Link) []iface.AddrInfo {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		logger().Warn("failed to list addresses", "interface", link.Attrs().Name, "err", err)
		return nil
	}
	result := make([]iface.AddrInfo, 0, len(addrs))
	for _, a := range addrs {
		fam := "ipv4" //nolint:goconst // AFI label; see ifacenetlink.go for siblings
		if a.IP.To4() == nil {
			fam = "ipv6" //nolint:goconst // AFI label; see ifacenetlink.go for siblings
		}
		ones, _ := a.Mask.Size()
		result = append(result, iface.AddrInfo{
			Address:      a.IP.String(),
			PrefixLength: ones,
			Family:       fam,
			// Surface IFA_F_TENTATIVE so OSPFv3 can prefer a DAD-complete link-local source.
			Tentative: a.Flags&ifaFlagTentative != 0,
			// Classify SLAAC/RA vs static from the kernel IFA_F_* flags, and
			// surface the RA/lease lifetimes (AC-6).
			Origin:            addrOrigin(fam == "ipv6", a.Flags),
			ValidLifetime:     normalizeLifetime(a.ValidLft),
			PreferredLifetime: normalizeLifetime(a.PreferedLft),
		})
	}
	return result
}
