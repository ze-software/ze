// Design: docs/contributing/terminal-demos.md -- runtime evidence must reject false greens
// Related: validate_topic_bfd.go, validate_topic_ospf.go, validate_topic_vrrp.go

package siteterminaldemo

import (
	"encoding/json"
	"strings"
	"testing"
)

// PREVENTS: a live reload recreating Ze's Peer and resetting its local counter
// to one can no longer pass the no-reconnect check. FRR's retained counter is
// independent of that object; missing fields are not evidence of continuity.
func TestNetworkBFDContinuityUsesIndependentFRRCounters(t *testing.T) {
	const baseline = `{"172.29.0.1":{"bgpState":"Established","connectionsEstablished":3,"connectionsDropped":2}}`
	before, err := parseNetworkFRRSessionCounters([]byte(baseline))
	if err != nil {
		t.Fatal(err)
	}
	zeBefore := networkBGPPeer{State: "established", ConnectionsEstablished: 1}
	for _, tc := range []struct {
		name          string
		after         string
		zeConnections uint64
		wantOK        bool
	}{
		{"unchanged", baseline, 1, true},
		{"Ze peer recreated", strings.Replace(baseline, `"connectionsEstablished":3`, `"connectionsEstablished":4`, 1), 1, false},
		{"Ze reconnect counted", baseline, 2, false},
		{"FRR drop advanced", strings.Replace(baseline, `"connectionsDropped":2`, `"connectionsDropped":3`, 1), 1, false},
		{"FRR counter reset", strings.Replace(baseline, `"connectionsEstablished":3`, `"connectionsEstablished":1`, 1), 1, false},
		{"missing established", strings.Replace(baseline, `"connectionsEstablished":3,`, "", 1), 1, false},
		{"missing dropped", strings.Replace(baseline, `,"connectionsDropped":2`, "", 1), 1, false},
		{"null established", strings.Replace(baseline, `"connectionsEstablished":3`, `"connectionsEstablished":null`, 1), 1, false},
		{"null dropped", strings.Replace(baseline, `"connectionsDropped":2`, `"connectionsDropped":null`, 1), 1, false},
		{"zero established", strings.Replace(baseline, `"connectionsEstablished":3`, `"connectionsEstablished":0`, 1), 1, false},
		{"not established", strings.Replace(baseline, `"Established"`, `"Active"`, 1), 1, false},
		{"wrong neighbor", strings.Replace(baseline, "172.29.0.1", "172.29.0.9", 1), 1, false},
		{"invalid JSON", "{", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			after, err := parseNetworkFRRSessionCounters([]byte(tc.after))
			if err == nil {
				zeAfter := networkBGPPeer{State: "established", ConnectionsEstablished: tc.zeConnections}
				err = requireNetworkBFDContinuity(zeBefore, zeAfter, before, after)
			}
			if (err == nil) != tc.wantOK {
				t.Fatalf("continuity error = %v, want accepted=%t", err, tc.wantOK)
			}
		})
	}
}

// PREVENTS: peer-detail's lowercase vocabulary being mistaken for the FSM
// history producer's uppercase states, or unrelated transitions proving loss.
func TestNetworkBGPDepartureUsesHistoryStates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer string
		wantOK bool
	}{
		{"departure", `{"transitions":[{"from":"ESTABLISHED","to":"IDLE"}]}`, true},
		{"departure followed by recovery", `{"transitions":[{"from":"ESTABLISHED","to":"IDLE"},{"from":"OPENCONFIRM","to":"ESTABLISHED"}]}`, true},
		{"no transitions", `{"transitions":[]}`, false},
		{"only establishment", `{"transitions":[{"from":"OPENCONFIRM","to":"ESTABLISHED"}]}`, false},
		{"missing destination", `{"transitions":[{"from":"ESTABLISHED"}]}`, false},
		{"no state change", `{"transitions":[{"from":"ESTABLISHED","to":"ESTABLISHED"}]}`, false},
		{"wrong vocabulary", `{"transitions":[{"from":"established","to":"idle"}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var history networkBGPHistory
			if err := json.Unmarshal([]byte(tc.answer), &history); err != nil {
				t.Fatal(err)
			}
			err := requireNetworkBGPDeparture(history)
			if (err == nil) != tc.wantOK {
				t.Fatalf("departure error = %v, want accepted=%t", err, tc.wantOK)
			}
		})
	}
}

// PREVENTS: rejecting the LSDB producer's four-octet area, or accepting a
// Router-LSA from another area/router as the requested FRR adjacency evidence.
func TestNetworkOSPFRouterLSAUsesArrayArea(t *testing.T) {
	const backbone = `[{"areas":[{"area":[0,0,0,0],"lsas":[{"type":"router","advertising-router":"172.29.0.2"}]}]}]`
	for _, tc := range []struct {
		name   string
		answer string
		wantOK bool
	}{
		{"backbone FRR Router-LSA", backbone, true},
		{"another area", strings.Replace(backbone, "[0,0,0,0]", "[0,0,0,1]", 1), false},
		{"another router", strings.Replace(backbone, "172.29.0.2", "192.0.2.1", 1), false},
		{"another LSA type", strings.Replace(backbone, `"type":"router"`, `"type":"network"`, 1), false},
		{"missing area", strings.Replace(backbone, `"area":[0,0,0,0],`, "", 1), false},
		{"null area", strings.Replace(backbone, "[0,0,0,0]", "null", 1), false},
		{"string instead of producer array", strings.Replace(backbone, "[0,0,0,0]", `"0.0.0.0"`, 1), false},
		{"empty database", `[]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var databases []networkOSPFDatabase
			err := json.Unmarshal([]byte(tc.answer), &databases)
			accepted := err == nil && networkHasFRRRouterLSA(databases)
			if accepted != tc.wantOK {
				t.Fatalf("accepted=%t, want %t; decode error=%v", accepted, tc.wantOK, err)
			}
		})
	}
}

// PREVENTS: a Backup's stray VIP on its parent or another root device being
// hidden by inspecting only the expected macvlan, and a Master using the wrong
// device/MAC or sharing ownership with another address/interface/namespace.
func TestNetworkVRRPWholeNamespaceOwnership(t *testing.T) {
	const owner = `[{"ifname":"vrrp.12","address":"00:00:5e:00:01:0c","addr_info":[{"local":"203.0.113.1"}]}]`
	const noVIP = `[{"ifname":"zeptrk0","address":"02:00:00:00:00:01","addr_info":[{"local":"203.0.113.251"}]}]`
	const strayParent = `[{"ifname":"zeptrk0","address":"02:00:00:00:00:01","addr_info":[{"local":"203.0.113.251"},{"local":"203.0.113.1"}]}]`
	for _, tc := range []struct {
		name   string
		ze     string
		peer   string
		master bool
		wantOK bool
	}{
		{"master exclusive", owner, noVIP, true, true},
		{"backup exclusive", noVIP, owner, false, true},
		{"backup stray parent VIP", strayParent, owner, false, false},
		{"backup stray other-device VIP", strings.Replace(owner, "vrrp.12", "dummy0", 1), owner, false, false},
		{"master wrong device", strings.Replace(owner, "vrrp.12", "zeptrk0", 1), noVIP, true, false},
		{"master wrong MAC", strings.Replace(owner, "00:00:5e:00:01:0c", "02:00:00:00:00:01", 1), noVIP, true, false},
		{"peer wrong MAC", noVIP, strings.Replace(owner, "00:00:5e:00:01:0c", "02:00:00:00:00:01", 1), false, false},
		{"both own VIP", owner, owner, true, false},
		{"neither owns VIP", noVIP, noVIP, false, false},
		{"master duplicate root VIP", strings.TrimSuffix(owner, "]") + "," + strings.TrimPrefix(strayParent, "["), noVIP, true, false},
		{"duplicate peer VIP", noVIP, strings.TrimSuffix(owner, "]") + "," + strings.TrimPrefix(strayParent, "["), false, false},
		{"invalid kernel JSON", "[", owner, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ze, err := parseNetworkVIPOwners([]byte(tc.ze))
			if err == nil {
				var peer []networkVIPOwner
				peer, err = parseNetworkVIPOwners([]byte(tc.peer))
				if err == nil {
					err = requireNetworkVIPOwnership(ze, peer, tc.master, "vrrp.12")
				}
			}
			if (err == nil) != tc.wantOK {
				t.Fatalf("ownership error = %v, want accepted=%t", err, tc.wantOK)
			}
		})
	}
}

// TestNetworkMetricDecodesProducerRows exercises the object rows returned by
// show metrics, including numeric failure rather than a fabricated zero.
func TestNetworkMetricDecodesProducerRows(t *testing.T) {
	for _, test := range []struct {
		name    string
		answer  string
		want    float64
		wantErr bool
	}{
		{"samples", `{"series":[{"line":"counter{port=\"8080\"} 1.25e2"},{"line":"counter{port=\"0\"} 3"}]}`, 128, false},
		{"empty selection", `{"series":[]}`, 0, false},
		{"invalid value", `{"series":[{"line":"counter not-a-number"}]}`, 0, true},
		{"missing line", `{"series":[{}]}`, 0, true},
		{"missing value", `{"series":[{"line":"counter"}]}`, 0, true},
		{"wrong row shape", `{"series":["counter 1"]}`, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var answer networkMetricAnswer
			err := json.Unmarshal([]byte(test.answer), &answer)
			var got float64
			if err == nil {
				got, err = sumNetworkMetric(answer)
			}
			if (err != nil) != test.wantErr {
				t.Fatalf("metric error = %v, want failure=%t", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("metric = %g, want %g", got, test.want)
			}
		})
	}
}
