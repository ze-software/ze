package bgp

import (
	"strings"
	"testing"
)

// The FRR answers below are composed from the fields FRR 10.7.1 writes for a
// neighbor (bgp_show_peer, bgp_show_peer_gr_extra_info and bgp_show_peer_afi
// in bgpd/bgp_vty.c) and for its routes (route_vty_out in bgpd/bgp_route.c,
// under show_ip_bgp_neighbor_routes). They are not captures of a lab run.
const rapidReconnectZe = "172.30.44.2"

// rapidReconnectNeighborJSON renders FRR's neighbor answer for ze with the
// given counters, End-of-RIB flags and accepted counts.
func rapidReconnectNeighborJSON(state, established, dropped, endOfRIB, ipv4Accepted, ipv6Accepted string) string {
	return `{"` + rapidReconnectZe + `":{"remoteAs":65001,"localAs":65002,"bgpState":"` + state + `",` +
		`"gracefulRestartInfo":{"endOfRibSend":{"ipv4Unicast":true,"ipv6Unicast":true},"endOfRibRecv":{` + endOfRIB + `},"localGrMode":"Helper*","remoteGrMode":"Restart"},` +
		`"addressFamilyInfo":{"ipv4Unicast":{"commAttriSentToNbr":"extendedAndStandard","acceptedPrefixCounter":` + ipv4Accepted + `,"sentPrefixCounter":0},` +
		`"ipv6Unicast":{"commAttriSentToNbr":"extendedAndStandard","acceptedPrefixCounter":` + ipv6Accepted + `,"sentPrefixCounter":0}},` +
		`"connectionsEstablished":` + established + `,"connectionsDropped":` + dropped + `}}`
}

const rapidReconnectBothEoR = `"ipv4Unicast":true,"ipv6Unicast":true`

// rapidReconnectRoutesJSON renders FRR's neighbor routes answer with the
// given routes object.
func rapidReconnectRoutesJSON(routes string) string {
	return `{"vrfId": 0,"vrfName": "default","tableVersion": 9,"routerId": "172.30.44.3","defaultLocPrf": 100,"localAS": 65002,"routes": {` + routes + `}}`
}

func rapidReconnectPath(peer string) string {
	return `{"valid":true,"bestpath":true,"pathFrom":"external","prefix":"10.83.1.0","prefixLen":24,"network":"10.83.1.0/24","weight":0,"peerId":"` + peer + `","path":"65001","origin":"IGP"}`
}

// rapidReconnectIPv4Routes holds the three IPv4 prefixes ze.conf originates,
// each with one path from ze.
var rapidReconnectIPv4Routes = `"10.83.1.0/24": [` + rapidReconnectPath(rapidReconnectZe) + `],` +
	`"10.83.2.0/24": [` + rapidReconnectPath(rapidReconnectZe) + `],` +
	`"10.83.3.0/24": [` + rapidReconnectPath(rapidReconnectZe) + `]`

// VALIDATES: the last session passes only Established on the fourth session,
// after exactly three drops, with End-of-RIB received in both families and
// each family's accepted count equal to the prefixes ze.conf originates.
// PREVENTS: a missing End-of-RIB, an End-of-RIB that arrived before its
// routes (a short accepted count), an unrequested session drop, and an
// answer with no Graceful Restart information passing as synced.
func TestRequireRapidReconnectNeighbor(t *testing.T) {
	pass := rapidReconnectNeighborJSON(stateEstablished, "4", "3", rapidReconnectBothEoR, "3", "2")
	if err := requireRapidReconnectNeighbor(pass, rapidReconnectZe, 4, rapidReconnectFamilies); err != nil {
		t.Fatalf("pass case refused: %v", err)
	}
	cases := []struct {
		name   string
		answer string
		want   string
	}{
		{"missing End-of-RIB for ipv6", rapidReconnectNeighborJSON(stateEstablished, "4", "3", `"ipv4Unicast":true`, "3", "2"), "no End-of-RIB received from ze for ipv6Unicast"},
		{"no End-of-RIB at all", rapidReconnectNeighborJSON(stateEstablished, "4", "3", ``, "3", "2"), "in no family"},
		{"End-of-RIB before a route", rapidReconnectNeighborJSON(stateEstablished, "4", "3", rapidReconnectBothEoR, "2", "2"), "accepted 2 ipv4Unicast prefixes"},
		{"an extra prefix counted", rapidReconnectNeighborJSON(stateEstablished, "4", "3", rapidReconnectBothEoR, "3", "3"), "accepted 3 ipv6Unicast prefixes"},
		{"an unrequested drop", rapidReconnectNeighborJSON(stateEstablished, "5", "4", rapidReconnectBothEoR, "3", "2"), "counts 5 established sessions"},
		{"dropped count off", rapidReconnectNeighborJSON(stateEstablished, "4", "4", rapidReconnectBothEoR, "3", "2"), "counts 4 dropped sessions"},
		{"not established", rapidReconnectNeighborJSON("Connect", "4", "3", ``, "3", "2"), `"Connect"`},
		{"an unlisted family", rapidReconnectNeighborJSON(stateEstablished, "4", "3", rapidReconnectBothEoR+`,"l2VpnEvpn":true`, "3", "2"), "l2VpnEvpn"},
		{"no counters", `{"` + rapidReconnectZe + `":{"bgpState":"Established","gracefulRestartInfo":{"endOfRibRecv":{` + rapidReconnectBothEoR + `}}}}`, "no connectionsEstablished"},
		{"no Graceful Restart information", `{"` + rapidReconnectZe + `":{"bgpState":"Established","connectionsEstablished":4,"connectionsDropped":3}}`, "no gracefulRestartInfo"},
		{"another neighbor", strings.Replace(pass, rapidReconnectZe, "172.30.44.9", 1), "holds no neighbor"},
		{"not json", `% Unknown command`, "decode"},
	}
	for _, c := range cases {
		err := requireRapidReconnectNeighbor(c.answer, rapidReconnectZe, 4, rapidReconnectFamilies)
		if err == nil {
			t.Errorf("%s: accepted %s", c.name, c.answer)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not name %q", c.name, err, c.want)
		}
	}
}

// VALIDATES: the wait hands over the answer only once the fourth session is
// Established with End-of-RIB in every family.
// PREVENTS: the judgement reading a session that has not finished its
// initial update, or the third session before the last clear took effect.
func TestRapidReconnectSyncPending(t *testing.T) {
	cases := []struct {
		name    string
		answer  string
		pending bool
	}{
		{"synced", rapidReconnectNeighborJSON(stateEstablished, "4", "3", rapidReconnectBothEoR, "3", "2"), false},
		{"ipv6 End-of-RIB pending", rapidReconnectNeighborJSON(stateEstablished, "4", "3", `"ipv4Unicast":true`, "3", "1"), true},
		{"the third session", rapidReconnectNeighborJSON(stateEstablished, "3", "2", rapidReconnectBothEoR, "3", "2"), true},
		{"reconnecting", rapidReconnectNeighborJSON("OpenConfirm", "3", "3", ``, "0", "0"), true},
	}
	for _, c := range cases {
		neighbor, err := decodeRapidReconnectNeighbor(c.answer, rapidReconnectZe)
		if err != nil {
			t.Fatalf("%s: decode: %v", c.name, err)
		}
		if got := neighbor.syncPending(4, rapidReconnectFamilies) != nil; got != c.pending {
			t.Errorf("%s: pending = %v, want %v", c.name, got, c.pending)
		}
	}
}

// VALIDATES: each clear waits for the session it is meant to clear, and a
// session count beyond it is refused.
// PREVENTS: a session that ended without the checker's clear passing as one
// of the three clears.
func TestRequireRapidReconnectGeneration(t *testing.T) {
	answer := rapidReconnectNeighborJSON(stateEstablished, "2", "1", rapidReconnectBothEoR, "3", "2")
	if err := requireRapidReconnectGeneration(answer, rapidReconnectZe, 2); err != nil {
		t.Fatalf("session 2 refused: %v", err)
	}
	if err := requireRapidReconnectGeneration(answer, rapidReconnectZe, 1); err == nil {
		t.Fatal("two established sessions accepted before clear 1")
	}
}

// VALIDATES: FRR's routes from ze pass only when every prefix ze.conf
// originates is held by exactly one path from ze, and nothing else is held.
// PREVENTS: a missing prefix, a duplicate path, a prefix ze.conf does not
// originate, another peer's path and an answer with no table passing.
func TestRequireRapidReconnectRoutes(t *testing.T) {
	ipv4 := rapidReconnectFamilies[0]
	if err := requireRapidReconnectRoutes(rapidReconnectRoutesJSON(rapidReconnectIPv4Routes), rapidReconnectZe, ipv4); err != nil {
		t.Fatalf("pass case refused: %v", err)
	}
	cases := []struct {
		name   string
		answer string
		want   string
	}{
		{"missing prefix", rapidReconnectRoutesJSON(`"10.83.1.0/24": [` + rapidReconnectPath(rapidReconnectZe) + `],"10.83.3.0/24": [` + rapidReconnectPath(rapidReconnectZe) + `]`), "no route for 10.83.2.0/24"},
		{"a prefix ze.conf does not originate", rapidReconnectRoutesJSON(rapidReconnectIPv4Routes + `,"10.83.4.0/24": [` + rapidReconnectPath(rapidReconnectZe) + `]`), "10.83.4.0/24"},
		{"two paths for one prefix", rapidReconnectRoutesJSON(strings.Replace(rapidReconnectIPv4Routes, `"10.83.2.0/24": [`, `"10.83.2.0/24": [`+rapidReconnectPath(rapidReconnectZe)+`,`, 1)), "2 paths for 10.83.2.0/24"},
		{"another peer's path", rapidReconnectRoutesJSON(strings.Replace(rapidReconnectIPv4Routes, `"10.83.3.0/24": [`+rapidReconnectPath(rapidReconnectZe), `"10.83.3.0/24": [`+rapidReconnectPath("172.30.44.9"), 1)), `"172.30.44.9"`},
		{"empty table", rapidReconnectRoutesJSON(``), "no route for 10.83.1.0/24"},
		{"no table", `{}`, "no ipv4Unicast route table"},
		{"not a prefix", rapidReconnectRoutesJSON(`"10.83.1.0": [` + rapidReconnectPath(rapidReconnectZe) + `]`), "not a prefix"},
	}
	for _, c := range cases {
		err := requireRapidReconnectRoutes(c.answer, rapidReconnectZe, ipv4)
		if err == nil {
			t.Errorf("%s: accepted %s", c.name, c.answer)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not name %q", c.name, err, c.want)
		}
	}
}
