// Design: docs/architecture/testing/interop.md -- reciprocal IGP loopback routes.
// Related: check_igp_loopback.go -- forwarding checks these scripted labs exercise.
package bgp

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestIGPLoopbackCheckersRequireForwarding rejects adjacency-only evidence,
// an omitted secondary prefix, a missing kernel route and either failed ping.
// MUTATION: omitting either route loop or either source-address ping admits
// the corresponding broken lab below.
func TestIGPLoopbackCheckersRequireForwarding(t *testing.T) {
	for _, scenario := range []struct {
		name            string
		protocol        string
		checker         interoplab.Checker
		zePrefixes      []string
		frrPrefix       string
		neighborCommand string
		neighbor        string
	}{
		{isisDefaultLevelScenario, "isis", checkISISDefaultLevelLoopbacks, []string{"10.80.0.2/32"}, "10.80.0.3/32", "show isis neighbor detail", "Interface: eth0, Level: 2, State: Up"},
		{ospfLoopbackScenario, "ospf", checkOSPFLoopbackPrefixes, []string{"10.81.0.2/32", "10.81.1.2/32"}, "10.81.0.3/32", frrShowOSPFNeighbor, "10.81.0.2 1 Full/- 10s 30s 172.30.44.2 eth0"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			for _, broken := range []string{"", "adjacency only", "last prefix absent", "Ze kernel route absent", "Ze ping fails", "FRR ping fails"} {
				t.Run(broken, func(t *testing.T) {
					synctest.Test(t, func(t *testing.T) {
						lab := &igpLoopbackLab{answers: map[string]string{}}
						addressReply := "inet 127.0.0.1/8 scope host lo\n"
						for _, prefix := range scenario.zePrefixes {
							addressReply += "inet " + prefix + " scope global lo\n"
							lab.answers[peerFRR+" vtysh -c show ip route "+prefix+" json"] = frrInstalledRouteAnswer(prefix, "172.30.44.2", scenario.protocol)
							address := netip.MustParsePrefix(prefix).Addr().String()
							frrAddress := netip.MustParsePrefix(scenario.frrPrefix).Addr().String()
							lab.expectedPings = append(lab.expectedPings,
								"ze ping -n -c 1 -W 2 -I "+address+" "+frrAddress,
								peerFRR+" ping -n -c 1 -W 2 -I "+frrAddress+" "+address)
						}
						lab.answers["ze ip -o -4 address show dev lo"] = addressReply
						lab.answers[peerFRR+" vtysh -c "+scenario.neighborCommand] = scenario.neighbor
						kernelQuery := "ze ip -N -4 route show exact " + scenario.frrPrefix + " proto 250"
						lab.answers[kernelQuery] = netip.MustParsePrefix(scenario.frrPrefix).Addr().String() + " via 172.30.44.3 dev eth0 metric 20 onlink"
						switch broken {
						case "":
						case "adjacency only":
							for _, prefix := range scenario.zePrefixes {
								lab.answers[peerFRR+" vtysh -c show ip route "+prefix+" json"] = "{}"
							}
						case "last prefix absent":
							prefix := scenario.zePrefixes[len(scenario.zePrefixes)-1]
							lab.answers[peerFRR+" vtysh -c show ip route "+prefix+" json"] = "{}"
						case "Ze kernel route absent":
							lab.answers[kernelQuery] = ""
						case "Ze ping fails":
							lab.failedPeer = "ze"
						case "FRR ping fails":
							lab.failedPeer = peerFRR
						default:
							t.Fatalf("unknown broken lab %q", broken)
						}
						ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
						defer cancel()
						err := scenario.checker(ctx, &interoplab.CheckContext{
							Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.44.0/24")}, Lab: lab,
						})
						if (err != nil) != (broken != "") {
							t.Fatalf("checker error=%v, broken lab=%q", err, broken)
						}
						if broken == "" && !slices.Equal(lab.pings, lab.expectedPings) {
							t.Fatalf("source-address pings=%v, want %v", lab.pings, lab.expectedPings)
						}
					})
				})
			}
		})
	}
}

type igpLoopbackLab struct {
	noEvidenceLab
	answers       map[string]string
	failedPeer    string
	pings         []string
	expectedPings []string
}

func (lab *igpLoopbackLab) Query(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	answer, ok := lab.answers[peer+" "+strings.Join(command, " ")]
	if !ok {
		return "", errors.New("unexpected query: " + peer + " " + strings.Join(command, " "))
	}
	return answer, nil
}

func (lab *igpLoopbackLab) Exec(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	query := peer + " " + strings.Join(command, " ")
	lab.pings = append(lab.pings, query)
	if !slices.Contains(lab.expectedPings, query) {
		return interoplab.CommandResult{}, errors.New("unexpected ping: " + query)
	}
	if peer == lab.failedPeer {
		return interoplab.CommandResult{}, errors.New("the source-address ping failed")
	}
	return interoplab.CommandResult{}, nil
}
