// sFlow v5 agent identity rules, checked by the flow-export config validator
// with the sflow package's collector validator registered.
//
// VALIDATES: every sflow collector names a specified agent-address, and all
// sflow collectors describe one agent: one agent-address and one sub-agent-id,
// so each data source is exported under one sub-agent of one agent.
// PREVENTS: a datagram header carrying 0.0.0.0 as the agent, and two
// collectors exporting the same interface under two sub-agents.

package sflow

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// sflowCollector returns a valid sflow collector config with the given name,
// agent address and sub-agent id.
func sflowCollector(name, agent string, subAgent uint32) flowexport.CollectorConfig {
	return flowexport.CollectorConfig{
		Name: name, Address: "10.0.0.1", Port: 6343, Protocol: "sflow",
		PollingInterval: 20, TemplateRefresh: 600, MaxDatagramSize: flowexport.DatagramSizeDefault,
		AgentAddress: agent, SubAgentID: subAgent,
	}
}

// validate runs the flow-export config validator over the collectors.
func validate(collectors ...flowexport.CollectorConfig) error {
	cfg := flowexport.Config{Collectors: collectors}
	return cfg.Validate()
}

// RFC requirement: SFLOW-V5-x-31 positive -- two sflow collectors with the
// same agent-address and the same sub-agent-id 7 are accepted, so every data
// source they export belongs to sub-agent 7.
// RFC requirement: SFLOW-V5-x-31 negative -- two sflow collectors with the
// same agent-address and sub-agent-id 7 and 8 would export each data source
// under two sub-agents; the config is refused, naming sub-agent-id.
func TestSFlowV5OneSubAgentPerDataSource(t *testing.T) {
	if err := validate(sflowCollector("a", "192.0.2.1", 7), sflowCollector("b", "192.0.2.1", 7)); err != nil {
		t.Fatalf("one sub-agent across collectors refused: %v", err)
	}
	err := validate(sflowCollector("a", "192.0.2.1", 7), sflowCollector("b", "192.0.2.1", 8))
	if err == nil {
		t.Fatal("sub-agent-id 7 and 8 on one agent accepted, want refusal")
	}
	if !strings.Contains(err.Error(), "sub-agent-id") {
		t.Fatalf("refusal does not name sub-agent-id: %v", err)
	}
}

// RFC requirement: SFLOW-V5-x-2 positive -- an sflow collector with agent-address
// 192.0.2.1 is accepted, and the counter encoder its factory builds writes
// 192.0.2.1 into every datagram header while the exported interfaces change
// (one interface, then three, then a different one).
// RFC requirement: SFLOW-V5-x-2 negative -- an agent address that cannot be a
// unique key for the agent is refused: agent-address left out,
// 0.0.0.0, ::, and two sflow collectors naming two agent addresses.
func TestSFlowV5AgentAddressIsTheAgentKey(t *testing.T) {
	if err := validate(sflowCollector("a", "192.0.2.1", 0)); err != nil {
		t.Fatalf("agent-address 192.0.2.1 refused: %v", err)
	}
	for _, tc := range []struct {
		name       string
		collectors []flowexport.CollectorConfig
	}{
		{"left out", []flowexport.CollectorConfig{sflowCollector("a", "", 0)}},
		{"0.0.0.0", []flowexport.CollectorConfig{sflowCollector("a", "0.0.0.0", 0)}},
		{"::", []flowexport.CollectorConfig{sflowCollector("a", "::", 0)}},
		{"two agents", []flowexport.CollectorConfig{sflowCollector("a", "192.0.2.1", 0), sflowCollector("b", "192.0.2.2", 0)}},
	} {
		err := validate(tc.collectors...)
		if err == nil {
			t.Errorf("%s: accepted, want refusal", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), "agent-address") {
			t.Errorf("%s: refusal does not name agent-address: %v", tc.name, err)
		}
	}

	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	counter := newSFlowEncoder(sflowCollector("a", "192.0.2.1", 0), time.Now())
	want := netip.MustParseAddr("192.0.2.1").As4()
	for round, ifs := range [][]flowexport.InterfaceCounters{
		{{IfIndex: 1}},
		{{IfIndex: 1}, {IfIndex: 2}, {IfIndex: 3}},
		{{IfIndex: 9}},
	} {
		snap := flowexport.CounterSnapshot{Time: time.Now().Add(time.Duration(round) * time.Minute), Interfaces: ifs}
		if _, err := counter.Encode(snap, s); err != nil {
			t.Fatal(err)
		}
		dg := recvFlowDatagram(t, pc)
		if got := [4]byte(dg[8:12]); got != want {
			t.Errorf("round %d: agent address %v, want 192.0.2.1", round, got)
		}
	}
}

// RFC requirement: SFLOW-V5-x-2 positive -- two sflow collectors that spell one
// agent address two ways (2001:db8::1 as 2001:DB8:0::1 and in full form) are
// accepted as one agent, while 2001:db8::1 and 2001:db8::2 are refused as two
// agents, naming agent-address.
func TestSFlowV5AgentAddressSpellingsNameOneAgent(t *testing.T) {
	for _, spelling := range []string{"2001:DB8:0::1", "2001:0db8:0000:0000:0000:0000:0000:0001"} {
		if err := validate(sflowCollector("a", "2001:db8::1", 0), sflowCollector("b", spelling, 0)); err != nil {
			t.Errorf("2001:db8::1 and %s are one agent address, refused: %v", spelling, err)
		}
	}
	err := validate(sflowCollector("a", "2001:db8::1", 0), sflowCollector("b", "2001:db8::2", 0))
	if err == nil {
		t.Fatal("2001:db8::1 and 2001:db8::2 accepted as one agent, want refusal")
	}
	if !strings.Contains(err.Error(), "agent-address") {
		t.Fatalf("refusal does not name agent-address: %v", err)
	}
}
