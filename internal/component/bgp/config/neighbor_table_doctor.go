// Design: docs/architecture/update-building.md -- the third-party Link-Local BGP takes from the neighbor table
// RFC: rfc/short/rfc2545.md -- Section 3
// Related: register.go -- the doctor-check registration from init()
//
// RFC 2545 Section 3 names the Link-Local of another router on the shared
// subnet as the second address of a next hop that is that router's global, and
// only the kernel neighbor table names it. A daemon that cannot read the table
// sends every such next hop as the global alone and says so once per peer at
// run time (reactor.warnNeighborTableUnread); this is the half that answers
// before the daemon starts.
package bgpconfig

import (
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeBGPNeighborTable is raised when BGP is configured and the kernel
// neighbor table cannot be read.
const codeBGPNeighborTable = "doctor-bgp-neighbor-table"

var neighborTableDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeBGPNeighborTable,
		Title: "BGP cannot read the kernel neighbor table",
		Description: "BGP is configured, and the kernel IPv6 neighbor table cannot be read: no interface backend " +
			"can be loaded on this platform, or this build carries no interface component. RFC 2545 Section 3 " +
			"names the link-local address of another router on the subnet shared with a peer as the second " +
			"address of a next hop that is that router's global address, and only the neighbor table names it. " +
			"Without the table every such next hop is sent as the global address alone, 16 octets, and the " +
			"daemon warns once per peer. The message names the fix for the cause it found.",
		Examples: []string{"ze doctor --json", "ze show diagnostic doctor-bgp-neighbor-table"},
	},
}

// neighborTableDoctorCheck describes the check. register.go registers it.
var neighborTableDoctorCheck = diagnostic.DoctorCheck{
	Name:         "bgp-neighbor-table",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        747,
	Component:    doctorComponentBGP,
	Dependencies: []string{"kernel-neighbor-table"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeBGPNeighborTable},
	Check:        checkBGPNeighborTable,
}

// readNeighborTable is the read the check makes. It is network.ReadNeighbors,
// the read the reactor makes; a test replaces it and restores it in
// t.Cleanup, because the table registry is a one-shot slot filled by the
// interface component's init().
var readNeighborTable = network.ReadNeighbors

// checkBGPNeighborTable reads the neighbor table the way the daemon does when
// the configuration carries a bgp block, and reports a read that fails with
// the fix the daemon's own warning names (reactor.NeighborTableFix).
func checkBGPNeighborTable(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree := doctorTree(ctx)
	if tree == nil || tree.GetContainer("bgp") == nil {
		return nil
	}
	_, err := readNeighborTable()
	if err == nil {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     codeBGPNeighborTable,
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("the kernel neighbor table cannot be read, so an RFC 2545 next hop naming another router goes out without that router's link-local: ").
			Err(err).Str("; fix: ").Str(reactor.NeighborTableFix(err)).String(),
	}}
}
