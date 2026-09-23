//go:build integration && linux

// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- independent collector interpretation.
// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- decoded flow records and sampled packets.
//
// These tests opt in with -flowexport-peer-dir pointing at the pinned peer tools.
// test/interop-flowexport/Dockerfile.collectors pins the upstream implementations.
// They exercise public encoders and Sender over UDP, without daemon ingestion,
// network namespaces, privileges, or a Ze decoder acting as the oracle.
package flowexport_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

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

var exportInteropPeerDir = flag.String("flowexport-peer-dir", "", "Directory containing pinned nfcapd, nfdump and sflowtool for collector interop")

// MUTATION: a wrong template IE/width, truncated counter, swapped address or port,
// or lost IPv6/ASN field changes nfdump's decoded records or prevents collection.
func TestFlowExportNFDumpInterop(t *testing.T) {
	nfcapd := exportInteropTool(t, "nfcapd")
	nfdump := exportInteropTool(t, "nfdump")
	for _, protocol := range []string{"netflow9", "ipfix"} {
		t.Run(protocol, func(t *testing.T) {
			flowDir := t.TempDir()
			port := exportInteropPort(t)
			peer := startExportCollector(t, nfcapd, "-4", "-b", "127.0.0.1", "-p", strconv.Itoa(port), "-w", flowDir, "-t", "2", "-C", "none")
			peer.wait(t, "owned UDP listener", 5*time.Second, func(context.Context) (bool, error) {
				return exportCollectorListening(peer.cmd.Process.Pid, port)
			})
			sender := exportInteropSender(t, port)
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
				added, err := exportNFDumpRecords(ctx, nfdump, flowDir, seenFiles)
				records = append(records, added...)
				return len(records) >= 3, err
			}
			peer.wait(t, "three nfdump records in completed files", 15*time.Second, collect)
			if err := peer.stop(); err != nil {
				t.Fatalf("stop nfcapd: %v\n%s", err, peer.diagnostics())
			}
			// Include the final rotation, so shutdown cannot hide extra records.
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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
	tool := exportInteropTool(t, "sflowtool")
	port := exportInteropPort(t)
	peer := startExportCollector(t, tool, "-4", "-b", "127.0.0.1", "-p", strconv.Itoa(port), "-j")
	peer.wait(t, "owned UDP listener", 5*time.Second, func(context.Context) (bool, error) {
		return exportCollectorListening(peer.cmd.Process.Pid, port)
	})
	sender := exportInteropSender(t, port)
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
	peer.wait(t, "two complete sflowtool JSON datagrams", 10*time.Second, func(context.Context) (bool, error) {
		data, err := exportCollectorOutput(peer.stdout.Name())
		if err != nil {
			return false, err
		}
		// -j flushes one JSON line per datagram. A partial last line is still
		// being written; a malformed complete line is a failure immediately.
		complete := data[:bytes.LastIndexByte(data, '\n')+1]
		datagrams, err := exportSFlowDatagrams(complete)
		return len(datagrams) >= 2, err
	})
	if err := peer.stop(); err != nil {
		t.Fatalf("stop sflowtool: %v\n%s", err, peer.diagnostics())
	}
	data, err := exportCollectorOutput(peer.stdout.Name())
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

func exportInteropTool(t *testing.T, name string) string {
	t.Helper()
	if *exportInteropPeerDir == "" {
		t.Skip("collector interop requires explicit -flowexport-peer-dir; an unprovisioned skip is not interoperability evidence")
	}
	directory, err := filepath.Abs(*exportInteropPeerDir)
	if err != nil {
		t.Fatalf("resolve -flowexport-peer-dir: %v", err)
	}
	path, err := exec.LookPath(filepath.Join(directory, name))
	if err != nil {
		t.Fatalf("explicit collector interop requires %s in %s: %v; build test/interop-flowexport/Dockerfile.collectors", name, directory, err)
	}
	return path
}

func exportInteropPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("allocate collector UDP port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	if err := conn.Close(); err != nil {
		t.Fatalf("release collector UDP port: %v", err)
	}
	return port
}

func exportInteropSender(t *testing.T, port int) *flowexport.Sender {
	t.Helper()
	sender, err := flowexport.NewSender("127.0.0.1", port, "", flowexport.DatagramSizeDefault)
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

// exportCollector owns exactly one foreground child and its Wait goroutine.
// startExportProcess MUST register cleanup; stop MUST terminate and reap it.
// File-backed output captures bytes before the stimulus and permits concurrent
// observation without racing an exec writer against a bytes.Buffer reader.
type exportCollector struct {
	cmd     *exec.Cmd
	done    chan struct{}
	waitErr error // read only after done closes
	cancel  context.CancelFunc
	stdout  *os.File
	stderr  *os.File
	stopped bool
}

func startExportCollector(t *testing.T, tool string, args ...string) *exportCollector {
	t.Helper()
	// Cleanup must still work after testing cancels t.Context().
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = append(os.Environ(), "TZ=UTC")
	return startExportProcess(t, cmd, cancel)
}

func startExportProcess(t *testing.T, cmd *exec.Cmd, cancel context.CancelFunc) *exportCollector {
	t.Helper()
	t.Cleanup(cancel)
	dir := t.TempDir()
	stdout, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stdout.Close(); err != nil {
			t.Errorf("close collector stdout: %v", err)
		}
	})
	stderr, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stderr.Close(); err != nil {
			t.Errorf("close collector stderr: %v", err)
		}
	})
	cmd.Stdout, cmd.Stderr = stdout, stderr
	peer := &exportCollector{cmd: cmd, done: make(chan struct{}), cancel: cancel, stdout: stdout, stderr: stderr}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start %s: %v", cmd.Path, err)
	}
	go func() {
		peer.waitErr = cmd.Wait()
		close(peer.done)
	}()
	t.Cleanup(func() {
		if err := peer.stop(); err != nil {
			t.Errorf("collector cleanup: %v", err)
		}
		if t.Failed() {
			t.Log(peer.diagnostics())
		}
	})
	return peer
}

// stop MUST be called for every started collector, including failed tests.
func (p *exportCollector) stop() error {
	if p.stopped {
		return nil
	}
	p.stopped = true
	defer p.cancel()
	select {
	case <-p.done:
		return fmt.Errorf("%s exited before requested shutdown: %v", p.cmd.Path, p.waitErr)
	default:
	}
	signalErr := p.cmd.Process.Signal(syscall.SIGTERM)
	if signalErr != nil {
		p.cancel()
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-p.done:
		if signalErr != nil {
			return fmt.Errorf("terminate collector: %w", signalErr)
		}
		if p.waitErr == nil {
			return nil
		}
		var exit *exec.ExitError
		if errors.As(p.waitErr, &exit) {
			if status, ok := exit.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() && status.Signal() == syscall.SIGTERM {
					return nil // sflowtool exits on the requested signal.
				}
			}
		}
		return fmt.Errorf("collector shutdown: %w", p.waitErr)
	case <-timer.C:
		p.cancel() // CommandContext kills only this owned process.
	}
	timer.Reset(3 * time.Second)
	select {
	case <-p.done:
		return errors.New("collector did not exit within three seconds of SIGTERM; killed and reaped")
	case <-timer.C:
		return errors.New("collector did not exit after kill")
	}
}

func (p *exportCollector) wait(t *testing.T, description string, timeout time.Duration, observe func(context.Context) (bool, error)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	// Every iteration observes a socket or complete collector output. The timer
	// bounds polling; elapsed time never substitutes for the observed condition.
	for {
		select {
		case <-p.done:
			t.Fatalf("collector exited while waiting for %s: %v\n%s", description, p.waitErr, p.diagnostics())
		default:
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
		case <-p.done:
			t.Fatalf("collector exited while waiting for %s: %v\n%s", description, p.waitErr, p.diagnostics())
		case <-ticker.C:
		}
	}
}

func exportCollectorListening(pid, port int) (bool, error) {
	proc := filepath.Join("/proc", strconv.Itoa(pid))
	fds, err := os.ReadDir(filepath.Join(proc, "fd"))
	if err != nil {
		return false, fmt.Errorf("read collector descriptors: %w", err)
	}
	inodes := make(map[string]bool)
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(proc, "fd", fd.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue // A descriptor may close during the procfs snapshot.
		}
		if err != nil {
			return false, fmt.Errorf("read collector descriptor %s: %w", fd.Name(), err)
		}
		if inode, ok := strings.CutPrefix(target, "socket:["); ok {
			inodes[strings.TrimSuffix(inode, "]")] = true
		}
	}
	table, err := os.ReadFile(filepath.Join(proc, "net", "udp"))
	if err != nil {
		return false, fmt.Errorf("read collector UDP table: %w", err)
	}
	local := fmt.Sprintf("%08X:%04X", binary.NativeEndian.Uint32([]byte{127, 0, 0, 1}), port)
	// /proc/PID/net/udp is namespace-wide. The fd inode intersection is what
	// proves this child owns the socket rather than another concurrent test.
	for _, line := range strings.Split(string(table), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		if fields[1] == local && inodes[fields[9]] {
			return true, nil
		}
	}
	return false, nil
}

func exportCollectorOutput(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("collector output %s exceeded 1 MiB", path)
	}
	return data, nil
}

func (p *exportCollector) diagnostics() string {
	var out strings.Builder
	for _, file := range []*os.File{p.stdout, p.stderr} {
		data, err := exportCollectorOutput(file.Name())
		fmt.Fprintf(&out, "%s (read error: %v):\n%s\n", filepath.Base(file.Name()), err, data)
	}
	return out.String()
}

var exportNFCapdFile = regexp.MustCompile(`^nfcapd\.[0-9]{12}([0-9]{2})?$`)

func exportNFDumpRecords(ctx context.Context, nfdump, dir string, seen map[string]bool) ([]map[string]json.RawMessage, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var records []map[string]json.RawMessage
	for _, file := range files {
		if file.IsDir() || seen[file.Name()] || !exportNFCapdFile.MatchString(file.Name()) {
			continue
		}
		readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cmd := exec.CommandContext(readCtx, nfdump, "-C", "none", "-r", filepath.Join(dir, file.Name()), "-o", "json")
		cmd.Env = append(os.Environ(), "TZ=UTC")
		data, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("nfdump %s: %w\n%s", file.Name(), err, data)
		}
		// nfdump 1.7.10 prints this non-JSON result for an empty rotation.
		// It contributes no records; the caller still requires all three.
		if string(bytes.TrimSpace(data)) == "[\nNo matching flows\n\n]" {
			seen[file.Name()] = true
			continue
		}
		var decoded []map[string]json.RawMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			return nil, fmt.Errorf("nfdump JSON from %s: %w\n%s", file.Name(), err, data)
		}
		seen[file.Name()] = true
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
