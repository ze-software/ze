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

// VALIDATES: IsValidNamespace returns correct results.
// PREVENTS: Namespace validation broken after refactor.
func TestIsValidNamespace(t *testing.T) {
	if !IsValidNamespace(NamespaceBGP) {
		t.Fatal("bgp should be a valid namespace")
	}
	if !IsValidNamespace(NamespaceRIB) {
		t.Fatal("rib should be a valid namespace")
	}
	if IsValidNamespace("nonexistent") {
		t.Fatal("nonexistent should not be a valid namespace")
	}
}
