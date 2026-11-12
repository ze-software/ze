// Design: docs/architecture/api/process-protocol.md -- event delivery to plugin processes.
// Related: dispatch.go -- deliverEvent and refuseUndecodedPayload, the engine side under test
// Related: dispatch_report_test.go -- the plugin side of the same refusal
package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/report"
)

// engineDecodeShape is the payload type of engineDecodeEvent.
type engineDecodeShape struct {
	Name string `json:"name"`
}

// engineDecodeEvent is registered for TestUndecodableEngineEmitIsADefectRaisedOnce
// alone: engineDecodeDefects is process-global, so an event another test
// emits could already hold its entry and silence the raise under test.
var engineDecodeEvent = events.Register[*engineDecodeShape]("test-engine-decode", "bad-up")

// TestUndecodableEngineEmitIsADefectRaisedOnce proves an engine emit (no
// emitting process) of a typed event whose string payload does not decode is
// a defect in ze, not a plugin's refused request: the emit answers an error
// naming the event, neither an engine subscriber nor a forked subscriber
// receives it, the bus holds one doctor-internal-defect event for the event
// and no doctor-plugin-request-refused event, and a second such emit raises
// no second event.
//
// METHOD: deliverEvent with a nil emitter, as the engine's own Emit reaches
// it, with a typed engine subscriber and a plugin-process subscriber on the
// event. The test forgets the event's engineDecodeDefects entry when it
// ends, so a repeated run raises again.
//
// PREVENTS: the engine arm of refuseUndecodedPayload being dropped with every
// other test still green; an engine publisher's drift then reads as a plugin
// refusal, or calls Name on a nil process.
func TestUndecodableEngineEmitIsADefectRaisedOnce(t *testing.T) {
	s := reportTestServer(t)
	s.engineSubscribers = newEngineEventSubscribers()
	s.subscriptions = newSubscriptionManager()
	namespace, eventType := engineDecodeEvent.Namespace(), engineDecodeEvent.EventType()
	event := namespace + "/" + eventType
	t.Cleanup(func() { engineDecodeDefects.Delete(event) })

	received := 0
	unsubscribe := engineDecodeEvent.Subscribe(s, func(*engineDecodeShape) { received++ })
	defer unsubscribe()
	subscriber := process.NewProcess(plugin.PluginConfig{Name: "engine-decode-consumer"})
	subscriber.StartDelivery(t.Context())
	s.subscriptions.Add(subscriber, &Subscription{
		Namespace: events.LookupNamespaceID(namespace),
		EventType: events.LookupEventTypeID(eventType),
		Direction: events.DirBoth,
	})

	// Red: the `if emitter == nil` arm of refuseUndecodedPayload (a nil
	// process then reaches the plugin arm), or the
	// engineDecodeDefects.LoadOrStore guard on its raise (the second emit
	// then raises a second event).
	for range 2 {
		delivered, err := s.deliverEvent(nil, namespace, eventType, "", "", "{not json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), event)
		assert.Zero(t, delivered, "the forked subscriber received an undecodable engine payload")
	}
	assert.Zero(t, received, "an engine subscriber received an undecodable engine payload")

	defects, refused := 0, 0
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorInternalDefect && issue.Subject == namespace+" "+eventType {
			defects++
		}
		if issue.Code == diagnostic.CodeDoctorPluginRequestRefused {
			refused++
		}
	}
	assert.Equal(t, 1, defects, "two undecodable engine emits raised %d defect events", defects)
	assert.Zero(t, refused, "an engine payload was reported as a plugin's refused request")
}
