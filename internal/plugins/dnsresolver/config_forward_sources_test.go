package dnsresolver

import (
	"strings"
	"testing"
)

// TestParseForwardSourcesAbsentAndWrongType: an absent forward container
// selects the built-in default list, and a forward value that is not a
// container is refused naming the leaf and the fix. Method: parse both
// shapes and check the answer.
//
// VALIDATES: parseForwardSources reads absence as the default and refuses a
// value it cannot read (spec-home-cpe-23b).
// PREVENTS: a forward value of the wrong type silently selecting the
// default list, as the absence does.
func TestParseForwardSourcesAbsentAndWrongType(t *testing.T) {
	list, err := parseForwardSources(map[string]any{})
	if err != nil {
		t.Fatalf("absent forward: %v", err)
	}
	if !list.Stated() {
		t.Fatal("absent forward: list is not the built-in default")
	}
	if got := list.Path(); got != forwardDNSSourcePath {
		t.Fatalf("absent forward: path = %q, want %q", got, forwardDNSSourcePath)
	}

	_, err = parseForwardSources(map[string]any{"forward": "192.0.2.53"})
	if err == nil {
		t.Fatal("forward as a string: no error, want a refusal")
	}
	if !strings.Contains(err.Error(), "service dns-resolver forward is not a container") {
		t.Fatalf("forward as a string: err = %v, want it to name the leaf", err)
	}
	if !strings.Contains(err.Error(), "ze config validate") {
		t.Fatalf("forward as a string: err = %v, want the fix", err)
	}
}
