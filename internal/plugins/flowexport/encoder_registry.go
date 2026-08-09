// Design: docs/architecture/flowexport/flow-export-0-umbrella.md -- Encoder factory registry
// Related: flowtypes.go -- FlowSampleEncoder / FlowRecordEncoder interfaces

package flowexport

import "time"

// EncoderFactory creates a ProtocolEncoder for a collector configuration.
// Registered by each protocol subpackage (sflow, netflow9, ipfix) in init().
type EncoderFactory func(cfg CollectorConfig, startTime time.Time) ProtocolEncoder

var encoderFactories = map[string]EncoderFactory{}

// RegisterEncoderFactory registers a protocol encoder factory.
// Called from init() in protocol subpackages.
func RegisterEncoderFactory(protocol string, f EncoderFactory) {
	encoderFactories[protocol] = f
}

// lookupEncoderFactory returns the factory for the given protocol, or nil.
func lookupEncoderFactory(protocol string) EncoderFactory {
	return encoderFactories[protocol]
}

// FlowSampleEncoderFactory creates a FlowSampleEncoder for a collector.
// Registered by the sflow subpackage in init().
type FlowSampleEncoderFactory func(cfg CollectorConfig, startTime time.Time) FlowSampleEncoder

// FlowRecordEncoderFactory creates a FlowRecordEncoder for a collector.
// Registered by the netflow9 and ipfix subpackages in init().
type FlowRecordEncoderFactory func(cfg CollectorConfig, startTime time.Time) FlowRecordEncoder

var flowSampleFactories = map[string]FlowSampleEncoderFactory{}

var flowRecordFactories = map[string]FlowRecordEncoderFactory{}

// RegisterFlowSampleEncoderFactory registers a flow_sample encoder factory.
// Called from init() in the sflow subpackage.
func RegisterFlowSampleEncoderFactory(protocol string, f FlowSampleEncoderFactory) {
	flowSampleFactories[protocol] = f
}

// RegisterFlowRecordEncoderFactory registers a per-flow record encoder factory.
// Called from init() in the netflow9 and ipfix subpackages.
func RegisterFlowRecordEncoderFactory(protocol string, f FlowRecordEncoderFactory) {
	flowRecordFactories[protocol] = f
}

func lookupFlowSampleFactory(protocol string) FlowSampleEncoderFactory {
	return flowSampleFactories[protocol]
}

func lookupFlowRecordFactory(protocol string) FlowRecordEncoderFactory {
	return flowRecordFactories[protocol]
}
