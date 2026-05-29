// Design: plan/spec-flow-export-1-counter-export.md -- IPFIX encoder registration

package ipfix

import (
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/flowexport"
)

func init() {
	flowexport.RegisterEncoderFactory("ipfix", newIPFIXEncoder)
	flowexport.RegisterFlowRecordEncoderFactory("ipfix", newIPFIXFlowEncoder)
}

func newIPFIXEncoder(cfg flowexport.CollectorConfig, _ time.Time) flowexport.ProtocolEncoder {
	return NewCounterEncoder(cfg.ObservationDomain)
}
