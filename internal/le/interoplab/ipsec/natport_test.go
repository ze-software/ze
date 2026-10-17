package ipsec

import "testing"

// VALIDATES: the flush answer is read for its deleted count, and an answer that
// carries no count (conntrack missing, refused, or printing an error) is an error.
// PREVENTS: the old `conntrack -D -p udp || true`, which let a flush that never ran
// pass as one that did, so the rebind step waited 90 s for a move that could not come.
func TestConntrackDeletedCount(t *testing.T) {
	count, err := conntrackDeletedCount("conntrack v1.4.8 (conntrack-tools): 3 flow entries have been deleted.\nconntrack-exit=0\n")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
	for _, refused := range []string{
		"sh: conntrack: not found\nconntrack-exit=127\n",
		"conntrack v1.4.8 (conntrack-tools): Operation not permitted\nconntrack-exit=1\n",
		"",
	} {
		if _, err := conntrackDeletedCount(refused); err == nil {
			t.Fatalf("answer %q was read as a flush", refused)
		}
	}
}

// VALIDATES: a listing that still holds the old public port is reported with its
// line, a listing with only the new port passes, and a listing with no summary is
// an error.
// PREVENTS: a flush that deleted something but left strongSwan's own flow on the old
// mapping being read as a remap.
func TestConntrackStaleMapping(t *testing.T) {
	const old = "udp      17 29 src=172.28.0.3 dst=172.28.0.2 sport=4500 dport=4500 src=172.28.0.2 dst=172.28.0.7 sport=4500 dport=14500 mark=0 use=1"
	const moved = "udp      17 29 src=172.28.0.3 dst=172.28.0.2 sport=4500 dport=4500 src=172.28.0.2 dst=172.28.0.7 sport=4500 dport=14501 mark=0 use=1"
	const summary = "conntrack v1.4.8 (conntrack-tools): 1 flow entries have been shown."

	stale, err := conntrackStaleMapping(old + "\n" + summary + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if stale != old {
		t.Fatalf("stale = %q, want the old-port line", stale)
	}
	stale, err = conntrackStaleMapping(moved + "\n" + summary + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if stale != "" {
		t.Fatalf("stale = %q, want none after the remap", stale)
	}
	if _, err := conntrackStaleMapping("sh: conntrack: not found\n"); err == nil {
		t.Fatal("a listing with no summary was read as a listing")
	}
}

// VALIDATES: waitZeOutboundEncap accepts only the outbound state that encapsulates to
// the asked port: with a 14501 state first and a 14500 state second, asking for 14500
// answers the second SPI.
// PREVENTS: the read of an already drained textbuf.Buffer inside the poll, which
// matched "" and so accepted any encapsulating state whatever its port.
func TestWaitZeOutboundEncapMatchesThePort(t *testing.T) {
	fake := scriptedLab()
	fake.answer(zePeer, []string{"ip", xfrmCommand, "state"},
		"src "+zeIP+" dst "+swanPublicIP+"\n\tproto esp spi 0x00001111 reqid 1 mode tunnel\n\tencap type espinudp sport 4500 dport 14501 addr 0.0.0.0\n",
		"src "+zeIP+" dst "+swanPublicIP+"\n\tproto esp spi 0x00002222 reqid 1 mode tunnel\n\tencap type espinudp sport 4500 dport 14500 addr 0.0.0.0\n")
	spi, err := checkerFixture(fake).waitZeOutboundEncap(t.Context(), natFirstPort)
	if err != nil {
		t.Fatal(err)
	}
	if spi != "0x00002222" {
		t.Fatalf("spi = %s, want 0x00002222, the state that encapsulates to %d", spi, natFirstPort)
	}
}
