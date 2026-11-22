// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/ospf.

package ospf

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeOSPFGracefulRestartNVS,
		Title:       "OSPF Graceful Restart NVS path unwritable",
		Description: "OSPF Graceful Restart is enabled (restarter support planned or planned-and-unplanned, RFC 3623 / RFC 5187), which requires a writable non-volatile store to persist the restart fact (RFC 3623 Section 2.1) across a planned restart, but the managed store directory cannot be resolved or written. Without it a planned restart cannot record its grace deadline, so the resumed engine boots normally instead of staying on the forwarding path, defeating non-stop forwarding. Ensure a persistent config directory is configured and writable.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-ospf-graceful-restart-nvs"},
	},
	{
		Code:        codeOSPFSegmentRoutingOverlap,
		Title:       "OSPF Segment Routing label range overlap or unsound",
		Description: "OSPF Segment Routing is enabled (RFC 8665 / RFC 8666) but the configured SRGB (global) and SRLB (local) label ranges overlap each other, overlap another range, sit outside the 20-bit MPLS label space, carry a zero Range Size, or a Prefix-SID index falls outside the total SRGB size. A double-claimed label maps two SIDs onto one forwarding entry and blackholes traffic. Make the SRGB and SRLB disjoint contiguous ranges within 16..1048575, each with Range Size greater than zero, not overlapping the LDP or RSVP-TE label space, and keep every node Prefix-SID index within the SRGB.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-ospf-segment-routing-overlap"},
	},
	{
		Code:        codeOSPFv3IPsec,
		Title:       "OSPFv3 IPsec (RFC 4552) kernel unavailable",
		Description: "An OSPFv3 (IPv6-family) interface configures RFC 4552 manual IPsec (AH/ESP), but the kernel XFRM dataplane cannot be reached to install the transport-mode Security Associations and policies. This needs CAP_NET_ADMIN and kernel IPsec support; without it the interface would form an UNPROTECTED adjacency, so Ze warns instead of silently claiming protection.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-ospfv3-ipsec"},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ospf: doctor code registration refused: " + err.Error())
		}
	}
}
