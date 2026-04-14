// VALIDATES: event namespace registration and validation machinery.
// PREVENTS: broken registration, validation, or query functions.
package events

import (
	"os"
	"strings"
	"testing"
)

// TestMain registers test namespaces before any tests run.
// Uses string literals because this package tests the machinery itself,
// not the component-owned constants. In production, components import
// their own events/ sub-packages and call RegisterNamespace from init().
func TestMain(m *testing.M) {
	_ = RegisterNamespace("bgp",
		"update", "open", "notification", "keepalive",
		"refresh", "state", "negotiated", "eor",
		"congested", "resumed", "rpki", "listener-ready",
		"update-notification", DirectionSent,
	)
	_ = RegisterNamespace("bgp-rib", "cache", "route", "best-change", "replay-request")
	_ = RegisterNamespace("config",
		"verify", "apply", "rollback", "committed", "applied", "rolled-back",
		"verify-abort", "verify-ok", "verify-failed",
		"apply-ok", "apply-failed", "rollback-ok",
	)
	_ = RegisterNamespace("system-rib", "best-change", "replay-request")
	_ = RegisterNamespace("fib", "external-change")
	_ = RegisterNamespace("interface",
		"created", "up", "down", "addr-added", "addr-removed",
		"dhcp-acquired", "dhcp-renewed", "dhcp-expired", "rollback",
		"router-discovered", "router-lost",
	)
	_ = RegisterNamespace("sysctl",
		"default", "set", "applied", "show-request", "show-result",
		"list-request", "list-result", "describe-request", "describe-result",
		"clear-profile-defaults",
	)
	_ = RegisterNamespace("system", "clock-synced")
	_ = RegisterNamespace("vpp", "connected", "disconnected", "reconnected")
	os.Exit(m.Run())
}

// unregisterEventType removes a dynamically registered event type. Test helper only.
func unregisterEventType(namespace, eventType string) {
	eventsMu.Lock()
	defer eventsMu.Unlock()
	delete(ValidEvents[namespace], eventType)
}

func TestValidBgpEventsIncludesRPKI(t *testing.T) {
	if !IsValidEvent("bgp", "rpki") {
		t.Fatal("rpki should be a valid BGP event type")
	}
}

func TestRegisterEventType(t *testing.T) {
	if err := RegisterEventType("bgp", "update-rpki"); err != nil {
		t.Fatalf("RegisterEventType failed: %v", err)
	}
	defer unregisterEventType("bgp", "update-rpki")

	if !IsValidEvent("bgp", "update-rpki") {
		t.Fatal("update-rpki should be a valid BGP event after registration")
	}
}

func TestRegisterEventTypeDuplicate(t *testing.T) {
	if err := RegisterEventType("bgp", "test-dup"); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	defer unregisterEventType("bgp", "test-dup")

	if err := RegisterEventType("bgp", "test-dup"); err != nil {
		t.Fatalf("duplicate registration should be idempotent: %v", err)
	}
}

func TestRegisterEventTypeInvalidNamespace(t *testing.T) {
	err := RegisterEventType("nonexistent", "some-event")
	if err == nil {
		t.Fatal("expected error for unknown namespace")
	}
}

func TestRegisterEventTypeEmpty(t *testing.T) {
	err := RegisterEventType("bgp", "")
	if err == nil {
		t.Fatal("expected error for empty event type")
	}
}

func TestRegisterEventTypeWhitespace(t *testing.T) {
	for _, input := range []string{"update rpki", "tab\there", "new\nline", "cr\rreturn"} {
		err := RegisterEventType("bgp", input)
		if err == nil {
			t.Fatalf("expected error for event type with whitespace: %q", input)
		}
	}
}

func TestValidEventNamesIncludesRegistered(t *testing.T) {
	if err := RegisterEventType("bgp", "update-rpki"); err != nil {
		t.Fatalf("RegisterEventType failed: %v", err)
	}
	defer unregisterEventType("bgp", "update-rpki")

	names := ValidEventNames("bgp")
	if !strings.Contains(names, "update-rpki") {
		t.Fatalf("ValidEventNames should include update-rpki, got: %s", names)
	}
}

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

func TestAllEventTypes(t *testing.T) {
	all := AllEventTypes()
	bgp, ok := all["bgp"]
	if !ok {
		t.Fatal("AllEventTypes should include bgp namespace")
	}
	if len(bgp) == 0 {
		t.Fatal("bgp namespace should have event types")
	}

	rib, ok := all["bgp-rib"]
	if !ok {
		t.Fatal("AllEventTypes should include rib namespace")
	}
	if len(rib) == 0 {
		t.Fatal("bgp-rib namespace should have event types")
	}

	all["bgp"] = nil
	fresh := AllEventTypes()
	if len(fresh["bgp"]) == 0 {
		t.Fatal("AllEventTypes should return a fresh copy")
	}
}

func TestAllValidEventNames(t *testing.T) {
	names := AllValidEventNames()
	if !strings.Contains(names, "update") {
		t.Fatalf("AllValidEventNames should include update, got: %s", names)
	}
	if !strings.Contains(names, "cache") {
		t.Fatalf("AllValidEventNames should include cache, got: %s", names)
	}
}

func TestIsValidEventConfig(t *testing.T) {
	want := []string{
		"verify", "apply", "rollback", "committed", "applied", "rolled-back",
		"verify-abort", "verify-ok", "verify-failed",
		"apply-ok", "apply-failed", "rollback-ok",
	}
	for _, value := range want {
		if !IsValidEvent("config", value) {
			t.Errorf("expected (config, %q) to be valid", value)
		}
	}
	if IsValidEvent("config", "nonsense") {
		t.Errorf("expected (config, nonsense) to be invalid")
	}
}

func TestRegisterConfigPerPluginEvent(t *testing.T) {
	if err := RegisterEventType("config", "verify-test"); err != nil {
		t.Fatalf("unexpected error registering verify-test: %v", err)
	}
	defer unregisterEventType("config", "verify-test")

	if !IsValidEvent("config", "verify-test") {
		t.Errorf("expected (config, verify-test) to be valid after registration")
	}
}

// unregisterSendType removes a dynamically registered send type. Test helper only.
func unregisterSendType(sendType string) {
	sendTypesMu.Lock()
	defer sendTypesMu.Unlock()
	delete(ValidSendTypes, sendType)
}

func TestRegisterSendType(t *testing.T) {
	if err := RegisterSendType("enhanced-refresh"); err != nil {
		t.Fatalf("RegisterSendType failed: %v", err)
	}
	defer unregisterSendType("enhanced-refresh")

	if !IsValidSendType("enhanced-refresh") {
		t.Fatal("enhanced-refresh should be a valid send type after registration")
	}
}

func TestRegisterSendTypeDuplicate(t *testing.T) {
	if err := RegisterSendType("test-send-dup"); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	defer unregisterSendType("test-send-dup")

	if err := RegisterSendType("test-send-dup"); err != nil {
		t.Fatalf("duplicate registration should be idempotent: %v", err)
	}
}

func TestRegisterSendTypeEmpty(t *testing.T) {
	if err := RegisterSendType(""); err == nil {
		t.Fatal("expected error for empty send type")
	}
}

func TestRegisterSendTypeWhitespace(t *testing.T) {
	for _, input := range []string{"enhanced refresh", "tab\there", "new\nline"} {
		if err := RegisterSendType(input); err == nil {
			t.Fatalf("expected error for send type with whitespace: %q", input)
		}
	}
}

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

func TestIsValidSendTypeRejectsUnregistered(t *testing.T) {
	if IsValidSendType("nonexistent-send-type") {
		t.Fatal("nonexistent send type should not be valid")
	}
}

func TestIsValidNamespace(t *testing.T) {
	if !IsValidNamespace("bgp") {
		t.Fatal("bgp should be a valid namespace")
	}
	if !IsValidNamespace("bgp-rib") {
		t.Fatal("bgp-rib should be a valid namespace")
	}
	if IsValidNamespace("nonexistent") {
		t.Fatal("nonexistent should not be a valid namespace")
	}
}

func TestRegisterNamespace(t *testing.T) {
	if err := RegisterNamespace("test-ns", "event-a", "event-b"); err != nil {
		t.Fatalf("RegisterNamespace failed: %v", err)
	}
	defer func() {
		eventsMu.Lock()
		delete(ValidEvents, "test-ns")
		eventsMu.Unlock()
	}()

	if !IsValidNamespace("test-ns") {
		t.Fatal("test-ns should be a valid namespace after registration")
	}
	if !IsValidEvent("test-ns", "event-a") {
		t.Fatal("event-a should be valid in test-ns")
	}
}

func TestRegisterNamespaceDuplicate(t *testing.T) {
	err := RegisterNamespace("bgp", "some-event")
	if err == nil {
		t.Fatal("expected error for duplicate namespace")
	}
}
