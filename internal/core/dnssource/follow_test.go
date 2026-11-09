package dnssource

import (
	"encoding/json"
	"net/netip"
	"slices"
	"testing"
	"time"
)

// TestFollowerKeepsNewestSnapshot: a Follower decodes the JSON the engine
// delivers, keeps the newest snapshot's records, and ignores a snapshot whose
// sequence is not above the last kept, so a replayed or reordered delivery
// never rolls a consumer back.
//
// VALIDATES: AC-13 (spec-home-cpe-23b): a consumer that restarts applies the
// replayed snapshot, and a duplicate replay changes nothing.
// PREVENTS: a stale replay answered after a newer change restoring a
// withdrawn lease's servers.
func TestFollowerKeepsNewestSnapshot(t *testing.T) {
	encode := func(sequence uint64, addr string) string {
		t.Helper()
		body, err := json.Marshal(&Snapshot{Sequence: sequence, Records: []Record{{
			Origin: "dhcpv4", Rank: 100, Interface: "eth0", Unit: "0",
			Addresses: []netip.Addr{netip.MustParseAddr(addr)}, Received: time.Unix(1, 0).UTC(),
		}}})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(body)
	}

	var follower Follower
	if follower.Held() {
		t.Fatal("zero Follower reports a snapshot held")
	}
	fresh, err := follower.Accept(encode(7, "192.0.2.53"))
	if err != nil || !fresh {
		t.Fatalf("first snapshot: fresh=%v err=%v, want fresh", fresh, err)
	}
	if fresh, _ := follower.Accept(encode(7, "198.51.100.53")); fresh {
		t.Fatal("a repeated sequence was accepted as fresh")
	}
	if fresh, _ := follower.Accept(encode(6, "198.51.100.53")); fresh {
		t.Fatal("an older sequence was accepted as fresh")
	}
	got := ReceivedAddresses(follower.Records(), time.Unix(2, 0))
	if want := []netip.Addr{netip.MustParseAddr("192.0.2.53")}; !slices.Equal(got, want) {
		t.Fatalf("records after stale deliveries = %v, want %v", got, want)
	}
	if fresh, _ := follower.Accept(encode(8, "198.51.100.53")); !fresh {
		t.Fatal("a newer sequence was not accepted")
	}
	if _, err := follower.Accept("{not json"); err == nil {
		t.Fatal("a body that does not decode was accepted")
	}
}
