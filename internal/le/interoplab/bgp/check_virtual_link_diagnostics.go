// Design: docs/architecture/testing/interop.md -- routed virtual-link failure evidence.
package bgp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

func virtualLinkFailureDiagnostics(ctx context.Context, lab interoplab.CheckerLab, v6 bool) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var output textbuf.Buffer
	command := zeCommand("show metrics values")
	answer, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	fmt.Fprintln(&output, "\n--- ze: OSPF NSM event counters ---")
	if err != nil {
		fmt.Fprintf(&output, "query error: %v\n", err)
	} else {
		var envelope struct {
			Metrics string `json:"metrics"`
		}
		if err := json.Unmarshal([]byte(answer), &envelope); err != nil {
			fmt.Fprintf(&output, "decode metrics error: %v\n", err)
		} else {
			for line := range strings.SplitSeq(envelope.Metrics, "\n") {
				if strings.HasPrefix(line, "ze_ospf_nsm_events_total{") {
					fmt.Fprintln(&output, line)
				}
			}
		}
	}
	query := func(peer string, command []string) {
		answer, err := lab.Query(ctx, peer, command, queryEnvironment(peer, command))
		fmt.Fprintf(&output, "\n--- %s: %s ---\n%s\nquery error: %v\n", peer, strings.Join(command, " "), answer, err)
	}
	family, ospf := "-4", "show ospf"
	if v6 {
		family, ospf = "-6", "show ospf ipv6"
		for _, command := range []string{"show route protocol vlink_ospf all", "show ospf neighbors vlink_ospf", "show protocols all vlink_ospf"} {
			query(peerBIRD, []string{cmdBirdc, command})
		}
	} else {
		for _, command := range []string{"show ip route json", "show ip ospf neighbor json", "show ip ospf database router json", "show ip ospf database summary json", "show ip ospf virtual-links"} {
			query(peerFRR, []string{cmdVtysh, "-c", command})
		}
	}
	for _, command := range []string{ospf + " interface detail", ospf + " neighbor detail", ospf + " database", "show ospf route", "show rib"} {
		query("ze", zeCommand(command))
	}
	query("ze", []string{"ip", "-j", family, "address", "show", "dev", "backbone0"})
	query("ze", []string{"ip", "-j", family, "route", "show", "table", "main"})
	return output.String()
}
