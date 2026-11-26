// Design: docs/architecture/api/architecture.md -- Phase 6 bus send authorization and refusal fixture registrations.
// Related: bus_send_fixtures.go -- driver implementations registered below.

package fixture

func init() {
	Register("plugin/bus-wait-file", busWaitFileDriver)
	Register("plugin/bus-touch-file", busTouchFileDriver)
	Register("plugin/bus-assert-refused-count", busAssertRefusedCountDriver)
	Register("plugin/bus-assert-refused-series", busAssertRefusedSeriesDriver)
	Register("plugin/bus-sighup-reload", busSIGHUPReloadDriver)
	Register("plugin/bus-sighup-reload-refused", busSIGHUPReloadRefusedDriver)
	Register("plugin/bus-assert-same-pid", busAssertSamePIDDriver)
	Register("plugin/bus-event-collector", busEventCollectorDriver)
	Register("plugin/bus-assert-collector", busAssertCollectorDriver)
	Register("plugin/bus-sender-stamp", busSenderStampDriver)
	Register("plugin/bus-send-allow-list-reload", busSendAllowListReloadDriver)
	Register("plugin/bus-send-allow-list-namespaces", busSendAllowListNamespacesDriver)
	Register("plugin/bus-send-refusal-doctor", busSendRefusalDoctorDriver)
	Register("plugin/bus-burst-observer", busBurstObserverDriver)
	Register("plugin/bus-burst-verify", busBurstVerifyDriver)
}
