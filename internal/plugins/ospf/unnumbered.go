// Design: docs/guide/ospf.md -- Unnumbered point-to-point interfaces
// Related: interface_addr.go -- interfaceUnnumbered, helloNetworkMask
// RFC: rfc/short/rfc2328.md (RFC2328-8.1-1)

package ospf

import (
	ifcomp "github.com/ze-software/ze/internal/component/iface"
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
