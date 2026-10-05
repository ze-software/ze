package bgp

import (
	"strings"
	"testing"
)

// TestMEDWholeSetCollectorPeerEpochs rejects identities from another peer, BMP
// connection or earlier session, while accepting a newly established valid epoch.
func TestMEDWholeSetCollectorPeerEpochs(t *testing.T) {
	t.Parallel()
	medWholeSetEpochBranches(t)
}

func medWholeSetEpochBranches(t *testing.T) {
	t.Helper()
	t.Run("selected Loc-RIB epoch", medWholeSetSelectedEpochBranches)
	want := newMEDWholeSetCandidate([4]byte{172, 30, 0, 0}, 9, 65004, 1, 100)
	newUp := strings.Replace(medWholeSetPeerUpFixture, `"seq": 5`, `"seq": 20`, 1)
	newRoute := strings.Replace(medWholeSetRouteFixture, `"seq": 12`, `"seq": 21`, 1)
	const down = `{"seq": 13, "bmp_router": "172.30.0.2", "bmp_router_port": 48476, "bmp_msg_type": "peer_down", "peer_ip": "172.30.0.9", "peer_asn": 65004}`
	const restart = `{"seq": 14, "event_type": "log_init", "bmp_router": "172.30.0.2", "bmp_router_port": 55555}`
	for _, test := range []struct {
		name   string
		rows   []string
		accept bool
	}{
		{name: "current epoch", rows: []string{medWholeSetPeerUpFixture, medWholeSetRouteFixture}, accept: true},
		{name: "no Peer Up", rows: []string{medWholeSetRouteFixture}},
		{name: "route Router ID is not Peer Up evidence", rows: []string{strings.Replace(medWholeSetRouteFixture, `"seq": 12`, `"seq": 12, "bgp_id": "198.51.100.1"`, 1)}},
		{name: "missing Peer Up Router ID", rows: []string{strings.Replace(medWholeSetPeerUpFixture, `, "bgp_id": "198.51.100.1"`, "", 1), medWholeSetRouteFixture}},
		{name: "different peer", rows: []string{strings.Replace(medWholeSetPeerUpFixture, `"peer_ip": "172.30.0.9"`, `"peer_ip": "172.30.0.3"`, 1), medWholeSetRouteFixture}},
		{name: "different Peer Up ASN", rows: []string{strings.Replace(medWholeSetPeerUpFixture, `"peer_asn": 65004`, `"peer_asn": 65005`, 1), medWholeSetRouteFixture}},
		{name: "different BMP connection", rows: []string{medWholeSetPeerUpFixture, strings.Replace(medWholeSetRouteFixture, "48476", "55555", 1)}},
		{name: "route before Peer Up", rows: []string{medWholeSetRouteFixture, medWholeSetPeerUpFixture}},
		{name: "route sequence before Peer Up", rows: []string{newUp, medWholeSetRouteFixture}},
		{name: "Peer Down invalidates route", rows: []string{medWholeSetPeerUpFixture, medWholeSetRouteFixture, down}},
		{name: "new Peer Up needs a new route", rows: []string{medWholeSetPeerUpFixture, medWholeSetRouteFixture, newUp}},
		{name: "new epoch wrong identifier", rows: []string{medWholeSetPeerUpFixture, medWholeSetRouteFixture, strings.Replace(newUp, "198.51.100.1", "198.51.100.3", 1), newRoute}},
		{name: "new epoch replaces old identifier", rows: []string{strings.Replace(medWholeSetPeerUpFixture, "198.51.100.1", "198.51.100.3", 1), medWholeSetRouteFixture, newUp, newRoute}, accept: true},
		{name: "BMP restart invalidates route", rows: []string{medWholeSetPeerUpFixture, medWholeSetRouteFixture, restart}},
		{name: "new BMP connection has own Peer Up", rows: []string{medWholeSetPeerUpFixture, medWholeSetRouteFixture, restart, strings.Replace(newUp, "48476", "55555", 1), strings.Replace(newRoute, "48476", "55555", 1)}, accept: true},
		{name: "withdrawn after valid route", rows: []string{medWholeSetPeerUpFixture, medWholeSetRouteFixture, strings.Replace(newRoute, `"log_type": "update"`, `"log_type": "withdraw"`, 1)}},
		{name: "unrelated peer down", rows: []string{medWholeSetPeerUpFixture, medWholeSetRouteFixture, strings.Replace(down, `"peer_ip": "172.30.0.9"`, `"peer_ip": "172.30.0.3"`, 1)}, accept: true},
		{name: "unrelated BMP router restart", rows: []string{medWholeSetPeerUpFixture, medWholeSetRouteFixture, strings.Replace(restart, "172.30.0.2", "172.30.0.254", 1)}, accept: true},
		{name: "missing Peer Up sequence", rows: []string{strings.Replace(medWholeSetPeerUpFixture, `"seq": 5, `, "", 1), medWholeSetRouteFixture}},
		{name: "missing route sequence", rows: []string{medWholeSetPeerUpFixture, strings.Replace(medWholeSetRouteFixture, `"seq": 12, `, "", 1)}},
		{name: "missing Peer Up connection", rows: []string{strings.Replace(medWholeSetPeerUpFixture, `"bmp_router_port": 48476, `, "", 1), medWholeSetRouteFixture}},
		{name: "missing route connection", rows: []string{medWholeSetPeerUpFixture, strings.Replace(medWholeSetRouteFixture, `"bmp_router_port": 48476, `, "", 1)}},
		{name: "malformed collector row", rows: []string{"{", medWholeSetPeerUpFixture, medWholeSetRouteFixture}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			output := strings.Join(test.rows, "\n") + "\n"
			err := requireMEDWholeSetInput(output, &want)
			if (err == nil) != test.accept {
				t.Fatalf("epoch acceptance=%v, want %v: %v; rows=%s", err == nil, test.accept, err, output)
			}
		})
	}
}

// TestMEDWholeSetCollectorAllSources requires the same joined proof for A, B and
// C, including an explicitly present zero MED rather than an absent attribute.
func TestMEDWholeSetCollectorAllSources(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name         string
		host         byte
		asn          uint32
		identifier   byte
		med          uint32
		replacements []string
	}{
		{name: "C", host: 9, asn: 65004, identifier: 1, med: 100},
		{name: "A", host: 3, asn: 65004, identifier: 3, med: 0,
			replacements: []string{"172.30.0.9", "172.30.0.3", "198.51.100.1", "198.51.100.3", `"med": 100`, `"med": 0`}},
		{name: "B", host: 5, asn: 65005, identifier: 2, med: 50,
			replacements: []string{"172.30.0.9", "172.30.0.5", "198.51.100.1", "198.51.100.2", "65004", "65005", `"med": 100`, `"med": 50`}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			want := newMEDWholeSetCandidate([4]byte{172, 30, 0, 0}, test.host, test.asn, test.identifier, test.med)
			output := medWholeSetPeerUpFixture + "\n" + medWholeSetRouteFixture + "\n"
			output = strings.NewReplacer(test.replacements...).Replace(output)
			if err := requireMEDWholeSetInput(output, &want); err != nil {
				t.Fatal(err)
			}
		})
	}
}
