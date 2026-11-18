// Design: docs/architecture/api/process-protocol.md -- original Session event lifetime.
package plugin

// SentObservation is a synchronous engine-owned publication boundary.
// BeginSentObservation MUST precede recipient capture; every accepted begin
// MUST pair with EndSentObservation after all deliveries have been enqueued.
// Refusal closes routing publication only; actual wire monitors still observe.
// This interface grants no received-source lease or route-send permission.
type SentObservation interface {
	BeginSentObservation() bool
	EndSentObservation()
}
