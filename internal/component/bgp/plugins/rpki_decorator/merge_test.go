// VALIDATES: mergeUpdateRPKI correctly injects rpki section into UPDATE JSON.
// PREVENTS: Malformed merged events delivered to consumers.
package rpki_decorator

import (
	"encoding/json"
	"testing"
)

func TestMergeUpdateRPKI(t *testing.T) {
	update := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","asn":65001},"message":{"id":42,"direction":"received","type":"update"},"update":{"attr":{"origin":"igp"},"ipv4/unicast":[{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}`
	rpki := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","asn":65001},"message":{"id":42,"type":"rpki"},"rpki":{"ipv4/unicast":{"10.0.0.0/24":"valid"}}}}`

	merged := mergeUpdateRPKI(update, rpki)
	if merged == "" {
		t.Fatal("mergeUpdateRPKI returned empty string")
	}

	// Parse the merged JSON.
	var result map[string]any
	if err := json.Unmarshal([]byte(merged), &result); err != nil {
		t.Fatalf("merged JSON is invalid: %v\nraw: %s", err, merged)
	}

	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp key in merged event")
	}

	// Must have update section (from primary).
	if _, ok := bgp["update"]; !ok {
		t.Fatal("missing update key in merged event")
	}

	// Must have rpki section (from secondary).
	rpkiSection, ok := bgp["rpki"].(map[string]any)
	if !ok {
		t.Fatal("missing rpki key in merged event")
	}
	family, ok := rpkiSection["ipv4/unicast"].(map[string]any)
	if !ok {
		t.Fatal("missing ipv4/unicast in rpki section")
	}
	if family["10.0.0.0/24"] != "valid" {
		t.Fatalf("expected valid, got %v", family["10.0.0.0/24"])
	}

	// Message type should be update-rpki.
	msg, ok := bgp["message"].(map[string]any)
	if !ok {
		t.Fatal("missing message key")
	}
	if msg["type"] != "update-rpki" {
		t.Fatalf("expected message type update-rpki, got %v", msg["type"])
	}
}

func TestMergeUpdateRPKIUnavailable(t *testing.T) {
	update := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","asn":65001},"message":{"id":7,"direction":"received","type":"update"},"update":{"attr":{"origin":"igp"}}}}`
	rpki := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","asn":65001},"message":{"id":7,"type":"rpki"},"rpki":{"status":"unavailable"}}}`

	merged := mergeUpdateRPKI(update, rpki)

	var result map[string]any
	if err := json.Unmarshal([]byte(merged), &result); err != nil {
		t.Fatalf("merged JSON is invalid: %v", err)
	}

	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp key in merged event")
	}
	rpkiSection, ok := bgp["rpki"].(map[string]any)
	if !ok {
		t.Fatal("missing rpki key in merged event")
	}
	if rpkiSection["status"] != "unavailable" {
		t.Fatalf("expected status=unavailable, got %v", rpkiSection["status"])
	}
}

func TestMergeUpdateRPKITimeoutNoSecondary(t *testing.T) {
	update := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","asn":65001},"message":{"id":3,"direction":"received","type":"update"},"update":{"attr":{"origin":"igp"}}}}`

	// Empty secondary means rpki event timed out.
	merged := mergeUpdateRPKI(update, "")
	if merged == "" {
		t.Fatal("mergeUpdateRPKI returned empty string for timeout case")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(merged), &result); err != nil {
		t.Fatalf("merged JSON is invalid: %v", err)
	}

	bgp, ok := result["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp key in timeout case")
	}

	// Must still have update section.
	if _, ok := bgp["update"]; !ok {
		t.Fatal("missing update key in timeout case")
	}

	// Should NOT have rpki section (no validation data).
	if _, ok := bgp["rpki"]; ok {
		t.Fatal("should not have rpki key in timeout case")
	}

	// Message type should still be update-rpki.
	msg, ok := bgp["message"].(map[string]any)
	if !ok {
		t.Fatal("missing message key in timeout case")
	}
	if msg["type"] != "update-rpki" {
		t.Fatalf("expected message type update-rpki, got %v", msg["type"])
	}
}

func TestMergeUpdateRPKIInvalidJSON(t *testing.T) {
	// Bad primary returns empty.
	if merged := mergeUpdateRPKI("not-json", ""); merged != "" {
		t.Fatalf("expected empty for bad primary, got: %s", merged)
	}

	// Good primary, bad secondary: should still produce result (without rpki section).
	update := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","asn":65001},"message":{"id":1,"type":"update"},"update":{}}}`
	merged := mergeUpdateRPKI(update, "not-json")
	if merged == "" {
		t.Fatal("expected non-empty for good primary with bad secondary")
	}
}

func TestMergeUpdateRPKIMissingBGPKey(t *testing.T) {
	// Valid JSON but no "bgp" key returns empty.
	if merged := mergeUpdateRPKI(`{"type":"bgp"}`, ""); merged != "" {
		t.Fatalf("expected empty for missing bgp key, got: %s", merged)
	}
}

func TestMergeUpdateRPKIMissingMessageKey(t *testing.T) {
	// Valid JSON with bgp but no message key returns empty (fix #12).
	update := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"},"update":{}}}`
	if merged := mergeUpdateRPKI(update, ""); merged != "" {
		t.Fatalf("expected empty for missing message key, got: %s", merged)
	}
}
