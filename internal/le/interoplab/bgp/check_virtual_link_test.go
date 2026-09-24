package bgp

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"
)

func TestVirtualLinkMetricsRequireEndpointEvidence(t *testing.T) {
	t.Parallel()
	for _, v6 := range []bool{false, true} {
		t.Run(fmt.Sprint(v6), func(t *testing.T) {
			t.Parallel()
			prefix, label := "ze_ospf", "neighbor"
			changes := "ze_ospf_virtual_link_adjacency_changes_total"
			changesLabels := `transit_area="0.0.0.1",neighbor="172.30.0.3"`
			if v6 {
				prefix, label = "ze_ospfv3", "remote_router_id"
				changes = "ze_ospfv3_virtual_link_reresolves_total"
				changesLabels = `transit_area="0.0.0.1"`
			}
			answer := fmt.Sprintf(`# TYPE %[1]s_virtual_link_cost gauge
%[1]s_virtual_link_cost{transit_area="0.0.0.1",%[2]s="172.30.0.3"} 20
# TYPE %[1]s_virtual_links gauge
%[1]s_virtual_links{transit_area="0.0.0.1",state="up"} 1
%[1]s_virtual_links{transit_area="0.0.0.1",state="down"} 0
# TYPE %[3]s counter
%[3]s{%[4]s} 1
# TYPE ze_ospf_nsm_events_total counter
ze_ospf_nsm_events_total{event="exchange-done"} 2
`, prefix, label, changes, changesLabels)
			observed, err := decodeVirtualLinkMetrics(answer, "172.30.0.3", v6)
			if err != nil {
				t.Fatal(err)
			}
			if observed.Cost != 20 || observed.Up != 1 || observed.Down != 0 || observed.Changes != 1 || observed.Events[0] != 2 {
				t.Fatalf("wrong measured virtual-link state: %+v", observed)
			}
			if _, err := decodeVirtualLinkMetrics(answer, "172.30.0.99", v6); err == nil {
				t.Fatal("accepted another endpoint's cost as virtual-link evidence")
			}
			missingDown := strings.ReplaceAll(answer, prefix+`_virtual_links{transit_area="0.0.0.1",state="down"} 0`+"\n", "")
			if _, err := decodeVirtualLinkMetrics(missingDown, "172.30.0.3", v6); err == nil {
				t.Fatal("an unmeasured down gauge silently became zero")
			}
		})
	}
}

func TestVirtualLinkContinuityRejectsRecoveredFlap(t *testing.T) {
	t.Parallel()
	before := virtualLinkObservation{Cost: 20, Up: 1, Changes: 1, Events: [4]float64{2}, Neighbor: virtualLinkNeighbor{Interface: "virtual", DDSequence: 42}}
	after := before
	after.Cost = 40
	if err := requireVirtualLinkContinuity(before, after); err != nil {
		t.Fatalf("cost-only change: %v", err)
	}
	for _, test := range []struct {
		name   string
		change func(*virtualLinkObservation)
	}{
		{"reachability", func(o *virtualLinkObservation) { o.Changes++ }},
		{"recovered exchange", func(o *virtualLinkObservation) { o.Events[0]++ }},
		{"interface down", func(o *virtualLinkObservation) { o.Events[1]++ }},
		{"new exchange identity", func(o *virtualLinkObservation) { o.Neighbor.DDSequence++ }},
		{"new interface", func(o *virtualLinkObservation) { o.Neighbor.Interface = "replacement" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			changed := after
			test.change(&changed)
			if err := requireVirtualLinkContinuity(before, changed); err == nil {
				t.Fatal("accepted a reset virtual adjacency as a cost-only update")
			}
		})
	}
}

func TestVirtualLinkKernelRouteRequiresIntermediateGateway(t *testing.T) {
	t.Parallel()
	gateway := netip.MustParseAddr("10.200.0.2")
	prefix := "10.200.0.4/30"
	for _, test := range []struct {
		name    string
		answer  string
		present bool
		valid   bool
	}{
		{"installed", `[{"dst":"10.200.0.4/30","gateway":"10.200.0.2","dev":"eth1"}]`, true, true},
		{"withdrawn", `[]`, false, true},
		{"unmeasured", `null`, false, false},
		{"far endpoint as gateway", `[{"dst":"10.200.0.4/30","gateway":"10.200.0.6","dev":"eth1"}]`, false, false},
		{"management egress", `[{"dst":"10.200.0.4/30","gateway":"10.200.0.2","dev":"eth0"}]`, false, false},
		{"covering route", `[{"dst":"10.200.0.0/16","gateway":"10.200.0.2","dev":"eth1"}]`, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			present, err := virtualLinkKernelRoutePresent(test.answer, prefix, gateway)
			if (err == nil) != test.valid {
				t.Fatalf("route evidence error = %v, valid = %v", err, test.valid)
			}
			if present != test.present {
				t.Fatalf("route present = %v, want %v", present, test.present)
			}
		})
	}
}

func TestVirtualLinkBIRDRouteRequiresExactOSPFRoute(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		answer  string
		present bool
	}{
		{"backbone route", "2001:db8:10::1/128 unicast [vlink_ospf 12:00:00] * I (150/20)", true},
		{"withdrawn", "BIRD 2.15.1 ready.\nTable master6:\n", false},
		{"covering route", "2001:db8:10::/64 unicast [vlink_ospf 12:00:00] * I (150/20)", false},
		{"another producer", "2001:db8:10::1/128 unicast [connected 12:00:00] * (240)", false},
		{"prefix only in attributes", "2001:db8:20::1/128 unicast [vlink_ospf 12:00:00] * I (150/20)\n\tvia 2001:db8:10::1/128", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if present := virtualLinkBIRDRoutePresent(test.answer); present != test.present {
				t.Fatalf("backbone route present = %v, want %v", present, test.present)
			}
		})
	}
}
