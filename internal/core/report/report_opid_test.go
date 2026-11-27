package report

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opid"
)

// VALIDATES: AC-1 -- RaiseErrorContext and RaiseWarningContext attach opid.From(ctx) to Issue.OpID without mutating the caller's detail map.
// PREVENTS: report-bus issues losing operation correlation IDs or mutating caller-owned detail maps.
func TestRaiseContextOpID(t *testing.T) {
	reset()
	t.Cleanup(reset)

	ctx := opid.With(context.Background(), "op-101")
	errDetail := map[string]any{"peer": "192.0.2.1"}
	warnDetail := map[string]any{"family": "ipv4/unicast"}

	RaiseErrorContext(ctx, "command", "doctor-command-handler-hung", "show bgp", "handler hung", "ze show system goroutines mode blocked", errDetail)
	RaiseWarningContext(ctx, "bgp", "prefix-threshold", "192.0.2.1", "prefix count high", "ze show bgp summary", warnDetail)

	if _, mutated := errDetail["operation-id"]; mutated {
		t.Error("RaiseErrorContext mutated caller's detail map")
	}
	if _, mutated := warnDetail["operation-id"]; mutated {
		t.Error("RaiseWarningContext mutated caller's detail map")
	}

	errs := Errors(0)
	if len(errs) != 1 {
		t.Fatalf("Errors len = %d, want 1", len(errs))
	}
	if errs[0].OpID != "op-101" {
		t.Errorf("error Issue.OpID = %q, want op-101", errs[0].OpID)
	}
	if errs[0].Fix != "ze show system goroutines mode blocked" {
		t.Errorf("error Issue.Fix = %q", errs[0].Fix)
	}

	warns := Warnings()
	if len(warns) != 1 {
		t.Fatalf("Warnings len = %d, want 1", len(warns))
	}
	if warns[0].OpID != "op-101" {
		t.Errorf("warning Issue.OpID = %q, want op-101", warns[0].OpID)
	}
}

// VALIDATES: AC-1 -- RaiseError and RaiseWarning promote a string "operation-id" detail entry to Issue.OpID without mutating caller detail.
// PREVENTS: existing detail-carrying producers from missing operation-id correlation.
func TestRaiseDetailOpIDPromotion(t *testing.T) {
	reset()
	t.Cleanup(reset)

	RaiseError("config", "reload-failed", "bgp", "verify refused", map[string]any{"operation-id": "op-202", "reason": "bad"})
	RaiseWarning("config", "reload-slow", "bgp", "verify slow", map[string]any{"operation-id": "op-203"})

	errs := Errors(0)
	if len(errs) != 1 || errs[0].OpID != "op-202" {
		t.Fatalf("promoted error OpID = %+v, want op-202", errs)
	}
	warns := Warnings()
	if len(warns) != 1 || warns[0].OpID != "op-203" {
		t.Fatalf("promoted warning OpID = %+v, want op-203", warns)
	}
}

// VALIDATES: AC-1 -- Issue JSON encodes "operation-id" when set and omits it via omitempty when empty.
// PREVENTS: breaking existing JSON consumers when operation-id is absent or dropping operation-id when present.
func TestIssueOpIDJSON(t *testing.T) {
	reset()
	t.Cleanup(reset)

	RaiseError("bgp", "no-op", "peer-1", "plain error")
	RaiseErrorContext(opid.With(context.Background(), "op-303"), "bgp", "with-op", "peer-2", "correlated error", "")

	errs := Errors(0)
	if len(errs) != 2 {
		t.Fatalf("Errors len = %d, want 2", len(errs))
	}

	withOpJSON, err := json.Marshal(errs[0])
	if err != nil {
		t.Fatalf("marshal with operation-id: %v", err)
	}
	if !strings.Contains(string(withOpJSON), `"operation-id":"op-303"`) {
		t.Errorf("JSON missing operation-id: %s", withOpJSON)
	}

	withoutOpJSON, err := json.Marshal(errs[1])
	if err != nil {
		t.Fatalf("marshal without operation-id: %v", err)
	}
	if strings.Contains(string(withoutOpJSON), `"operation-id"`) {
		t.Errorf("JSON did not omit empty operation-id: %s", withoutOpJSON)
	}
}
