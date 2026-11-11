// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS label input on an interface
// Overview: ifacevpp.go -- vppBackendImpl
// Related: rpf.go -- ReplaceIPv4RPF, the same whole-set read-back-and-retire shape

package ifacevpp

import (
	"fmt"
	"slices"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/mpls"
)

// ReplaceMPLSInput makes interfaces the complete set of VPP interfaces that
// accept labeled frames. VPP receives outside the Linux kernel, so the
// net.mpls.conf.<dev>.input sysctl the iface component also writes judges only
// what VPP punts to the Linux Control Plane tap.
//
// VPP refuses sw_interface_set_mpls_enable until MPLS table 0 exists
// (mpls_sw_interface_enable_disable answers VNET_API_ERROR_NO_SUCH_FIB), so the
// table is created first; creating it again is harmless. The enable is
// reference-counted ("enable/disable only on the 1<->0 transition"), so the
// enabled set is read back with mpls_interface_dump and an interface already
// enabled is not enabled again, while one the set no longer names is disabled
// once, including one enabled while ze was down.
func (b *vppBackendImpl) ReplaceMPLSInput(interfaces []string) error {
	want := make([]interface_types.InterfaceIndex, 0, len(interfaces))
	for _, name := range interfaces {
		idx, err := b.resolveIndex(name)
		if err != nil {
			return fmt.Errorf("ifacevpp: mpls enable on %s: %w", name, err)
		}
		want = append(want, idx)
	}
	if err := b.ensureChannel(); err != nil {
		return err
	}
	if len(want) > 0 {
		if err := b.createMPLSTableZero(); err != nil {
			return err
		}
	}
	enabled, err := b.mplsEnabledInterfaces()
	if err != nil {
		return err
	}
	for _, idx := range enabled {
		if slices.Contains(want, idx) {
			continue
		}
		if err := b.setMPLSInput(idx, false); err != nil {
			return err
		}
	}
	for i, idx := range want {
		if slices.Contains(enabled, idx) {
			continue
		}
		if err := b.setMPLSInput(idx, true); err != nil {
			return fmt.Errorf("ifacevpp: mpls enable on %s: %w", interfaces[i], err)
		}
	}
	return nil
}

// createMPLSTableZero creates MPLS table 0, the label space every
// MPLS-enabled interface looks up.
func (b *vppBackendImpl) createMPLSTableZero() error {
	req := &mpls.MplsTableAddDel{MtIsAdd: true, MtTable: mpls.MplsTable{MtTableID: 0}}
	reply := &mpls.MplsTableAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: mpls_table_add_del(table 0): %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: mpls_table_add_del(table 0): VPP answered retval %d", reply.Retval)
	}
	return nil
}

// mplsEnabledInterfaces reads back every interface VPP accepts labeled frames on.
func (b *vppBackendImpl) mplsEnabledInterfaces() ([]interface_types.InterfaceIndex, error) {
	ctx := b.ch.SendMultiRequest(&mpls.MplsInterfaceDump{SwIfIndex: ^interface_types.InterfaceIndex(0)})
	var out []interface_types.InterfaceIndex
	for {
		row := mpls.MplsInterfaceDetails{}
		last, err := ctx.ReceiveReply(&row)
		if err != nil {
			return nil, fmt.Errorf("ifacevpp: mpls_interface_dump: %w", err)
		}
		if last {
			return out, nil
		}
		out = append(out, row.SwIfIndex)
	}
}

// setMPLSInput enables or disables MPLS input on one interface.
func (b *vppBackendImpl) setMPLSInput(idx interface_types.InterfaceIndex, enable bool) error {
	req := &mpls.SwInterfaceSetMplsEnable{SwIfIndex: idx, Enable: enable}
	reply := &mpls.SwInterfaceSetMplsEnableReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: sw_interface_set_mpls_enable(index %d, %t): %w", idx, enable, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: sw_interface_set_mpls_enable(index %d, %t): VPP answered retval %d",
			idx, enable, reply.Retval)
	}
	return nil
}
