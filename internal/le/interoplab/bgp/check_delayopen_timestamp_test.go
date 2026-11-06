// Design: docs/architecture/testing/interop.md -- native capture time is ordered within each TCP connection.
// These native histories preserve each connection's causal timing checks.
package bgp

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
)

// TestDelayOpenTimestampOrderPerConnection retains two complete native
// histories. Queue interleaving may reverse their global record timestamps,
// while both connections still establish the same valid DelayOpen timing.
func TestDelayOpenTimestampOrderPerConnection(t *testing.T) {
	ze := netip.MustParseAddr("172.30.0.2")
	for _, interleaved := range []bool{false, true} {
		name := "chronological native records"
		if interleaved {
			name = "unrelated connections interleaved"
		}
		t.Run(name, func(t *testing.T) {
			records := delayOpenTimelineRecords(t)
			if interleaved {
				// These are different peers' first SYNs, 500 microseconds apart.
				// Neither connection's own record sequence changes.
				if records[0].Data[33] == records[1].Data[33] {
					t.Fatal("declared interleaving does not cross native peers")
				}
				records[0], records[1] = records[1], records[0]
			}
			history, err := delayOpenHistory(delayOpenTimelineBytes(t, records), ze)
			if err != nil {
				t.Fatalf("complete native histories were refused: %v", err)
			}
			if len(history) != 2 {
				t.Fatalf("history has %d native connections, want 2", len(history))
			}
			for _, peer := range []netip.Addr{netip.MustParseAddr("172.30.0.3"), netip.MustParseAddr("172.30.0.4")} {
				if _, err := judgeDelayOpen(history, peer, 5*time.Second, 30*time.Second, false); err != nil {
					t.Fatalf("native peer %s lost its timing proof: %v", peer, err)
				}
			}
		})
	}
}

// TestDelayOpenPermitsMeasuredEpochFraction preserves the existing nonzero
// epoch fraction. Only the exact encoded zero lacks a native measured time.
func TestDelayOpenPermitsMeasuredEpochFraction(t *testing.T) {
	records := delayOpenTestRecords(t, delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false))
	original := records[0].Timestamp
	start := time.Unix(0, int64(500*time.Microsecond)).UTC()
	for index := range records {
		records[index].Timestamp = start.Add(records[index].Timestamp.Sub(original))
	}
	history, err := delayOpenHistory(delayOpenTimelineBytes(t, records), netip.MustParseAddr("172.30.0.2"))
	if err != nil {
		t.Fatalf("measured nonzero epoch fraction was refused: %v", err)
	}
	if _, err := judgeDelayOpen(history, netip.MustParseAddr("172.30.0.3"), 5*time.Second, 30*time.Second, false); err != nil {
		t.Fatalf("measured epoch fraction lost timing proof: %v", err)
	}
}

// TestDelayOpenRefusesInvalidConnectionTime requires a measured time and a
// monotonic timeline for the entire normalized connection, not just adjacent
// file records that happen to have the same peer.
func TestDelayOpenRefusesInvalidConnectionTime(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		records func(*testing.T) []pcap.Record
	}{
		{name: "encoded zero first SYN", records: func(t *testing.T) []pcap.Record {
			records := delayOpenTestRecords(t, delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false))
			records[0].Timestamp = time.Unix(0, 0).UTC()
			return records
		}},
		{name: "same connection reversal", records: func(t *testing.T) []pcap.Record {
			records := delayOpenTestRecords(t, delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false))
			records[1].Timestamp = records[0].Timestamp.Add(-time.Microsecond)
			return records
		}},
		{name: "connection reversal behind another flow", records: func(t *testing.T) []pcap.Record {
			records := delayOpenTimelineRecords(t)
			// In the declared merge, row 2 is the first connection's SYN-ACK.
			// Row 4 is its final ACK, separated by the other peer's SYN-ACK.
			if records[2].Data[47] != 18 {
				t.Fatal("declared prior packet is not the SYN-ACK")
			}
			if records[4].Data[47] != 16 {
				t.Fatal("declared reversed packet is not the final ACK")
			}
			if records[2].Data[29] != 3 {
				t.Fatal("declared SYN-ACK does not come from the first peer")
			}
			if records[4].Data[33] != 3 {
				t.Fatal("declared ACK does not target the first peer")
			}
			records[4].Timestamp = records[2].Timestamp.Add(-time.Microsecond)
			return records
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			capture := delayOpenTimelineBytes(t, scenario.records(t))
			_, err := delayOpenHistory(capture, netip.MustParseAddr("172.30.0.2"))
			if err == nil {
				t.Fatal("unknown or reversed native connection time supplied a wire proof")
			}
			if !strings.Contains(err.Error(), "timestamp") {
				t.Fatalf("capture refused for an unrelated reason: %v", err)
			}
		})
	}
}

// delayOpenTimelineRecords uses the existing complete native history builder.
// The second peer has independent native ports and an offset clock sequence.
func delayOpenTimelineRecords(t *testing.T) []pcap.Record {
	t.Helper()
	records := delayOpenTestRecords(t, delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false))
	second := make([]pcap.Record, len(records))
	for index, record := range records {
		second[index] = pcap.Record{Timestamp: record.Timestamp.Add(500 * time.Microsecond), Data: bytes.Clone(record.Data), OriginalLen: record.OriginalLen}
		frame := second[index].Data
		if frame[29] == 2 {
			frame[33] = 4
			binary.BigEndian.PutUint16(frame[34:36], 40001)
		} else {
			frame[29] = 4
			binary.BigEndian.PutUint16(frame[36:38], 40001)
		}
	}
	records = append(records, second...)
	slices.SortStableFunc(records, func(a, b pcap.Record) int { return a.Timestamp.Compare(b.Timestamp) })
	return records
}

func delayOpenTimelineBytes(t *testing.T, records []pcap.Record) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := pcap.WriteFileHeader(&out, 65535, pcap.LinkTypeEthernet); err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if err := pcap.WriteRecord(&out, record.Timestamp, record.Data, len(record.Data)); err != nil {
			t.Fatal(err)
		}
	}
	return out.Bytes()
}
