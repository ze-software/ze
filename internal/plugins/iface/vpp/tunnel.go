// Design: docs/features/interfaces.md -- Tunnel interface creation on the VPP dataplane
// Overview: ifacevpp.go -- vppBackendImpl, recordDeleter, naming
//
// CreateTunnel programs GRE (L3), GRETAP (L2/TEB), IPIP, and VXLAN tunnels on
// the VPP dataplane via the GoVPP binary API. The kernel backend's
// tunnel_linux.go is the parity reference. The other tunnel kinds (ip6gre*,
// sit, ip6tnl, ipip6) remain kernel-only; the ze:backend commit gate rejects
// them under the vpp backend before they reach this method, and the
// exact-or-reject default below is defense in depth.

package ifacevpp

import (
	"fmt"
	"net/netip"

	"go.fd.io/govpp/binapi/gre"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/ipip"
	"go.fd.io/govpp/binapi/tunnel_types"
	"go.fd.io/govpp/binapi/vxlan"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CreateTunnel dispatches on the tunnel kind. VPP supports the v4-underlay
// point-to-point kinds gre/gretap/ipip plus vxlan (a distinct message family,
// see vxlan.go); every other kind is rejected so a widened ze:backend
// annotation can never silently no-op an unsupported kind.
func (b *vppBackendImpl) CreateTunnel(spec iface.TunnelSpec) error {
	if err := b.ensureChannel(); err != nil {
		return err
	}
	switch spec.Kind {
	case iface.TunnelKindGRE:
		return b.createGRETunnel(spec, gre.GRE_API_TUNNEL_TYPE_L3)
	case iface.TunnelKindGRETap:
		return b.createGRETunnel(spec, gre.GRE_API_TUNNEL_TYPE_TEB)
	case iface.TunnelKindIPIP:
		return b.createIPIPTunnel(spec)
	case iface.TunnelKindVxlan:
		return b.createVxlanTunnel(spec)
	case iface.TunnelKindUnknown, iface.TunnelKindIP6GRE, iface.TunnelKindIP6GRETap,
		iface.TunnelKindSIT, iface.TunnelKindIP6Tnl, iface.TunnelKindIPIP6:
		// These named kinds retain the unsupported-kind error below.
	default:
		// Backend callers can bypass config validation; reject unknown kinds.
	}
	var tb textbuf.Buffer
	return errNotSupported(tb.Str("CreateTunnel kind ").Str(spec.Kind.String()).Str(" (kernel-only on this backend)").String())
}

// tunnelEndpoints resolves the local/remote endpoint addresses shared by the
// gre and ipip families. VPP terminates tunnels on an address, not on a
// parent ifindex, so a local-interface source (valid on kernel) is rejected
// with a clear error rather than silently ignored.
func (b *vppBackendImpl) tunnelEndpoints(spec iface.TunnelSpec) (src, dst ip_types.Address, err error) {
	if spec.LocalInterface != "" {
		return src, dst, fmt.Errorf("ifacevpp: tunnel %q: local-interface source not supported on VPP backend (use local ip)", spec.Name)
	}
	if spec.LocalAddress == "" {
		return src, dst, fmt.Errorf("ifacevpp: tunnel %q: local ip is required on VPP backend", spec.Name)
	}
	src, err = ip_types.ParseAddress(spec.LocalAddress)
	if err != nil {
		return src, dst, fmt.Errorf("ifacevpp: tunnel %q local %q: %w", spec.Name, spec.LocalAddress, err)
	}
	// RFC 4213 Section 3.6. Config verify already refused an unparsable or
	// unspecified remote through the same check; this is its pair on the
	// backend path, as the kernel backend's parseTunnelRemote does.
	if err = spec.CheckRemote(); err != nil {
		return src, dst, fmt.Errorf("ifacevpp: tunnel %q: %w", spec.Name, err)
	}
	dst, err = ip_types.ParseAddress(spec.RemoteAddress)
	if err != nil {
		return src, dst, fmt.Errorf("ifacevpp: tunnel %q remote %q: %w", spec.Name, spec.RemoteAddress, err)
	}
	return src, dst, nil
}

// createGRETunnel programs a gre_tunnel_add_del for the gre (L3) or gretap
// (TEB) kind. The v0.13.0 gre_tunnel API carries no GRE key field, so a
// configured key is rejected rather than silently dropped.
func (b *vppBackendImpl) createGRETunnel(spec iface.TunnelSpec, typ gre.GreTunnelType) error {
	if spec.KeySet {
		return fmt.Errorf("ifacevpp: tunnel %q: GRE key not supported on VPP backend (gre_tunnel API has no key field)", spec.Name)
	}
	src, dst, err := b.tunnelEndpoints(spec)
	if err != nil {
		return err
	}
	tun := gre.GreTunnel{
		Type: typ,
		Mode: tunnel_types.TUNNEL_API_MODE_P2P,
		Src:  src,
		Dst:  dst,
	}
	reply := &gre.GreTunnelAddDelReply{}
	if err := b.ch.SendRequest(&gre.GreTunnelAddDel{IsAdd: true, Tunnel: tun}).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: GreTunnelAddDel %q: %w", spec.Name, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: GreTunnelAddDel %q retval=%d", spec.Name, reply.Retval)
	}
	b.names.Add(spec.Name, uint32(reply.SwIfIndex), spec.Name)
	delTun := tun
	delTun.SwIfIndex = reply.SwIfIndex
	name := spec.Name
	b.recordDeleter(name, func() error {
		r := &gre.GreTunnelAddDelReply{}
		if err := b.ch.SendRequest(&gre.GreTunnelAddDel{IsAdd: false, Tunnel: delTun}).ReceiveReply(r); err != nil {
			return fmt.Errorf("ifacevpp: GreTunnelAddDel delete %q: %w", name, err)
		}
		if r.Retval != 0 {
			return fmt.Errorf("ifacevpp: GreTunnelAddDel delete %q retval=%d", name, r.Retval)
		}
		return nil
	})
	b.emitCreated(name, spec.Kind.String(), reply.SwIfIndex)
	return nil
}

// createIPIPTunnel programs an ipip_add_tunnel (RFC 2003 IPv4-in-IPv4). Delete
// uses ipip_del_tunnel keyed on the returned sw_if_index.
func (b *vppBackendImpl) createIPIPTunnel(spec iface.TunnelSpec) error {
	src, dst, err := b.tunnelEndpoints(spec)
	if err != nil {
		return err
	}
	reply := &ipip.IpipAddTunnelReply{}
	req := &ipip.IpipAddTunnel{Tunnel: ipip.IpipTunnel{
		Src:  src,
		Dst:  dst,
		Mode: tunnel_types.TUNNEL_API_MODE_P2P,
	}}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: IpipAddTunnel %q: %w", spec.Name, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: IpipAddTunnel %q retval=%d", spec.Name, reply.Retval)
	}
	b.names.Add(spec.Name, uint32(reply.SwIfIndex), spec.Name)
	idx := reply.SwIfIndex
	name := spec.Name
	b.recordDeleter(name, func() error {
		r := &ipip.IpipDelTunnelReply{}
		if err := b.ch.SendRequest(&ipip.IpipDelTunnel{SwIfIndex: idx}).ReceiveReply(r); err != nil {
			return fmt.Errorf("ifacevpp: IpipDelTunnel %q: %w", name, err)
		}
		if r.Retval != 0 {
			return fmt.Errorf("ifacevpp: IpipDelTunnel %q retval=%d", name, r.Retval)
		}
		return nil
	})
	b.emitCreated(name, spec.Kind.String(), idx)
	return nil
}

// GetTunnelInfo reads live encapsulation state for the named VPP tunnel
// interface via VPP's GRE, IPIP, and VXLAN dump APIs. Returns an error when
// the interface does not exist or is not one of VPP's supported tunnel kinds.
func (b *vppBackendImpl) GetTunnelInfo(name string) (iface.TunnelInfo, error) {
	if err := b.ensureChannel(); err != nil {
		return iface.TunnelInfo{}, err
	}
	idx, err := b.resolveIndex(name)
	if err != nil {
		return iface.TunnelInfo{}, err
	}
	info, found, err := b.dumpGRETunnelInfo(name, idx)
	if err != nil {
		return iface.TunnelInfo{}, err
	}
	if found {
		return info, nil
	}
	info, found, err = b.dumpIPIPTunnelInfo(name, idx)
	if err != nil {
		return iface.TunnelInfo{}, err
	}
	if found {
		return info, nil
	}
	info, found, err = b.dumpVxlanTunnelInfo(name, idx)
	if err != nil {
		return iface.TunnelInfo{}, err
	}
	if found {
		return info, nil
	}
	return iface.TunnelInfo{}, fmt.Errorf("ifacevpp: %q is not a VPP tunnel interface", name)
}

func (b *vppBackendImpl) dumpGRETunnelInfo(name string, idx interface_types.InterfaceIndex) (iface.TunnelInfo, bool, error) {
	ctx := b.ch.SendMultiRequest(&gre.GreTunnelDump{SwIfIndex: idx})
	var found bool
	var info iface.TunnelInfo
	// Bounded by VPP: the dump ends with its last-reply marker.
	for {
		d := &gre.GreTunnelDetails{}
		last, err := ctx.ReceiveReply(d)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: GreTunnelDump %q: %w", name, err)
		}
		if last {
			break
		}
		if d.Tunnel.SwIfIndex != idx {
			continue
		}
		kind, err := vppGRETunnelKind(d.Tunnel.Type)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: GreTunnelDump %q: %w", name, err)
		}
		flowBased, err := vppTunnelModeFlowBased(d.Tunnel.Mode)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: GreTunnelDump %q: %w", name, err)
		}
		local, err := vppAddrToNetip(d.Tunnel.Src)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: GreTunnelDump %q src: %w", name, err)
		}
		remote, err := vppAddrToNetip(d.Tunnel.Dst)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: GreTunnelDump %q dst: %w", name, err)
		}
		if err := requireIPv4VPPEndpoints(name, kind.String(), local, remote); err != nil {
			return iface.TunnelInfo{}, false, err
		}
		info = iface.TunnelInfo{
			Kind:      kind,
			Local:     local,
			Remote:    remote,
			FlowBased: flowBased,
		}
		found = true
	}
	return info, found, nil
}

func (b *vppBackendImpl) dumpIPIPTunnelInfo(name string, idx interface_types.InterfaceIndex) (iface.TunnelInfo, bool, error) {
	ctx := b.ch.SendMultiRequest(&ipip.IpipTunnelDump{SwIfIndex: idx})
	var found bool
	var info iface.TunnelInfo
	// Bounded by VPP: the dump ends with its last-reply marker.
	for {
		d := &ipip.IpipTunnelDetails{}
		last, err := ctx.ReceiveReply(d)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: IpipTunnelDump %q: %w", name, err)
		}
		if last {
			break
		}
		if d.Tunnel.SwIfIndex != idx {
			continue
		}
		flowBased, err := vppTunnelModeFlowBased(d.Tunnel.Mode)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: IpipTunnelDump %q: %w", name, err)
		}
		local, err := vppAddrToNetip(d.Tunnel.Src)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: IpipTunnelDump %q src: %w", name, err)
		}
		remote, err := vppAddrToNetip(d.Tunnel.Dst)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: IpipTunnelDump %q dst: %w", name, err)
		}
		if err := requireIPv4VPPEndpoints(name, iface.TunnelKindIPIP.String(), local, remote); err != nil {
			return iface.TunnelInfo{}, false, err
		}
		info = iface.TunnelInfo{
			Kind:      iface.TunnelKindIPIP,
			Local:     local,
			Remote:    remote,
			FlowBased: flowBased,
		}
		found = true
	}
	return info, found, nil
}

func vppTunnelModeFlowBased(mode tunnel_types.TunnelMode) (bool, error) {
	switch mode {
	case tunnel_types.TUNNEL_API_MODE_P2P:
		return false, nil
	case tunnel_types.TUNNEL_API_MODE_MP:
		return true, nil
	default:
		// VPP tunnel modes are open; refuse any mode we cannot interpret safely.
		return false, fmt.Errorf("unsupported VPP tunnel mode %d", mode)
	}
}

func requireIPv4VPPEndpoints(name, kind string, local, remote netip.Addr) error {
	if local.IsValid() && local.Is6() {
		return errNotSupported("IPv6-underlay VPP " + kind + " tunnel " + name)
	}
	if remote.IsValid() && remote.Is6() {
		return errNotSupported("IPv6-underlay VPP " + kind + " tunnel " + name)
	}
	return nil
}

func (b *vppBackendImpl) dumpVxlanTunnelInfo(name string, idx interface_types.InterfaceIndex) (iface.TunnelInfo, bool, error) {
	ctx := b.ch.SendMultiRequest(&vxlan.VxlanTunnelV2Dump{SwIfIndex: idx})
	var found bool
	var info iface.TunnelInfo
	// Bounded by VPP: the dump ends with its last-reply marker.
	for {
		d := &vxlan.VxlanTunnelV2Details{}
		last, err := ctx.ReceiveReply(d)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: VxlanTunnelV2Dump %q: %w", name, err)
		}
		if last {
			break
		}
		if d.SwIfIndex != idx {
			continue
		}
		local, err := vppAddrToNetip(d.SrcAddress)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: VxlanTunnelV2Dump %q src: %w", name, err)
		}
		remote, err := vppAddrToNetip(d.DstAddress)
		if err != nil {
			return iface.TunnelInfo{}, false, fmt.Errorf("ifacevpp: VxlanTunnelV2Dump %q dst: %w", name, err)
		}
		info = iface.TunnelInfo{
			Kind:   iface.TunnelKindVxlan,
			Local:  local,
			Remote: remote,
			VNI:    d.Vni,
		}
		found = true
	}
	return info, found, nil
}

func vppGRETunnelKind(typ gre.GreTunnelType) (iface.TunnelKind, error) {
	switch typ {
	case gre.GRE_API_TUNNEL_TYPE_L3:
		return iface.TunnelKindGRE, nil
	case gre.GRE_API_TUNNEL_TYPE_TEB:
		return iface.TunnelKindGRETap, nil
	case gre.GRE_API_TUNNEL_TYPE_ERSPAN:
		return iface.TunnelKindUnknown, fmt.Errorf("unsupported GRE tunnel type %s", typ)
	default:
		// VPP's GreTunnelType set is open; reject unknown types.
		return iface.TunnelKindUnknown, fmt.Errorf("unknown GRE tunnel type %d", typ)
	}
}

// vppAddrToNetip converts a VPP ip_types.Address into a netip.Addr. The
// address family comes from VPP (an open set): an unknown family returns an
// error instead of a zero address.
func vppAddrToNetip(a ip_types.Address) (netip.Addr, error) {
	switch a.Af {
	case ip_types.ADDRESS_IP4:
		return netip.AddrFrom4(a.Un.GetIP4()), nil
	case ip_types.ADDRESS_IP6:
		return netip.AddrFrom16(a.Un.GetIP6()), nil
	default:
		// VPP's AddressFamily set is open; refuse unknown values.
		return netip.Addr{}, fmt.Errorf("unknown VPP address family %d", a.Af)
	}
}
