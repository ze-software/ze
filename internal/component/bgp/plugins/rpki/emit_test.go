// VALIDATES: RPKI plugin builds correct validation event JSON
// PREVENTS: Malformed rpki events or missing per-prefix states
package rpki

import (
	"encoding/json"
	"testing"
)

func TestBuildRPKIEvent(t *testing.T) {
	results := map[string]uint8{
		"10.0.1.0/24":    ValidationValid,
		"10.0.2.0/24":    ValidationInvalid,
		"192.168.0.0/16": ValidationNotFound,
	}

	event := buildRPKIEvent("10.0.0.1", uint32(65001), uint64(42), "ipv4/unicast", results)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(event), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	bgpMap, ok := parsed["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp key")
	}

	// Verify peer address and ASN
	peerMap, ok := bgpMap["peer"].(map[string]any)
	if !ok {
		t.Fatal("missing peer key")
	}
	if peerMap["address"] != "10.0.0.1" {
		t.Fatalf("expected peer address=10.0.0.1, got %v", peerMap["address"])
	}
	peerASN, ok := peerMap["asn"].(float64)
	if !ok || peerASN != 65001 {
		t.Fatalf("expected peer asn=65001, got %v", peerMap["asn"])
	}

	msgMap, ok := bgpMap["message"].(map[string]any)
	if !ok {
		t.Fatal("missing message key")
	}
	if msgMap["type"] != "rpki" {
		t.Fatalf("expected type=rpki, got %v", msgMap["type"])
	}
	msgID, ok := msgMap["id"].(float64)
	if !ok || msgID != 42 {
		t.Fatalf("expected id=42, got %v", msgMap["id"])
	}

	rpkiMap, ok := bgpMap["rpki"].(map[string]any)
	if !ok {
		t.Fatal("missing rpki key")
	}
	familyMap, ok := rpkiMap["ipv4/unicast"].(map[string]any)
	if !ok {
		t.Fatal("missing ipv4/unicast key in rpki")
	}

	if familyMap["10.0.1.0/24"] != stateStringValid {
		t.Fatalf("expected valid for 10.0.1.0/24, got %v", familyMap["10.0.1.0/24"])
	}
	if familyMap["10.0.2.0/24"] != stateStringInvalid {
		t.Fatalf("expected invalid for 10.0.2.0/24, got %v", familyMap["10.0.2.0/24"])
	}
	if familyMap["192.168.0.0/16"] != stateStringNotFound {
		t.Fatalf("expected not-found for 192.168.0.0/16, got %v", familyMap["192.168.0.0/16"])
	}
}

func TestBuildRPKIEventSinglePrefix(t *testing.T) {
	results := map[string]uint8{
		"10.0.1.0/24": ValidationValid,
	}

	event := buildRPKIEvent("10.0.0.1", uint32(65001), uint64(1), "ipv4/unicast", results)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(event), &parsed); err != nil {
		t.Fatalf("invalid JSON for single prefix: %v", err)
	}

	bgpMap, ok := parsed["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp key")
	}
	rpkiMap, ok := bgpMap["rpki"].(map[string]any)
	if !ok {
		t.Fatal("missing rpki key")
	}
	familyMap, ok := rpkiMap["ipv4/unicast"].(map[string]any)
	if !ok {
		t.Fatal("missing ipv4/unicast key")
	}

	if familyMap["10.0.1.0/24"] != stateStringValid {
		t.Fatalf("expected valid, got %v", familyMap["10.0.1.0/24"])
	}
}

func TestBuildRPKIEventUnavailable(t *testing.T) {
	event := buildRPKIEventUnavailable("10.0.0.1", uint32(65001), uint64(7))

	var parsed map[string]any
	if err := json.Unmarshal([]byte(event), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	bgpMap, ok := parsed["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp key")
	}

	// Verify peer address and ASN
	peerMap, ok := bgpMap["peer"].(map[string]any)
	if !ok {
		t.Fatal("missing peer key")
	}
	if peerMap["address"] != "10.0.0.1" {
		t.Fatalf("expected peer address=10.0.0.1, got %v", peerMap["address"])
	}

	// rpki field is now always an object: {"status":"unavailable"}
	rpkiMap, ok := bgpMap["rpki"].(map[string]any)
	if !ok {
		t.Fatal("expected rpki to be an object")
	}
	if rpkiMap["status"] != "unavailable" {
		t.Fatalf("expected rpki.status=unavailable, got %v", rpkiMap["status"])
	}
}

func TestBuildRPKIEventWithdrawal(t *testing.T) {
	event := buildRPKIEvent("10.0.0.1", uint32(65001), uint64(3), "ipv4/unicast", nil)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(event), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	bgpMap, ok := parsed["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp key")
	}
	rpkiMap, ok := bgpMap["rpki"].(map[string]any)
	if !ok {
		t.Fatal("missing rpki key")
	}

	// Empty rpki section for withdrawal
	if len(rpkiMap) != 0 {
		t.Fatalf("expected empty rpki section for withdrawal, got %v", rpkiMap)
	}
}

func TestBuildRPKIEventEscaping(t *testing.T) {
	// Verify that special characters in peer address and prefix are properly escaped.
	results := map[string]uint8{
		`10.0.0.0/24"inject`: ValidationValid,
	}

	event := buildRPKIEvent(`peer"addr`, uint32(65001), uint64(1), `ipv4/"unicast`, results)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(event), &parsed); err != nil {
		t.Fatalf("JSON injection: special chars produced invalid JSON: %v", err)
	}

	bgpMap, ok := parsed["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp key")
	}
	peerMap, ok := bgpMap["peer"].(map[string]any)
	if !ok {
		t.Fatal("missing peer key")
	}
	if peerMap["address"] != `peer"addr` {
		t.Fatalf("peer address not preserved through escaping: %v", peerMap["address"])
	}
}

func TestValidationStateString(t *testing.T) {
	tests := []struct {
		state uint8
		want  string
	}{
		{ValidationValid, stateStringValid},
		{ValidationInvalid, stateStringInvalid},
		{ValidationNotFound, stateStringNotFound},
		{ValidationNotValidated, stateStringNotValidated},
		{255, stateStringNotValidated}, // unknown state
	}

	for _, tt := range tests {
		got := validationStateString(tt.state)
		if got != tt.want {
			t.Errorf("validationStateString(%d) = %q, want %q", tt.state, got, tt.want)
		}
	}
}
