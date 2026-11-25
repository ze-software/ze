// Design: docs/architecture/api/architecture.md -- worked bus scenario S1-S44 external plugin fixture registrations.
// Related: bus_scenario_fixtures.go -- driver implementations registered below.

package fixture

func init() {
	Register("plugin/bus-bgp-collector", busBGPCollectorDriver)
	Register("plugin/bus-bgp-sender", busBGPSenderDriver)
	Register("plugin/bus-acme-agent", busAcmeAgentDriver)
	Register("plugin/bus-detector", busDetectorDriver)
	Register("plugin/bus-state-spoof-probe", busStateSpoofProbeDriver)
	Register("plugin/bus-tag-sender", busTagSenderDriver)
}
