package sflow

import (
	"context"
	"encoding/binary"
	"math"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// newLoopbackFlowTarget opens a loopback UDP listener and a Sender aimed at it,
// so a test can read back the exact datagram EncodeFlowSample transmitted.
func newLoopbackFlowTarget(t *testing.T) (net.PacketConn, *flowexport.Sender) {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	s, err := flowexport.NewSender("127.0.0.1", addr.Port, "", flowexport.DatagramSizeDefault)
	if err != nil {
		t.Fatal(err)
	}
	return pc, s
}

// recvFlowDatagram reads one datagram from pc with a short deadline.
func recvFlowDatagram(t *testing.T, pc net.PacketConn) []byte {
	t.Helper()
	_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	nr, _, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatalf("read datagram: %v", err)
	}
	return buf[:nr]
}

// samplePoolOffset is the flow_sample_expanded sample_pool field in an
// IPv4-agent datagram: 28-byte header + 8-byte sample tag/length + 16.
const samplePoolOffset = HeaderSizeIPv4 + 24

// RFC requirement: SFLOW-V5-x-11 positive -- sample_pool tracks the total packets seen by the data source: EncodeFlowSample reports pool == min(seq*rate, MaxUint32), so it grows by rate per exported sample and saturates instead of wrapping (flow_adapter.go:73-79). The decoded on-wire sample_pool confirms both the seq*rate value and the saturation.
func TestSFlowFlowSamplePoolTracksTotal(t *testing.T) {
	agent := netip.MustParseAddr("10.0.0.1")

	t.Run("grows by rate per sample", func(t *testing.T) {
		pc, s := newLoopbackFlowTarget(t)
		defer func() { _ = pc.Close() }()
		defer func() { _ = s.Close() }()

		enc := NewFlowEncoder(agent, 1, time.Unix(1716000000, 0))
		const rate = 1024

		for seq := uint32(1); seq <= 3; seq++ {
			if err := enc.EncodeFlowSample(flowexport.FlowSample{
				IfIndex: 7, Rate: rate, OrigSize: 128, Header: []byte{1, 2, 3, 4},
			}, s); err != nil {
				t.Fatal(err)
			}
			dg := recvFlowDatagram(t, pc)
			pool := binary.BigEndian.Uint32(dg[samplePoolOffset:])
			if want := seq * rate; pool != want {
				t.Errorf("sample %d: sample_pool = %d, want %d (seq*rate)", seq, pool, want)
			}
		}
	})

	t.Run("saturates rather than wraps", func(t *testing.T) {
		pc, s := newLoopbackFlowTarget(t)
		defer func() { _ = pc.Close() }()
		defer func() { _ = s.Close() }()

		enc := NewFlowEncoder(agent, 1, time.Unix(1716000000, 0))
		const rate = math.MaxUint32

		// seq=1 -> 1*MaxUint32 == MaxUint32 (exact). seq=2 -> 2*MaxUint32 would wrap a
		// uint32 to MaxUint32-1; the uint64 saturation must pin it at MaxUint32 instead.
		for seq := uint32(1); seq <= 2; seq++ {
			if err := enc.EncodeFlowSample(flowexport.FlowSample{
				IfIndex: 7, Rate: rate, OrigSize: 128, Header: []byte{1, 2, 3, 4},
			}, s); err != nil {
				t.Fatal(err)
			}
			dg := recvFlowDatagram(t, pc)
			pool := binary.BigEndian.Uint32(dg[samplePoolOffset:])
			if pool != math.MaxUint32 {
				t.Errorf("sample %d: sample_pool = %d, want %d (saturated MaxUint32, not wrapped)", seq, pool, uint32(math.MaxUint32))
			}
		}
	})
}

// TestSFlowExpandedFlowDatagramBound reads the transmitted datagram and checks
// that expanded metadata and a truncated capture fit both agent address headers.
func TestSFlowExpandedFlowDatagramBound(t *testing.T) {
	for _, agent := range []netip.Addr{
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("2001:db8::1"),
	} {
		t.Run(agent.String(), func(t *testing.T) {
			pc, sender := newLoopbackFlowTarget(t)
			defer func() { _ = pc.Close() }()
			defer func() { _ = sender.Close() }()
			capture := make([]byte, 1500)
			for i := range capture {
				capture[i] = byte(i)
			}
			enc := NewFlowEncoder(agent, 1, time.Unix(1716000000, 0))
			sample := flowexport.FlowSample{
				IfIndex: 0x01000000, Rate: 64, OrigSize: 1500,
				Output: 0x40000102, Header: capture,
			}
			if err := enc.EncodeFlowSample(sample, sender); err != nil {
				t.Fatal(err)
			}
			dg := recvFlowDatagram(t, pc)
			if len(dg) > sender.MaxDatagram() {
				t.Fatalf("datagram length = %d, limit %d", len(dg), sender.MaxDatagram())
			}
			headerSize := 28
			if agent.Is6() {
				headerSize = 40
			}
			record := dg[headerSize:]
			if got := binary.BigEndian.Uint32(record); got != 3 {
				t.Fatalf("sample format = %d, want 3", got)
			}
			if got := binary.BigEndian.Uint32(record[4:]); got != uint32(len(record)-8) {
				t.Errorf("sample length = %d, want %d", got, len(record)-8)
			}
			for _, offset := range []int{12, 32, 40} {
				if got := binary.BigEndian.Uint32(record[offset:]); got != 0 {
					t.Errorf("source/interface format at %d = %d, want 0", offset, got)
				}
			}
			if got := binary.BigEndian.Uint32(record[16:]); got != sample.IfIndex {
				t.Errorf("source index = %#x, want %#x", got, sample.IfIndex)
			}
			if got := binary.BigEndian.Uint32(record[36:]); got != sample.IfIndex {
				t.Errorf("input index = %#x, want %#x", got, sample.IfIndex)
			}
			if got := binary.BigEndian.Uint32(record[44:]); got != sample.Output {
				t.Errorf("output index = %#x, want %#x", got, sample.Output)
			}
			if got := binary.BigEndian.Uint32(record[48:]); got != 1 {
				t.Fatalf("flow record count = %d, want 1", got)
			}
			header := record[52:]
			if got := binary.BigEndian.Uint32(header); got != 1 {
				t.Fatalf("flow record format = %d, want sampled_header 1", got)
			}
			captured := int(binary.BigEndian.Uint32(header[20:]))
			wantCaptured := sender.MaxDatagram() - headerSize - 52 - 24 - 4
			if captured != wantCaptured {
				t.Fatalf("captured bytes = %d, want %d", captured, wantCaptured)
			}
			padded := (captured + 3) &^ 3
			if len(header) != 24+padded {
				t.Fatalf("sampled header size = %d, want %d", len(header), 24+padded)
			}
			for i := range captured {
				if header[24+i] != capture[i] {
					t.Fatalf("capture byte %d = %d, want %d", i, header[24+i], capture[i])
				}
			}
		})
	}
}
