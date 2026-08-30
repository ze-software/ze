// Design: docs/features/interfaces.md -- XFRM interface netlink backend
// Related: tunnel_linux.go -- CreateTunnel follows the same LinkAdd pattern

//go:build linux

package ifacenetlink

import (
	"fmt"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func (b *netlinkBackend) CreateXFRM(spec iface.XFRMSpec) error {
	link := &netlink.Xfrmi{
		Name: spec.Name,
		Ifid: spec.IfID,
	}
	if spec.PhysicalDev != "" {
		parent, err := netlink.LinkByName(spec.PhysicalDev)
		if err != nil {
			return fmt.Errorf("xfrm %s: parent dev %q: %w", spec.Name, spec.PhysicalDev, err)
		}
		link.ParentIndex = parent.Attrs().Index
	}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("xfrm %s: link add: %w", spec.Name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("xfrm %s: link up: %w", spec.Name, err)
	}
	return nil
}

func (b *netlinkBackend) GetXFRMInfo(name string) (iface.XFRMInfo, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return iface.XFRMInfo{}, fmt.Errorf("xfrm info %s: %w", name, err)
	}
	xfrmi, ok := link.(*netlink.Xfrmi)
	if !ok {
		return iface.XFRMInfo{}, fmt.Errorf("xfrm info %s: not an xfrm interface", name)
	}

	info := iface.XFRMInfo{
		IfID: xfrmi.Ifid,
	}

	if xfrmi.ParentIndex > 0 {
		if parent, pErr := netlink.LinkByIndex(xfrmi.ParentIndex); pErr == nil {
			info.ParentDev = parent.Attrs().Name
		}
	}

	addrs, aErr := netlink.AddrList(link, netlink.FAMILY_ALL)
	if aErr == nil {
		for _, a := range addrs {
			ones, _ := a.Mask.Size()
			var buf textbuf.Buffer
			info.Addresses = append(info.Addresses, buf.Reset().Str(a.IP.String()).Byte('/').Int(int64(ones)).String())
		}
	}

	policies, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		return info, nil //nolint:nilerr // best-effort: XFRM policy list is optional enrichment, absent on error
	}
	for i := range policies {
		p := &policies[i]
		if p.Ifid != int(xfrmi.Ifid) {
			continue
		}
		pi := iface.XFRMPolicyInfo{
			Dir: xfrmDirString(p.Dir),
		}
		if p.Src != nil {
			pi.Src = p.Src.String()
		}
		if p.Dst != nil {
			pi.Dst = p.Dst.String()
		}
		if p.Proto != 0 {
			pi.Proto = textbuf.StringInt(int64(p.Proto))
		}
		for _, tmpl := range p.Tmpls {
			pi.Mode = xfrmModeString(tmpl.Mode)
			break
		}
		info.Policies = append(info.Policies, pi)
	}

	return info, nil
}

func xfrmDirString(dir netlink.Dir) string {
	switch dir {
	case netlink.XFRM_DIR_IN:
		return "in"
	case netlink.XFRM_DIR_OUT:
		return "out"
	case netlink.XFRM_DIR_FWD:
		return "fwd"
	default:
		return textbuf.StringInt(int64(dir))
	}
}

func xfrmModeString(mode netlink.Mode) string {
	switch mode {
	case netlink.XFRM_MODE_TRANSPORT:
		return "transport"
	case netlink.XFRM_MODE_TUNNEL:
		return "tunnel"
	default:
		return textbuf.StringInt(int64(mode))
	}
}
