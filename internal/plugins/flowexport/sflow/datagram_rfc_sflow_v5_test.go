// sFlow v5 datagram size bound (Section 4.3 sFlowRcvrMaximumDatagramSize) and
// the one-second send bound (Section 5), observed on the collector socket with
// the real sflow encoders and a real Sender.
//
// VALIDATES: a non-default max-datagram-size bounds every counter and flow
// datagram the collector receives, the Sender refuses a larger payload, and a
// flow sample reaches the collector within one second of EncodeFlowSample.
// PREVENTS: an encoder that ignores the configured bound, and a buffering
// encoder that holds a sample.

package sflow

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// newBoundedFlowTarget is newLoopbackFlowTarget with a chosen datagram bound.
func newBoundedFlowTarget(t *testing.T, maxDatagram int) (net.PacketConn, *flowexport.Sender) {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	s, err := flowexport.NewSender("127.0.0.1", addr.Port, "", maxDatagram)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return pc, s
}

// RFC requirement: SFLOW-V5-x-5 positive -- with max-datagram-size 600 (not the
// default), every counter datagram the collector receives for 30 interfaces is
// at most 600 octets and together they carry all 30 samples; a flow sample with
// a 1400-octet header also arrives in at most 600 octets; and the Sender
// refuses a 601-octet payload.
func TestSFlowV5ConfiguredMaxDatagramBoundsEveryDatagram(t *testing.T) {
	const maxDatagram = 600
	pc, s := newBoundedFlowTarget(t, maxDatagram)
	cfg := flowexport.CollectorConfig{Name: "c1", Protocol: "sflow", AgentAddress: "192.0.2.1"}

	ifs := make([]flowexport.InterfaceCounters, 30)
	for i := range ifs {
		ifs[i].IfIndex = uint32(i + 1)
	}
	if _, err := newSFlowEncoder(cfg, time.Now()).Encode(flowexport.CounterSnapshot{Time: time.Now(), Interfaces: ifs}, s); err != nil {
		t.Fatal(err)
	}
	samples := 0
	for samples < len(ifs) {
		dg := recvFlowDatagram(t, pc)
		if len(dg) > maxDatagram {
			t.Fatalf("counter datagram of %d octets over the %d bound", len(dg), maxDatagram)
		}
		samples += int(uint32(dg[24])<<24 | uint32(dg[25])<<16 | uint32(dg[26])<<8 | uint32(dg[27]))
	}
	if samples != len(ifs) {
		t.Fatalf("datagrams carried %d counter samples, want %d", samples, len(ifs))
	}

	sample := flowexport.FlowSample{IfIndex: 1, Rate: 100, OrigSize: 1500, Header: make([]byte, 1400)}
	if err := newSFlowFlowEncoder(cfg, time.Now()).EncodeFlowSample(sample, s); err != nil {
		t.Fatal(err)
	}
	if dg := recvFlowDatagram(t, pc); len(dg) > maxDatagram {
		t.Fatalf("flow datagram of %d octets over the %d bound", len(dg), maxDatagram)
	}

	if err := s.Send(make([]byte, maxDatagram+1)); err == nil {
		t.Fatalf("Sender accepted a %d-octet payload over the %d bound", maxDatagram+1, maxDatagram)
	}
}

// RFC requirement: SFLOW-V5-x-6 positive -- a flow sample handed to the real
// sflow flow encoder reaches the collector socket within one second of the
// call: the encoder sends before it returns and holds nothing.
func TestSFlowV5FlowSampleSentWithinOneSecond(t *testing.T) {
	pc, s := newBoundedFlowTarget(t, flowexport.DatagramSizeDefault)
	enc := newSFlowFlowEncoder(flowexport.CollectorConfig{Name: "c1", Protocol: "sflow", AgentAddress: "192.0.2.1"}, time.Now())

	start := time.Now()
	if err := enc.EncodeFlowSample(flowexport.FlowSample{IfIndex: 1, Rate: 100, OrigSize: 64, Header: make([]byte, 20)}, s); err != nil {
		t.Fatal(err)
	}
	if err := pc.SetReadDeadline(start.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 2048)
	if _, _, err := pc.ReadFrom(buf); err != nil {
		t.Fatalf("no datagram within one second of the sample: %v", err)
	}
}
