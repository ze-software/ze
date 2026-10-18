// Design: docs/features/interfaces.md -- reverse-path check
// Overview: ifacevpp.go -- vppBackendImpl

package ifacevpp

import (
	"errors"
	"fmt"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/urpf"

	"github.com/ze-software/ze/internal/component/iface"
)

// urpfPlugin is the VPP plugin that carries the reverse-path check. The
// startup.conf ze writes for a managed VPP always enables it.
const urpfPlugin = "urpf_plugin.so"

// errIPv6RPFUnsupported is the refusal of an IPv6 rpf-check on this backend.
// VPP's uRPF looks a source up in the interface's FIB, and VPP's fe80::/10
// entry (the ip6-link-local DPO) carries an empty uRPF list, so both strict
// and loose drop every unicast packet from a link-local source: neighbor
// advertisements, DHCPv6 and link-local BGP. Measured on VPP 26.06 with the
// packet generator. The only exemption VPP offers is `set urpf-accept`, a CLI
// its own help calls "for testing purposes only", which has no binary API and
// does not pass strict at all. Refusing is the only answer that neither
// breaks neighbor discovery nor leaves the operator believing the check is on.
var errIPv6RPFUnsupported = errors.New("ifacevpp: ipv6 rpf-check cannot be enforced on the vpp backend, " +
	"because VPP's uRPF drops every unicast packet from a link-local source, which breaks neighbor discovery; " +
	"remove `ipv6 rpf-check` from the unit, or run the interface on the netlink backend")

// ReplaceIPv6RPF accepts the empty set, which is what every config without an
// IPv6 rpf-check asks for, and refuses any check.
func (b *vppBackendImpl) ReplaceIPv6RPF(checks []iface.RPFCheck) error {
	if len(checks) == 0 {
		return nil
	}
	return errIPv6RPFUnsupported
}

// ReplaceIPv4RPF makes checks the complete set of IPv4 receive uRPF checks
// VPP enforces. VPP forwards outside the Linux kernel, so the rp_filter
// sysctl the iface component also writes judges only what VPP punts to the
// Linux Control Plane tap; this is the check on the traffic VPP forwards.
//
// VPP's semantics (src/plugins/urpf/urpf_dp.h): strict passes a packet whose
// source's FIB entry has the arrival interface in its uRPF list, loose one
// whose source's entry has any interface there, so a default route with a
// real path counts and VPP's own drop default does not. Both pass a packet
// to a multicast destination or to 255.255.255.255, so a DHCP DISCOVER from
// 0.0.0.0 passes.
//
// Ze owns every receive IPv4 uRPF setting in VPP: the set is read back from
// VPP (urpf_interface_dump) and each check it no longer names is turned off,
// so a check whose leaf or interface left the config while ze was down does
// not survive the next apply. Transmit and IPv6 settings are not touched.
func (b *vppBackendImpl) ReplaceIPv4RPF(checks []iface.RPFCheck) error {
	want, err := b.urpfWanted(checks)
	if err != nil {
		return err
	}
	if err := b.ensureChannel(); err != nil {
		return err
	}
	if err := b.ch.CheckCompatiblity(&urpf.UrpfInterfaceDump{}, &urpf.UrpfInterfaceDetails{},
		&urpf.UrpfUpdate{}, &urpf.UrpfUpdateReply{}); err != nil {
		// A VPP without the plugin holds no uRPF check, so an empty set is
		// already what it enforces. This is the answer for every config
		// without an IPv4 rpf-check on an external VPP that lacks the plugin.
		if len(want) == 0 {
			return nil
		}
		return fmt.Errorf("ifacevpp: ipv4 rpf-check needs VPP's %s, which this VPP does not load (%w); "+
			"load it in VPP's startup.conf with `plugins { plugin %s { enable } }`, "+
			"or remove `ipv4 rpf-check` from the units of backend vpp", urpfPlugin, err, urpfPlugin)
	}
	installed, err := b.urpfInstalledIPv4()
	if err != nil {
		return err
	}
	for _, row := range installed {
		if wanted(want, row.SwIfIndex) {
			continue
		}
		if err := b.urpfUpdate(row.SwIfIndex, urpf.URPF_API_MODE_OFF); err != nil {
			return err
		}
	}
	for _, w := range want {
		if current, held := installedMode(installed, w.idx); held && current == w.mode {
			continue
		}
		if err := b.urpfUpdate(w.idx, w.mode); err != nil {
			return fmt.Errorf("ifacevpp: ipv4 rpf-check on %s: %w", w.name, err)
		}
	}
	return nil
}

// urpfWant is one check resolved to VPP's index and mode; name is the device
// the operator configured, kept for the error that names it.
type urpfWant struct {
	name string
	idx  interface_types.InterfaceIndex
	mode urpf.UrpfMode
}

// wanted reports whether the set names idx.
func wanted(want []urpfWant, idx interface_types.InterfaceIndex) bool {
	for _, w := range want {
		if w.idx == idx {
			return true
		}
	}
	return false
}

// urpfWanted resolves each check to VPP's index and mode, refusing an
// interface VPP does not know and a check without a mode before any message
// reaches VPP.
func (b *vppBackendImpl) urpfWanted(checks []iface.RPFCheck) ([]urpfWant, error) {
	want := make([]urpfWant, 0, len(checks))
	for _, c := range checks {
		idx, err := b.resolveIndex(c.Interface)
		if err != nil {
			return nil, fmt.Errorf("ifacevpp: ipv4 rpf-check on %s: %w", c.Interface, err)
		}
		mode, err := urpfMode(c)
		if err != nil {
			return nil, err
		}
		want = append(want, urpfWant{name: c.Interface, idx: idx, mode: mode})
	}
	return want, nil
}

// urpfMode is VPP's mode for one check.
func urpfMode(c iface.RPFCheck) (urpf.UrpfMode, error) {
	switch c.Mode {
	case iface.RPFStrict:
		return urpf.URPF_API_MODE_STRICT, nil
	case iface.RPFLoose:
		return urpf.URPF_API_MODE_LOOSE, nil
	case iface.RPFUnspecified:
		// desiredIPv4RPF never emits Unspecified; a check without a mode is a
		// caller defect, refused rather than read as either mode.
		return 0, fmt.Errorf("ifacevpp: ipv4 rpf-check on %s has no mode", c.Interface)
	}
	return 0, fmt.Errorf("ifacevpp: ipv4 rpf-check on %s has unknown mode %d", c.Interface, c.Mode)
}

// urpfInstalledIPv4 reads back every receive IPv4 uRPF check VPP holds.
func (b *vppBackendImpl) urpfInstalledIPv4() ([]urpf.UrpfInterfaceDetails, error) {
	ctx := b.ch.SendMultiRequest(&urpf.UrpfInterfaceDump{SwIfIndex: ^interface_types.InterfaceIndex(0)})
	var rows []urpf.UrpfInterfaceDetails
	for {
		row := urpf.UrpfInterfaceDetails{}
		last, err := ctx.ReceiveReply(&row)
		if err != nil {
			return nil, fmt.Errorf("ifacevpp: urpf_interface_dump: %w", err)
		}
		if last {
			return rows, nil
		}
		if !row.IsInput {
			continue
		}
		if row.Af != ip_types.ADDRESS_IP4 {
			continue
		}
		if row.Mode == urpf.URPF_API_MODE_OFF {
			continue
		}
		rows = append(rows, row)
	}
}

// installedMode is the mode VPP holds for idx, if it holds one.
func installedMode(installed []urpf.UrpfInterfaceDetails, idx interface_types.InterfaceIndex) (urpf.UrpfMode, bool) {
	for _, row := range installed {
		if row.SwIfIndex == idx {
			return row.Mode, true
		}
	}
	return 0, false
}

// urpfUpdate sets one interface's receive IPv4 uRPF mode.
func (b *vppBackendImpl) urpfUpdate(idx interface_types.InterfaceIndex, mode urpf.UrpfMode) error {
	req := &urpf.UrpfUpdate{IsInput: true, Mode: mode, Af: ip_types.ADDRESS_IP4, SwIfIndex: idx}
	reply := &urpf.UrpfUpdateReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: urpf_update(index %d, %s): %w", idx, mode, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: urpf_update(index %d, %s): VPP answered retval %d", idx, mode, reply.Retval)
	}
	return nil
}
