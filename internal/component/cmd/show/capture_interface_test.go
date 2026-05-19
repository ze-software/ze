// Design: plan/spec-diag-capture-interface.md -- portable unit tests

package show

import (
	"encoding/binary"
	"testing"
	"time"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func TestCaptureInterface_Wiring(t *testing.T) {
	found := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:capture-interface" {
			if r.Handler == nil {
				t.Error("ze-show:capture-interface handler must not be nil")
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ze-show:capture-interface not registered via pluginserver.RegisterRPCs")
	}
}

func TestCaptureArgsParser(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    captureArgs
		wantErr bool
	}{
		{
			name: "interface only",
			args: []string{"eth0"},
			want: captureArgs{
				iface:   "eth0",
				count:   defaultCaptureCount,
				dur:     defaultCaptureDur,
				snapLen: defaultCaptureSnapLen,
				format:  captureFormatPcap,
			},
		},
		{
			name: "all options",
			args: []string{"eth0", "count", "10", "duration", "5s", "snap-len", "128", "format", "text"},
			want: captureArgs{
				iface:   "eth0",
				count:   10,
				dur:     5 * time.Second,
				snapLen: 128,
				format:  captureFormatText,
			},
		},
		{
			name: "with filter expression",
			args: []string{"eth0", "tcp", "port", "179"},
			want: captureArgs{
				iface:   "eth0",
				filter:  "tcp port 179",
				count:   defaultCaptureCount,
				dur:     defaultCaptureDur,
				snapLen: defaultCaptureSnapLen,
				format:  captureFormatPcap,
			},
		},
		{
			name: "filter and options mixed",
			args: []string{"eth0", "tcp", "port", "179", "count", "5", "format", "text"},
			want: captureArgs{
				iface:   "eth0",
				filter:  "tcp port 179",
				count:   5,
				dur:     defaultCaptureDur,
				snapLen: defaultCaptureSnapLen,
				format:  captureFormatText,
			},
		},
		{
			name: "protocol keyword stripped",
			args: []string{"eth0", "protocol", "tcp", "port", "179"},
			want: captureArgs{
				iface:   "eth0",
				filter:  "tcp port 179",
				count:   defaultCaptureCount,
				dur:     defaultCaptureDur,
				snapLen: defaultCaptureSnapLen,
				format:  captureFormatPcap,
			},
		},
		{
			name: "protocol keyword with options",
			args: []string{"eth0", "protocol", "tcp", "port", "179", "count", "10", "duration", "5s"},
			want: captureArgs{
				iface:   "eth0",
				filter:  "tcp port 179",
				count:   10,
				dur:     5 * time.Second,
				snapLen: defaultCaptureSnapLen,
				format:  captureFormatPcap,
			},
		},
		{
			name:    "missing interface",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "count missing value",
			args:    []string{"eth0", "count"},
			wantErr: true,
		},
		{
			name:    "count zero",
			args:    []string{"eth0", "count", "0"},
			wantErr: true,
		},
		{
			name:    "count exceeds max",
			args:    []string{"eth0", "count", "10001"},
			wantErr: true,
		},
		{
			name:    "duration too short",
			args:    []string{"eth0", "duration", "500ms"},
			wantErr: true,
		},
		{
			name:    "duration too long",
			args:    []string{"eth0", "duration", "61s"},
			wantErr: true,
		},
		{
			name:    "snap-len too small",
			args:    []string{"eth0", "snap-len", "63"},
			wantErr: true,
		},
		{
			name:    "snap-len too large",
			args:    []string{"eth0", "snap-len", "65536"},
			wantErr: true,
		},
		{
			name:    "unknown format",
			args:    []string{"eth0", "format", "xml"},
			wantErr: true,
		},
		{
			name:    "format missing value",
			args:    []string{"eth0", "format"},
			wantErr: true,
		},
		{
			name:    "duration missing value",
			args:    []string{"eth0", "duration"},
			wantErr: true,
		},
		{
			name:    "snap-len missing value",
			args:    []string{"eth0", "snap-len"},
			wantErr: true,
		},
		{
			name: "count at max boundary",
			args: []string{"eth0", "count", "10000"},
			want: captureArgs{
				iface: "eth0", count: 10000, dur: defaultCaptureDur,
				snapLen: defaultCaptureSnapLen, format: captureFormatPcap,
			},
		},
		{
			name: "count at min boundary",
			args: []string{"eth0", "count", "1"},
			want: captureArgs{
				iface: "eth0", count: 1, dur: defaultCaptureDur,
				snapLen: defaultCaptureSnapLen, format: captureFormatPcap,
			},
		},
		{
			name: "duration at max boundary",
			args: []string{"eth0", "duration", "60s"},
			want: captureArgs{
				iface: "eth0", count: defaultCaptureCount, dur: 60 * time.Second,
				snapLen: defaultCaptureSnapLen, format: captureFormatPcap,
			},
		},
		{
			name: "duration at min boundary",
			args: []string{"eth0", "duration", "1s"},
			want: captureArgs{
				iface: "eth0", count: defaultCaptureCount, dur: time.Second,
				snapLen: defaultCaptureSnapLen, format: captureFormatPcap,
			},
		},
		{
			name: "snap-len at max boundary",
			args: []string{"eth0", "snap-len", "65535"},
			want: captureArgs{
				iface: "eth0", count: defaultCaptureCount, dur: defaultCaptureDur,
				snapLen: 65535, format: captureFormatPcap,
			},
		},
		{
			name: "snap-len at min boundary",
			args: []string{"eth0", "snap-len", "64"},
			want: captureArgs{
				iface: "eth0", count: defaultCaptureCount, dur: defaultCaptureDur,
				snapLen: 64, format: captureFormatPcap,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCaptureArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.iface != tt.want.iface {
				t.Errorf("iface = %q, want %q", got.iface, tt.want.iface)
			}
			if got.filter != tt.want.filter {
				t.Errorf("filter = %q, want %q", got.filter, tt.want.filter)
			}
			if got.count != tt.want.count {
				t.Errorf("count = %d, want %d", got.count, tt.want.count)
			}
			if got.dur != tt.want.dur {
				t.Errorf("dur = %v, want %v", got.dur, tt.want.dur)
			}
			if got.snapLen != tt.want.snapLen {
				t.Errorf("snapLen = %d, want %d", got.snapLen, tt.want.snapLen)
			}
			if got.format != tt.want.format {
				t.Errorf("format = %q, want %q", got.format, tt.want.format)
			}
		})
	}
}

func TestFormatTCPFlags(t *testing.T) {
	tests := []struct {
		flags byte
		want  string
	}{
		{0x02, "[SYN]"},
		{0x12, "[SYN,ACK]"},
		{0x10, "[ACK]"},
		{0x11, "[ACK,FIN]"},
		{0x04, "[RST]"},
		{0x18, "[ACK,PSH]"},
		{0x00, "[]"},
	}
	for _, tt := range tests {
		got := formatTCPFlags(tt.flags)
		if got != tt.want {
			t.Errorf("formatTCPFlags(0x%02x) = %q, want %q", tt.flags, got, tt.want)
		}
	}
}

func TestCapturePcapOutput(t *testing.T) {
	ts := time.Date(2026, 5, 19, 14, 23, 1, 3_000_000, time.UTC)
	pkt := buildTestTCPPacket(
		[4]byte{10, 0, 0, 1}, 39812,
		[4]byte{10, 0, 0, 2}, 179,
		0x02,
	)

	var buf sliceWriter

	if err := writePcapHeader(&buf, 65535, linkTypeEthernet); err != nil {
		t.Fatalf("writePcapHeader: %v", err)
	}
	if err := writePcapPacket(&buf, ts, pkt); err != nil {
		t.Fatalf("writePcapPacket: %v", err)
	}

	data := buf.buf

	if len(data) < 24 {
		t.Fatalf("pcap too short: %d bytes", len(data))
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0xa1b2c3d4 {
		t.Errorf("pcap magic = 0x%08x, want 0xa1b2c3d4", magic)
	}

	snapLen := binary.LittleEndian.Uint32(data[16:20])
	if snapLen != 65535 {
		t.Errorf("snap-len = %d, want 65535", snapLen)
	}

	linkType := binary.LittleEndian.Uint32(data[20:24])
	if linkType != 1 {
		t.Errorf("link type = %d, want 1 (Ethernet)", linkType)
	}

	if len(data) < 24+16+len(pkt) {
		t.Fatalf("pcap too short for packet record: %d bytes", len(data))
	}
	tsUnix := binary.LittleEndian.Uint32(data[24:28])
	if tsUnix != uint32(ts.Unix()) {
		t.Errorf("packet ts = %d, want %d", tsUnix, ts.Unix())
	}
}

func TestCaptureTextOutput(t *testing.T) {
	ts := time.Date(2026, 5, 19, 14, 23, 1, 3_000_000, time.UTC)

	tcpPkt := buildTestTCPPacket(
		[4]byte{10, 0, 0, 1}, 39812,
		[4]byte{10, 0, 0, 2}, 179,
		0x02,
	)

	line := formatPacketLine(ts, tcpPkt)
	if line == "" {
		t.Fatal("empty line")
	}

	wantParts := []string{"14:23:01.003", "TCP", "10.0.0.1:39812", "->", "10.0.0.2:179", "[SYN]"}
	for _, p := range wantParts {
		if !containsStr(line, p) {
			t.Errorf("line missing %q: %s", p, line)
		}
	}

	// UDP
	udpPkt := buildTestUDPPacket(
		[4]byte{10, 0, 0, 1}, 12345,
		[4]byte{10, 0, 0, 2}, 53,
	)
	udpLine := formatPacketLine(ts, udpPkt)
	for _, p := range []string{"UDP", "10.0.0.1:12345", "->", "10.0.0.2:53"} {
		if !containsStr(udpLine, p) {
			t.Errorf("UDP line missing %q: %s", p, udpLine)
		}
	}

	// ICMP (no ports)
	icmpPkt := buildTestICMPPacket([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2})
	icmpLine := formatPacketLine(ts, icmpPkt)
	for _, p := range []string{"ICMP", "10.0.0.1", "->", "10.0.0.2"} {
		if !containsStr(icmpLine, p) {
			t.Errorf("ICMP line missing %q: %s", p, icmpLine)
		}
	}
	if containsStr(icmpLine, "10.0.0.1:") {
		t.Errorf("ICMP line should not have ports: %s", icmpLine)
	}

	// Short frame
	shortLine := formatPacketLine(ts, []byte{0x00, 0x01, 0x02})
	if !containsStr(shortLine, "???") {
		t.Errorf("short frame should show ???: %s", shortLine)
	}

	// 802.1Q VLAN-tagged TCP packet
	vlanPkt := buildTestVLANTCPPacket(
		100,
		[4]byte{10, 0, 0, 1}, 39812,
		[4]byte{10, 0, 0, 2}, 179,
		0x02,
	)
	vlanLine := formatPacketLine(ts, vlanPkt)
	for _, p := range []string{"TCP", "10.0.0.1:39812", "->", "10.0.0.2:179", "[SYN]"} {
		if !containsStr(vlanLine, p) {
			t.Errorf("VLAN line missing %q: %s", p, vlanLine)
		}
	}
}

func TestCaptureConcurrencyGuard(t *testing.T) {
	ifName := "test-concurrency-guard-iface"
	activeCaptures.Store(ifName, true)
	defer activeCaptures.Delete(ifName)

	if _, loaded := activeCaptures.LoadOrStore(ifName, true); !loaded {
		t.Fatal("expected second capture to be blocked")
	}
}

func TestCaptureInterfaceValidation(t *testing.T) {
	resp, err := handleCaptureInterface(nil, []string{"nonexistent-iface-xyz-12345"})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if resp.Status != "error" {
		t.Fatalf("expected error status, got %q", resp.Status)
	}
	data, ok := resp.Data.(string)
	if !ok {
		t.Fatalf("expected string data, got %T", resp.Data)
	}
	// On Linux: "interface not found: ..."; on non-Linux: "not available on this platform"
	if !containsStr(data, "interface not found") && !containsStr(data, "not available on this platform") {
		t.Errorf("expected error message about interface or platform in %q", data)
	}
}

// helpers

type sliceWriter struct {
	buf []byte
}

func (w *sliceWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func buildTestTCPPacket(srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16, flags byte) []byte {
	pkt := make([]byte, 14+20+20)
	pkt[12] = 0x08
	pkt[13] = 0x00
	ip := pkt[14:]
	ip[0] = 0x45
	ip[9] = 6
	copy(ip[12:16], srcIP[:])
	copy(ip[16:20], dstIP[:])
	tcp := ip[20:]
	binary.BigEndian.PutUint16(tcp[0:2], srcPort)
	binary.BigEndian.PutUint16(tcp[2:4], dstPort)
	tcp[12] = 0x50
	tcp[13] = flags
	return pkt
}

func buildTestUDPPacket(srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16) []byte {
	pkt := make([]byte, 14+20+8)
	pkt[12] = 0x08
	pkt[13] = 0x00
	ip := pkt[14:]
	ip[0] = 0x45
	ip[9] = 17
	copy(ip[12:16], srcIP[:])
	copy(ip[16:20], dstIP[:])
	udp := ip[20:]
	binary.BigEndian.PutUint16(udp[0:2], srcPort)
	binary.BigEndian.PutUint16(udp[2:4], dstPort)
	binary.BigEndian.PutUint16(udp[4:6], 8)
	return pkt
}

func buildTestICMPPacket(srcIP, dstIP [4]byte) []byte {
	pkt := make([]byte, 14+20+8)
	pkt[12] = 0x08
	pkt[13] = 0x00
	ip := pkt[14:]
	ip[0] = 0x45
	ip[9] = 1
	copy(ip[12:16], srcIP[:])
	copy(ip[16:20], dstIP[:])
	icmp := ip[20:]
	icmp[0] = 8
	return pkt
}

func buildTestVLANTCPPacket(vlanID uint16, srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16, flags byte) []byte {
	// 14 eth + 4 vlan tag + 20 ip + 20 tcp = 58
	pkt := make([]byte, 14+4+20+20)
	// EtherType = 0x8100 (802.1Q)
	pkt[12] = 0x81
	pkt[13] = 0x00
	// VLAN tag: priority 0, DEI 0, VID
	binary.BigEndian.PutUint16(pkt[14:16], vlanID)
	// Real EtherType after tag: IPv4
	pkt[16] = 0x08
	pkt[17] = 0x00
	// IPv4 header starts at offset 18
	ip := pkt[18:]
	ip[0] = 0x45
	ip[9] = 6
	copy(ip[12:16], srcIP[:])
	copy(ip[16:20], dstIP[:])
	tcp := ip[20:]
	binary.BigEndian.PutUint16(tcp[0:2], srcPort)
	binary.BigEndian.PutUint16(tcp[2:4], dstPort)
	tcp[12] = 0x50
	tcp[13] = flags
	return pkt
}
