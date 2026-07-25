// Design: docs/features/interfaces.md -- Tunnel interface creation on the VPP dataplane
// Overview: ifacevpp.go -- vppBackendImpl, recordDeleter, naming
//
// CreateTunnel programs GRE (L3), GRETAP (L2/TEB), IPIP, and VXLAN tunnels on
// the VPP dataplane via the GoVPP binary API. The netlink backend's
// tunnel_linux.go is the parity reference. The other tunnel kinds (ip6gre*,
// sit, ip6tnl, ipip6) remain netlink-only; the ze:backend commit gate rejects
// them under the vpp backend before they reach this method, and the
// exact-or-reject default below is defense in depth.

package ifacevpp

import (
	"fmt"

	"go.fd.io/govpp/binapi/gre"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/ipip"
	"go.fd.io/govpp/binapi/tunnel_types"

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
	default:
		var tb textbuf.Buffer
		return errNotSupported(tb.Str("CreateTunnel kind ").Str(spec.Kind.String()).Str(" (netlink-only on this backend)").String())
	}
}

// tunnelEndpoints resolves the local/remote endpoint addresses shared by the
// gre and ipip families. VPP terminates tunnels on an address, not on a
// parent ifindex, so a local-interface source (valid on netlink) is rejected
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
	return nil
}
