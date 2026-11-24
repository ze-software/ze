// Design: docs/architecture/api/process-protocol.md -- bus lifecycle message registration.
// Related: events.go -- NamespaceBus, EventParticipantExit, and ParticipantExitEvent.

package events

func init() {
	if err := RegisterMessage(NamespaceBus, EventParticipantExit); err != nil {
		panic("BUG: events: register " + MessageParticipantExit + " failed: " + err.Error())
	}
	BusParticipantExit = Register[ParticipantExitEvent](NamespaceBus, EventParticipantExit)
}
