// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- sFlow sub-agent
// Related: register.go -- newSFlowEncoder and newSFlowFlowEncoder
//
// VALIDATES: the counter and flow encoders the registered factories build from
// one collector's config report every data source under that config's single
// sub-agent id, poll after poll.
// PREVENTS: a data source whose counters and flow samples travel under two
// sub-agents of one agent.

package sflow

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// RFC requirement: SFLOW-V5-x-31 positive -- for one collector configured with
// sub-agent 7, the counter and flow encoders built by the registered factories
// send data source 3's counter samples and flow samples, twice each, all with
// sub_agent_id 7.
func TestSFlowV5DataSourceKeepsOneSubAgent(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	cfg := flowexport.CollectorConfig{Protocol: "sflow", AgentAddress: "192.0.2.1", SubAgentID: 7}
	counter := newSFlowEncoder(cfg, time.Now())
	flow := newSFlowFlowEncoder(cfg, time.Now())

	for round := range 2 {
		snap := flowexport.CounterSnapshot{
			Time:       time.Now().Add(time.Duration(round) * time.Minute),
			Interfaces: []flowexport.InterfaceCounters{{IfIndex: 3, IfInOctets: uint64(1000 * (round + 1))}},
		}
		if _, err := counter.Encode(snap, s); err != nil {
			t.Fatal(err)
		}
		if got := binary.BigEndian.Uint32(recvFlowDatagram(t, pc)[12:]); got != 7 {
			t.Errorf("round %d: counter datagram sub_agent_id = %d, want 7", round, got)
		}
		if err := flow.EncodeFlowSample(flowexport.FlowSample{IfIndex: 3, Rate: 100, OrigSize: 64, Header: make([]byte, 20)}, s); err != nil {
			t.Fatal(err)
		}
		if got := binary.BigEndian.Uint32(recvFlowDatagram(t, pc)[12:]); got != 7 {
			t.Errorf("round %d: flow datagram sub_agent_id = %d, want 7", round, got)
		}
	}
}
