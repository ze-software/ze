package iface

import (
	"encoding/json"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func TestIfacePluginRegistered(t *testing.T) {
	// VALIDATES: Plugin is registered in the registry via init().
	// PREVENTS: Plugin not discoverable by engine.
	reg := registry.Lookup("interface")
	if reg == nil {
		t.Fatal("interface plugin not found in registry")
	}
	if reg.Name != "interface" {
		t.Errorf("expected name %q, got %q", "interface", reg.Name)
	}
	if reg.Description == "" {
		t.Error("description is empty")
	}
}

func TestBusTopicCreation(t *testing.T) {
	// VALIDATES: Plugin defines correct Bus topic constants.
	// PREVENTS: Typos in topic strings causing missed subscriptions.
	expectedTopics := []string{
		TopicCreated,
		TopicDeleted,
		TopicUp,
		TopicDown,
		TopicAddrAdded,
		TopicAddrRemoved,
	}

	for _, topic := range expectedTopics {
		if topic == "" {
			t.Errorf("topic constant is empty")
		}
	}

	// Verify hierarchical naming.
	if TopicCreated != "interface/created" {
		t.Errorf("TopicCreated = %q, want %q", TopicCreated, "interface/created")
	}
	if TopicAddrAdded != "interface/addr/added" {
		t.Errorf("TopicAddrAdded = %q, want %q", TopicAddrAdded, "interface/addr/added")
	}
	if TopicAddrRemoved != "interface/addr/removed" {
		t.Errorf("TopicAddrRemoved = %q, want %q", TopicAddrRemoved, "interface/addr/removed")
	}
}

func TestPayloadFormat(t *testing.T) {
	// VALIDATES: JSON payload uses kebab-case and includes unit field.
	// PREVENTS: Mismatched field names breaking Bus consumers.
	payload := AddrPayload{
		Name:         "eth0",
		Unit:         0,
		Index:        2,
		Address:      "10.0.0.1",
		PrefixLength: 24,
		Family:       "ipv4",
		Managed:      false,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify kebab-case keys.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	requiredKeys := []string{"name", "unit", "index", "address", "prefix-length", "family", "managed"}
	for _, key := range requiredKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in JSON payload", key)
		}
	}

	// Verify no camelCase or snake_case keys.
	for key := range raw {
		for _, banned := range []string{"prefixLength", "prefix_length"} {
			if key == banned {
				t.Errorf("found non-kebab-case key %q", key)
			}
		}
	}

	// Round-trip check.
	var decoded AddrPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded != payload {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, payload)
	}
}

func TestLinkPayloadFormat(t *testing.T) {
	// VALIDATES: Link event payload uses correct fields.
	// PREVENTS: Missing fields in interface created/deleted events.
	payload := LinkPayload{
		Name:    "eth0",
		Type:    "ethernet",
		Index:   2,
		MTU:     1500,
		Managed: true,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	for _, key := range []string{"name", "type", "index", "mtu", "managed"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in link payload", key)
		}
	}
}

func TestStatePayloadFormat(t *testing.T) {
	// VALIDATES: State event payload (up/down) uses correct fields.
	// PREVENTS: Missing index in state change events.
	payload := StatePayload{
		Name:  "eth0",
		Index: 2,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	for _, key := range []string{"name", "index"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in state payload", key)
		}
	}
}
