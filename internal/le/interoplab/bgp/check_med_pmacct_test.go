package bgp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestMEDWholeSetCollectorFreshRows runs the actual collector query against an
// append-only msglog. A historical winner, a fresh wrong winner and a later
// withdrawal must not satisfy the next transition; a fresh exact winner must.
func TestMEDWholeSetCollectorFreshRows(t *testing.T) {
	t.Parallel()
	wanted := strings.NewReplacer(`"as_path": "65004"`, `"as_path": "65005"`, `"bgp_nexthop": "172.30.0.9"`, `"bgp_nexthop": "172.30.0.5"`).Replace(medWholeSetLocRouteFixture)
	other := medWholeSetLocRouteFixture
	withdrawn := strings.Replace(wanted, `"log_type": "update"`, `"log_type": "withdraw"`, 1)
	unrelated := strings.ReplaceAll(wanted, "10.99.77.0/24", "10.99.78.0/24")
	want := newMEDWholeSetCandidate([4]byte{172, 30, 0, 0}, 5, 65005, 2, 50)
	for _, test := range []struct {
		name   string
		fresh  []string
		accept bool
	}{
		{name: "historical winner only"},
		{name: "fresh wrong winner", fresh: []string{other}},
		{name: "fresh exact winner", fresh: []string{other, wanted}, accept: true},
		{name: "winner subsequently withdrawn", fresh: []string{wanted, withdrawn}},
		{name: "unrelated prefix cannot restore winner", fresh: []string{wanted, withdrawn, unrelated}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			logPath := filepath.Join(directory, "bmp.log")
			checkpoint := filepath.Join(directory, "checkpoint")
			before := medWholeSetLocUpFixture + "\n" + wanted + "\n"
			if err := os.WriteFile(logPath, []byte(before), 0o600); err != nil {
				t.Fatal(err)
			}
			medWholeSetRunQuery(t, medWholeSetSaveCheckpoint().command, logPath, checkpoint)
			content := before
			for index, row := range test.fresh {
				content += strings.Replace(row, `"seq": 9`, `"seq": `+strconv.Itoa(25+index), 1) + "\n"
			}
			if err := os.WriteFile(logPath, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			output := medWholeSetRunQuery(t, []string{"sh", "-c", medWholeSetSelectionRows}, logPath, checkpoint)
			err := requireMEDWholeSetSelected(output, &want)
			if (err == nil) != test.accept {
				t.Fatalf("collector acceptance=%v, want %v: %v; row=%s", err == nil, test.accept, err, output)
			}
		})
	}
}

// TestMEDWholeSetCollectorInputCriteria exercises the epoch predicate that
// admits an input into the interop proof. Path criteria come from the latest
// decoded route and Router ID from its applicable independently decoded Peer Up.
func TestMEDWholeSetCollectorInputCriteria(t *testing.T) {
	t.Parallel()
	want := newMEDWholeSetCandidate([4]byte{172, 30, 0, 0}, 9, 65004, 1, 100)
	for _, test := range []struct {
		name        string
		old         string
		replacement string
		peerUp      bool
	}{
		{name: "exact received input"},
		{name: "wrong MED", old: `"med": 100`, replacement: `"med": 0`},
		{name: "absent MED", old: `, "med": 100`},
		{name: "wrong neighbor AS", old: `"peer_asn": 65004`, replacement: `"peer_asn": 65005`},
		{name: "longer AS path", old: `"as_path": "65004"`, replacement: `"as_path": "65004 65100"`},
		{name: "wrong Router ID", old: `198.51.100.1`, replacement: `198.51.100.3`, peerUp: true},
		{name: "wrong next hop", old: `"bgp_nexthop": "172.30.0.9"`, replacement: `"bgp_nexthop": "172.30.0.3"`},
		{name: "worse origin", old: `"origin": 0`, replacement: `"origin": 2`},
		{name: "absent ORIGIN", old: `, "origin": 0`},
		{name: "absent neighbor AS", old: `, "peer_asn": 65004`},
		{name: "absent AS path", old: `, "as_path": "65004"`},
		{name: "absent next hop", old: `, "bgp_nexthop": "172.30.0.9"`},
		{name: "null MED", old: `"med": 100`, replacement: `"med": null`},
		{name: "LOCAL_PREF null", old: `"med": 100`, replacement: `"med": 100, "local_pref": null`},
		{name: "AIGP null", old: `"med": 100`, replacement: `"med": 100, "aigp": null`},
		{name: "LOCAL_PREF present", old: `"med": 100`, replacement: `"med": 100, "local_pref": 100`},
		{name: "AIGP present", old: `"med": 100`, replacement: `"med": 100, "aigp": 0`},
		{name: "route withdrawn", old: `"log_type": "update"`, replacement: `"log_type": "withdraw"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			peerUp, route := medWholeSetPeerUpFixture, medWholeSetRouteFixture
			if test.old != "" {
				if test.peerUp {
					peerUp = strings.Replace(peerUp, test.old, test.replacement, 1)
				} else {
					route = strings.Replace(route, test.old, test.replacement, 1)
				}
			}
			// A prior correct row cannot fill missing fields in the latest row.
			output := peerUp + "\n" + medWholeSetRouteFixture + "\n" + route + "\n"
			err := requireMEDWholeSetInput(output, &want)
			if (err == nil) != (test.old == "") {
				t.Fatalf("collector input acceptance=%v: %v; row=%s", err == nil, err, output)
			}
		})
	}
}

func medWholeSetRunQuery(t *testing.T, command []string, logPath, checkpoint string) string {
	t.Helper()
	script := strings.NewReplacer(pmacctMsgLogPath, strconv.Quote(logPath), medWholeSetCheckpoint, strconv.Quote(checkpoint)).Replace(command[2])
	output, err := exec.CommandContext(t.Context(), command[0], command[1], script).CombinedOutput()
	if err != nil {
		t.Fatalf("collector query: %v: %s", err, output)
	}
	return string(output)
}

// TestMEDWholeSetCollectorActualSchema uses the independent collector's actual
// Peer Up and Route Monitoring schema from artifact1181. Router ID belongs to
// the preceding peer epoch, not to the route row.
func TestMEDWholeSetCollectorActualSchema(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	logPath := filepath.Join(directory, "bmp.log")
	if err := os.WriteFile(logPath, []byte(medWholeSetPeerUpFixture+"\n"+medWholeSetRouteFixture+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := newMEDWholeSetCandidate([4]byte{172, 30, 0, 0}, 9, 65004, 1, 100)
	output, err := exec.CommandContext(t.Context(), "tail", "-n", "256", logPath).CombinedOutput()
	if err != nil {
		t.Fatalf("read collector evidence: %v: %s", err, output)
	}
	if err := requireMEDWholeSetInput(string(output), &want); err != nil {
		t.Fatalf("actual Peer Up + Route Monitoring evidence rejected: %v; rows=%s", err, output)
	}
}

const (
	medWholeSetPeerUpFixture = `{"event_type": "log", "seq": 5, "bmp_router": "172.30.0.2", "bmp_router_port": 48476, "bmp_msg_type": "peer_up", "peer_ip": "172.30.0.9", "peer_asn": 65004, "peer_type": 0, "bmp_rib_type": "Adj-Rib-In Pre-Policy", "bgp_id": "198.51.100.1"}`
	medWholeSetRouteFixture  = `{"seq": 12, "log_type": "update", "bmp_rib_type": "Adj-Rib-In Pre-Policy", "event_type": "log", "afi": 1, "safi": 1, "ip_prefix": "10.99.77.0/24", "as_path_id": 3, "bgp_nexthop": "172.30.0.9", "as_path": "65004", "origin": 0, "med": 100, "bmp_router": "172.30.0.2", "bmp_router_port": 48476, "peer_ip": "172.30.0.9", "peer_asn": 65004, "bmp_msg_type": "route_monitor"}`
)
