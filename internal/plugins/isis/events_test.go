// Design: plan/spec-isis-4-component-config.md -- IS-IS event namespace test
//
// VALIDATES: the IS-IS event namespace registers its session up/down and
// LSP-change event types without collision (TestISISEventNamespace), and the
// typed Event handles carry the declared event-type strings.
package isis

import (
	"testing"

	"github.com/ze-software/ze/internal/core/events"
)

func TestISISEventNamespace(t *testing.T) {
	// The namespace is registered by an init() in events.go; the three event
	// types must be valid in it.
	for _, et := range []string{EventSessionUp, EventSessionDown, EventLSPChange} {
		if !events.IsValidEvent(Namespace, et) {
			t.Errorf("event %q not valid in namespace %q", et, Namespace)
		}
	}
	// A made-up event type must not be valid.
	if events.IsValidEvent(Namespace, "not-an-isis-event") {
		t.Error("unexpected event type reported valid")
	}
	// The typed handles carry the right event-type strings.
	if SessionUp.EventType() != EventSessionUp {
		t.Errorf("SessionUp.EventType() = %q, want %q", SessionUp.EventType(), EventSessionUp)
	}
	if SessionDown.EventType() != EventSessionDown {
		t.Errorf("SessionDown.EventType() = %q, want %q", SessionDown.EventType(), EventSessionDown)
	}
	if LSPChange.EventType() != EventLSPChange {
		t.Errorf("LSPChange.EventType() = %q, want %q", LSPChange.EventType(), EventLSPChange)
	}
}
