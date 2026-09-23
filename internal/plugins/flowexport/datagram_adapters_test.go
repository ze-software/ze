// Design: docs/architecture/flowexport/flow-export-0-umbrella.md -- Collector datagram limits

package flowexport_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
	"github.com/ze-software/ze/internal/plugins/flowexport/ipfix"
	"github.com/ze-software/ze/internal/plugins/flowexport/netflow9"
	"github.com/ze-software/ze/internal/plugins/flowexport/sflow"
)

// TestCollectorDatagramBounds receives every encoded datagram. Odd bounds
// exercise padding that fits the pool but exceeds the collector's limit.
func TestCollectorDatagramBounds(t *testing.T) {
	for _, bound := range []int{464, 548, 557, 581, 1232, 1399, 1400} {
		for _, protocol := range []string{"ipfix", "netflow9", "sflow"} {
			t.Run(fmt.Sprintf("%s/%d", protocol, bound), func(t *testing.T) {
				var lc net.ListenConfig
				pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = pc.Close() }()
				addr, ok := pc.LocalAddr().(*net.UDPAddr)
				if !ok {
					t.Fatal("unexpected listener address")
				}
				sender, err := flowexport.NewSender("127.0.0.1", addr.Port, "", bound)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = sender.Close() }()
				// Exercise wrap while counter templates/data and flow
				// templates/data share one transport session.
				const initialSequence = ^uint32(0) - 1
				sender.AdvanceSequence(initialSequence)
				expectedSequence := initialSequence
				start := time.Now()
				var counters flowexport.ProtocolEncoder
				var flows flowexport.FlowRecordEncoder
				switch protocol {
				case "ipfix":
					counters = ipfix.NewCounterEncoder(1)
					flows = ipfix.NewFlowEncoder(1)
				case "netflow9":
					counters = netflow9.NewCounterEncoder(1, start)
					flows = netflow9.NewFlowEncoder(1, start)
				case "sflow":
					counters = sflow.NewCounterEncoder(netip.MustParseAddr("192.0.2.1"), 1, start)
				}
				var received uint64
				receive := func() [][]byte {
					t.Helper()
					sent, _, _ := sender.Stats()
					var datagrams [][]byte
					if err := pc.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
						t.Fatal(err)
					}
					for ; received < sent; received++ {
						buf := make([]byte, flowexport.MaxDatagramSize+1)
						n, _, err := pc.ReadFrom(buf)
						if err != nil {
							t.Fatal(err)
						}
						if n > bound {
							t.Fatalf("datagram size = %d, bound = %d", n, bound)
						}
						var sequence, advance uint32
						switch protocol {
						case "ipfix":
							sequence = binary.BigEndian.Uint32(buf[8:])
							setID := binary.BigEndian.Uint16(buf[16:])
							var recordSize int
							switch setID {
							case 256:
								recordSize = ipfix.CounterRecordSize()
							case 257:
								recordSize = ipfix.FlowRecordSize()
							case 258:
								recordSize = ipfix.FlowRecordSize6()
							}
							if recordSize > 0 {
								advance = uint32((int(binary.BigEndian.Uint16(buf[18:])) - 4) / recordSize)
							}
						case "netflow9":
							sequence, advance = binary.BigEndian.Uint32(buf[12:]), 1
						case "sflow":
							sequence, advance = binary.BigEndian.Uint32(buf[16:]), 1
						}
						if sequence != expectedSequence {
							t.Fatalf("datagram %d sequence = %d, want %d", received, sequence, expectedSequence)
						}
						expectedSequence += advance
						datagrams = append(datagrams, buf[:n])
					}
					return datagrams
				}
				if err := counters.EncodeTemplate(sender); err != nil {
					t.Fatal(err)
				}
				receive()
				interfaces := make([]flowexport.InterfaceCounters, 40)
				for i := range interfaces {
					interfaces[i].IfIndex = uint32(i + 1)
				}
				if sent, err := counters.Encode(flowexport.CounterSnapshot{Time: start, Interfaces: interfaces}, sender); err != nil || sent != len(interfaces) {
					t.Fatalf("counter export = %d, %v; want %d", sent, err, len(interfaces))
				}
				count := 0
				for _, dg := range receive() {
					switch protocol {
					case "ipfix":
						count += (int(binary.BigEndian.Uint16(dg[18:])) - 4) / ipfix.CounterRecordSize()
					case "netflow9":
						count += int(binary.BigEndian.Uint16(dg[2:]))
					case "sflow":
						count += int(binary.BigEndian.Uint32(dg[24:]))
					}
				}
				if count != len(interfaces) {
					t.Fatalf("wire counter records = %d, want %d", count, len(interfaces))
				}
				if flows == nil {
					enc := sflow.NewFlowEncoder(netip.MustParseAddr("192.0.2.1"), 1, start)
					if err := enc.EncodeFlowSample(flowexport.FlowSample{IfIndex: 1, Rate: 1, OrigSize: 1500, Header: make([]byte, 1500)}, sender); err != nil {
						t.Fatal(err)
					}
					if got := len(receive()); got != 1 {
						t.Fatalf("sample datagrams = %d, want 1", got)
					}
					return
				}
				if err := flows.EncodeFlowTemplate(sender); err != nil {
					t.Fatal(err)
				}
				receive()
				batch := make([]flowexport.ConntrackFlow, 40)
				for i := range batch {
					addr := netip.MustParseAddr("192.0.2.1")
					if i%2 != 0 {
						addr = netip.MustParseAddr("2001:db8::1")
					}
					batch[i] = flowexport.ConntrackFlow{SrcAddr: addr, DstAddr: addr, SrcPort: uint16(i + 1)}
				}
				if sent, err := flows.EncodeFlows(batch, sender); err != nil || sent != len(batch) {
					t.Fatalf("flow export = %d, %v; want %d", sent, err, len(batch))
				}
				seen := make(map[uint16]bool)
				for _, dg := range receive() {
					header, size4, size6 := 16, ipfix.FlowRecordSize(), ipfix.FlowRecordSize6()
					if protocol == "netflow9" {
						header, size4, size6 = 20, netflow9.FlowRecordSize(), netflow9.FlowRecordSize6()
					}
					recordSize, portOffset := size4, 8
					if binary.BigEndian.Uint16(dg[header:]) == 258 {
						recordSize, portOffset = size6, 32
					}
					for off := header + 4; off+recordSize <= len(dg); off += recordSize {
						port := binary.BigEndian.Uint16(dg[off+portOffset:])
						if port == 0 || int(port) > len(batch) || seen[port] {
							t.Fatalf("unexpected or duplicate flow %d", port)
						}
						seen[port] = true
					}
				}
				if len(seen) != len(batch) {
					t.Fatalf("wire flow records = %d, want %d", len(seen), len(batch))
				}
			})
		}
	}
}
