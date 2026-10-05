package bgp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures retain pmacct's captured Loc-RIB Peer Up/Route Monitoring shape:
// the local instance peer is 0.0.0.0/AS65001, not the selected route's neighbor.
const (
	medWholeSetLocUpFixture    = `{"event_type": "log", "seq": 2, "bmp_router": "172.30.0.2", "bmp_router_port": 48476, "bmp_msg_type": "peer_up", "peer_ip": "0.0.0.0", "peer_asn": 65001, "peer_type": 3, "bmp_rib_type": "Loc-Rib", "bgp_id": "172.30.0.2"}`
	medWholeSetLocRouteFixture = `{"seq": 9, "log_type": "update", "bmp_rib_type": "Loc-Rib", "event_type": "log", "afi": 1, "safi": 1, "ip_prefix": "10.99.77.0/24", "bgp_nexthop": "172.30.0.9", "as_path": "65004", "origin": 0, "bmp_router": "172.30.0.2", "bmp_router_port": 48476, "peer_ip": "0.0.0.0", "peer_asn": 65001, "bmp_msg_type": "route_monitor"}`
)

// TestMEDWholeSetCollectorSelectedEpoch drives the actual selected-winner query.
// A fresh announcement is not current evidence after its Loc-RIB epoch ends.
func TestMEDWholeSetCollectorSelectedEpoch(t *testing.T) {
	t.Parallel()
	medWholeSetSelectedEpochBranches(t)
}

func medWholeSetSelectedEpochBranches(t *testing.T) {
	t.Helper()
	fresh := strings.Replace(medWholeSetLocRouteFixture, `"seq": 9`, `"seq": 25`, 1)
	newUp := strings.Replace(medWholeSetLocUpFixture, `"seq": 2`, `"seq": 26`, 1)
	newRoute := strings.Replace(medWholeSetLocRouteFixture, `"seq": 9`, `"seq": 27`, 1)
	const down = `{"event_type": "log", "seq": 26, "bmp_router": "172.30.0.2", "bmp_router_port": 48476, "bmp_msg_type": "peer_down", "peer_ip": "0.0.0.0", "peer_asn": 65001, "bmp_rib_type": "Loc-Rib"}`
	const closeEvent = `{"event_type": "log_close", "seq": 26, "bmp_router": "172.30.0.2", "bmp_router_port": 48476}`
	const initEvent = `{"event_type": "log", "seq": 26, "bmp_router": "172.30.0.2", "bmp_router_port": 48476, "bmp_msg_type": "init"}`
	const restart = `{"seq": 26, "event_type": "log_init", "bmp_router": "172.30.0.2", "bmp_router_port": 55555}`
	want := newMEDWholeSetCandidate([4]byte{172, 30, 0, 0}, 9, 65004, 1, 100)
	for _, test := range []struct {
		name   string
		rows   []string
		accept bool
	}{
		{name: "active epoch", rows: []string{fresh}, accept: true},
		{name: "Loc-RIB Peer Down after winner", rows: []string{fresh, down}},
		{name: "BMP close after winner", rows: []string{fresh, closeEvent}},
		{name: "BMP init after winner", rows: []string{fresh, initEvent}},
		{name: "BMP stream reset after winner", rows: []string{fresh, restart}},
		{name: "replacement epoch without fresh winner", rows: []string{fresh, newUp}},
		{name: "replacement epoch with fresh winner", rows: []string{fresh, newUp, newRoute}, accept: true},
		{name: "new connection with fresh epoch and winner", rows: []string{fresh, restart, strings.Replace(newUp, "48476", "55555", 1), strings.Replace(newRoute, "48476", "55555", 1)}, accept: true},
		{name: "unrelated Adj-RIB-In Peer Up", rows: []string{fresh, medWholeSetPeerUpFixture}, accept: true},
		{name: "unrelated Adj-RIB-In Peer Down", rows: []string{fresh, strings.NewReplacer("0.0.0.0", "172.30.0.9", "Loc-Rib", "Adj-Rib-In Pre-Policy", "65001", "65004").Replace(down)}, accept: true},
		{name: "other connection Peer Down", rows: []string{fresh, strings.Replace(down, "48476", "55555", 1)}, accept: true},
		{name: "other connection closes", rows: []string{fresh, strings.Replace(closeEvent, "48476", "55555", 1)}, accept: true},
		{name: "different connection without Peer Up", rows: []string{strings.Replace(fresh, "48476", "55555", 1)}},
		{name: "foreign router winner", rows: []string{strings.Replace(fresh, "172.30.0.2", "172.30.0.254", 1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			logPath := filepath.Join(directory, "bmp.log")
			checkpoint := filepath.Join(directory, "checkpoint")
			before := medWholeSetLocUpFixture + "\n" + medWholeSetLocRouteFixture + "\n"
			if err := os.WriteFile(logPath, []byte(before), 0o600); err != nil {
				t.Fatal(err)
			}
			medWholeSetRunQuery(t, medWholeSetSaveCheckpoint().command, logPath, checkpoint)
			content := before + strings.Join(test.rows, "\n") + "\n"
			if err := os.WriteFile(logPath, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			output := medWholeSetRunQuery(t, []string{"sh", "-c", medWholeSetSelectionRows}, logPath, checkpoint)
			err := requireMEDWholeSetSelected(output, &want)
			if (err == nil) != test.accept {
				t.Fatalf("selected epoch acceptance=%v, want %v: %v; rows=%s", err == nil, test.accept, err, output)
			}
		})
	}
}

func TestMEDWholeSetCollectorSelectedCriteria(t *testing.T) {
	t.Parallel()
	want := newMEDWholeSetCandidate([4]byte{172, 30, 0, 0}, 9, 65004, 1, 100)
	for _, test := range []struct {
		name        string
		old         string
		replacement string
		peerUp      bool
	}{
		{name: "exact selected route"},
		{name: "wrong AS path", old: `"as_path": "65004"`, replacement: `"as_path": "65005"`},
		{name: "longer AS path", old: `"as_path": "65004"`, replacement: `"as_path": "65004 65100"`},
		{name: "noncanonical AS path", old: `"as_path": "65004"`, replacement: `"as_path": "+65004"`},
		{name: "wrong next hop", old: `"bgp_nexthop": "172.30.0.9"`, replacement: `"bgp_nexthop": "172.30.0.5"`},
		{name: "wrong ORIGIN", old: `"origin": 0`, replacement: `"origin": 2`},
		{name: "missing ORIGIN", old: `, "origin": 0`},
		{name: "wrong local ASN", old: `"peer_asn": 65001`, replacement: `"peer_asn": 65004`},
		{name: "wrong local Peer Up ASN", old: `"peer_asn": 65001`, replacement: `"peer_asn": 65004`, peerUp: true},
		{name: "wrong local Router ID", old: `"bgp_id": "172.30.0.2"`, replacement: `"bgp_id": "198.51.100.1"`, peerUp: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			peerUp := medWholeSetLocUpFixture
			route := strings.Replace(medWholeSetLocRouteFixture, `"seq": 9`, `"seq": 25`, 1)
			if test.old != "" {
				if test.peerUp {
					peerUp = strings.Replace(peerUp, test.old, test.replacement, 1)
				} else {
					route = strings.Replace(route, test.old, test.replacement, 1)
				}
			}
			output := "2\n" + peerUp + "\n" + medWholeSetLocRouteFixture + "\n" + route + "\n"
			err := requireMEDWholeSetSelected(output, &want)
			if (err == nil) != (test.old == "") {
				t.Fatalf("selected criteria acceptance=%v: %v; rows=%s", err == nil, err, output)
			}
		})
	}
}

func TestMEDWholeSetCollectorSelectionBounds(t *testing.T) {
	t.Parallel()
	want := newMEDWholeSetCandidate([4]byte{172, 30, 0, 0}, 9, 65004, 1, 100)
	const unrelated = `{"event_type": "log", "bmp_router": "172.30.0.254"}` + "\n"
	const down = `{"seq": 26, "bmp_router": "172.30.0.2", "bmp_router_port": 48476, "bmp_msg_type": "peer_down", "peer_ip": "0.0.0.0", "bmp_rib_type": "Loc-Rib"}`
	fresh := strings.Replace(medWholeSetLocRouteFixture, `"seq": 9`, `"seq": 25`, 1) + "\n"
	for _, test := range []struct {
		name    string
		padding string
		after   string
		accept  bool
	}{
		{name: "absolute checkpoint at query bound", padding: strings.Repeat(unrelated, 253), after: fresh, accept: true},
		{name: "blank lines retain physical positions", padding: "\n\n", after: fresh, accept: true},
		{name: "oversized snapshot cannot hide Peer Down", padding: strings.Repeat(unrelated, 253), after: fresh + unrelated + down + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			logPath := filepath.Join(directory, "bmp.log")
			checkpoint := filepath.Join(directory, "checkpoint")
			before := medWholeSetLocUpFixture + "\n" + medWholeSetLocRouteFixture + "\n" + test.padding
			if err := os.WriteFile(logPath, []byte(before), 0o600); err != nil {
				t.Fatal(err)
			}
			medWholeSetRunQuery(t, medWholeSetSaveCheckpoint().command, logPath, checkpoint)
			if err := os.WriteFile(logPath, []byte(before+test.after), 0o600); err != nil {
				t.Fatal(err)
			}
			output := medWholeSetRunQuery(t, []string{"sh", "-c", medWholeSetSelectionRows}, logPath, checkpoint)
			err := requireMEDWholeSetSelected(output, &want)
			if (err == nil) != test.accept {
				t.Fatalf("bounded selection acceptance=%v, want %v: %v; rows=%s", err == nil, test.accept, err, output)
			}
		})
	}
}
