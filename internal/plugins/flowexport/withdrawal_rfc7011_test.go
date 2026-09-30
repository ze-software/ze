// Design: docs/architecture/flowexport/flow-export-0-umbrella.md -- template refresh
// Related: export_test.go -- the exporter wrappers this external test drives
// Related: ipfix/template.go -- the counter Template Record, Field Count
// Related: ipfix/flow_template.go -- the flow Template Records, Field Count
//
// VALIDATES: the IPFIX exporter, which sends over UDP only, never sends a
// Template Withdrawal: every Template Record that reaches a UDP collector,
// across start, refresh, a reload that replaces the Templates and a reload
// that removes the collector, carries a Field Count above zero.
// PREVENTS: a Field Count of 0 on the wire, which RFC 7011 Section 8.1 defines
// as a withdrawal and Section 8.4 forbids over UDP.
//
// The test package is external so it can link the IPFIX encoder, which
// imports flowexport to register itself.

package flowexport_test

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
	_ "github.com/ze-software/ze/internal/plugins/flowexport/ipfix"
)

// ipfixTemplateRecords counts the Template Records in the IPFIX Messages a
// collector received, and names every one whose Field Count is 0.
type ipfixTemplateRecords struct {
	defined    int
	withdrawn  []uint16
	datagrams  int
	nonIPFIX   int
	malformed  int
	templateOK map[uint16]bool
}

// udpCollector is a bound UDP socket standing in for an IPFIX collector.
func udpCollector(t *testing.T) (net.PacketConn, int) {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("collector address %T is not UDP", pc.LocalAddr())
	}
	return pc, addr.Port
}

// ipfixCollectorConfig is one IPFIX collector sending to port on loopback.
func ipfixCollectorConfig(port int, domain uint32) *flowexport.Config {
	return &flowexport.Config{Collectors: []flowexport.CollectorConfig{{
		Name: "c1", Address: "127.0.0.1", Port: port, Protocol: "ipfix",
		ObservationDomain: domain, PollingInterval: 1, TemplateRefresh: 600,
		MaxDatagramSize: flowexport.DatagramSizeDefault,
	}}}
}

func ipv4Flow() []flowexport.ConntrackFlow {
	return []flowexport.ConntrackFlow{{
		SrcAddr:  netip.MustParseAddr("192.0.2.1"),
		DstAddr:  netip.MustParseAddr("198.51.100.1"),
		Protocol: 6,
		Bytes:    100,
		Packets:  2,
	}}
}

// drainQuiet is how long drain waits for another datagram before it takes
// the collector's queue as empty.
const drainQuiet = 300 * time.Millisecond

// drain reads every datagram queued on pc until none arrives for drainQuiet,
// and walks each as an IPFIX Message.
func drain(t *testing.T, pc net.PacketConn, into *ipfixTemplateRecords) {
	t.Helper()
	buf := make([]byte, 65535)
	for {
		if err := pc.SetReadDeadline(time.Now().Add(drainQuiet)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		n, _, err := pc.ReadFrom(buf)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		into.datagrams++
		walkIPFIXMessage(buf[:n], into)
	}
}

// walkIPFIXMessage walks the Sets of one IPFIX Message (RFC 7011 Section 3.1
// header of 16 octets, Section 3.3.2 Set Header) and reads the Template
// Record Header of every record in a Template Set (Set ID 2) or an Options
// Template Set (Set ID 3).
func walkIPFIXMessage(msg []byte, into *ipfixTemplateRecords) {
	if len(msg) < 16 {
		into.malformed++
		return
	}
	if binary.BigEndian.Uint16(msg) != 10 {
		into.nonIPFIX++
		return
	}
	for off := 16; off+4 <= len(msg); {
		setID := binary.BigEndian.Uint16(msg[off:])
		setLength := int(binary.BigEndian.Uint16(msg[off+2:]))
		if setLength < 4 || off+setLength > len(msg) {
			into.malformed++
			return
		}
		if setID == 2 || setID == 3 {
			walkTemplateSet(msg[off+4:off+setLength], setID, into)
		}
		off += setLength
	}
}

// walkTemplateSet reads each Template Record Header: Template ID, Field Count,
// and for an Options Template Record the Scope Field Count.
func walkTemplateSet(set []byte, setID uint16, into *ipfixTemplateRecords) {
	header := 4
	if setID == 3 {
		header = 6
	}
	for off := 0; off+header <= len(set); {
		templateID := binary.BigEndian.Uint16(set[off:])
		fieldCount := int(binary.BigEndian.Uint16(set[off+2:]))
		off += header
		if fieldCount == 0 {
			into.withdrawn = append(into.withdrawn, templateID)
			continue
		}
		into.defined++
		into.templateOK[templateID] = true
		for range fieldCount {
			if off+4 > len(set) {
				into.malformed++
				return
			}
			fieldID := binary.BigEndian.Uint16(set[off:])
			off += 4
			if fieldID&0x8000 != 0 {
				off += 4 // Enterprise Number, RFC 7011 Section 3.2.
			}
		}
	}
}

// RFC requirement: RFC7011-8-3 positive -- the IPFIX exporter sending over UDP
// sends no Template Withdrawal while it runs: the Templates it sends at start,
// with the first flow records and at the 600 s refresh each carry a Field
// Count above zero, and no Template Record with a Field Count of 0 reaches the
// collector.
func TestRFC7011UDPExporterSendsNoTemplateWithdrawal(t *testing.T) {
	pc, port := udpCollector(t)
	exp, err := flowexport.NewWiredExporter(ipfixCollectorConfig(port, 1))
	if err != nil {
		t.Fatalf("exporter: %v", err)
	}
	t.Cleanup(exp.Stop)

	t0 := time.Now()
	exp.NotifySnapshot(flowexport.CounterSnapshot{Time: t0, Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}}})
	exp.ExportFlows(ipv4Flow())
	exp.NotifySnapshot(flowexport.CounterSnapshot{Time: t0.Add(600 * time.Second), Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}}})

	got := ipfixTemplateRecords{templateOK: map[uint16]bool{}}
	drain(t, pc, &got)

	if got.nonIPFIX != 0 || got.malformed != 0 {
		t.Fatalf("the collector received %d non-IPFIX and %d malformed datagrams", got.nonIPFIX, got.malformed)
	}
	if len(got.withdrawn) != 0 {
		t.Fatalf("the UDP exporter sent a Template Withdrawal (Field Count 0) for Template IDs %v", got.withdrawn)
	}
	// The counter Template twice (start, refresh) and the flow Templates once:
	// at least three defined records, and the counter and the IPv4 flow
	// Template among them.
	if got.defined < 3 {
		t.Fatalf("the collector received %d Template Records over %d datagrams, want at least 3", got.defined, got.datagrams)
	}
	for _, id := range []uint16{256, 257} {
		if !got.templateOK[id] {
			t.Fatalf("no Template Record defined Template ID %d; received %v", id, got.templateOK)
		}
	}
}

// RFC requirement: RFC7011-8-3 negative -- a reload drives the exporter toward
// withdrawing its Templates, and still none is sent over UDP: a reload that
// moves the collector to another Observation Domain replaces every running
// Template (the replacing exporter starts, then the old one stops, the order
// configure uses), and a reload that removes the collector stops the exporter;
// the Template Records that reach the collector across both carry a Field
// Count above zero, and after the removal nothing reaches it, not even when
// a straggling worker feeds the stopped exporter.
func TestRFC7011UDPExporterReloadSendsNoTemplateWithdrawal(t *testing.T) {
	pc, port := udpCollector(t)
	first, err := flowexport.NewWiredExporter(ipfixCollectorConfig(port, 1))
	if err != nil {
		t.Fatalf("exporter: %v", err)
	}
	t.Cleanup(first.Stop)

	t0 := time.Now()
	first.NotifySnapshot(flowexport.CounterSnapshot{Time: t0, Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}}})
	first.ExportFlows(ipv4Flow())

	running := ipfixTemplateRecords{templateOK: map[uint16]bool{}}
	drain(t, pc, &running)
	if running.defined == 0 || len(running.withdrawn) != 0 {
		t.Fatalf("before the reload: %d Templates defined, withdrawals %v", running.defined, running.withdrawn)
	}

	// Reload: the Observation Domain changes, so every running Template is
	// dropped and the replacing exporter defines its own.
	second, err := flowexport.NewWiredExporter(ipfixCollectorConfig(port, 2))
	if err != nil {
		t.Fatalf("replacing exporter: %v", err)
	}
	t.Cleanup(second.Stop)
	second.NotifySnapshot(flowexport.CounterSnapshot{Time: t0.Add(time.Second), Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}}})
	second.ExportFlows(ipv4Flow())
	first.Stop()

	reloaded := ipfixTemplateRecords{templateOK: map[uint16]bool{}}
	drain(t, pc, &reloaded)
	if len(reloaded.withdrawn) != 0 {
		t.Fatalf("a reload that dropped the running Templates sent a Template Withdrawal (Field Count 0) for %v", reloaded.withdrawn)
	}
	if !reloaded.templateOK[256] || !reloaded.templateOK[257] {
		t.Fatalf("the replacing exporter defined %v, want Template IDs 256 and 257", reloaded.templateOK)
	}

	// Reload that removes the collector: the exporter stops. A worker still
	// holding it feeds a snapshot and a flow batch.
	second.Stop()
	second.NotifySnapshot(flowexport.CounterSnapshot{Time: t0.Add(700 * time.Second), Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}}})
	second.ExportFlows(ipv4Flow())
	first.NotifySnapshot(flowexport.CounterSnapshot{Time: t0.Add(700 * time.Second), Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}}})

	removed := ipfixTemplateRecords{templateOK: map[uint16]bool{}}
	drain(t, pc, &removed)
	if removed.datagrams != 0 {
		t.Fatalf("after the collector was removed %d datagrams reached it (withdrawals %v)", removed.datagrams, removed.withdrawn)
	}
}
