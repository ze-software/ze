package flowexport

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/observation"
	"github.com/ze-software/ze/internal/plugins/flowexport/enrich"
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
func newTestExporter(t *testing.T, protocol string) *exporter {
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
	exp, err := newExporter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(exp.stop)
	return exp
}

// orderRecordingEncoder records the order of EncodeTemplate / Encode calls so a
// test can assert the exporter never asks for a Data FlowSet before its Template.
type orderRecordingEncoder struct {
	events []string
}

func (e *orderRecordingEncoder) Encode(snap CounterSnapshot, _ *Sender) (int, error) {
	e.events = append(e.events, "data")
	return len(snap.Interfaces), nil
}

func (e *orderRecordingEncoder) EncodeTemplate(_ *Sender) error {
	e.events = append(e.events, "template")
	return nil
}

// TestExporterSendsTemplateBeforeData drives notifySnapshot on a fresh collector
// (lastTemplate is zero) and asserts the exporter emits the Template FlowSet
// before any Data FlowSet: a data FlowSet is never sent while no template has
// been sent, which is the guard at exporter.go:193-204.
// RFC requirement: RFC3954-x-1 negative -- on a fresh collector (lastTemplate zero) notifySnapshot sends the Template FlowSet before any Data FlowSet (exporter.go:193-204); the first emission the encoder is asked for is the template, so a data FlowSet is never emitted while no template has been sent.
func TestExporterSendsTemplateBeforeData(t *testing.T) {
	exp := newTestExporter(t, "netflow9")
	enc := &orderRecordingEncoder{}
	exp.setEncoder("c1", enc)

	exp.notifySnapshot(CounterSnapshot{
		Time:       time.Now(),
		Interfaces: []InterfaceCounters{{IfIndex: 1}},
	})

	if len(enc.events) == 0 {
		t.Fatal("encoder was never invoked; expected a template then data")
	}
	// The first thing the exporter emits on a fresh collector must be the template.
	if enc.events[0] != "template" {
		t.Fatalf("first emission was %q, want \"template\" (a Data FlowSet must never precede its Template)", enc.events[0])
	}
	// No data emission may appear before the first template emission.
	for i, ev := range enc.events {
		if ev == "template" {
			break
		}
		if ev == "data" {
			t.Fatalf("data FlowSet emitted at position %d before any template: %v", i, enc.events)
		}
	}
}

func TestExportFlowsAppliesEnrichment(t *testing.T) {
	exp := newTestExporter(t, "netflow9")

	stub := &stubFlowRecordEncoder{}
	exp.setFlowRecordEncoder("c1", stub)

	tree := enrich.NewRadixTree()
	tree.Insert(netip.MustParsePrefix("192.0.2.0/24"), enrich.ASEntry{AS: 64500})
	tree.Insert(netip.MustParsePrefix("203.0.113.0/24"), enrich.ASEntry{AS: 64600, NextHop: netip.MustParseAddr("10.0.0.2")})
	en := enrich.NewEnricher()
	en.UpdateTree(tree)
	exp.setEnricher(en)

	exp.exportFlows([]ConntrackFlow{{
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

// RFC requirement: SFLOW-V5-x-6 positive -- a sampled packet is not buffered: exportFlowSample encodes and dispatches it synchronously in the calling goroutine (exporter.go:242), so immediately after the call returns the encoder has already received the sample -- there is no holding queue that could delay it past 1 second.
func TestExportFlowSampleDispatch(t *testing.T) {
	exp := newTestExporter(t, "sflow")

	stub := &stubFlowSampleEncoder{}
	exp.setFlowSampleEncoder("c1", stub)

	exp.exportFlowSample(FlowSample{IfIndex: 5, Rate: 1024, OrigSize: 1500, Header: []byte{1, 2, 3}})

	if len(stub.got) != 1 {
		t.Fatalf("encoder received %d samples, want 1", len(stub.got))
	}
	if stub.got[0].IfIndex != 5 || stub.got[0].Rate != 1024 {
		t.Errorf("sample = %+v, want IfIndex=5 Rate=1024", stub.got[0])
	}
}

// TestExporterStopRunsStoppers verifies stop runs every registered stopper
// without deadlock (stoppers execute outside e.mu) and that dispatch is a
// no-op after stop.
func TestExporterStopRunsStoppers(t *testing.T) {
	exp := newTestExporter(t, "netflow9")

	done := make(chan struct{}, 3)
	for range 3 {
		exp.addStopper(func() { done <- struct{}{} })
	}

	exp.stop()

	for range 3 {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("a stopper did not run within 2s (possible deadlock)")
		}
	}

	// Dispatch after stop must be a no-op.
	stub := &stubFlowRecordEncoder{}
	exp.setFlowRecordEncoder("c1", stub)
	exp.exportFlows([]ConntrackFlow{{
		SrcAddr: netip.MustParseAddr("192.0.2.1"),
		DstAddr: netip.MustParseAddr("192.0.2.2"),
	}})
	if len(stub.got) != 0 {
		t.Errorf("exportFlows dispatched %d flows after stop, want 0", len(stub.got))
	}
}

// VALIDATES: AC-5 -- exportFlows publishes per-flow observations to the feed.
func TestConntrackPublishesFlowObs(t *testing.T) {
	exp := newTestExporter(t, "netflow9")
	stub := &stubFlowRecordEncoder{}
	exp.setFlowRecordEncoder("c1", stub)

	feed := observation.Global()
	var received []observation.Observation
	ch := make(chan struct{}, 1)
	subID := feed.Subscribe("test", func(obs observation.Observation) {
		received = append(received, obs)
		select {
		case ch <- struct{}{}:
		default:
		}
	})
	defer feed.Unsubscribe(subID)

	exp.exportFlows([]ConntrackFlow{{
		SrcAddr:  netip.MustParseAddr("192.0.2.1"),
		DstAddr:  netip.MustParseAddr("10.0.0.1"),
		SrcPort:  12345,
		DstPort:  80,
		Protocol: 6,
		Bytes:    9999,
	}})

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no observation received")
	}

	if len(received) == 0 {
		t.Fatal("no observations published")
	}
	obs := received[0]
	if obs.Kind != observation.KindFlow {
		t.Errorf("kind = %v, want Flow", obs.Kind)
	}
	if obs.Feature != observation.FeatureFlowBytes {
		t.Errorf("feature = %v, want FlowBytes", obs.Feature)
	}
	if obs.Value != 9999 {
		t.Errorf("value = %f, want 9999", obs.Value)
	}
	if obs.Flow.Src != netip.MustParseAddr("192.0.2.1") {
		t.Errorf("flow src = %v, want 192.0.2.1", obs.Flow.Src)
	}
	if obs.Flow.Dst != netip.MustParseAddr("10.0.0.1") {
		t.Errorf("flow dst = %v, want 10.0.0.1", obs.Flow.Dst)
	}
	if obs.Flow.DstPort != 80 {
		t.Errorf("flow dst port = %d, want 80", obs.Flow.DstPort)
	}
	if obs.Flow.Proto != 6 {
		t.Errorf("flow proto = %d, want 6", obs.Flow.Proto)
	}
}

// countingEncoder counts how many times Encode (a counter poll) is asked for, so
// a test can assert the poll-interval gate at exporter.go:186-189 admits or
// suppresses a snapshot.
type countingEncoder struct {
	encodeCalls   int
	templateCalls int
}

func (c *countingEncoder) Encode(snap CounterSnapshot, _ *Sender) (int, error) {
	c.encodeCalls++
	return len(snap.Interfaces), nil
}

func (c *countingEncoder) EncodeTemplate(_ *Sender) error {
	c.templateCalls++
	return nil
}

// RFC requirement: SFLOW-V5-x-14 positive -- with PollingInterval=1s, a second snapshot whose time is a full interval after the first passes the gate (now.Sub(lastPoll) < pollInterval is false at exactly one interval) and produces a second counter poll (exporter.go:186-189,213).
func TestSFlowCounterPollAtInterval(t *testing.T) {
	exp := newTestExporter(t, "sflow") // PollingInterval: 1 (second)
	enc := &countingEncoder{}
	exp.setEncoder("c1", enc)

	t0 := time.Now()
	exp.notifySnapshot(CounterSnapshot{Time: t0, Interfaces: []InterfaceCounters{{IfIndex: 1}}})
	if enc.encodeCalls != 1 {
		t.Fatalf("first snapshot: encodeCalls = %d, want 1 (fresh collector always polls)", enc.encodeCalls)
	}

	// One full polling interval later: the gate must admit the snapshot.
	exp.notifySnapshot(CounterSnapshot{Time: t0.Add(time.Second), Interfaces: []InterfaceCounters{{IfIndex: 1}}})
	if enc.encodeCalls != 2 {
		t.Fatalf("snapshot at +1 interval: encodeCalls = %d, want 2 (a counter poll is produced each polling interval)", enc.encodeCalls)
	}
}

// RFC requirement: SFLOW-V5-x-14 negative -- a snapshot arriving sooner than the configured PollingInterval is suppressed: with lastPoll set by the first poll, a snapshot 500ms later (< 1s) trips now.Sub(lastPoll) < pollInterval and the gate `continue`s, so no extra counter sample is produced (exporter.go:186-189).
func TestSFlowCounterPollBeforeInterval(t *testing.T) {
	exp := newTestExporter(t, "sflow") // PollingInterval: 1 (second)
	enc := &countingEncoder{}
	exp.setEncoder("c1", enc)

	t0 := time.Now()
	exp.notifySnapshot(CounterSnapshot{Time: t0, Interfaces: []InterfaceCounters{{IfIndex: 1}}})
	if enc.encodeCalls != 1 {
		t.Fatalf("first snapshot: encodeCalls = %d, want 1 (fresh collector always polls)", enc.encodeCalls)
	}

	// Well within the polling interval: the gate must suppress this snapshot.
	exp.notifySnapshot(CounterSnapshot{Time: t0.Add(500 * time.Millisecond), Interfaces: []InterfaceCounters{{IfIndex: 1}}})
	if enc.encodeCalls != 1 {
		t.Fatalf("snapshot at +500ms: encodeCalls = %d, want 1 (no poll before the polling interval elapses)", enc.encodeCalls)
	}
}
