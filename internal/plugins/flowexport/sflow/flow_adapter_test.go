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
	s, err := flowexport.NewSender("127.0.0.1", addr.Port, "")
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

// samplePoolOffset is the byte offset of the flow_sample sample_pool field in an
// IPv4-agent datagram: 28-byte header + flow_sample data_format(4) +
// sample_length(4) + seqNum(4) + source_id(4) + sampling_rate(4) = 48.
const samplePoolOffset = HeaderSizeIPv4 + 20

// RFC requirement: SFLOW-V5-x-11 positive -- sample_pool tracks the total packets seen by the data source: EncodeFlowSample reports pool == min(seq*rate, MaxUint32), so it grows by rate per exported sample and saturates instead of wrapping (flow_adapter.go:73-79). The decoded on-wire sample_pool confirms both the seq*rate value and the saturation.
func TestSFlowFlowSamplePoolTracksTotal(t *testing.T) {
	agent := netip.MustParseAddr("10.0.0.1") // IPv4 -> 28-byte header, pool at offset 48

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
