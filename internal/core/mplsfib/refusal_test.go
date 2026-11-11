package mplsfib

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// TestRefusalsReportsAndClears proves a Refusals record keeps one entry per
// data-plane slot with the writer's cause, renders each as an ERROR finding
// under doctor-fib-mpls-no-writer with the source's fix, orders swaps and
// pops by in-label before pushes by FEC, and forgets an entry once Clear,
// ClearIf or Reset says the writer carried it out.
//
// VALIDATES: AC-22 -- every label source reports the entries no writer
// installed with their cause, through one shared record.
// METHOD: record a push, a pop and a swap, read the findings, then clear them
// one way at a time.
func TestRefusalsReportsAndClears(t *testing.T) {
	var refusals Refusals
	if got := refusals.Findings("ospf-sr", "fix it"); got != nil {
		t.Fatalf("empty record answered %v", got)
	}
	push := Entry{Action: ActionAdd, Op: OpPush, FEC: netip.MustParsePrefix("10.0.0.9/32"), OutLabels: []uint32{16009}}
	pop := Entry{Action: ActionAdd, Op: OpPop, InLabel: 24001}
	swap := Entry{Action: ActionRemove, Op: OpSwap, InLabel: 16005}
	refusals.Record(&push, errors.New("vpp: mpls table 0 missing"))
	refusals.Record(&pop, errors.New("in-label owned by source 2"))
	refusals.Record(&swap, errors.New("netlink: no such device"))

	findings := refusals.Findings("ospf-sr", "fix it")
	if len(findings) != 3 {
		t.Fatalf("want 3 findings, got %d: %v", len(findings), findings)
	}
	for i := range findings {
		if findings[i].Code != CodeDoctorNoWriter {
			t.Errorf("finding %d code %q", i, findings[i].Code)
		}
		if findings[i].Severity != "error" {
			t.Errorf("finding %d severity %q", i, findings[i].Severity)
		}
		if findings[i].Fix != "fix it" {
			t.Errorf("finding %d fix %q", i, findings[i].Fix)
		}
		if !strings.HasPrefix(findings[i].Message, "ospf-sr: ") {
			t.Errorf("finding %d does not name its source: %q", i, findings[i].Message)
		}
	}
	wantOrder := []string{"swap of in-label 16005", "pop of in-label 24001", "push for FEC 10.0.0.9/32"}
	for i, want := range wantOrder {
		if !strings.Contains(findings[i].Message, want) {
			t.Errorf("finding %d: want %q in %q", i, want, findings[i].Message)
		}
	}
	if !strings.Contains(findings[0].Message, "no such device") {
		t.Errorf("the writer's cause is not named: %q", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "still hold") {
		t.Errorf("a refused removal is not reported as still held: %q", findings[0].Message)
	}
	if !strings.Contains(findings[2].Message, "does not hold") {
		t.Errorf("a refused add is not reported as missing: %q", findings[2].Message)
	}

	// A later apply of the same slot, whatever its action, clears it.
	refusals.Clear(&Entry{Action: ActionRemove, Op: OpPush, FEC: push.FEC})
	refusals.ClearIf(func(e *Entry) bool { return e.Op == OpSwap })
	findings = refusals.Findings("ospf-sr", "fix it")
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "in-label 24001") {
		t.Fatalf("after Clear and ClearIf want only the pop, got %v", findings)
	}
	refusals.Reset()
	if got := refusals.Findings("ospf-sr", "fix it"); got != nil {
		t.Fatalf("after Reset want nothing, got %v", got)
	}
}

// TestRefusalsOneSlotPerInLabel proves a swap and a pop of one in-label share
// a slot: a writer holds one entry per in-label, so the later record replaces
// the earlier one rather than reporting both.
//
// VALIDATES: a SID label that moves between swap and pop is reported once.
// METHOD: record a pop then a swap of the same in-label.
func TestRefusalsOneSlotPerInLabel(t *testing.T) {
	var refusals Refusals
	refusals.Record(&Entry{Action: ActionAdd, Op: OpPop, InLabel: 16001}, errors.New("first"))
	refusals.Record(&Entry{Action: ActionAdd, Op: OpSwap, InLabel: 16001}, errors.New("second"))
	findings := refusals.Findings("ldp", "fix it")
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "swap of in-label 16001") || !strings.Contains(findings[0].Message, "second") {
		t.Fatalf("want the later swap, got %q", findings[0].Message)
	}
}

// TestRefusalsBoundsFindings proves one answer lists at most
// refusalFindingsMax entries and counts the rest in one more finding.
//
// VALIDATES: the check stays bounded when every FEC of a large IGP is refused.
// METHOD: record refusalFindingsMax+5 pops.
func TestRefusalsBoundsFindings(t *testing.T) {
	var refusals Refusals
	for i := range refusalFindingsMax + 5 {
		refusals.Record(&Entry{Action: ActionAdd, Op: OpPop, InLabel: uint32(16 + i)}, errors.New("no writer acknowledged"))
	}
	findings := refusals.Findings("ldp", "fix it")
	if len(findings) != refusalFindingsMax+1 {
		t.Fatalf("want %d findings, got %d", refusalFindingsMax+1, len(findings))
	}
	last := findings[refusalFindingsMax]
	if !strings.Contains(last.Message, "5 more") || last.Code != CodeDoctorNoWriter {
		t.Fatalf("summary finding: %+v", last)
	}
}
