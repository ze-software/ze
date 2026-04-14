package transaction

import (
	"strings"
	"testing"

	txevents "codeberg.org/thomas-mangin/ze/internal/component/config/transaction/events"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

// VALIDATES: All event type constants match their plugin package source values.
// PREVENTS: Drift between transaction package re-exports and plugin event registry.
func TestConfigEventTypeConstants(t *testing.T) {
	if Namespace != "config" {
		t.Errorf("Namespace = %q, want %q", Namespace, "config")
	}

	// Engine -> plugin (per-plugin event base names).
	if EventVerify != "verify" {
		t.Errorf("EventVerify = %q, want %q", EventVerify, "verify")
	}
	if EventApply != "apply" {
		t.Errorf("EventApply = %q, want %q", EventApply, "apply")
	}
	if EventRollback != "rollback" {
		t.Errorf("EventRollback = %q, want %q", EventRollback, "rollback")
	}

	// Engine -> plugins (broadcast).
	if EventVerifyAbort != "verify-abort" {
		t.Errorf("EventVerifyAbort = %q, want %q", EventVerifyAbort, "verify-abort")
	}
	if EventCommitted != "committed" {
		t.Errorf("EventCommitted = %q, want %q", EventCommitted, "committed")
	}
	if EventApplied != "applied" {
		t.Errorf("EventApplied = %q, want %q", EventApplied, "applied")
	}
	if EventRolledBack != "rolled-back" {
		t.Errorf("EventRolledBack = %q, want %q", EventRolledBack, "rolled-back")
	}

	// Plugin -> engine (acks).
	if EventVerifyOK != "verify-ok" {
		t.Errorf("EventVerifyOK = %q, want %q", EventVerifyOK, "verify-ok")
	}
	if EventVerifyFailed != "verify-failed" {
		t.Errorf("EventVerifyFailed = %q, want %q", EventVerifyFailed, "verify-failed")
	}
	if EventApplyOK != "apply-ok" {
		t.Errorf("EventApplyOK = %q, want %q", EventApplyOK, "apply-ok")
	}
	if EventApplyFailed != "apply-failed" {
		t.Errorf("EventApplyFailed = %q, want %q", EventApplyFailed, "apply-failed")
	}
	if EventRollbackOK != "rollback-ok" {
		t.Errorf("EventRollbackOK = %q, want %q", EventRollbackOK, "rollback-ok")
	}

	// Per-plugin helpers.
	if got := EventVerifyFor("bgp"); got != "verify-bgp" {
		t.Errorf("EventVerifyFor(bgp) = %q, want %q", got, "verify-bgp")
	}
	if got := EventApplyFor("interface"); got != "apply-interface" {
		t.Errorf("EventApplyFor(interface) = %q, want %q", got, "apply-interface")
	}

	// Failure codes.
	codes := []string{CodeOK, CodeTimeout, CodeTransient, CodeError, CodeBroken}
	for _, code := range codes {
		if code == "" {
			t.Errorf("failure code constant is empty")
		}
	}
	if CodeOK != "ok" {
		t.Errorf("CodeOK = %q, want %q", CodeOK, "ok")
	}
	if CodeBroken != "broken" {
		t.Errorf("CodeBroken = %q, want %q", CodeBroken, "broken")
	}
}

// VALIDATES: ValidatePluginName rejects empty, whitespace, and reserved names;
// accepts normal plugin identifiers.
// PREVENTS: Phase 4 orchestrator silently building per-plugin event types that
// collide with broadcast or ack event types in the stream registry.
func TestValidatePluginName(t *testing.T) {
	reject := []struct {
		name   string
		reason string
	}{
		{"", "empty"},
		{" ", "space"},
		{"a b", "embedded space"},
		{"a\tb", "embedded tab"},
		{"a\nb", "embedded newline"},
		{"ok", "reserved: collides with verify-ok/apply-ok/rollback-ok"},
		{"failed", "reserved: collides with verify-failed/apply-failed"},
		{"abort", "reserved: collides with verify-abort"},
	}
	for _, tc := range reject {
		if err := ValidatePluginName(tc.name); err == nil {
			t.Errorf("ValidatePluginName(%q) returned nil, want error (%s)", tc.name, tc.reason)
		}
	}

	accept := []string{"bgp", "interface", "bgp-rib", "bgp-gr", "engine", "rib", "rpki"}
	for _, name := range accept {
		if err := ValidatePluginName(name); err != nil {
			t.Errorf("ValidatePluginName(%q) = %v, want nil", name, err)
		}
	}
}

// VALIDATES: every string in ReservedPluginNames collides with at least one
// broadcast/ack event type, and conversely every broadcast/ack event type
// that uses a "<base>-<suffix>" shape has its suffix in ReservedPluginNames.
// PREVENTS: ReservedPluginNames drifting out of sync with ValidConfigEvents
// after a new ack/broadcast event type is added in a future phase.
func TestReservedPluginNamesMatchValidEvents(t *testing.T) {
	bases := []string{EventVerify, EventApply, EventRollback}

	// Every reserved name must actually collide with at least one event.
	for reserved := range ReservedPluginNames {
		collided := false
		for _, base := range bases {
			candidate := base + "-" + reserved
			if events.IsValidEvent(txevents.Namespace, candidate) {
				collided = true
				break
			}
		}
		if !collided {
			t.Errorf("ReservedPluginNames has %q but it does not collide with any ValidConfigEvents entry", reserved)
		}
	}

	// Every event type of shape "<base>-<suffix>" must have its suffix reserved.
	configEvents := events.AllEventTypes()["config"]
	for _, event := range configEvents {
		for _, base := range bases {
			prefix := base + "-"
			if !strings.HasPrefix(event, prefix) {
				continue
			}
			suffix := event[len(prefix):]
			if !ReservedPluginNames[suffix] {
				t.Errorf("config event %q (suffix %q) but suffix is not in ReservedPluginNames", event, suffix)
			}
		}
	}

	// EventVerifyFor/EventApplyFor on every reserved name must collide;
	// on every accepted name must not.
	for reserved := range ReservedPluginNames {
		vf := EventVerifyFor(reserved)
		af := EventApplyFor(reserved)
		if !events.IsValidEvent(txevents.Namespace, vf) && !events.IsValidEvent(txevents.Namespace, af) {
			t.Errorf("neither EventVerifyFor(%q) nor EventApplyFor(%q) collide with config events", reserved, reserved)
		}
	}
	for _, safe := range []string{"bgp", "interface", "rib", "bgp-rib"} {
		if events.IsValidEvent(txevents.Namespace, EventVerifyFor(safe)) {
			t.Errorf("EventVerifyFor(%q) = %q collides with a broadcast/ack event type", safe, EventVerifyFor(safe))
		}
		if events.IsValidEvent(txevents.Namespace, EventApplyFor(safe)) {
			t.Errorf("EventApplyFor(%q) = %q collides with a broadcast/ack event type", safe, EventApplyFor(safe))
		}
	}
}
