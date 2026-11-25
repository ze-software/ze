// Design: docs/guide/firewall.md -- nftables NAT placement at configuration verify

package firewall_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type nftNATPlacementCase struct {
	name string
	data string
	want []string
}

func TestVerifyConfigNftNATPlacement(t *testing.T) {
	for _, test := range nftNATPlacementCases() {
		t.Run(test.name, func(t *testing.T) {
			err := firewall.VerifyConfig([]sdk.ConfigSection{{Root: "firewall", Data: test.data}})
			if test.want == nil {
				require.NoError(t, err)
				return
			}
			var refusal *opererr.Error
			require.ErrorAs(t, err, &refusal, "VerifyConfig must refuse this nftables placement")
			require.Contains(t, refusal.Problem(), `table "placement"`)
			for _, fragment := range test.want {
				require.Contains(t, strings.ToLower(refusal.Error()), fragment)
			}
			require.NotEmpty(t, refusal.Why())
			require.NotEmpty(t, refusal.Fix())
		})
	}
}

// nftNATPlacementCases provides the original configured input for offline and
// guest tests. A nil want identifies the neighboring shapes Linux accepts.
func nftNATPlacementCases() []nftNATPlacementCase {
	const (
		snat4      = `"snat":{"to":"192.0.2.10"}`
		snat6      = `"snat":{"to":"2001:db8::10"}`
		dnat4      = `"dnat":{"to":"192.0.2.20"}`
		dnat6      = `"dnat":{"to":"2001:db8::20"}`
		masquerade = `"masquerade":""`
		redirect   = `"redirect":{"to":"8080"}`
	)
	direct := func(name, family, kind, hook, action string, want ...string) nftNATPlacementCase {
		return nftNATPlacementCase{name: name, data: natPlacementData(family,
			natPlacementBase("entry", kind, hook, "0", natPlacementTerm(action))), want: want}
	}
	chain := func(name, family, hook, priority string, want ...string) nftNATPlacementCase {
		return nftNATPlacementCase{name: name, data: natPlacementData(family,
			natPlacementBase("entry", "nat", hook, priority, "")), want: want}
	}
	reachable := func(name, otherType, otherHook string, want ...string) nftNATPlacementCase {
		return nftNATPlacementCase{name: name, data: natPlacementData("ip", strings.Join([]string{
			natPlacementBase("allowed", "nat", "prerouting", "0", natPlacementTerm(`"jump":"translate"`)),
			natPlacementBase("other", otherType, otherHook, "0", natPlacementTerm(`"goto":"middle"`)),
			natPlacementRegular("middle", natPlacementTerm(`"jump":"translate"`)),
			natPlacementRegular("translate", natPlacementTerm(dnat4)),
		}, ",")), want: want}
	}
	return []nftNATPlacementCase{
		direct("type/snat-filter", "ip", "filter", "postrouting", snat4, `chain "entry"`, `term "translate"`, "snat", "filter", "nat"),
		direct("type/dnat-route", "ip", "route", "output", dnat4, `chain "entry"`, `term "translate"`, "dnat", "route", "nat"),
		direct("type/masquerade-filter", "ip", "filter", "postrouting", masquerade, `chain "entry"`, `term "translate"`, "masquerade", "filter", "nat"),
		direct("type/redirect-filter", "ip", "filter", "prerouting", redirect, `chain "entry"`, `term "translate"`, "redirect", "filter", "nat"),

		direct("hook/snat-prerouting", "ip", "nat", "prerouting", snat4, `chain "entry"`, `term "translate"`, "snat", "prerouting", "postrouting", "input"),
		direct("hook/dnat-postrouting", "ip", "nat", "postrouting", dnat4, `chain "entry"`, `term "translate"`, "dnat", "postrouting", "prerouting", "output"),
		direct("hook/masquerade-input", "ip", "nat", "input", masquerade, `chain "entry"`, `term "translate"`, "masquerade", "input", "postrouting"),
		direct("hook/redirect-input", "ip", "nat", "input", redirect, `chain "entry"`, `term "translate"`, "redirect", "input", "prerouting", "output"),
		direct("hook/snat-postrouting", "ip", "nat", "postrouting", snat4),
		direct("hook/snat-input", "ip", "nat", "input", snat4),
		direct("hook/dnat-prerouting", "ip", "nat", "prerouting", dnat4),
		direct("hook/dnat-output", "ip", "nat", "output", dnat4),
		direct("hook/masquerade-postrouting", "ip", "nat", "postrouting", masquerade),
		direct("hook/redirect-prerouting", "ip", "nat", "prerouting", redirect),
		direct("hook/redirect-output", "ip", "nat", "output", redirect),

		reachable("regular/mixed-type", "filter", "output", `chain "translate"`, `term "translate"`, "dnat", "other", "filter"),
		reachable("regular/mixed-hook", "nat", "postrouting", `chain "translate"`, `term "translate"`, "dnat", "other", "postrouting"),
		reachable("regular/both-valid", "nat", "output"),
		{name: "regular/unreached", data: natPlacementData("ip",
			natPlacementBase("unrelated", "filter", "postrouting", "0", natPlacementTerm(`"accept":""`))+","+
				natPlacementRegular("translate", natPlacementTerm(dnat4)))},

		direct("target/ip-snat-ip6", "ip", "nat", "postrouting", snat6, `chain "entry"`, `term "translate"`, "snat", "2001:db8::10", "family"),
		direct("target/ip6-snat-ip", "ip6", "nat", "postrouting", snat4, `chain "entry"`, `term "translate"`, "snat", "192.0.2.10", "family"),
		direct("target/ip-dnat-ip6", "ip", "nat", "prerouting", dnat6, `chain "entry"`, `term "translate"`, "dnat", "2001:db8::20", "family"),
		direct("target/ip6-dnat-ip", "ip6", "nat", "prerouting", dnat4, `chain "entry"`, `term "translate"`, "dnat", "192.0.2.20", "family"),
		direct("target/ip6-snat-ip6", "ip6", "nat", "postrouting", snat6),
		direct("target/ip6-dnat-ip6", "ip6", "nat", "prerouting", dnat6),
		direct("target/inet-snat-ip", "inet", "nat", "postrouting", snat4),
		direct("target/inet-dnat-ip6", "inet", "nat", "prerouting", dnat6),
		direct("target/inet-masquerade", "inet", "nat", "postrouting", masquerade),
		direct("target/ip6-masquerade", "ip6", "nat", "postrouting", masquerade),
		direct("target/inet-redirect", "inet", "nat", "output", redirect),
		direct("target/ip6-redirect", "ip6", "nat", "output", redirect),

		{name: "family/arp-action", data: natPlacementData("arp", natPlacementRegular("translate", natPlacementTerm(snat4))), want: []string{`chain "translate"`, `term "translate"`, "snat", "arp", "family"}},
		{name: "family/bridge-action", data: natPlacementData("bridge", natPlacementRegular("translate", natPlacementTerm(dnat4))), want: []string{`chain "translate"`, `term "translate"`, "dnat", "bridge", "family"}},
		{name: "family/netdev-action", data: natPlacementData("netdev", natPlacementRegular("translate", natPlacementTerm(masquerade))), want: []string{`chain "translate"`, `term "translate"`, "masquerade", "netdev", "family"}},

		chain("chain/forward", "ip", "forward", "0", `chain "entry"`, "nat", "forward"),
		chain("chain/ingress", "inet", "ingress", "0", `chain "entry"`, "nat", "ingress"),
		chain("chain/egress", "inet", "egress", "0", `chain "entry"`, "nat", "egress"),
		chain("chain/priority-at-conntrack", "ip", "prerouting", "-200", `chain "entry"`, "nat", "priority", "-200"),
		chain("chain/priority-before-conntrack", "ip", "prerouting", "-201", `chain "entry"`, "nat", "priority", "-201"),
		chain("chain/priority-after-conntrack", "ip", "prerouting", "-199"),
		chain("chain/arp", "arp", "input", "0", `chain "entry"`, "nat", "arp", "family"),
		chain("chain/bridge", "bridge", "prerouting", "0", `chain "entry"`, "nat", "bridge", "family"),
		chain("chain/netdev", "netdev", "ingress", "0", `chain "entry"`, "nat", "netdev", "family"),
	}
}

func natPlacementData(family, chains string) string {
	return fmt.Sprintf(`{"firewall":{"backend":"kernel","table":{"placement":{"family":%q,"chain":{%s}}}}}`, family, chains)
}

func natPlacementBase(name, kind, hook, priority, terms string) string {
	return fmt.Sprintf(`%q:{"type":%q,"hook":%q,"priority":%q,"policy":"accept","term":{%s}}`, name, kind, hook, priority, terms)
}

func natPlacementRegular(name, terms string) string {
	return fmt.Sprintf(`%q:{"term":{%s}}`, name, terms)
}

func natPlacementTerm(action string) string {
	return `"translate":{"then":{` + action + `}}`
}
