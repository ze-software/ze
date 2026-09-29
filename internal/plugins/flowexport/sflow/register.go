// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- sFlow encoder registration

package sflow

import (
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func init() {
	flowexport.RegisterEncoderFactory("sflow", newSFlowEncoder)
	flowexport.RegisterFlowSampleEncoderFactory("sflow", newSFlowFlowEncoder)
	flowexport.RegisterCollectorsValidator("sflow", validateSFlowCollectors)
}

func newSFlowEncoder(cfg flowexport.CollectorConfig, startTime time.Time) flowexport.ProtocolEncoder {
	return NewCounterEncoder(agentAddress(cfg), cfg.SubAgentID, startTime)
}

// agentAddress answers the collector's validated agent address.
// validateSFlowCollectors refuses a config without one before any factory
// runs, so a failure here is a Ze defect.
func agentAddress(cfg flowexport.CollectorConfig) netip.Addr {
	addr, err := netip.ParseAddr(cfg.AgentAddress)
	if err != nil {
		panic("BUG: sflow collector " + cfg.Name + " reached its encoder factory without a validated agent-address")
	}
	return addr
}

// validateSFlowCollectors checks that the sflow collectors describe one agent.
// Every sflow collector exports every data source, so the agent address and
// the sub-agent id must be the same on all of them.
func validateSFlowCollectors(collectors []flowexport.CollectorConfig) error {
	var errs []error
	for i := range collectors {
		if err := validateAgentAddress(collectors[i].AgentAddress); err != nil {
			errs = append(errs, fmt.Errorf("collector %q: %w", collectors[i].Name, err))
		}
	}
	first := &collectors[0]
	for i := 1; i < len(collectors); i++ {
		c := &collectors[i]
		// sFlow v5 Section 4.3: "A manager should be able to use the
		// sFlowAgentAddress as a unique key that will identify this agent".
		if c.AgentAddress != first.AgentAddress {
			errs = append(errs, fmt.Errorf("collector %q: agent-address %q differs from %q on collector %q: Ze is one sFlow agent",
				c.Name, c.AgentAddress, first.AgentAddress, first.Name))
		}
		// sFlow v5 Section 5: "Each sFlowDataSource must be associated with
		// only one sub-agent."
		if c.SubAgentID != first.SubAgentID {
			errs = append(errs, fmt.Errorf("collector %q: sub-agent-id %d differs from %d on collector %q: each interface is exported under one sub-agent",
				c.Name, c.SubAgentID, first.SubAgentID, first.Name))
		}
	}
	return errors.Join(errs...)
}

// validateAgentAddress refuses an agent address that cannot identify the agent.
func validateAgentAddress(agent string) error {
	// sFlow v5 Section 4.3: "The sFlowAgent address must provide SNMP
	// connectivity to the agent."
	if agent == "" {
		return errors.New("agent-address is required for sflow: it is the agent's identity in every datagram")
	}
	addr, err := netip.ParseAddr(agent)
	if err != nil {
		return fmt.Errorf("agent-address %q: %w", agent, err)
	}
	if addr.IsUnspecified() {
		return fmt.Errorf("agent-address %q is unspecified: it cannot identify or reach the agent", agent)
	}
	return nil
}
