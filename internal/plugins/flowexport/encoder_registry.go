// Design: docs/architecture/flowexport/flow-export-0-umbrella.md -- Encoder factory registry
// Related: flowtypes.go -- FlowSampleEncoder / FlowRecordEncoder interfaces

package flowexport

import (
	"errors"
	"maps"
	"slices"
	"time"
)

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

// RegisteredProtocols answers every protocol an encoder registered for,
// sorted. It is what a collector's protocol is validated against: a protocol
// this binary links no encoder for is refused at commit, rather than accepted
// and then exported to nobody.
func RegisteredProtocols() []string {
	return slices.Sorted(maps.Keys(encoderFactories))
}

// CollectorsValidator checks every collector of one protocol together, for a
// rule of that protocol that spans collectors. It returns nil when the
// collectors are acceptable. Registered by a protocol subpackage in init().
type CollectorsValidator func(collectors []CollectorConfig) error

var collectorsValidators = map[string]CollectorsValidator{}

// RegisterCollectorsValidator registers the validator for one protocol's
// collectors. Called from init() in protocol subpackages.
func RegisterCollectorsValidator(protocol string, v CollectorsValidator) {
	collectorsValidators[protocol] = v
}

// validateProtocolCollectors runs each registered protocol validator over the
// configured collectors of its protocol, in protocol order.
func validateProtocolCollectors(collectors []CollectorConfig) error {
	var errs []error
	for _, protocol := range slices.Sorted(maps.Keys(collectorsValidators)) {
		var matched []CollectorConfig
		for i := range collectors {
			if collectors[i].Protocol == protocol {
				matched = append(matched, collectors[i])
			}
		}
		if len(matched) == 0 {
			continue
		}
		if err := collectorsValidators[protocol](matched); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
