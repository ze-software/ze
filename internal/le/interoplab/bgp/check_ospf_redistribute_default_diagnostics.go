// Design: docs/architecture/testing/interop.md -- OSPF redistribution to eBGP.
// Related: check_ospf_redistribute_default.go -- preserves its failed assertion.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c --
// show_ip_bgp_neighbor_routes and show_ip_bgp_instance_neighbor_advertised_route
// expose accepted and retained received routes; received-routes may report that
// inbound soft reconfiguration is disabled, which remains diagnostic evidence.
package bgp

import (
	"context"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// ospfRedistributeDefaultDiagnostics reads each producer boundary after failure.
// All reads share one independent deadline; no query changes a route or session.
// Query records retain complete answers, while the error carries bounded excerpts.
func ospfRedistributeDefaultDiagnostics(ctx context.Context, lab interoplab.CheckerLab, zeHop, frrHop string) string {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	var output textbuf.Buffer
	query := func(peer string, command []string) {
		answer, err := lab.Query(ctx, peer, command, queryEnvironment(peer, command))
		output.Str("\n--- redistribution diagnostic: ").Str(peer).Byte(' ').Str(strings.Join(command, " ")).Str(" ---\n")
		output.Str(interoplab.OutputExcerpt(answer)).Byte('\n')
		if err != nil {
			output.Str("query error: ").Str(err.Error()).Byte('\n')
		}
	}
	for _, command := range []string{
		"show bgp ipv4 unicast json",
		"show bgp neighbor " + zeHop + " json",
		"show bgp ipv4 unicast neighbors " + zeHop + " routes json",
		"show bgp ipv4 unicast neighbors " + zeHop + " received-routes json",
	} {
		query(peerFRR, []string{cmdVtysh, "-c", command})
	}
	for _, command := range []string{
		"show log recent component bgp.redistribute count 100",
		"show log recent component redistribute.orchestrator count 100",
		"show bgp peer " + frrHop + " detail",
		"show bgp peer " + frrHop + " statistics",
		"show bgp peer " + frrHop + " rib advertised",
		"show bgp rib status",
		"show ospf route",
		"show rib candidates prefix " + ospfRedistributeDefaultPrefix,
		"show log levels",
		"show metrics values",
	} {
		query("ze", zeCommand(command))
	}
	logs, err := lab.Logs(ctx, "ze", 300)
	output.Str("\n--- redistribution diagnostic: ze startup log ---\n").Str(interoplab.OutputExcerpt(logs.Text)).Byte('\n')
	if err != nil {
		output.Str("logs error: ").Str(err.Error()).Byte('\n')
	} else if !logs.Available {
		output.Str("logs unavailable\n")
	}
	return output.String()
}
