package bgp

import (
	"fmt"
	"strings"
	"testing"
)

// FRR 10.7.1 zebra_vty.c:vty_show_ip_route emits each protocol, selection and
// installation state in one prefix-keyed route entry. lib/nexthop.c emits
// active/fib on the individual nexthop; neither may be borrowed from a sibling.
func TestInstalledRouteFieldsBelongToOneObject(t *testing.T) {
	const prefix = "10.10.0.0/24"
	const hop = "172.30.0.2"
	const entry = `{"prefix":"10.10.0.0/24","protocol":"bgp","selected":true,"destSelected":true,"installed":true,"nexthops":[{"ip":"172.30.0.2","active":true,"fib":true}]}`
	wrap := func(route string) string { return fmt.Sprintf(`{%q:[%s]}`, prefix, route) }
	for _, test := range []struct {
		name, output string
		accepted     bool
	}{
		{"native installed BGP", wrap(entry), true},
		{"wrong prefix key", fmt.Sprintf(`{"110.10.0.0/24":[%s]}`, entry), false},
		{"wrong entry prefix", wrap(strings.Replace(entry, prefix, "110.10.0.0/24", 1)), false},
		{"wrong protocol", wrap(strings.Replace(entry, `"bgp"`, `"static"`, 1)), false},
		{"unselected", wrap(strings.Replace(entry, `"selected":true`, `"selected":false`, 1)), false},
		{"not forwarding selection", wrap(strings.Replace(entry, `"destSelected":true`, `"destSelected":false`, 1)), false},
		{"not installed", wrap(strings.Replace(entry, `"installed":true`, `"installed":false`, 1)), false},
		{"inactive hop", wrap(strings.Replace(entry, `"active":true`, `"active":false`, 1)), false},
		{"hop not in FIB", wrap(strings.Replace(entry, `"fib":true`, `"fib":false`, 1)), false},
		{"hop substring", wrap(strings.Replace(entry, hop, "172.30.0.20", 1)), false},
		{"state from another entry", wrap(strings.Replace(entry, `"installed":true`, `"installed":false`, 1) + "," + strings.Replace(entry, hop, "172.30.0.9", 1)), false},
		{"state from another hop", wrap(strings.Replace(entry, `{"ip":"172.30.0.2","active":true,"fib":true}`, `{"ip":"172.30.0.2","active":true},{"ip":"172.30.0.9","fib":true,"active":true}`, 1)), false},
		{"native installed control", wrap(entry), true},
		{"native wrong protocol", wrap(strings.Replace(entry, `"bgp"`, `"static"`, 1)), false},
		{"native unselected", wrap(strings.Replace(entry, `"selected":true`, `"selected":false`, 1)), false},
		{"native not installed", wrap(strings.Replace(entry, `"installed":true`, `"installed":false`, 1)), false},
		{"native prefix substring", strings.ReplaceAll(wrap(entry), prefix, "110.10.0.0/24"), false},
		{"native hop substring", wrap(strings.Replace(entry, hop, "172.30.0.20", 1)), false},
		{"native state from another entry", wrap(strings.Replace(entry, `"selected":true`, `"selected":false`, 1) + "," + strings.Replace(entry, hop, "172.30.0.9", 1)), false},
		{"native hop from another prefix", strings.TrimSuffix(wrap(strings.Replace(entry, hop, "172.30.0.9", 1)), "}") + "," + strings.TrimPrefix(frrInstalledRouteAnswer("10.20.0.0/24", hop, "bgp"), "{"), false},
		{"empty", "", false},
		{"unrelated JSON", `{"error":"10.10.0.0/24 via 172.30.0.2"}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireRouteInstalledVia(test.output, prefix, hop, "bgp")
			if (err == nil) != test.accepted {
				t.Fatalf("installed route accepted=%t, want %t: %v; answer %s", err == nil, test.accepted, err, test.output)
			}
		})
	}
	t.Run("OSPF protocol identity", func(t *testing.T) {
		ospf := frrInstalledRouteAnswer(prefix, hop, "ospf")
		if err := requireRouteInstalledVia(ospf, prefix, hop, "ospf"); err != nil {
			t.Fatal(err)
		}
		if requireRouteInstalledVia(ospf, prefix, hop, "bgp") == nil {
			t.Fatal("OSPF route proved BGP installation")
		}
		if requireRouteInstalledVia(wrap(entry), prefix, hop, "ospf") == nil {
			t.Fatal("BGP route proved OSPF installation")
		}
	})
	t.Run("IPv6 link-local", func(t *testing.T) {
		const network = "2001:db8:5601::/48"
		const linkLocal = "fe80::be:ef:2"
		if err := requireRouteInstalledVia(frrInstalledRouteAnswer(network, linkLocal, "bgp"), network, linkLocal, "bgp"); err != nil {
			t.Fatal(err)
		}
		if requireRouteInstalledVia(frrInstalledRouteAnswer(network, "fe80::be:ef:20", "bgp"), network, linkLocal, "bgp") == nil {
			t.Fatal("IPv6 next-hop substring proved link-local installation")
		}
	})
}

func frrInstalledRouteAnswer(prefix, nextHop, protocol string) string {
	return fmt.Sprintf(`{%q:[{"prefix":%q,"protocol":%q,"selected":true,"destSelected":true,"installed":true,"nexthops":[{"ip":%q,"active":true,"fib":true}]}]}`, prefix, prefix, protocol, nextHop)
}
