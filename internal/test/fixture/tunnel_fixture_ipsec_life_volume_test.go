// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA volume lifetimes on Linux XFRM.
// Related: tunnel_fixture_ipsec_life_volume.go -- parseXFRMStateCounters, verifyReplacementCountersReset.

package fixture

import (
	"strings"
	"testing"
)

// VALIDATES: parseXFRMStateCounters extracts only ESP states belonging to the requested
// endpoint pair, reads 64-bit bytes and packets from lifetime current (ignoring lifetime
// config), and rejects states missing lifetime-current counters.
// PREVENTS: reading another peer's XFRM counters or misreading lifetime config as live usage.
func TestParseXFRMStateCounters(t *testing.T) {
	sample := `src 127.0.0.1 dst 127.0.0.2
	proto esp spi 0x22222222 reqid 1 mode tunnel
	lifetime config:
	  limit: soft (INF)(bytes), hard (INF)(bytes)
	  limit: soft (INF)(packets), hard (INF)(packets)
	lifetime current:
	  125440(bytes), 210(packets)
	  add 2026-11-28 00:00:01 use 2026-11-28 00:00:02
	stats:
	  replay-window 0 replay 0 failed 0
src 127.0.0.2 dst 127.0.0.1
	proto esp spi 0x11111111 reqid 1 mode tunnel
	lifetime current:
	  0(bytes), 0(packets)
src 127.0.0.3 dst 127.0.0.4
	proto esp spi 0x99999999 reqid 2 mode tunnel
	lifetime current:
	  999999(bytes), 888(packets)
`
	states, err := parseXFRMStateCounters(sample, "127.0.0.1", "127.0.0.2")
	if err != nil {
		t.Fatalf("parseXFRMStateCounters: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("len(states) = %d, want 2", len(states))
	}
	outBytes, outPackets, ok := outboundCounter(states, "127.0.0.1", "127.0.0.2")
	if !ok {
		t.Fatal("outbound state 127.0.0.1 -> 127.0.0.2 not found")
	}
	if outBytes != 125440 {
		t.Errorf("outBytes = %d, want 125440", outBytes)
	}
	if outPackets != 210 {
		t.Errorf("outPackets = %d, want 210", outPackets)
	}

	incomplete := `src 127.0.0.1 dst 127.0.0.2
	proto esp spi 0x33333333 reqid 1 mode tunnel
`
	if _, err := parseXFRMStateCounters(incomplete, "127.0.0.1", "127.0.0.2"); err == nil {
		t.Fatal("expected error for ESP state missing lifetime current counters")
	}
}

// VALIDATES: verifyReplacementCountersReset accepts fresh replacement states with zero or
// 1-packet-tolerance counters below the retired SA peak and rejects stale/unreset counters
// or fewer than 2 replacement states.
// PREVENTS: falsely reporting counter reset when a rekey never installed fresh SAs.
func TestVerifyReplacementCountersReset(t *testing.T) {
	freshZero := []xfrmStateCounter{
		{src: "127.0.0.1", dst: "127.0.0.2", spi: "0xaaaaaaaa", bytes: 0, packets: 0},
		{src: "127.0.0.2", dst: "127.0.0.1", spi: "0xbbbbbbbb", bytes: 0, packets: 0},
	}
	gotBytes, gotPkts, err := verifyReplacementCountersReset("BYTES", freshZero, 125000, 200)
	if err != nil {
		t.Fatalf("zero counters rejected: %v", err)
	}
	if gotBytes != 0 {
		t.Errorf("gotBytes = %d, want 0", gotBytes)
	}
	if gotPkts != 0 {
		t.Errorf("gotPkts = %d, want 0", gotPkts)
	}

	stale := []xfrmStateCounter{
		{src: "127.0.0.1", dst: "127.0.0.2", spi: "0xaaaaaaaa", bytes: 120000, packets: 190},
		{src: "127.0.0.2", dst: "127.0.0.1", spi: "0xbbbbbbbb", bytes: 0, packets: 0},
	}
	_, _, err = verifyReplacementCountersReset("BYTES", stale, 125000, 200)
	if err == nil {
		t.Fatal("stale non-reset counters accepted")
	}
	if !strings.Contains(err.Error(), "did not reset") {
		t.Errorf("err = %q, want mention of reset", err)
	}

	if _, _, err := verifyReplacementCountersReset("PACKETS", freshZero[:1], 10000, 191); err == nil {
		t.Fatal("single replacement state accepted")
	}
}
