// Design: docs/guide/ospf.md -- Unnumbered point-to-point interfaces
// Related: interface_addr.go -- interfaceUnnumbered, helloNetworkMask
// RFC: rfc/short/rfc2328.md (RFC2328-8.1-1)

package ospf

import (
	ifcomp "github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// checkUnnumberedNetworkTypes refuses an OSPFv2 interface on an unnumbered
// iface unit unless it runs as point-to-point. The unit holds only a borrowed
// /32, so a broadcast or NBMA election would find no shared subnet, and RFC
// 2328 Section 8.1 defines the borrowed source only for point-to-point:
// "Interfaces to unnumbered point-to-point networks have no associated IP
// address. On these interfaces, the IP source should be set to any of the
// other IP addresses belonging to the router." It reads the candidate
// interface section, so the refusal judges the commit and not the kernel.
func checkUnnumberedNetworkTypes(cfg ospfConfig, sections []configSection) error {
	ifaceSections := make([]sdk.ConfigSection, 0, 1)
	for _, s := range sections {
		if s.Root == ifcomp.ConfigRoot {
			ifaceSections = append(ifaceSections, sdk.ConfigSection{Root: s.Root, Data: s.Data})
		}
	}
	if len(ifaceSections) == 0 {
		return nil
	}
	devices, err := ifcomp.UnnumberedDevices(ifaceSections)
	if err != nil {
		return err
	}
	for _, ic := range cfg.Interfaces {
		if _, unnumbered := devices[ic.Name]; !unnumbered {
			continue
		}
		if ic.NetworkType == types.NetworkPointToPoint {
			continue
		}
		return &unnumberedNetworkTypeError{name: ic.Name, network: string(ic.NetworkType)}
	}
	return nil
}

// unnumberedNetworkTypeError names the interface, why it is refused, and the fix.
type unnumberedNetworkTypeError struct {
	name    string
	network string
}

func (e *unnumberedNetworkTypeError) Error() string {
	network := e.network
	if network == "" {
		network = "broadcast (the default)"
	}
	return "ospf interface " + e.name + ": network type " + network +
		" on an unnumbered interface; an unnumbered link borrows a /32 and has no subnet to share, " +
		"so OSPF runs on it only as point-to-point; set `ospf ... interface " + e.name +
		" network-type point-to-point` or give the unit its own ipv4 address"
}

// unnumberedDonorDiagnostics warns, once per unnumbered OSPFv2 interface,
// when the interface it borrows its address from is not an OSPF interface.
// The unnumbered link advertises no stub for the borrowed address (RFC 2328
// Section 12.4.1.1 describes it by ifIndex), so only the donor's own OSPF
// interface makes that address reachable; without it the link forms an
// adjacency but no other router learns a route to this router's address. The
// donor counts as present under its config name or its OS device name.
func unnumberedDonorDiagnostics(cfg ospfConfig, borrowers map[string]ifcomp.UnnumberedDonor) []diagnostic.Diagnostic {
	if len(borrowers) == 0 {
		return nil
	}
	inOSPF := make(map[string]struct{}, len(cfg.Interfaces))
	for _, ic := range cfg.Interfaces {
		inOSPF[ic.Name] = struct{}{}
	}
	var diags []diagnostic.Diagnostic
	for _, ic := range cfg.Interfaces {
		donor, unnumbered := borrowers[ic.Name]
		if !unnumbered {
			continue
		}
		if _, ok := inOSPF[donor.Name]; ok {
			continue
		}
		if _, ok := inOSPF[donor.Device]; ok {
			continue
		}
		diags = append(diags, unnumberedDonorAbsent(ic.Name, donor, ic.AreaID))
	}
	return diags
}

// unnumberedDonorAbsent builds the warning: the problem, why it matters, and
// the two editor commands that add the donor as a passive OSPF interface.
func unnumberedDonorAbsent(name string, donor ifcomp.UnnumberedDonor, area types.AreaID) diagnostic.Diagnostic {
	var message textbuf.Buffer
	message.Str("ospf interface ").Quoted(name).Str(" borrows its address from ").Quoted(donor.Name).
		Str(" (ipv4 unnumbered), but ").Quoted(donor.Device).Str(" is not an ospf interface. ").
		Str("An unnumbered link advertises no route to the borrowed address, so no other router ").
		Str("learns how to reach this router at that address.")
	var help textbuf.Buffer
	help.Str("Add the donor to ospf as a passive interface:\n  set ospf interfaces interface ").Str(donor.Device).
		Str(" area ").Str(area.String()).Str("\n  set ospf interfaces interface ").Str(donor.Device).Str(" passive true")
	return diagnostic.Diagnostic{
		Code:     codeOSPFUnnumberedDonorAbsent,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Help:     help.String(),
	}
}
