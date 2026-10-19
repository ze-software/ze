//go:build docker

// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- independent collector interpretation.
// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- decoded flow records and sampled packets.
// Related: ../../../test/interop-flowexport/Dockerfile.collectors -- the pinned nfdump and sflowtool image.
//
// These tests build the pinned collectors from test/interop-flowexport/Dockerfile.collectors,
// start each in its own container through labtest, and send to its published UDP port.
// They exercise public encoders and Sender over UDP, without daemon ingestion,
// network namespaces, privileges, or a Ze decoder acting as the oracle.
package flowexport_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/labtest"
	"github.com/ze-software/ze/internal/plugins/flowexport"
	"github.com/ze-software/ze/internal/plugins/flowexport/ipfix"
	"github.com/ze-software/ze/internal/plugins/flowexport/netflow9"
	"github.com/ze-software/ze/internal/plugins/flowexport/sflow"
)

// Both indexes exceed the compact sFlow source and port limits (24 and 30 bits).
const (
	exportInteropIfIndex = uint32(0xf1234567)
	exportInteropOutput  = uint32(0xe2345678)
)

const (
	// exportCollectorImage is the tag labtest builds from exportCollectorDockerfile.
	exportCollectorImage      = "ze-flowexport-collectors:test"
	exportCollectorDockerfile = "test/interop-flowexport/Dockerfile.collectors"
	// The collector's own UDP port inside its container; the host side is
	// whatever port Docker publishes it at.
	exportNFCapdPort    = 2055
	exportSFlowToolPort = 6343
	// exportFlowDir is where nfcapd writes its rotation files in the container.
	exportFlowDir = "/tmp/flows"
	// exportOutputMax bounds what one read of collector output accepts.
	exportOutputMax = 1 << 20
	// exportPollInterval paces the observations; each one is a docker exec.
	exportPollInterval = 100 * time.Millisecond
	// exportReadTimeout bounds one docker exec that reads collector state.
	exportReadTimeout = 10 * time.Second
)

// exportCollectorScript runs the collector as a child of the container's
// shell, so the container, and the files the collector wrote, outlive its
// exit: the test stops the collector itself, then reads the final rotation.
// It records the child's pid and, on exit, its status ($? is 128+N for a
// child killed by signal N).
const exportCollectorScript = `mkdir -p ` + exportFlowDir + `
"$@" >/tmp/collector.out 2>/tmp/collector.err &
pid=$!
echo "$pid" >/tmp/collector.pid
wait "$pid"
echo "$?" >/tmp/collector.exit
exec sleep infinity`

// MUTATION: a wrong template IE/width, truncated counter, swapped address or port,
// or lost IPv6/ASN field changes nfdump's decoded records or prevents collection.
func TestFlowExportNFDumpInterop(t *testing.T) {
	for _, protocol := range []string{"netflow9", "ipfix"} {
		t.Run(protocol, func(t *testing.T) {
			peer := startExportCollector(t, "nfcapd", exportNFCapdPort,
				"nfcapd", "-4", "-p", strconv.Itoa(exportNFCapdPort), "-w", exportFlowDir, "-t", "2", "-C", "none")
			sender := exportInteropSender(t, peer.container.Address)
			now := time.Now().Truncate(time.Second)
			start := now.Add(-time.Minute)
			var counters flowexport.ProtocolEncoder
			var flows flowexport.FlowRecordEncoder
			switch protocol {
			case "netflow9":
				counters = netflow9.NewCounterEncoder(42, start)
				flows = netflow9.NewFlowEncoder(42, start)
			case "ipfix":
				counters = ipfix.NewCounterEncoder(42)
				flows = ipfix.NewFlowEncoder(42)
			}
			if err := counters.EncodeTemplate(sender); err != nil {
				t.Fatalf("send counter template: %v", err)
			}
			if count, err := counters.Encode(exportInteropSnapshot(now), sender); err != nil || count != 1 {
				t.Fatalf("send counter: records=%d err=%v; want 1", count, err)
			}
			if err := flows.EncodeFlowTemplate(sender); err != nil {
				t.Fatalf("send flow templates: %v", err)
			}
			if count, err := flows.EncodeFlows(exportInteropFlows(now), sender); err != nil || count != 2 {
				t.Fatalf("send flows: records=%d err=%v; want 2", count, err)
			}

			// The stimulus is sent once. Only completed rotation files are decoded;
			// retrying a send here would hide lost templates or malformed records.
			seenFiles := make(map[string]bool)
			var records []map[string]json.RawMessage
			collect := func(ctx context.Context) (bool, error) {
				added, err := exportNFDumpRecords(ctx, peer.container, seenFiles)
				records = append(records, added...)
				return len(records) >= 3, err
			}
			peer.wait(t, "three nfdump records in completed files", 15*time.Second, collect)
			if err := peer.stop(t.Context()); err != nil {
				t.Fatalf("stop nfcapd: %v\n%s", err, peer.diagnostics())
			}
			// Include the final rotation, so shutdown cannot hide extra records.
			ctx, cancel := context.WithTimeout(t.Context(), exportReadTimeout)
			defer cancel()
			if _, err := collect(ctx); err != nil {
				t.Fatalf("read final nfcapd rotation: %v", err)
			}
			assertExportNFDumpRecords(t, protocol, records)
			t.Logf("%s: nfdump decoded exactly one counter and both IPv4/IPv6 flows", protocol)
		})
	}
}

// MUTATION: compact samples truncate these indexes; a wrong sampled-header
// length, FCS count, protocol or offset changes sflowtool's Ethernet/IP/UDP decode.
func TestFlowExportSFlowToolInterop(t *testing.T) {
	peer := startExportCollector(t, "sflowtool", exportSFlowToolPort,
		"sflowtool", "-4", "-p", strconv.Itoa(exportSFlowToolPort), "-j")
	sender := exportInteropSender(t, peer.container.Address)
	now := time.Now().Truncate(time.Second)
	start := now.Add(-time.Minute)
	agent := netip.MustParseAddr("192.0.2.9")
	counters := sflow.NewCounterEncoder(agent, 7, start)
	flows := sflow.NewFlowEncoder(agent, 7, start)
	if count, err := counters.Encode(exportInteropSnapshot(now), sender); err != nil || count != 1 {
		t.Fatalf("send counter: records=%d err=%v; want 1", count, err)
	}
	sample := flowexport.FlowSample{
		IfIndex: exportInteropIfIndex, Output: exportInteropOutput,
		Rate: 64, OrigSize: 60, Header: exportInteropEthernetFrame(),
	}
	if err := flows.EncodeFlowSample(sample, sender); err != nil {
		t.Fatalf("send sampled frame: %v", err)
	}
	peer.wait(t, "two complete sflowtool JSON datagrams", 10*time.Second, func(ctx context.Context) (bool, error) {
		data, err := peer.read(ctx, "/tmp/collector.out")
		if err != nil {
			return false, err
		}
		// -j flushes one JSON line per datagram. A partial last line is still
		// being written; a malformed complete line is a failure immediately.
		complete := data[:bytes.LastIndexByte(data, '\n')+1]
		datagrams, err := exportSFlowDatagrams(complete)
		return len(datagrams) >= 2, err
	})
	if err := peer.stop(t.Context()); err != nil {
		t.Fatalf("stop sflowtool: %v\n%s", err, peer.diagnostics())
	}
	data, err := peer.read(t.Context(), "/tmp/collector.out")
	if err != nil {
		t.Fatal(err)
	}
	datagrams, err := exportSFlowDatagrams(data)
	if err != nil {
		t.Fatalf("decode sflowtool output: %v", err)
	}
	assertExportSFlowDatagrams(t, datagrams)
	t.Log("sflowtool decoded full-width counter/source/port indexes and the sampled Ethernet/IPv4/UDP frame")
}

func exportInteropSnapshot(now time.Time) flowexport.CounterSnapshot {
	return flowexport.CounterSnapshot{
		Time: now,
		Interfaces: []flowexport.InterfaceCounters{{
			IfIndex: exportInteropIfIndex, IfType: 6, IfSpeed: 1000000000,
			IfDirection: flowexport.IfDirectionFullDuplex,
			IfStatus:    flowexport.IfStatusAdminUp | flowexport.IfStatusOperUp,
			IfInOctets:  4294967301, IfOutOctets: 8589934599,
			IfInUcastPkts: 101, IfOutUcastPkts: 202,
			InPackets: 4294967307, OutPackets: 8589934605,
			IfInMulticastPkts: 17, IfInBroadcastPkts: flowexport.CounterUnavailable,
			IfInDiscards: 19, IfInErrors: 23, IfInUnknownProtos: flowexport.CounterUnavailable,
			IfOutMulticastPkts: flowexport.CounterUnavailable, IfOutBroadcastPkts: flowexport.CounterUnavailable,
			IfOutDiscards: 29, IfOutErrors: 31, IfPromiscuousMode: 1,
		}},
	}
}

func exportInteropFlows(now time.Time) []flowexport.ConntrackFlow {
	return []flowexport.ConntrackFlow{
		{
			SrcAddr: netip.MustParseAddr("192.0.2.1"), DstAddr: netip.MustParseAddr("198.51.100.2"),
			SrcPort: 12345, DstPort: 443, Protocol: 6,
			Bytes: 4294967329, Packets: 37,
			FirstMs: uint64(now.Add(-2 * time.Second).UnixMilli()), LastMs: uint64(now.Add(-time.Second).UnixMilli()),
			SrcAS: 65551, DstAS: 4200000001,
		},
		{
			SrcAddr: netip.MustParseAddr("2001:db8:1::1"), DstAddr: netip.MustParseAddr("2001:db8:2::2"),
			SrcPort: 23456, DstPort: 53, Protocol: 17,
			Bytes: 8589934611, Packets: 73,
			FirstMs: uint64(now.Add(-2 * time.Second).UnixMilli()), LastMs: uint64(now.Add(-time.Second).UnixMilli()),
			SrcAS: 65552, DstAS: 4200000002,
		},
	}
}

// exportInteropEthernetFrame builds the sampled packet, never the export protocol.
func exportInteropEthernetFrame() []byte {
	frame := make([]byte, 60)
	copy(frame[:6], []byte{0x02, 0, 0, 0, 0, 2})
	copy(frame[6:12], []byte{0x02, 0, 0, 0, 0, 1})
	binary.BigEndian.PutUint16(frame[12:14], 0x0800)
	ip := frame[14:34]
	ip[0], ip[8], ip[9] = 0x45, 64, 17
	binary.BigEndian.PutUint16(ip[2:4], 46)
	copy(ip[12:16], []byte{192, 0, 2, 1})
	copy(ip[16:20], []byte{198, 51, 100, 2})
	var sum uint32
	for i := 0; i < len(ip); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(ip[i:]))
	}
	sum = (sum & 0xffff) + (sum >> 16)
	sum = (sum & 0xffff) + (sum >> 16)
	binary.BigEndian.PutUint16(ip[10:12], ^uint16(sum))
	udp := frame[34:42]
	binary.BigEndian.PutUint16(udp[:2], 12345)
	binary.BigEndian.PutUint16(udp[2:4], 53)
	binary.BigEndian.PutUint16(udp[4:6], 26)
	// A zero UDP checksum is valid for IPv4.
	copy(frame[42:], "collector-fixture!")
	return frame
}

func exportInteropSender(t *testing.T, address netip.AddrPort) *flowexport.Sender {
	t.Helper()
	sender, err := flowexport.NewSender(address.Addr().String(), int(address.Port()), "", flowexport.DatagramSizeDefault)
	if err != nil {
		t.Fatalf("create production UDP sender: %v", err)
	}
	t.Cleanup(func() {
		if err := sender.Close(); err != nil {
			t.Errorf("close sender: %v", err)
		}
	})
	return sender
}

// exportCollector is one collector running as the child of its container's
// shell (exportCollectorScript). labtest removes the container, and the
// collector with it, in t.Cleanup; stop is the requested shutdown a test
// MUST call before it reads the collector's final output. Not safe for
// concurrent use: the test goroutine owns it.
type exportCollector struct {
	container labtest.Container
	stopped   bool
}

// startExportCollector starts argv in a container built from the pinned
// collector image and returns once the collector's UDP socket is bound.
func startExportCollector(t *testing.T, name string, port uint16, argv ...string) *exportCollector {
	t.Helper()
	command := append([]string{"-c", exportCollectorScript, "collector"}, argv...)
	container := labtest.StartContainer(t, labtest.Spec{
		Name: "flowexport-" + name, Image: exportCollectorImage, Dockerfile: exportCollectorDockerfile,
		Entrypoint: "sh", Command: command, Port: interoplab.UDPPort(port),
		ReadyInContainer: []string{"sh", "-c", exportCollectorBound(port)},
		ReadyTimeout:     30 * time.Second, LogLines: 50,
	})
	peer := &exportCollector{container: container}
	t.Cleanup(func() {
		if t.Failed() {
			t.Log(peer.diagnostics())
		}
	})
	return peer
}

// exportCollectorBound is the in-container readiness check: the collector
// recorded its pid and an IPv4 UDP socket is bound on port. A UDP port
// published through Docker accepts datagrams before the collector binds, so
// no probe from the host can observe readiness. The container's network
// namespace holds the collector alone, so the bound socket is the collector's.
func exportCollectorBound(port uint16) string {
	local := fmt.Sprintf(" 00000000:%04X 00000000:0000 07 ", port)
	return "test -s /tmp/collector.pid && grep -q '" + local + "' /proc/net/udp"
}

// exited answers the collector's exit status, or false while it runs.
func (p *exportCollector) exited(ctx context.Context) (string, bool, error) {
	result, err := p.container.Exec(ctx, "sh", "-c", "cat /tmp/collector.exit 2>/dev/null || true")
	if err != nil {
		return "", false, fmt.Errorf("read collector exit status: %w", err)
	}
	status := strings.TrimSpace(result.Stdout)
	return status, status != "", nil
}

// stop MUST be called before reading a collector's final output. It sends
// SIGTERM to the collector and waits three seconds for it to exit.
func (p *exportCollector) stop(ctx context.Context) error {
	if p.stopped {
		return nil
	}
	p.stopped = true
	status, exited, err := p.exited(ctx)
	if err != nil {
		return err
	}
	if exited {
		return fmt.Errorf("collector exited with status %s before requested shutdown", status)
	}
	if _, err := p.container.Exec(ctx, "sh", "-c", `kill -TERM "$(cat /tmp/collector.pid)"`); err != nil {
		return fmt.Errorf("terminate collector: %w", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status, exited, err := p.exited(ctx)
		if err != nil {
			return err
		}
		if exited {
			return exportCollectorShutdown(status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(exportPollInterval):
		}
	}
	return errors.New("collector did not exit within three seconds of SIGTERM; removing its container kills it")
}

// exportCollectorShutdown judges the status the container's shell recorded.
func exportCollectorShutdown(status string) error {
	if status == "0" {
		return nil
	}
	if status == "143" {
		return nil // 128+SIGTERM: sflowtool exits on the requested signal.
	}
	return fmt.Errorf("collector shutdown: exit status %s", status)
}

func (p *exportCollector) wait(t *testing.T, description string, timeout time.Duration, observe func(context.Context) (bool, error)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	ticker := time.NewTicker(exportPollInterval)
	defer ticker.Stop()
	// Every iteration observes the collector's status and its complete output.
	// The timer bounds polling; elapsed time never substitutes for the observed condition.
	for {
		status, exited, err := p.exited(ctx)
		if err != nil {
			t.Fatalf("observe %s: %v\n%s", description, err, p.diagnostics())
		}
		if exited {
			t.Fatalf("collector exited with status %s while waiting for %s\n%s", status, description, p.diagnostics())
		}
		ready, err := observe(ctx)
		if err != nil {
			t.Fatalf("observe %s: %v\n%s", description, err, p.diagnostics())
		}
		if ready {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s: %v\n%s", description, ctx.Err(), p.diagnostics())
		case <-ticker.C:
		}
	}
}

// read answers one file the collector wrote in its container.
func (p *exportCollector) read(ctx context.Context, path string) ([]byte, error) {
	readCtx, cancel := context.WithTimeout(ctx, exportReadTimeout)
	defer cancel()
	result, err := p.container.Exec(readCtx, "cat", path)
	if err != nil {
		return nil, fmt.Errorf("read collector %s: %w", path, err)
	}
	if len(result.Stdout) > exportOutputMax {
		return nil, fmt.Errorf("collector output %s exceeded 1 MiB", path)
	}
	return []byte(result.Stdout), nil
}

// diagnostics runs from a failed test and from its cleanup, after the test's
// context ends, so it reads under its own bound.
func (p *exportCollector) diagnostics() string {
	ctx, cancel := context.WithTimeout(context.Background(), exportReadTimeout)
	defer cancel()
	var out strings.Builder
	for _, path := range []string{"/tmp/collector.out", "/tmp/collector.err"} {
		data, err := p.read(ctx, path)
		fmt.Fprintf(&out, "%s (read error: %v):\n%s\n", path, err, data)
	}
	return out.String()
}

var exportNFCapdFile = regexp.MustCompile(`^nfcapd\.\d{12}(\d{2})?$`)

func exportNFDumpRecords(ctx context.Context, container labtest.Container, seen map[string]bool) ([]map[string]json.RawMessage, error) {
	listing, err := container.Exec(ctx, "ls", "-1", exportFlowDir)
	if err != nil {
		return nil, fmt.Errorf("list nfcapd files: %w", err)
	}
	var records []map[string]json.RawMessage
	for name := range strings.SplitSeq(strings.TrimSpace(listing.Stdout), "\n") {
		if seen[name] || !exportNFCapdFile.MatchString(name) {
			continue
		}
		readCtx, cancel := context.WithTimeout(ctx, exportReadTimeout)
		result, err := container.Exec(readCtx, "nfdump", "-C", "none", "-r", exportFlowDir+"/"+name, "-o", "json")
		cancel()
		if err != nil {
			return nil, fmt.Errorf("nfdump %s: %w", name, err)
		}
		data := []byte(result.Stdout)
		// nfdump 1.7.10 prints this non-JSON result for an empty rotation.
		// It contributes no records; the caller still requires all three.
		if string(bytes.TrimSpace(data)) == "[\nNo matching flows\n\n]" {
			seen[name] = true
			continue
		}
		var decoded []map[string]json.RawMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			return nil, fmt.Errorf("nfdump JSON from %s: %w\n%s\n%s", name, err, data, result.Stderr)
		}
		seen[name] = true
		records = append(records, decoded...)
	}
	return records, nil
}

func assertExportNFDumpRecords(t *testing.T, protocol string, records []map[string]json.RawMessage) {
	t.Helper()
	if len(records) != 3 {
		t.Fatalf("nfdump decoded %d records, want exactly 3: %v", len(records), records)
	}
	seen := make(map[string]bool)
	for _, record := range records {
		kind := "counter"
		if _, ok := record["src4_addr"]; ok {
			kind = "ipv4"
		}
		if _, ok := record["src6_addr"]; ok {
			kind = "ipv6"
		}
		if seen[kind] {
			t.Fatalf("duplicate nfdump %s record: %v", kind, records)
		}
		seen[kind] = true
		var numbers map[string]uint64
		switch kind {
		case "counter":
			numbers = map[string]uint64{"input_snmp": uint64(exportInteropIfIndex), "output_snmp": uint64(exportInteropIfIndex)}
			if protocol == "netflow9" {
				// NetFlow's implemented counter template exports unicast packets.
				numbers["in_bytes"], numbers["out_bytes"] = 4294967301, 8589934599
				numbers["in_packets"], numbers["out_packets"] = 101, 202
			} else {
				// IPFIX Total IEs combine full-width receive/transmit counters.
				numbers["in_bytes"], numbers["in_packets"] = 12884901900, 12884901912
			}
		case "ipv4":
			assertExportJSONAddress(t, record, "src4_addr", netip.MustParseAddr("192.0.2.1"))
			assertExportJSONAddress(t, record, "dst4_addr", netip.MustParseAddr("198.51.100.2"))
			numbers = map[string]uint64{
				"src_port": 12345, "dst_port": 443, "proto": 6,
				"in_bytes": 4294967329, "in_packets": 37, "src_as": 65551, "dst_as": 4200000001,
			}
		case "ipv6":
			assertExportJSONAddress(t, record, "src6_addr", netip.MustParseAddr("2001:db8:1::1"))
			assertExportJSONAddress(t, record, "dst6_addr", netip.MustParseAddr("2001:db8:2::2"))
			numbers = map[string]uint64{
				"src_port": 23456, "dst_port": 53, "proto": 17,
				"in_bytes": 8589934611, "in_packets": 73, "src_as": 65552, "dst_as": 4200000002,
			}
		}
		for key, want := range numbers {
			// Parsing the raw number rejects missing, null, rounded or quoted values.
			got, err := strconv.ParseUint(string(record[key]), 10, 64)
			if err != nil || got != want {
				t.Errorf("nfdump %s %s=%s (%v), want %d", kind, key, record[key], err, want)
			}
		}
	}
}

func assertExportJSONAddress(t *testing.T, record map[string]json.RawMessage, key string, want netip.Addr) {
	t.Helper()
	var text string
	if err := json.Unmarshal(record[key], &text); err != nil {
		t.Fatalf("nfdump %s=%s: %v", key, record[key], err)
	}
	got, err := netip.ParseAddr(text)
	if err != nil || got != want {
		t.Errorf("nfdump %s=%q (%v), want %s", key, text, err, want)
	}
}

func exportSFlowDatagrams(data []byte) ([]map[string]json.RawMessage, error) {
	var datagrams []map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var datagram map[string]json.RawMessage
		if err := decoder.Decode(&datagram); err != nil {
			if errors.Is(err, io.EOF) {
				return datagrams, nil
			}
			return nil, fmt.Errorf("sflowtool JSON: %w\n%s", err, data)
		}
		datagrams = append(datagrams, datagram)
	}
}

func assertExportSFlowDatagrams(t *testing.T, datagrams []map[string]json.RawMessage) {
	t.Helper()
	if len(datagrams) != 2 {
		t.Fatalf("sflowtool decoded %d datagrams, want exactly 2: %v", len(datagrams), datagrams)
	}
	index := strconv.FormatUint(uint64(exportInteropIfIndex), 10)
	output := strconv.FormatUint(uint64(exportInteropOutput), 10)
	seen := make(map[string]bool)
	for _, datagram := range datagrams {
		assertExportJSONStrings(t, datagram, map[string]string{
			"datagramVersion": "5", "agent": "192.0.2.9", "agentSubId": "7", "samplesInPacket": "1",
		})
		sample := exportJSONOnlyElement(t, datagram, "samples")
		element := exportJSONOnlyElement(t, sample, "elements")
		var kind string
		if err := json.Unmarshal(sample["sampleType"], &kind); err != nil {
			t.Fatalf("sflowtool sampleType: %v", err)
		}
		if seen[kind] {
			t.Fatalf("duplicate sflowtool sample type %q", kind)
		}
		seen[kind] = true
		assertExportJSONStrings(t, sample, map[string]string{"sourceId": "0:" + index, "sampleSequenceNo": "1"})
		switch kind {
		case "COUNTERSSAMPLE":
			assertExportJSONStrings(t, datagram, map[string]string{"packetSequenceNo": "0"})
			assertExportJSONStrings(t, sample, map[string]string{"sampleType_tag": "0:4"})
			assertExportJSONStrings(t, element, map[string]string{
				"counterBlock_tag": "0:1", "ifIndex": index, "networkType": "6", "ifSpeed": "1000000000",
				"ifDirection": "1", "ifStatus": "3", "ifInOctets": "4294967301", "ifOutOctets": "8589934599",
				"ifInUcastPkts": "101", "ifOutUcastPkts": "202", "ifInMulticastPkts": "17",
				"ifInBroadcastPkts": "4294967295", "ifInDiscards": "19", "ifInErrors": "23",
				"ifInUnknownProtos": "4294967295", "ifOutMulticastPkts": "4294967295", "ifOutBroadcastPkts": "4294967295",
				"ifOutDiscards": "29", "ifOutErrors": "31", "ifPromiscuousMode": "1",
			})
		case "FLOWSAMPLE":
			assertExportJSONStrings(t, datagram, map[string]string{"packetSequenceNo": "1"})
			assertExportJSONStrings(t, sample, map[string]string{
				"sampleType_tag": "0:3", "meanSkipCount": "64", "samplePool": "64", "dropEvents": "0",
				"inputPort": index, "outputPort": output,
			})
			assertExportJSONStrings(t, element, map[string]string{
				"flowBlock_tag": "0:1", "flowSampleType": "HEADER", "headerProtocol": "1",
				"sampledPacketSize": "64", "strippedBytes": "4", "headerLen": "60",
				"srcMAC": "020000000001", "dstMAC": "020000000002", "ethernet_type": "2048",
				"srcIP": "192.0.2.1", "dstIP": "198.51.100.2", "IPProtocol": "17", "IPSize": "46",
				"IPTTL": "64", "UDPSrcPort": "12345", "UDPDstPort": "53", "UDPBytes": "26",
			})
		default:
			t.Fatalf("unexpected sflowtool sample type %q", kind)
		}
	}
}

func exportJSONOnlyElement(t *testing.T, parent map[string]json.RawMessage, key string) map[string]json.RawMessage {
	t.Helper()
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(parent[key], &elements); err != nil {
		t.Fatalf("sflowtool %s=%s: %v", key, parent[key], err)
	}
	if len(elements) != 1 {
		t.Fatalf("sflowtool %s has %d elements, want exactly 1", key, len(elements))
	}
	return elements[0]
}

func assertExportJSONStrings(t *testing.T, record map[string]json.RawMessage, expected map[string]string) {
	t.Helper()
	// sflowtool quotes every scalar, including counters, to preserve all 64 bits.
	for key, want := range expected {
		var got string
		err := json.Unmarshal(record[key], &got)
		if err != nil || got != want {
			t.Errorf("sflowtool %s=%s (%v), want %q", key, record[key], err, want)
		}
	}
}
