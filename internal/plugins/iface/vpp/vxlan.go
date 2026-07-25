// Design: docs/features/interfaces.md -- VXLAN overlay on the VPP dataplane
// Overview: tunnel.go -- CreateTunnel kind dispatch calls createVxlanTunnel
//
// VXLAN is a new tunnel kind landed in both backends: L2 Ethernet frames over
// a UDP/IPv4 underlay, discriminated by a 24-bit VNI. The VPP half programs
// vxlan_add_del_tunnel_v3; the netlink half builds a netlink.Vxlan link (see
// internal/plugins/iface/netlink/tunnel_linux.go).

package ifacevpp

import (
	"fmt"

	"go.fd.io/govpp/binapi/vxlan"

	"github.com/ze-software/ze/internal/component/iface"
)

// vxlanMaxVNI is the largest valid 24-bit VXLAN Network Identifier.
const vxlanMaxVNI = 16777215

// vxlanDefaultPort is the IANA-assigned VXLAN UDP destination port (RFC 7348).
const vxlanDefaultPort = 4789

// vxlanAutoInstance lets VPP auto-assign the vxlan tunnel instance number.
const vxlanAutoInstance = ^uint32(0)

// createVxlanTunnel programs a vxlan_add_del_tunnel_v3. VNI is validated
// backend-side (1..16777215) as defense in depth behind the YANG range, and
// the UDP destination port defaults to 4789 when the operator omits it.
func (b *vppBackendImpl) createVxlanTunnel(spec iface.TunnelSpec) error {
	if !spec.VNISet || spec.VNI == 0 {
		return fmt.Errorf("ifacevpp: vxlan %q: vni is required (1..%d)", spec.Name, vxlanMaxVNI)
	}
	if spec.VNI > vxlanMaxVNI {
		return fmt.Errorf("ifacevpp: vxlan %q: vni %d out of range (1..%d)", spec.Name, spec.VNI, vxlanMaxVNI)
	}
	src, dst, err := b.tunnelEndpoints(spec)
	if err != nil {
		return err
	}
	port := uint16(vxlanDefaultPort)
	if spec.PortSet && spec.Port != 0 {
		port = spec.Port
	}
	req := &vxlan.VxlanAddDelTunnelV3{
		IsAdd:      true,
		Instance:   vxlanAutoInstance,
		SrcAddress: src,
		DstAddress: dst,
		DstPort:    port,
		Vni:        spec.VNI,
	}
	reply := &vxlan.VxlanAddDelTunnelV3Reply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: VxlanAddDelTunnelV3 %q: %w", spec.Name, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: VxlanAddDelTunnelV3 %q retval=%d", spec.Name, reply.Retval)
	}
	b.names.Add(spec.Name, uint32(reply.SwIfIndex), spec.Name)
	del := *req
	del.IsAdd = false
	del.Instance = uint32(reply.SwIfIndex)
	name := spec.Name
	b.recordDeleter(name, func() error {
		r := &vxlan.VxlanAddDelTunnelV3Reply{}
		if err := b.ch.SendRequest(&del).ReceiveReply(r); err != nil {
			return fmt.Errorf("ifacevpp: VxlanAddDelTunnelV3 delete %q: %w", name, err)
		}
		if r.Retval != 0 {
			return fmt.Errorf("ifacevpp: VxlanAddDelTunnelV3 delete %q retval=%d", name, r.Retval)
		}
		return nil
	})
	return nil
}
