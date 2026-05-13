// Design: docs/research/vpp-deployment-reference.md -- VPP MPLS FIB programming
// Related: backend.go -- vppBackend interface extended with MPLS methods
// Related: fibvpp.go -- processEvent dispatches to MPLS when labels present

package fibvpp

import (
	"fmt"
	"net/netip"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/mpls"
)

const (
	maxMPLSLabel   = 1048575 // 20-bit, RFC 3032
	maxLabelStack  = 16      // VPP FibPath.LabelStack fixed array
	defaultMPLSTTL = 64
	mplsEOSBit     = 1
)

// mplsBackend extends vppBackend with MPLS operations.
type mplsBackend interface {
	addMPLSRoute(prefix netip.Prefix, nextHop netip.Addr, labels []uint32) error
	delMPLSRoute(prefix netip.Prefix, labels []uint32) error
	addMPLSSwap(inLabel, outLabel uint32, nextHop netip.Addr) error
	delMPLSSwap(inLabel uint32) error
	enableMPLS(swIfIndex uint32) error
	disableMPLS(swIfIndex uint32) error
}

// validateLabels checks that all labels are within the 20-bit range and
// the stack does not exceed the VPP FibPath limit.
func validateLabels(labels []uint32) error {
	if len(labels) == 0 {
		return fmt.Errorf("mpls: empty label stack")
	}
	if len(labels) > maxLabelStack {
		return fmt.Errorf("mpls: label stack depth %d exceeds limit %d", len(labels), maxLabelStack)
	}
	for _, l := range labels {
		if l > maxMPLSLabel {
			return fmt.Errorf("mpls: label %d exceeds 20-bit maximum %d", l, maxMPLSLabel)
		}
	}
	return nil
}

// govppMPLSBackend implements mplsBackend using GoVPP binary API.
type govppMPLSBackend struct {
	ch      api.Channel
	tableID uint32
}

func newGovppMPLSBackend(ch api.Channel, tableID uint32) *govppMPLSBackend {
	return &govppMPLSBackend{ch: ch, tableID: tableID}
}

// addMPLSRoute programs a label push: IP route with MPLS label stack on the FibPath.
func (b *govppMPLSBackend) addMPLSRoute(prefix netip.Prefix, nextHop netip.Addr, labels []uint32) error {
	if err := validateLabels(labels); err != nil {
		return err
	}
	path := toFibPath(nextHop)
	path.NLabels = uint8(len(labels)) //nolint:gosec // validated <= 16
	for i, l := range labels {
		path.LabelStack[i] = fib_types.FibMplsLabel{
			Label: l,
			TTL:   defaultMPLSTTL,
			Exp:   0,
		}
	}
	req := &ip.IPRouteAddDel{
		IsAdd: true,
		Route: ip.IPRoute{
			TableID: b.tableID,
			Prefix:  toVPPPrefix(prefix),
			NPaths:  1,
			Paths:   []fib_types.FibPath{path},
		},
	}
	reply := &ip.IPRouteAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("mpls IPRouteAddDel: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("mpls IPRouteAddDel retval=%d", reply.Retval)
	}
	return nil
}

// delMPLSRoute removes a labeled IP route.
func (b *govppMPLSBackend) delMPLSRoute(prefix netip.Prefix, _ []uint32) error {
	req := &ip.IPRouteAddDel{
		IsAdd: false,
		Route: ip.IPRoute{
			TableID: b.tableID,
			Prefix:  toVPPPrefix(prefix),
		},
	}
	reply := &ip.IPRouteAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("mpls del IPRouteAddDel: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("mpls del IPRouteAddDel retval=%d", reply.Retval)
	}
	return nil
}

// addMPLSSwap programs label swap: in-label -> out-label + next-hop.
func (b *govppMPLSBackend) addMPLSSwap(inLabel, outLabel uint32, nextHop netip.Addr) error {
	if inLabel > maxMPLSLabel {
		return fmt.Errorf("mpls: in-label %d exceeds maximum", inLabel)
	}
	if outLabel > maxMPLSLabel {
		return fmt.Errorf("mpls: out-label %d exceeds maximum", outLabel)
	}
	path := toFibPath(nextHop)
	path.NLabels = 1
	path.LabelStack[0] = fib_types.FibMplsLabel{
		Label: outLabel,
		TTL:   defaultMPLSTTL,
	}
	req := &mpls.MplsRouteAddDel{
		MrIsAdd: true,
		MrRoute: mpls.MplsRoute{
			MrTableID: b.tableID,
			MrLabel:   inLabel,
			MrEos:     mplsEOSBit,
			MrNPaths:  1,
			MrPaths:   []fib_types.FibPath{path},
		},
	}
	reply := &mpls.MplsRouteAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("mpls MplsRouteAddDel: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("mpls MplsRouteAddDel retval=%d", reply.Retval)
	}
	return nil
}

// delMPLSSwap removes an MPLS swap/pop entry.
func (b *govppMPLSBackend) delMPLSSwap(inLabel uint32) error {
	if inLabel > maxMPLSLabel {
		return fmt.Errorf("mpls: in-label %d exceeds maximum", inLabel)
	}
	req := &mpls.MplsRouteAddDel{
		MrIsAdd: false,
		MrRoute: mpls.MplsRoute{
			MrTableID: b.tableID,
			MrLabel:   inLabel,
			MrEos:     mplsEOSBit,
		},
	}
	reply := &mpls.MplsRouteAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("mpls del MplsRouteAddDel: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("mpls del MplsRouteAddDel retval=%d", reply.Retval)
	}
	return nil
}

// enableMPLS enables MPLS on a VPP interface.
func (b *govppMPLSBackend) enableMPLS(swIfIndex uint32) error {
	return b.setMPLSInterface(swIfIndex, true)
}

// disableMPLS disables MPLS on a VPP interface.
func (b *govppMPLSBackend) disableMPLS(swIfIndex uint32) error {
	return b.setMPLSInterface(swIfIndex, false)
}

func (b *govppMPLSBackend) setMPLSInterface(swIfIndex uint32, enable bool) error {
	req := &mpls.SwInterfaceSetMplsEnable{
		SwIfIndex: interface_types.InterfaceIndex(swIfIndex),
		Enable:    enable,
	}
	reply := &mpls.SwInterfaceSetMplsEnableReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("mpls SwInterfaceSetMplsEnable: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("mpls SwInterfaceSetMplsEnable retval=%d", reply.Retval)
	}
	return nil
}

// mockMPLSBackend is a test double for MPLS operations.
type mockMPLSBackend struct {
	pushes    []mplsPushOp
	swaps     []mplsSwapOp
	delPushes []netip.Prefix
	delSwaps  []uint32
	enables   []uint32
	disables  []uint32
	err       error
}

type mplsPushOp struct {
	prefix  netip.Prefix
	nextHop netip.Addr
	labels  []uint32
}

type mplsSwapOp struct {
	inLabel  uint32
	outLabel uint32
	nextHop  netip.Addr
}

func (m *mockMPLSBackend) addMPLSRoute(prefix netip.Prefix, nextHop netip.Addr, labels []uint32) error {
	if m.err != nil {
		return m.err
	}
	if err := validateLabels(labels); err != nil {
		return err
	}
	cp := make([]uint32, len(labels))
	copy(cp, labels)
	m.pushes = append(m.pushes, mplsPushOp{prefix, nextHop, cp})
	return nil
}

func (m *mockMPLSBackend) delMPLSRoute(prefix netip.Prefix, _ []uint32) error {
	if m.err != nil {
		return m.err
	}
	m.delPushes = append(m.delPushes, prefix)
	return nil
}

func (m *mockMPLSBackend) addMPLSSwap(inLabel, outLabel uint32, nextHop netip.Addr) error {
	if m.err != nil {
		return m.err
	}
	m.swaps = append(m.swaps, mplsSwapOp{inLabel, outLabel, nextHop})
	return nil
}

func (m *mockMPLSBackend) delMPLSSwap(inLabel uint32) error {
	if m.err != nil {
		return m.err
	}
	m.delSwaps = append(m.delSwaps, inLabel)
	return nil
}

func (m *mockMPLSBackend) enableMPLS(swIfIndex uint32) error {
	if m.err != nil {
		return m.err
	}
	m.enables = append(m.enables, swIfIndex)
	return nil
}

func (m *mockMPLSBackend) disableMPLS(swIfIndex uint32) error {
	if m.err != nil {
		return m.err
	}
	m.disables = append(m.disables, swIfIndex)
	return nil
}
