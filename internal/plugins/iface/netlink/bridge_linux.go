// Design: docs/features/interfaces.md -- Bridge interface management
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

func (b *netlinkBackend) BridgeAddPort(bridgeName, portName string) error {
	if err := iface.ValidateIfaceName(bridgeName); err != nil {
		return fmt.Errorf("iface: bridge add port: bridge: %w", err)
	}
	if err := iface.ValidateIfaceName(portName); err != nil {
		return fmt.Errorf("iface: bridge add port: port: %w", err)
	}
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("iface: bridge %q not found: %w", bridgeName, err)
	}
	if br.Type() != linkTypeBridge {
		return fmt.Errorf("iface: %q is not a bridge (type %q)", bridgeName, br.Type())
	}
	port, err := netlink.LinkByName(portName)
	if err != nil {
		return fmt.Errorf("iface: bridge port %q not found: %w", portName, err)
	}
	if err := netlink.LinkSetMaster(port, br); err != nil {
		return fmt.Errorf("iface: add port %q to bridge %q: %w", portName, bridgeName, err)
	}
	return nil
}

func (b *netlinkBackend) BridgeDelPort(portName string) error {
	if err := iface.ValidateIfaceName(portName); err != nil {
		return fmt.Errorf("iface: bridge del port: %w", err)
	}
	port, err := netlink.LinkByName(portName)
	if err != nil {
		return fmt.Errorf("iface: bridge del port %q: not found: %w", portName, err)
	}
	if err := netlink.LinkSetNoMaster(port); err != nil {
		return fmt.Errorf("iface: remove port %q from bridge: %w", portName, err)
	}
	return nil
}

// BridgeSetSTP turns the kernel spanning tree on or off for an existing
// bridge. The setting travels over rtnetlink (IFLA_BR_STP_STATE), never through
// /sys/class/net/<bridge>/bridge/stp_state: a container mounts /sys read-only,
// and sysfs shows the network namespace that mounted it, not the caller's, so
// a sysfs write fails or reaches the wrong bridge. CAP_NET_ADMIN in the
// bridge's namespace is all the rtnetlink change needs.
func (b *netlinkBackend) BridgeSetSTP(bridgeName string, enabled bool) error {
	if err := iface.ValidateIfaceName(bridgeName); err != nil {
		return fmt.Errorf("iface: bridge stp: %w", err)
	}
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("iface: bridge stp: bridge %q not found: %w", bridgeName, err)
	}
	if br.Type() != linkTypeBridge {
		return fmt.Errorf("iface: bridge stp: %q is not a bridge (type %q)", bridgeName, br.Type())
	}
	if _, err := bridgeSTPRequest(br.Attrs().Index, enabled).Execute(unix.NETLINK_ROUTE, 0); err != nil {
		return fmt.Errorf("iface: bridge stp on %q: the kernel refused the rtnetlink change "+
			"(IFLA_BR_STP_STATE): %w; ze needs CAP_NET_ADMIN in the bridge's network namespace, "+
			"check the current state with: ip -d link show %s", bridgeName, err, bridgeName)
	}
	return nil
}

// bridgeSTPRequest builds the RTM_NEWLINK change request that sets the STP
// state of the bridge at index. The request names the link by index and
// carries no NLM_F_CREATE, so the kernel changes an existing bridge and never
// creates one. IFLA_INFO_KIND must name "bridge": rtnl_newlink hands
// IFLA_INFO_DATA to the changelink of the kind it names, and refuses the
// request when that kind is not the device's own.
//
// Layout (net/bridge/br_netlink.c br_changelink reads the u32):
//
//	struct ifinfomsg            family AF_UNSPEC, ifi_index = index
//	IFLA_LINKINFO               nested
//	  IFLA_INFO_KIND            "bridge" (no NUL)
//	  IFLA_INFO_DATA            nested
//	    IFLA_BR_STP_STATE       u32, 1 = on, 0 = off
func bridgeSTPRequest(index int, enabled bool) *nl.NetlinkRequest {
	var state uint32
	if enabled {
		state = 1
	}
	req := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_ACK)
	msg := nl.NewIfInfomsg(unix.AF_UNSPEC)
	msg.Index = int32(index) //nolint:gosec // a kernel ifindex is a positive int32 (struct ifinfomsg ifi_index)
	req.AddData(msg)
	linkInfo := nl.NewRtAttr(unix.IFLA_LINKINFO, nil)
	linkInfo.AddRtAttr(nl.IFLA_INFO_KIND, nl.NonZeroTerminated(linkTypeBridge))
	data := linkInfo.AddRtAttr(nl.IFLA_INFO_DATA, nil)
	data.AddRtAttr(unix.IFLA_BR_STP_STATE, nl.Uint32Attr(state))
	req.AddData(linkInfo)
	return req
}
