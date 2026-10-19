// Design: docs/features/interfaces.md -- VPP backend, MTU
// Related: ifacevpp.go -- vppBackendImpl and the other Backend methods
// Related: query.go -- SwInterfaceDump helpers

package ifacevpp

import (
	"errors"
	"fmt"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// SetMTU sets the MTU of the named interface.
//
// VPP keeps two MTUs. The link MTU is the NIC's frame size less its overhead.
// HwInterfaceSetMtu sets it by handing the frame size to the driver (DPDK's
// rte_eth_dev_set_mtu, af_packet's netlink link set), so the driver refuses a
// size the NIC cannot carry; on success VPP writes the new value as the L3 MTU
// of every sw interface on that NIC, its VLAN sub-interfaces included. The
// protocol MTUs belong to one sw interface and SwInterfaceSetMtu sets them.
// Its handler (VPP 25.10 interface_api.c) stores them and answers 0 without
// reading the link, so on its own it accepts mtu 9000 over a NIC capped at
// 1500 in silence.
//
// A hardware interface therefore asks for the link first. When VPP refuses it
// (a loopback or a tunnel has a frame size VPP cannot change, a driver refuses
// the size), the protocol MTU alone serves an MTU the link already carries,
// and a larger one is refused naming the link MTU. A sub-interface has no link
// of its own: VPP reports its parent's, which bounds it the same way. A link
// MTU of 0 is an interface that states no frame limit, as VPP's GRE and
// WireGuard interfaces do. A driver that changes the frame size of a stopped
// port only answers busy to a raise; raiseStoppedLink retries it with the port
// stopped.
func (b *vppBackendImpl) SetMTU(ifaceName string, mtu int) error {
	if mtu < 68 {
		return fmt.Errorf("ifacevpp: MTU %d out of range (68-65535)", mtu)
	}
	if mtu > 65535 {
		return fmt.Errorf("ifacevpp: MTU %d out of range (68-65535)", mtu)
	}
	idx, err := b.resolveIndex(ifaceName)
	if err != nil {
		return err
	}
	details, err := b.dumpByIndex(idx)
	if err != nil {
		return err
	}
	linkMTU := int(details.LinkMtu)
	if details.Type != interface_types.IF_API_TYPE_HARDWARE {
		if exceedsLink(mtu, linkMTU) {
			return b.parentLinkRefusal(ifaceName, details.SupSwIfIndex, mtu, linkMTU)
		}
		return b.setProtocolMTU(idx, mtu)
	}
	err = b.setLinkMTU(idx, mtu)
	if err == nil {
		return b.setProtocolMTU(idx, mtu)
	}
	if errors.Is(err, api.BUSY) {
		return b.raiseStoppedLink(ifaceName, details, mtu, err)
	}
	return b.serveRefusedLink(ifaceName, idx, mtu, linkMTU, err)
}

// serveRefusedLink answers a link MTU change VPP did not make. A failure to
// reach VPP is returned as it is. A refusal fails an MTU above the link, and
// an MTU the link already carries takes the protocol MTU alone.
func (b *vppBackendImpl) serveRefusedLink(ifaceName string, idx interface_types.InterfaceIndex, mtu, linkMTU int, err error) error {
	var refusal api.VPPApiError
	if !errors.As(err, &refusal) {
		return err
	}
	if exceedsLink(mtu, linkMTU) {
		return linkRefusal(ifaceName, mtu, linkMTU, refusal)
	}
	return b.setProtocolMTU(idx, mtu)
}

// raiseStoppedLink retries a link MTU raise that the driver refused as busy,
// with the port stopped. Some DPDK drivers change the frame size of a stopped
// port only: i40e_dev_mtu_set (DPDK 24.11) answers -EBUSY while
// dev->data->dev_started is set, and VPP 25.10's dpdk_set_max_frame_size
// returns that as VNET_ERR_BUSY ("port is running"). VPP stops the DPDK port
// when the interface goes admin down (dpdk_interface_admin_up_down calls
// dpdk_device_stop) and starts it again on admin up, so the retry sets the
// interface down, raises the link, and sets it up again whatever the raise
// answered. A port left down is reported as such, never as a refusal.
//
// The stop interrupts the link, so it happens only when it is needed: an MTU
// the link already carries is served by the protocol MTU alone, as for any
// other refusal, and a port that is already down is not touched.
func (b *vppBackendImpl) raiseStoppedLink(ifaceName string, details *interfaces.SwInterfaceDetails, mtu int, busy error) error {
	idx, linkMTU := details.SwIfIndex, int(details.LinkMtu)
	if !exceedsLink(mtu, linkMTU) {
		return b.serveRefusedLink(ifaceName, idx, mtu, linkMTU, busy)
	}
	if details.Flags&interface_types.IF_STATUS_API_FLAG_ADMIN_UP == 0 {
		return b.serveRefusedLink(ifaceName, idx, mtu, linkMTU, busy)
	}
	loggerPtr.Load().Info("ifacevpp: driver changes the frame size of a stopped port only, setting it down to raise its link MTU",
		"interface", ifaceName, "mtu", mtu, "link-mtu", details.LinkMtu)
	if err := b.SetAdminDown(ifaceName); err != nil {
		return fmt.Errorf("ifacevpp: interface %s: the driver raises its link MTU only on a stopped port, "+
			"and setting it down failed: %w", ifaceName, err)
	}
	raised := b.setLinkMTU(idx, mtu)
	if err := b.SetAdminUp(ifaceName); err != nil {
		return fmt.Errorf("ifacevpp: interface %s is left admin down: it was set down to raise its link MTU to %d, "+
			"and setting it up again failed: %w; commit again to bring it up", ifaceName, mtu, err)
	}
	if raised != nil {
		return b.serveRefusedLink(ifaceName, idx, mtu, linkMTU, raised)
	}
	return b.setProtocolMTU(idx, mtu)
}

// inheritParentMTU gives a sub-interface VPP has just created its parent's
// MTU. CreateVlanSubif builds the sub-interface from a zeroed template
// (vl_api_create_vlan_subif_t_handler, VPP 25.10), so its L3 MTU is 0, and
// vnet_sw_interface_get_mtu reads 0 as 9000: over a 1500 parent it would send
// frames the parent cannot carry. Linux gives a new VLAN its parent's MTU, and
// this does the same, so a parent with no mtu leaf still bounds its VLANs.
// VPP's own hardware MTU walk keeps the value in step when the parent's link
// MTU later changes.
//
// The parent's L3 MTU is the MTU it carries; a parent whose L3 MTU is unset
// carries its link MTU. A parent that states neither is read as 9000 by VPP,
// the same answer the sub-interface gets, so it is left as it is: that zero is
// a guard, the one case with no MTU to inherit.
func (b *vppBackendImpl) inheritParentMTU(parentIdx, subIdx interface_types.InterfaceIndex) error {
	parent, err := b.dumpByIndex(parentIdx)
	if err != nil {
		return err
	}
	mtu := int(parent.LinkMtu)
	if len(parent.Mtu) > 0 && parent.Mtu[0] > 0 {
		mtu = int(parent.Mtu[0])
	}
	if mtu == 0 {
		return nil
	}
	return b.setProtocolMTU(subIdx, mtu)
}

// exceedsLink reports whether mtu is larger than a link that states a limit.
func exceedsLink(mtu, linkMTU int) bool {
	if linkMTU <= 0 {
		return false
	}
	return mtu > linkMTU
}

// dumpByIndex returns VPP's details for one sw interface.
func (b *vppBackendImpl) dumpByIndex(idx interface_types.InterfaceIndex) (*interfaces.SwInterfaceDetails, error) {
	ctx := b.ch.SendMultiRequest(&interfaces.SwInterfaceDump{SwIfIndex: idx})
	var found *interfaces.SwInterfaceDetails
	for {
		details := &interfaces.SwInterfaceDetails{}
		last, err := ctx.ReceiveReply(details)
		if err != nil {
			return nil, fmt.Errorf("ifacevpp: SwInterfaceDump(index %d): %w", idx, err)
		}
		if last {
			break
		}
		if details.SwIfIndex == idx {
			found = details
		}
	}
	if found == nil {
		return nil, fmt.Errorf("ifacevpp: VPP reports no interface at index %d", idx)
	}
	return found, nil
}

// setLinkMTU asks VPP to set the hardware link MTU. A refusal by VPP comes
// back as an api.VPPApiError; any other error is a failure to reach VPP.
func (b *vppBackendImpl) setLinkMTU(idx interface_types.InterfaceIndex, mtu int) error {
	req := &interfaces.HwInterfaceSetMtu{SwIfIndex: idx, Mtu: uint16(mtu)}
	reply := &interfaces.HwInterfaceSetMtuReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		if refusal, ok := errors.AsType[api.VPPApiError](err); ok {
			return refusal
		}
		return fmt.Errorf("ifacevpp: HwInterfaceSetMtu: %w", err)
	}
	if reply.Retval != 0 {
		return api.VPPApiError(reply.Retval)
	}
	return nil
}

// setProtocolMTU sets the L3 MTU and leaves each per-protocol MTU at 0, which
// VPP reads as "follow the L3 MTU". Ze configures one MTU per interface, and a
// per-protocol value would stop following the L3 MTU that VPP's hardware MTU
// walk rewrites on a later link change.
func (b *vppBackendImpl) setProtocolMTU(idx interface_types.InterfaceIndex, mtu int) error {
	req := &interfaces.SwInterfaceSetMtu{
		SwIfIndex: idx,
		Mtu:       []uint32{uint32(mtu), 0, 0, 0},
	}
	reply := &interfaces.SwInterfaceSetMtuReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: SetMTU: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: SetMTU retval=%d", reply.Retval)
	}
	return nil
}

// linkRefusal is the error for an MTU above a hardware link VPP would not
// raise: it names the interface, the asked MTU, the link MTU, VPP's answer
// and the command that brings the config within the limit.
func linkRefusal(ifaceName string, mtu, linkMTU int, refusal api.VPPApiError) error {
	var fix textbuf.Buffer
	fix.Reset().Str("set interface ethernet ").Str(ifaceName).Str(" mtu ").Int(int64(linkMTU))
	return fmt.Errorf("ifacevpp: interface %s cannot carry mtu %d: VPP refused to raise its link MTU from %d (%w), "+
		"so the NIC or its driver takes no larger frame; lower the mtu to %d or less with `%s` and commit",
		ifaceName, mtu, linkMTU, refusal, linkMTU, fix.String())
}

// parentLinkRefusal is the error for a sub-interface MTU above its parent's
// link MTU. The schema gives a VLAN no MTU of its own, so the fix is on the
// parent.
func (b *vppBackendImpl) parentLinkRefusal(ifaceName string, parentIdx uint32, mtu, linkMTU int) error {
	parent, known := b.names.lookupName(parentIdx)
	if !known {
		return fmt.Errorf("ifacevpp: interface %s cannot carry mtu %d: its parent (VPP index %d) has link MTU %d; "+
			"lower the parent's mtu to %d or less and commit", ifaceName, mtu, parentIdx, linkMTU, linkMTU)
	}
	var fix textbuf.Buffer
	fix.Reset().Str("set interface ethernet ").Str(parent).Str(" mtu ").Int(int64(linkMTU))
	return fmt.Errorf("ifacevpp: interface %s cannot carry mtu %d: its parent %s has link MTU %d; "+
		"lower the parent's mtu to %d or less with `%s` and commit", ifaceName, mtu, parent, linkMTU, linkMTU, fix.String())
}
