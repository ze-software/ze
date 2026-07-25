// Design: plan/learned/818-flow-export-1-counter-export.md -- NetFlow v9 encoder registration

package netflow9

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func init() {
	flowexport.RegisterEncoderFactory("netflow9", newNetflow9Encoder)
	flowexport.RegisterFlowRecordEncoderFactory("netflow9", newNetflow9FlowEncoder)
}

func newNetflow9Encoder(cfg flowexport.CollectorConfig, startTime time.Time) flowexport.ProtocolEncoder {
	return NewCounterEncoder(cfg.ObservationDomain, startTime)
}
