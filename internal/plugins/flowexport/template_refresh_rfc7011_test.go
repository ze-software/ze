// Design: docs/architecture/flowexport/flow-export-0-umbrella.md -- template refresh
// Related: rfc7011_test.go -- the first counter Template retransmission
//
// VALIDATES: over UDP the exporter retransmits each active Template, the
// counter Template and the flow Templates alike, once for every elapsed
// template-refresh interval, and the interval it uses is the one configured.
// PREVENTS: a flow Template that is never refreshed, a single refresh that is
// not repeated, and a refresh interval hard-coded to the 600 s default.

package flowexport

import (
	"net/netip"
	"testing"
	"testing/synctest"
	"time"
)

// flowTemplateCounter is a FlowRecordEncoder that counts flow Template sends.
type flowTemplateCounter struct {
	templateCalls int
}

func (c *flowTemplateCounter) EncodeFlows(flows []ConntrackFlow, _ *Sender) (int, error) {
	return len(flows), nil
}

func (c *flowTemplateCounter) EncodeFlowTemplate(_ *Sender) error {
	c.templateCalls++
	return nil
}

func oneConntrackFlow() []ConntrackFlow {
	return []ConntrackFlow{{
		SrcAddr:  netip.MustParseAddr("192.0.2.1"),
		DstAddr:  netip.MustParseAddr("198.51.100.1"),
		Protocol: 6,
		Bytes:    100,
		Packets:  2,
	}}
}

// RFC requirement: RFC7011-8-1 positive -- on a UDP collector both active
// Templates, the counter Template and the flow Templates, are retransmitted at
// every elapsed 600 s refresh interval: over three intervals each is sent at
// 0, 600, 1200 and 1800 s, four times, and never at the half-interval exports.
func TestRFC7011EachActiveTemplateRetransmittedEveryInterval(t *testing.T) {
	exp := newTestExporter(t, "ipfix") // TemplateRefresh: 600s, PollingInterval: 1s, UDP.
	counter := &countingEncoder{}
	flow := &flowTemplateCounter{}
	exp.setEncoder("c1", counter)
	exp.setFlowRecordEncoder("c1", flow)

	t0 := time.Now()
	for half := range 7 {
		at := time.Duration(half) * 300 * time.Second
		exp.notifySnapshot(CounterSnapshot{Time: t0.Add(at), Interfaces: []InterfaceCounters{{IfIndex: 1}}})
		if want := half/2 + 1; counter.templateCalls != want {
			t.Fatalf("counter Template sends at +%v = %d, want %d", at, counter.templateCalls, want)
		}
	}

	// exportFlows reads time.Now; synctest's clock moves only on Sleep, so
	// the flow Template's refresh is driven over the same half intervals.
	synctest.Test(t, func(t *testing.T) {
		for half := range 7 {
			exp.exportFlows(oneConntrackFlow())
			if want := half/2 + 1; flow.templateCalls != want {
				t.Fatalf("flow Template sends after %d half intervals = %d, want %d", half, flow.templateCalls, want)
			}
			time.Sleep(300 * time.Second)
		}
	})
}

// RFC requirement: RFC7011-8-2 positive -- the configured template-refresh sets
// the retransmission interval: with 30 s the counter and flow Templates are
// resent at 30 s and not at 29 s after the previous send.
func TestRFC7011ConfiguredRefreshDrivesRetransmit(t *testing.T) {
	exp := newTestExporter(t, "ipfix")
	exp.collectors[0].cfg.TemplateRefresh = 30
	counter := &countingEncoder{}
	flow := &flowTemplateCounter{}
	exp.setEncoder("c1", counter)
	exp.setFlowRecordEncoder("c1", flow)

	t0 := time.Now()
	for _, step := range []struct {
		at   time.Duration
		want int
	}{{0, 1}, {29 * time.Second, 1}, {30 * time.Second, 2}, {59 * time.Second, 2}, {60 * time.Second, 3}} {
		exp.notifySnapshot(CounterSnapshot{Time: t0.Add(step.at), Interfaces: []InterfaceCounters{{IfIndex: 1}}})
		if counter.templateCalls != step.want {
			t.Fatalf("counter Template sends at +%v = %d, want %d", step.at, counter.templateCalls, step.want)
		}
	}

	synctest.Test(t, func(t *testing.T) {
		for _, want := range []int{1, 1, 2} {
			exp.exportFlows(oneConntrackFlow())
			if flow.templateCalls != want {
				t.Fatalf("flow Template sends = %d, want %d", flow.templateCalls, want)
			}
			time.Sleep(15 * time.Second)
		}
	})
}

// RFC requirement: RFC7011-8-2 negative -- a configured template-refresh of 900 s
// replaces the 600 s default: no Template is resent at 600 s or at 899 s, and
// the retransmission happens at 900 s.
func TestRFC7011ConfiguredRefreshOverridesDefault(t *testing.T) {
	exp := newTestExporter(t, "ipfix")
	exp.collectors[0].cfg.TemplateRefresh = 900
	counter := &countingEncoder{}
	flow := &flowTemplateCounter{}
	exp.setEncoder("c1", counter)
	exp.setFlowRecordEncoder("c1", flow)

	t0 := time.Now()
	for _, step := range []struct {
		at   time.Duration
		want int
	}{{0, 1}, {600 * time.Second, 1}, {899 * time.Second, 1}, {900 * time.Second, 2}} {
		exp.notifySnapshot(CounterSnapshot{Time: t0.Add(step.at), Interfaces: []InterfaceCounters{{IfIndex: 1}}})
		if counter.templateCalls != step.want {
			t.Fatalf("counter Template sends at +%v = %d, want %d", step.at, counter.templateCalls, step.want)
		}
	}

	synctest.Test(t, func(t *testing.T) {
		exp.exportFlows(oneConntrackFlow())
		time.Sleep(600 * time.Second)
		exp.exportFlows(oneConntrackFlow())
		if flow.templateCalls != 1 {
			t.Fatalf("flow Template sends at +600s with a 900s refresh = %d, want 1", flow.templateCalls)
		}
		time.Sleep(300 * time.Second)
		exp.exportFlows(oneConntrackFlow())
		if flow.templateCalls != 2 {
			t.Fatalf("flow Template sends at +900s = %d, want 2", flow.templateCalls)
		}
	})
}
