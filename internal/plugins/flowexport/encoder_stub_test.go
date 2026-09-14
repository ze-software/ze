// Related: encoder_registry.go -- the registry Validate reads
// Related: config_test.go, rfc7011_test.go, sender_test.go -- the fixtures that name a protocol

package flowexport

import "time"

// The encoder subpackages import this package, so an internal test cannot
// link them, and a registry with no encoder refuses every collector. The
// fixtures in this package name sflow and ipfix, so those two get a stub
// encoder here. A stub that encodes nothing is enough: no test in this package
// drives the exporter through a registered factory, only the config guard.
func init() {
	RegisterEncoderFactory("sflow", stubEncoderFactory)
	RegisterEncoderFactory("ipfix", stubEncoderFactory)
}

func stubEncoderFactory(CollectorConfig, time.Time) ProtocolEncoder {
	return nil
}
