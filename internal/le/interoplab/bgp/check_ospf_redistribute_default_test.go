// Design: docs/architecture/testing/interop.md -- OSPF redistribution to eBGP.
// Related: check_ospf_redistribute_default.go -- live checker.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c --
// route_vty_out_detail produces the JSON shape exercised here.
package bgp

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const ospfRedistributeAcceptedAnswer = `{"prefix":"10.82.0.0/24","paths":[{"valid":true,"bestpath":{"overall":true},"peer":{"peerId":"172.30.44.2","type":"external"},"aspath":{"string":"65001"},"nexthops":[{"ip":"172.30.44.2"}]}]}`

// MUTATION: accepting a prefix alone, or finding attributes across unrelated
// paths, admits one of these replies without proving Ze's accepted announcement.
func TestRedistributedExternalPathRequiresAcceptedRoute(t *testing.T) {
	for _, test := range []struct {
		name      string
		answer    string
		wantError bool
	}{
		{"accepted", ospfRedistributeAcceptedAnswer, false},
		{"empty", "", true},
		{"malformed", "{", true},
		{"no process or prefix", "{}", true},
		{"wrong prefix", strings.Replace(ospfRedistributeAcceptedAnswer, "10.82.0.0/24", "10.82.1.0/24", 1), true},
		{"local host prefix", strings.Replace(ospfRedistributeAcceptedAnswer, "10.82.0.0/24", "10.82.0.3/32", 1), true},
		{"no path", `{"prefix":"10.82.0.0/24","paths":[]}`, true},
		{"multiple paths", strings.Replace(ospfRedistributeAcceptedAnswer, `"paths":[`, `"paths":[{},`, 1), true},
		{"invalid", strings.Replace(ospfRedistributeAcceptedAnswer, `"valid":true`, `"valid":false`, 1), true},
		{"missing validity", strings.Replace(ospfRedistributeAcceptedAnswer, `"valid":true,`, "", 1), true},
		{"not best", strings.Replace(ospfRedistributeAcceptedAnswer, `"overall":true`, `"overall":false`, 1), true},
		{"missing best", strings.Replace(ospfRedistributeAcceptedAnswer, `"bestpath":{"overall":true},`, "", 1), true},
		{"wrong peer", strings.Replace(ospfRedistributeAcceptedAnswer, `"peerId":"172.30.44.2"`, `"peerId":"172.30.44.9"`, 1), true},
		{"internal peer", strings.Replace(ospfRedistributeAcceptedAnswer, `"type":"external"`, `"type":"internal"`, 1), true},
		{"wrong path", strings.Replace(ospfRedistributeAcceptedAnswer, `"string":"65001"`, `"string":"65001 65002"`, 1), true},
		{"missing path", strings.Replace(ospfRedistributeAcceptedAnswer, `"aspath":{"string":"65001"},`, "", 1), true},
		{"old explicit gateway", strings.Replace(ospfRedistributeAcceptedAnswer, `"ip":"172.30.44.2"`, `"ip":"172.30.44.3"`, 1), true},
		{"missing hop", strings.Replace(ospfRedistributeAcceptedAnswer, `"nexthops":[{"ip":"172.30.44.2"}]`, `"nexthops":[]`, 1), true},
		{"multiple hops", strings.Replace(ospfRedistributeAcceptedAnswer, `"nexthops":[`, `"nexthops":[{"ip":"172.30.44.9"},`, 1), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireRedistributedExternalPath(test.answer, netip.MustParsePrefix(ospfRedistributeDefaultPrefix), netip.MustParseAddr("172.30.44.2"))
			if (err != nil) != test.wantError {
				t.Fatalf("path verdict = %v, want error %t", err, test.wantError)
			}
		})
	}
}

// MUTATION: bypassing a prerequisite or the final session check admits the
// corresponding broken lab. These scripted replies judge the checker only;
// the real daemon-to-FRR run remains the redistribution proof.
func TestOSPFRedistributeDefaultCheckerRequiresProducer(t *testing.T) {
	const sessionQuery = "frr vtysh -c show bgp neighbor 172.30.44.2"
	for _, missing := range []string{"", "producer address", "producer OSPF type", "adjacency", "IGP route", "BGP session", "accepted BGP path", "final BGP session"} {
		t.Run(missing, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				lab := &ospfRedistributeDefaultLab{answers: map[string]string{
					"frr ip -o -4 address show dev ze-redist0":            "inet 10.82.0.3/24 scope global ze-redist0",
					"frr vtysh -c show ip ospf interface ze-redist0":      "Internet Address 10.82.0.3/24, Area 0.0.0.0\nRouter ID 10.82.0.3, Network Type POINTOPOINT, Cost: 10",
					"frr vtysh -c " + frrShowOSPFNeighbor:                 "172.30.44.2 1 Full/- 10s 30s 172.30.44.2 eth0",
					"ze ip -N -4 route show exact 10.82.0.0/24 proto 250": "10.82.0.0/24 via 172.30.44.3 dev eth0 metric 20 onlink",
					sessionQuery: "BGP state = Established",
					"frr vtysh -c show bgp ipv4 unicast 10.82.0.0/24 json": ospfRedistributeAcceptedAnswer,
				}, failFinalSession: missing == "final BGP session"}
				switch missing {
				case "", "final BGP session":
				case "producer address":
					lab.answers["frr ip -o -4 address show dev ze-redist0"] = "no IPv4 address"
				case "producer OSPF type":
					lab.answers["frr vtysh -c show ip ospf interface ze-redist0"] = "Internet Address 10.82.0.3/24, Area 0.0.0.0\nRouter ID 10.82.0.3, Network Type LOOPBACK, Cost: 10"
				case "adjacency":
					lab.answers["frr vtysh -c "+frrShowOSPFNeighbor] = "No neighbors"
				case "IGP route":
					lab.answers["ze ip -N -4 route show exact 10.82.0.0/24 proto 250"] = ""
				case "BGP session":
					lab.answers[sessionQuery] = "BGP state = Active"
				case "accepted BGP path":
					lab.answers["frr vtysh -c show bgp ipv4 unicast 10.82.0.0/24 json"] = "{}"
				default:
					t.Fatalf("unknown missing evidence %q", missing)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				defer cancel()
				err := checkOSPFRedistributeDefault(ctx, &interoplab.CheckContext{
					Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.44.0/24")}, Lab: lab,
				})
				if (err != nil) != (missing != "") {
					t.Fatalf("checker error = %v, missing evidence %q", err, missing)
				}
				if missing == "" && lab.sessionQueries != 2 {
					t.Fatalf("session queries = %d, want 2", lab.sessionQueries)
				}
			})
		})
	}
}

type ospfRedistributeDefaultLab struct {
	noEvidenceLab
	answers          map[string]string
	sessionQueries   int
	failFinalSession bool
}

func (lab *ospfRedistributeDefaultLab) Query(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	query := peer + " " + strings.Join(command, " ")
	if query == "frr vtysh -c show bgp neighbor 172.30.44.2" {
		lab.sessionQueries++
		if lab.failFinalSession && lab.sessionQueries > 1 {
			return "BGP state = Active", nil
		}
	}
	answer, ok := lab.answers[query]
	if !ok {
		return "", errors.New("unexpected query: " + query)
	}
	return answer, nil
}
