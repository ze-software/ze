package cli

import "testing"

// TestDashboardReadsAnAsdotPeerList proves the dashboard decodes a summary the
// daemon wrote under bgp/as-notation asdot, and shows the AS number as the
// daemon rendered it. The method parses one payload of each spelling.
//
// The CLI deliberately configures NO notation here. That is the contract: this
// process renders the spelling it received, so it cannot disagree with the
// daemon that produced it.
//
// VALIDATES: dashboardPeer.RemoteAS and dashboardSnapshot.LocalAS are
// asn.Number, so a quoted AS number decodes and keeps its spelling.
// PREVENTS: the whole peer list failing to decode under asdot, which leaves an
// empty dashboard and no message saying why. A uint32 field did exactly that.
func TestDashboardReadsAnAsdotPeerList(t *testing.T) {
	const payload = `{"router-id":"1.2.3.4","local-as":"1.10","uptime":"1s",
		"peers-configured":1,"peers-established":1,
		"peers":[{"address":"192.0.2.1","remote-as":"1.10","state":"established","uptime":"1s"}]}`

	snap, err := parseDashboardSnapshot(payload)
	if err != nil {
		t.Fatalf("parseDashboardSnapshot: %v", err)
	}
	if len(snap.Peers) != 1 {
		t.Fatalf("peers = %d, want 1: an asdot payload must decode", len(snap.Peers))
	}
	if snap.Peers[0].RemoteAS.Value() != 65546 {
		t.Errorf("remote-as = %d, want 65546", snap.Peers[0].RemoteAS.Value())
	}
	if got := peerColumnValue(snap.Peers[0], sortColumnASN, nil); got != "1.10" {
		t.Errorf("ASN column = %q, want %q", got, "1.10")
	}
	if got := snap.LocalAS.String(); got != "1.10" {
		t.Errorf("local AS = %q, want %q", got, "1.10")
	}
}

// TestDashboardReadsAnAsplainPeerList proves the asplain payload every release
// before the leaf existed wrote still decodes and still renders decimal. The
// method parses one JSON-number payload.
//
// VALIDATES: the asn.Number fields accept a JSON number.
// PREVENTS: the tolerant decoder being written for the string form alone.
func TestDashboardReadsAnAsplainPeerList(t *testing.T) {
	const payload = `{"router-id":"1.2.3.4","local-as":65546,"uptime":"1s",
		"peers":[{"address":"192.0.2.1","remote-as":65546,"state":"established","uptime":"1s"}]}`

	snap, err := parseDashboardSnapshot(payload)
	if err != nil {
		t.Fatalf("parseDashboardSnapshot: %v", err)
	}
	if got := peerColumnValue(snap.Peers[0], sortColumnASN, nil); got != "65546" {
		t.Errorf("ASN column = %q, want %q", got, "65546")
	}
}
