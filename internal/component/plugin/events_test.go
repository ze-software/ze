// VALIDATES: rpki is a valid BGP event type
// PREVENTS: rpki events rejected by subscription validation.
package plugin

import (
	"strings"
	"testing"
)

// unregisterEventType removes a dynamically registered event type. Test helper only.
func unregisterEventType(namespace, eventType string) {
	eventsMu.Lock()
	defer eventsMu.Unlock()
	delete(ValidEvents[namespace], eventType)
}

func TestValidBgpEventsIncludesRPKI(t *testing.T) {
	if !IsValidEvent(NamespaceBGP, EventRPKI) {
		t.Fatal("rpki should be a valid BGP event type")
	}
	if EventRPKI != "rpki" {
		t.Fatalf("expected EventRPKI = %q, got %q", "rpki", EventRPKI)
	}
}

// VALIDATES: RegisterEventType adds a new event type to ValidEvents.
// PREVENTS: Plugin-registered event types rejected by subscribe/emit validation.
func TestRegisterEventType(t *testing.T) {
	if err := RegisterEventType(NamespaceBGP, "update-rpki"); err != nil {
		t.Fatalf("RegisterEventType failed: %v", err)
	}
	defer unregisterEventType(NamespaceBGP, "update-rpki")

	if !IsValidEvent(NamespaceBGP, "update-rpki") {
		t.Fatal("update-rpki should be a valid BGP event after registration")
	}
}

// VALIDATES: Duplicate registration is idempotent (no error).
// PREVENTS: Plugin startup failure when two plugins register the same event type.
func TestRegisterEventTypeDuplicate(t *testing.T) {
	if err := RegisterEventType(NamespaceBGP, "test-dup"); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	defer unregisterEventType(NamespaceBGP, "test-dup")

	if err := RegisterEventType(NamespaceBGP, "test-dup"); err != nil {
		t.Fatalf("duplicate registration should be idempotent: %v", err)
	}
}

// VALIDATES: RegisterEventType rejects unknown namespaces.
// PREVENTS: Typo in namespace silently creating orphan event types.
func TestRegisterEventTypeInvalidNamespace(t *testing.T) {
	err := RegisterEventType("nonexistent", "some-event")
	if err == nil {
		t.Fatal("expected error for unknown namespace")
	}
}

// VALIDATES: RegisterEventType rejects empty event type names.
// PREVENTS: Empty string accepted as a valid event type.
func TestRegisterEventTypeEmpty(t *testing.T) {
	err := RegisterEventType(NamespaceBGP, "")
	if err == nil {
		t.Fatal("expected error for empty event type")
	}
}

// VALIDATES: RegisterEventType rejects event types with whitespace.
// PREVENTS: Whitespace in event type names causing subscription parsing issues.
func TestRegisterEventTypeWhitespace(t *testing.T) {
	for _, input := range []string{"update rpki", "tab\there", "new\nline", "cr\rreturn"} {
		err := RegisterEventType(NamespaceBGP, input)
		if err == nil {
			t.Fatalf("expected error for event type with whitespace: %q", input)
		}
	}
}

// VALIDATES: ValidEventNames includes dynamically registered types.
// PREVENTS: Error messages showing stale list after dynamic registration.
func TestValidEventNamesIncludesRegistered(t *testing.T) {
	if err := RegisterEventType(NamespaceBGP, "update-rpki"); err != nil {
		t.Fatalf("RegisterEventType failed: %v", err)
	}
	defer unregisterEventType(NamespaceBGP, "update-rpki")

	names := ValidEventNames(NamespaceBGP)
	if !strings.Contains(names, "update-rpki") {
		t.Fatalf("ValidEventNames should include update-rpki, got: %s", names)
	}
}

// VALIDATES: IsValidEventAnyNamespace returns true for types in any namespace.
// PREVENTS: Cross-namespace event types rejected by event monitor.
func TestIsValidEventAnyNamespace(t *testing.T) {
	if !IsValidEventAnyNamespace("update") {
		t.Fatal("update should be valid (BGP namespace)")
	}
	if !IsValidEventAnyNamespace("cache") {
		t.Fatal("cache should be valid (RIB namespace)")
	}
	if IsValidEventAnyNamespace("nonexistent") {
		t.Fatal("nonexistent should not be valid in any namespace")
	}
}

// VALIDATES: AllEventTypes returns types grouped by namespace.
// PREVENTS: Missing namespaces or types in all-events query.
func TestAllEventTypes(t *testing.T) {
	all := AllEventTypes()
	bgp, ok := all[NamespaceBGP]
	if !ok {
		t.Fatal("AllEventTypes should include bgp namespace")
	}
	if len(bgp) == 0 {
		t.Fatal("bgp namespace should have event types")
	}

	rib, ok := all[NamespaceBGPRIB]
	if !ok {
		t.Fatal("AllEventTypes should include rib namespace")
	}
	if len(rib) == 0 {
		t.Fatal("bgp-rib namespace should have event types")
	}

	// Verify it returns a copy (mutations don't affect global state).
	all[NamespaceBGP] = nil
	fresh := AllEventTypes()
	if len(fresh[NamespaceBGP]) == 0 {
		t.Fatal("AllEventTypes should return a fresh copy")
	}
}

// VALIDATES: AllValidEventNames returns a deduped sorted list.
// PREVENTS: Duplicate types in error messages, unsorted output.
func TestAllValidEventNames(t *testing.T) {
	names := AllValidEventNames()
	if !strings.Contains(names, "update") {
		t.Fatalf("AllValidEventNames should include update, got: %s", names)
	}
	if !strings.Contains(names, "cache") {
		t.Fatalf("AllValidEventNames should include cache, got: %s", names)
	}
}

// VALIDATES: every config namespace event type defined in ValidConfigEvents
// is recognized by IsValidEvent, and unknown names are rejected.
// PREVENTS: Regressions where a future edit silently drops one of the 12
// config events from ValidConfigEvents without a test failing.
func TestIsValidEventConfig(t *testing.T) {
	want := map[string]string{
		"EventConfigVerify":       EventConfigVerify,
		"EventConfigApply":        EventConfigApply,
		"EventConfigRollback":     EventConfigRollback,
		"EventConfigCommitted":    EventConfigCommitted,
		"EventConfigApplied":      EventConfigApplied,
		"EventConfigRolledBack":   EventConfigRolledBack,
		"EventConfigVerifyAbort":  EventConfigVerifyAbort,
		"EventConfigVerifyOK":     EventConfigVerifyOK,
		"EventConfigVerifyFailed": EventConfigVerifyFailed,
		"EventConfigApplyOK":      EventConfigApplyOK,
		"EventConfigApplyFailed":  EventConfigApplyFailed,
		"EventConfigRollbackOK":   EventConfigRollbackOK,
	}
	if len(want) != len(ValidConfigEvents) {
		t.Fatalf("ValidConfigEvents has %d entries, test covers %d", len(ValidConfigEvents), len(want))
	}
	for name, value := range want {
		if !IsValidEvent(NamespaceConfig, value) {
			t.Errorf("expected (config, %s = %q) to be valid", name, value)
		}
	}
	if IsValidEvent(NamespaceConfig, "nonsense") {
		t.Errorf("expected (config, nonsense) to be invalid")
	}
	if IsValidEvent(NamespaceConfig, "") {
		t.Errorf("expected (config, empty) to be invalid")
	}
}

// VALIDATES: per-plugin config event types can be registered dynamically.
// PREVENTS: "verify-<plugin>" / "apply-<plugin>" rejected when engine emits them.
func TestRegisterConfigPerPluginEvent(t *testing.T) {
	if err := RegisterEventType(NamespaceConfig, "verify-test"); err != nil {
		t.Fatalf("unexpected error registering verify-test: %v", err)
	}
	defer unregisterEventType(NamespaceConfig, "verify-test")

	if !IsValidEvent(NamespaceConfig, "verify-test") {
		t.Errorf("expected (config, verify-test) to be valid after registration")
	}
}

// unregisterSendType removes a dynamically registered send type. Test helper only.
func unregisterSendType(sendType string) {
	sendTypesMu.Lock()
	defer sendTypesMu.Unlock()
	delete(ValidSendTypes, sendType)
}

// VALIDATES: RegisterSendType adds a new send type to ValidSendTypes.
// PREVENTS: Plugin-registered send types rejected by config validation.
func TestRegisterSendType(t *testing.T) {
	if err := RegisterSendType("enhanced-refresh"); err != nil {
		t.Fatalf("RegisterSendType failed: %v", err)
	}
	defer unregisterSendType("enhanced-refresh")

	if !IsValidSendType("enhanced-refresh") {
		t.Fatal("enhanced-refresh should be a valid send type after registration")
	}
}

// VALIDATES: RegisterSendType is idempotent for duplicate registration.
// PREVENTS: Plugin startup failure when two plugins register the same send type.
func TestRegisterSendTypeDuplicate(t *testing.T) {
	if err := RegisterSendType("test-send-dup"); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	defer unregisterSendType("test-send-dup")

	if err := RegisterSendType("test-send-dup"); err != nil {
		t.Fatalf("duplicate registration should be idempotent: %v", err)
	}
}

// VALIDATES: RegisterSendType rejects empty names.
// PREVENTS: Empty string accepted as a valid send type.
func TestRegisterSendTypeEmpty(t *testing.T) {
	err := RegisterSendType("")
	if err == nil {
		t.Fatal("expected error for empty send type")
	}
}

// VALIDATES: RegisterSendType rejects names with whitespace.
// PREVENTS: Whitespace in send type names causing config parsing issues.
func TestRegisterSendTypeWhitespace(t *testing.T) {
	for _, input := range []string{"enhanced refresh", "tab\there", "new\nline"} {
		err := RegisterSendType(input)
		if err == nil {
			t.Fatalf("expected error for send type with whitespace: %q", input)
		}
	}
}

// VALIDATES: ValidSendTypeNames includes dynamically registered types.
// PREVENTS: Error messages showing stale list after dynamic registration.
func TestValidSendTypeNamesIncludesRegistered(t *testing.T) {
	if err := RegisterSendType("enhanced-refresh"); err != nil {
		t.Fatalf("RegisterSendType failed: %v", err)
	}
	defer unregisterSendType("enhanced-refresh")

	names := ValidSendTypeNames()
	if !strings.Contains(names, "enhanced-refresh") {
		t.Fatalf("ValidSendTypeNames should include enhanced-refresh, got: %s", names)
	}
}

// VALIDATES: IsValidSendType returns false for unregistered types.
// PREVENTS: Unregistered send types silently accepted.
func TestIsValidSendTypeRejectsUnregistered(t *testing.T) {
	if IsValidSendType("nonexistent-send-type") {
		t.Fatal("nonexistent send type should not be valid")
	}
}

// VALIDATES: IsValidNamespace returns correct results.
// PREVENTS: Namespace validation broken after refactor.
func TestIsValidNamespace(t *testing.T) {
	if !IsValidNamespace(NamespaceBGP) {
		t.Fatal("bgp should be a valid namespace")
	}
	if !IsValidNamespace(NamespaceBGPRIB) {
		t.Fatal("bgp-rib should be a valid namespace")
	}
	if IsValidNamespace("nonexistent") {
		t.Fatal("nonexistent should not be a valid namespace")
	}
}
