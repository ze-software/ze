// Design: docs/architecture/testing/interop.md -- independent receiver predicates.
// Related: check_vpn_withdraw.go -- FRR table, log and session predicates.
package bgp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestVPNWithdrawTableIdentity requires exact RD/prefix and peer identity, then
// checks that disappearance cannot pass on a failed or structurally alien query.
func TestVPNWithdrawTableIdentity(t *testing.T) {
	const neighbor = "172.30.44.2"
	for _, family := range vpnWithdrawFamilies() {
		t.Run(family.prefix, func(t *testing.T) {
			initial := vpnWithdrawTableFixture(t, neighbor, family.prefix, "65004:10", "65004:20", "65004:30")
			control := vpnWithdrawTableFixture(t, neighbor, family.prefix, "65004:30")
			empty := vpnWithdrawTableFixture(t, neighbor, family.prefix)
			for _, test := range []struct {
				name   string
				output string
				phase  vpnWithdrawPhase
				want   bool
			}{
				{"all RDs before withdrawal", initial, vpnWithdrawAnnounced, true},
				{"only control RD after withdrawal", control, vpnWithdrawTargetsGone, true},
				{"explicit empty table after DOWN", empty, vpnWithdrawAllGone, true},
				{"FRR empty table form after DOWN", `{}`, vpnWithdrawAllGone, true},
				{"family-wrapped populated table", `{"ipv4Vpn":` + initial + `}`, vpnWithdrawAnnounced, false},
				{"family-wrapped empty table", `{"ipv6Vpn":{}}`, vpnWithdrawAllGone, false},
				{"empty before withdrawal", `{}`, vpnWithdrawAnnounced, false},
				{"empty while control must survive", `{}`, vpnWithdrawTargetsGone, false},
				{"one target survives", initial, vpnWithdrawTargetsGone, false},
				{"wrong RD survives", strings.ReplaceAll(control, "65004:30", "65004:10"), vpnWithdrawTargetsGone, false},
				{"RD is a substring", strings.ReplaceAll(control, "65004:30", "65004:300"), vpnWithdrawTargetsGone, false},
				{"wrong prefix", strings.ReplaceAll(control, family.prefix, "10.12.1.0/24"), vpnWithdrawTargetsGone, false},
				{"wrong peer", strings.ReplaceAll(control, neighbor, "172.30.44.22"), vpnWithdrawTargetsGone, false},
				{"default network peer", strings.ReplaceAll(control, neighbor, zeLabAddress), vpnWithdrawTargetsGone, false},
				{"wrong AS path", strings.ReplaceAll(control, "65001 65004", "65001 65005"), vpnWithdrawTargetsGone, false},
				{"control leaked after DOWN", control, vpnWithdrawAllGone, false},
				{"missing count", strings.ReplaceAll(empty, "totalRoutes", "foreignCount"), vpnWithdrawAllGone, false},
				{"missing RD schema", strings.ReplaceAll(empty, "routeDistinguishers", "foreignTables"), vpnWithdrawAllGone, false},
				{"malformed JSON", `{"routes":`, vpnWithdrawAllGone, false},
				{"null", `null`, vpnWithdrawAllGone, false},
				{"empty output", "", vpnWithdrawAllGone, false},
				{"warning", `{"warning":"No BGP process is configured"}`, vpnWithdrawAllGone, false},
				{"unspecified phase", initial, vpnWithdrawUnspecified, false},
				{"unknown phase", initial, vpnWithdrawPhase(99), false},
			} {
				t.Run(test.name, func(t *testing.T) {
					err := requireVPNWithdrawTable(test.output, neighbor, family.prefix, test.phase)
					if (err == nil) != test.want {
						t.Fatalf("table verdict = %v, want success %t", err, test.want)
					}
				})
			}
		})
	}
}

func vpnWithdrawTableFixture(t *testing.T, neighbor, prefix string, rds ...string) string {
	t.Helper()
	tables := make(map[string]any)
	for _, rd := range rds {
		tables[rd] = map[string]any{prefix: []any{map[string]any{
			"peerId": neighbor, "path": "65001 65004", "origin": "IGP",
		}}}
	}
	output, err := json.Marshal(map[string]any{
		"routerId": "172.30.44.3", "localAS": 65002,
		"totalRoutes": len(rds), "totalPaths": len(rds),
		"routes": map[string]any{"routeDistinguishers": tables},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

// TestFRRVPNDecodeIdentity makes same-prefix/different-RD and second-withdrawal
// mistakes observable, while rejecting send direction, other peers and log fragments.
func TestFRRVPNDecodeIdentity(t *testing.T) {
	const neighbor = "172.30.44.2"
	const rd = "65004:10"
	const prefix = "2001:db8:12::/48"
	const received = "BGP: [T1234-56789] 172.30.44.2(Unknown) rcvd "
	withdrawal := received + "UPDATE about RD 65004:10 2001:db8:12::/48 label 524288 IPv6 VPN -- withdrawn\n"
	announcement := received + "RD 65004:10 2001:db8:12::/48 label 300 IPv6 VPN\n"
	for _, test := range []struct {
		name string
		log  string
		want int
	}{
		{"exact", withdrawal, 1},
		{"duplicate target on DOWN", withdrawal + withdrawal, 2},
		{"equal-prefix other RD", strings.ReplaceAll(withdrawal, rd, "65004:30"), 0},
		{"RD substring", strings.ReplaceAll(withdrawal, rd, "65004:100"), 0},
		{"other prefix", strings.ReplaceAll(withdrawal, prefix, "2001:db8:120::/48"), 0},
		{"other peer", strings.ReplaceAll(withdrawal, neighbor, "172.30.44.22"), 0},
		{"old network", strings.ReplaceAll(withdrawal, neighbor, zeLabAddress), 0},
		{"send direction", strings.ReplaceAll(withdrawal, "rcvd", "send"), 0},
		{"announcement", announcement, 0},
		{"denied update", strings.ReplaceAll(withdrawal, "withdrawn", "DENIED"), 0},
		{"split identity", strings.ReplaceAll(withdrawal, "RD 65004:10", "RD\n65004:10"), 0},
		{"missing RD marker", strings.ReplaceAll(withdrawal, "RD ", ""), 0},
		{"unrelated RD mention", strings.ReplaceAll(withdrawal, "RD 65004:10", "RD 65004:30") + " RD 65004:10", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := frrVPNDecodes(test.log, neighbor, rd, prefix, true, 0); got != test.want {
				t.Fatalf("withdrawal count = %d, want %d", got, test.want)
			}
		})
	}
	if got := frrVPNDecodes(announcement+withdrawal, neighbor, rd, prefix, false, 300); got != 1 {
		t.Fatalf("announcement count = %d, want 1", got)
	}
	if got := frrVPNDecodes(announcement, neighbor, rd, prefix, false, 301); got != 0 {
		t.Fatalf("wrong label counted %d announcements", got)
	}
}

// TestVPNWithdrawKeepaliveFence rejects a reset masked by re-establishment and
// missing counters rather than treating zero-value JSON fields as wire progress.
func TestVPNWithdrawKeepaliveFence(t *testing.T) {
	const valid = `{"172.30.44.2":{"bgpState":"Established","connectionsEstablished":1,"messageStats":{"keepalivesRecv":9}}}`
	for _, test := range []struct {
		name   string
		output string
		want   bool
	}{
		{"valid", valid, true},
		{"reset", strings.ReplaceAll(valid, `"connectionsEstablished":1`, `"connectionsEstablished":2`), false},
		{"not established", strings.ReplaceAll(valid, "Established\"", "Idle\""), false},
		{"missing counter", strings.ReplaceAll(valid, "keepalivesRecv", "keepalivesSent"), false},
		{"wrong peer", strings.ReplaceAll(valid, "172.30.44.2", "172.30.44.22"), false},
		{"empty", `{}`, false},
		{"malformed", "not json", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			lab := &recordingLab{output: test.output}
			count, err := vpnWithdrawKeepalives(t.Context(), lab, "172.30.44.2", 1)
			if (err == nil) != test.want {
				t.Fatalf("KEEPALIVE verdict = %v, want success %t", err, test.want)
			}
			if test.want && count != 9 {
				t.Fatalf("KEEPALIVE count = %d, want 9", count)
			}
		})
	}
}
