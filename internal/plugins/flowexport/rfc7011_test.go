// RFC 7011 conformance tests for the IPFIX exporter's UDP transport and its
// configurable Template retransmission interval. These requirements live in the
// exporter/config layer (exporter.go, config.go) rather than the ipfix encoder,
// so they are tested here in the flowexport package where the refresh timer and
// the config surface are reachable.
//
// VALIDATES: over UDP the exporter periodically retransmits its active Template
// at the configured template-refresh interval, and that interval is an
// operator-configurable, range-validated knob.
// PREVENTS: regressions where the template is never refreshed, is flooded every
// poll, or where the refresh interval stops being configurable/validated.

package flowexport

import (
	"testing"
	"time"
)

// TestRFC7011TemplateRetransmittedAtInterval drives notifySnapshot on an
// ipfix collector across a full template-refresh interval. newTestExporter opens
// a real loopback UDP socket, so the retransmit travels the UDP path
// (sender.go:82). countingEncoder records each EncodeTemplate call.
func TestRFC7011TemplateRetransmittedAtInterval(t *testing.T) {
	// RFC requirement: RFC7011-8-1 positive -- a snapshot one full template-refresh interval after the first re-sends the Template over the UDP sender (exporter.go:193-201), so templateCalls advances from 1 to 2
	exp := newTestExporter(t, "ipfix") // TemplateRefresh: 600s, PollingInterval: 1s
	enc := &countingEncoder{}
	exp.setEncoder("c1", enc)

	t0 := time.Now()
	exp.notifySnapshot(CounterSnapshot{Time: t0, Interfaces: []InterfaceCounters{{IfIndex: 1}}})
	if enc.templateCalls != 1 {
		t.Fatalf("first snapshot: templateCalls = %d, want 1 (a fresh collector always sends the Template)", enc.templateCalls)
	}

	exp.notifySnapshot(CounterSnapshot{Time: t0.Add(600 * time.Second), Interfaces: []InterfaceCounters{{IfIndex: 1}}})
	if enc.templateCalls != 2 {
		t.Fatalf("snapshot at +1 refresh interval: templateCalls = %d, want 2 (the Template is retransmitted)", enc.templateCalls)
	}
}

// TestRFC7011TemplateNotRetransmittedBeforeInterval verifies the exporter does not
// re-send the Template before its refresh interval elapses, so it does not flood
// the collector.
func TestRFC7011TemplateNotRetransmittedBeforeInterval(t *testing.T) {
	// RFC requirement: RFC7011-8-1 negative -- a snapshot arriving before the template-refresh interval elapses does not re-send the Template (exporter.go:193), so templateCalls stays at 1
	exp := newTestExporter(t, "ipfix") // TemplateRefresh: 600s, PollingInterval: 1s
	enc := &countingEncoder{}
	exp.setEncoder("c1", enc)

	t0 := time.Now()
	exp.notifySnapshot(CounterSnapshot{Time: t0, Interfaces: []InterfaceCounters{{IfIndex: 1}}})
	if enc.templateCalls != 1 {
		t.Fatalf("first snapshot: templateCalls = %d, want 1", enc.templateCalls)
	}

	// +2s clears the 1s poll gate but is far short of the 600s refresh interval.
	exp.notifySnapshot(CounterSnapshot{Time: t0.Add(2 * time.Second), Interfaces: []InterfaceCounters{{IfIndex: 1}}})
	if enc.templateCalls != 1 {
		t.Fatalf("snapshot at +2s: templateCalls = %d, want 1 (no retransmit before the refresh interval)", enc.templateCalls)
	}
}

// TestRFC7011TemplateRefreshConfigurable verifies an operator-supplied
// template-refresh flows through config parsing into the collector state.
func TestRFC7011TemplateRefreshConfigurable(t *testing.T) {
	// RFC requirement: RFC7011-8-2 positive -- an operator-supplied template-refresh is parsed into CollectorConfig.TemplateRefresh and accepted by Validate (config.go:288-290,372-374), so the retransmit interval is configurable
	data := `{"flow-export":{"collector":{"c1":{"address":"10.0.0.1","port":4739,"protocol":"ipfix","template-refresh":300}}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Collectors) != 1 {
		t.Fatalf("collectors = %d, want 1", len(cfg.Collectors))
	}
	if cfg.Collectors[0].TemplateRefresh != 300 {
		t.Fatalf("template-refresh = %d, want 300 (the operator value is honored)", cfg.Collectors[0].TemplateRefresh)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate rejected a valid template-refresh: %v", err)
	}
}

// TestRFC7011TemplateRefreshRangeRejected verifies an out-of-range
// template-refresh is rejected on the config surface.
func TestRFC7011TemplateRefreshRangeRejected(t *testing.T) {
	// RFC requirement: RFC7011-8-2 negative -- an out-of-range template-refresh (0 or 86401) is rejected by Validate (config.go:372-374), so a garbage interval cannot reach the exporter
	for _, refresh := range []int{0, 86401} {
		cfg := &Config{Collectors: []CollectorConfig{{
			Name: "c1", Address: "10.0.0.1", Port: 4739, Protocol: "ipfix",
			PollingInterval: 20, TemplateRefresh: refresh,
		}}}
		if err := cfg.Validate(); err == nil {
			t.Errorf("template-refresh %d: Validate accepted an out-of-range value", refresh)
		}
	}
}
