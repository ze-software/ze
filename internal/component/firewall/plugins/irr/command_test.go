package irr

// VALIDATES: show firewall irr answers a JSON object whose entry list is an
// array, and an unknown command is refused.
// PREVENTS: CLI commands returning wrong status or malformed JSON.
// The stored copies the show reports are TestShowReportsTheStoredCopy
// (stored_test.go); fetching and clearing moved to the irr plugin.

import (
	"encoding/json"
	"testing"
)

// VALIDATES: with no reference configured, show firewall irr answers done and
// an empty entry array, never null and never an error.
// METHOD: run the command on a plugin holding no reference and decode the raw
// answer.
// PREVENTS: a script reading `"entries":null` as a malformed answer.
func TestShowIRRCommandEmpty(t *testing.T) {
	plug := &irrPlugin{refs: make(map[string]*refState)}
	status, data, err := plug.handleCommand(cmdShowIRR, nil)
	if err != nil {
		t.Fatalf("show firewall irr error: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want %q", status, statusDone)
	}
	raw, ok := data.(json.RawMessage)
	if !ok {
		t.Fatalf("data type = %T, want json.RawMessage", data)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result %s: %v", raw, err)
	}
	entries, present := result["entries"]
	if !present {
		t.Fatalf("answer %s carries no entries", raw)
	}
	if string(entries) != "[]" {
		t.Errorf("entries = %s, want the empty array []", entries)
	}
}

func TestHandleCommandUnknown(t *testing.T) {
	plug := &irrPlugin{}
	status, _, err := plug.handleCommand("show firewall unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if status != statusError {
		t.Errorf("status = %q, want %q", status, statusError)
	}
}

// showEntries runs "show firewall irr" and returns its entry list.
func showEntries(t *testing.T, plug *irrPlugin) []map[string]any {
	t.Helper()
	status, data, err := plug.handleCommand("show firewall irr", nil)
	if err != nil {
		t.Fatalf("show firewall irr: %v", err)
	}
	if status != statusDone {
		t.Fatalf("status = %q, want %q", status, statusDone)
	}
	raw, ok := data.(json.RawMessage)
	if !ok {
		t.Fatalf("data type = %T, want json.RawMessage", data)
	}
	var result struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return result.Entries
}
