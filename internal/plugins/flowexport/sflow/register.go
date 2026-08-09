// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- sFlow encoder registration

package sflow

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func init() {
	flowexport.RegisterEncoderFactory("sflow", newSFlowEncoder)
	flowexport.RegisterFlowSampleEncoderFactory("sflow", newSFlowFlowEncoder)
}

func newSFlowEncoder(cfg flowexport.CollectorConfig, startTime time.Time) flowexport.ProtocolEncoder {
	// sFlow v5: agent address is the device's own stable IP, not the collector's.
	agentAddr := netip.IPv4Unspecified()
	if cfg.AgentAddress != "" {
		if parsed, err := netip.ParseAddr(cfg.AgentAddress); err == nil {
			agentAddr = parsed
		}
	}
	return NewCounterEncoder(agentAddr, cfg.SubAgentID, startTime)
}
