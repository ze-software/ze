package flowexport

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/flowexport/enrich"
)

type stubFlowRecordEncoder struct {
	got           []ConntrackFlow
	templateCalls int
}

func (s *stubFlowRecordEncoder) EncodeFlows(flows []ConntrackFlow, _ *Sender) (int, error) {
	s.got = append(s.got, flows...)
	return len(flows), nil
}

func (s *stubFlowRecordEncoder) EncodeFlowTemplate(_ *Sender) error {
	s.templateCalls++
	return nil
}

type stubFlowSampleEncoder struct {
	got []FlowSample
}

func (s *stubFlowSampleEncoder) EncodeFlowSample(sample FlowSample, _ *Sender) error {
	s.got = append(s.got, sample)
	return nil
}

// newTestExporter builds an exporter with one collector pointing at a live
// loopback UDP socket (kept open so sends do not draw ICMP errors). Teardown
// is registered via t.Cleanup (exporter stopped before the socket closes).
func newTestExporter(t *testing.T, protocol string) *Exporter {
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
	cfg := &Config{Collectors: []CollectorConfig{{
		Name: "c1", Address: "127.0.0.1", Port: addr.Port,
		Protocol: protocol, PollingInterval: 1, TemplateRefresh: 600,
	}}}
	exp, err := NewExporter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(exp.Stop)
	return exp
}

func TestExportFlowsAppliesEnrichment(t *testing.T) {
	exp := newTestExporter(t, "netflow9")

	stub := &stubFlowRecordEncoder{}
	exp.SetFlowRecordEncoder("c1", stub)

	tree := enrich.NewRadixTree()
	tree.Insert(netip.MustParsePrefix("192.0.2.0/24"), enrich.ASEntry{AS: 64500})
	tree.Insert(netip.MustParsePrefix("203.0.113.0/24"), enrich.ASEntry{AS: 64600, NextHop: netip.MustParseAddr("10.0.0.2")})
	en := enrich.NewEnricher()
	en.UpdateTree(tree)
	exp.SetEnricher(en)

	exp.ExportFlows([]ConntrackFlow{{
		SrcAddr: netip.MustParseAddr("192.0.2.5"),
		DstAddr: netip.MustParseAddr("203.0.113.9"),
	}})

	if len(stub.got) != 1 {
		t.Fatalf("encoder received %d flows, want 1", len(stub.got))
	}
	f := stub.got[0]
	if f.SrcAS != 64500 {
		t.Errorf("SrcAS = %d, want 64500", f.SrcAS)
	}
	if f.DstAS != 64600 {
		t.Errorf("DstAS = %d, want 64600", f.DstAS)
	}
	if f.NextHop != netip.MustParseAddr("10.0.0.2") {
		t.Errorf("NextHop = %v, want 10.0.0.2", f.NextHop)
	}
}

func TestExportFlowSampleDispatch(t *testing.T) {
	exp := newTestExporter(t, "sflow")

	stub := &stubFlowSampleEncoder{}
	exp.SetFlowSampleEncoder("c1", stub)

	exp.ExportFlowSample(FlowSample{IfIndex: 5, Rate: 1024, OrigSize: 1500, Header: []byte{1, 2, 3}})

	if len(stub.got) != 1 {
		t.Fatalf("encoder received %d samples, want 1", len(stub.got))
	}
	if stub.got[0].IfIndex != 5 || stub.got[0].Rate != 1024 {
		t.Errorf("sample = %+v, want IfIndex=5 Rate=1024", stub.got[0])
	}
}

// TestExporterStopRunsStoppers verifies Stop runs every registered stopper
// without deadlock (stoppers execute outside e.mu) and that dispatch is a
// no-op after Stop.
func TestExporterStopRunsStoppers(t *testing.T) {
	exp := newTestExporter(t, "netflow9")

	done := make(chan struct{}, 3)
	for range 3 {
		exp.AddStopper(func() { done <- struct{}{} })
	}

	exp.Stop()

	for range 3 {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("a stopper did not run within 2s (possible deadlock)")
		}
	}

	// Dispatch after Stop must be a no-op.
	stub := &stubFlowRecordEncoder{}
	exp.SetFlowRecordEncoder("c1", stub)
	exp.ExportFlows([]ConntrackFlow{{
		SrcAddr: netip.MustParseAddr("192.0.2.1"),
		DstAddr: netip.MustParseAddr("192.0.2.2"),
	}})
	if len(stub.got) != 0 {
		t.Errorf("ExportFlows dispatched %d flows after Stop, want 0", len(stub.got))
	}
}
