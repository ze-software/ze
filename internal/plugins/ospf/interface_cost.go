// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPF interface output cost.
// Related: interface_addr.go -- the other live OS properties the engine reads per interface.
// RFC: rfc/short/rfc2328.md (Appendix C.3 interface output cost)
//
// One rule, in one place, for the cost an interface advertises. An explicit `cost` leaf is
// the cost. An interface that configures none takes the auto-cost convention: the
// `ospf/reference-bandwidth` leaf divided by the speed of the link, so a fast link costs
// less than a slow one with no per-interface configuration.
//
// The speed comes from the kernel, which reports none for a loopback, a dummy, a bridge, a
// tunnel or a link with no carrier. Such an interface keeps costLinkSpeedUnknown, the cost
// Ze advertised for every interface before auto-cost existed. A veth is NOT in that set:
// the kernel prices one at 10 Gbit/s, so a container link is auto-costed like a real NIC.
//
// The derivation is re-read rather than cached, so a link that renegotiates changes cost on
// the next origination pass with nothing to invalidate. It costs two sysfs file reads for
// each interface, beside the five netlink address dumps lsdbTopology already performs there.

package ospf

import (
	ifcomp "github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// costLinkSpeedUnknown is the output cost of an interface with no `cost` leaf whose link
// speed the kernel does not report. It is the lowest cost RFC 2328 Appendix C.3 permits, so
// such a link is preferred over any link auto-cost has priced.
const costLinkSpeedUnknown uint16 = 1

// interfaceLinkSpeedMbps reports the speed of a link in Mbit/s, and 0 when the kernel
// reports none. It is a variable so a unit test can state a speed without a device: the
// interface names a unit test uses do not exist on the host, and the ones that do exist
// report whatever the kernel decides rather than what the case needs.
var interfaceLinkSpeedMbps = func(name string) uint64 {
	speedMbps, _ := ifcomp.LinkSpeedDuplex(name)
	if speedMbps <= 0 {
		return 0
	}
	return uint64(speedMbps)
}

// interfaceCost returns the OSPF output cost of an interface: the configured `cost` leaf, or
// the auto-cost derivation from referenceBandwidthMbps when the leaf is absent. An unknown
// link speed and a zero reference bandwidth are the two cases DefaultMetric cannot divide,
// and both take costLinkSpeedUnknown.
func interfaceCost(ic interfaceConfig, referenceBandwidthMbps uint32) uint16 {
	if ic.HasCost {
		return ic.Cost
	}
	return interfaceCostAtSpeed(referenceBandwidthMbps, interfaceLinkSpeedMbps(ic.Name))
}

// interfaceCostAtSpeed is the auto-cost quotient over a link speed the caller has already
// read. A caller that prices one link twice reads the speed once and calls this, so the two
// costs it compares describe one sample: two reads can straddle a renegotiation and report a
// cost change no config change produced.
func interfaceCostAtSpeed(referenceBandwidthMbps uint32, speedMbps uint64) uint16 {
	metric, err := types.DefaultMetric(uint64(referenceBandwidthMbps), speedMbps)
	if err != nil {
		return costLinkSpeedUnknown
	}
	return uint16(metric)
}
