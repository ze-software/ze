//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- draft and commit boundaries.

package cli

import (
	"os"
	"strings"
	"testing"

	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
)

// TestBrowserDraftStagingKeepsOriginalUntilCommitValidation distinguishes an
// accepted intermediate editor draft from a configuration permitted to commit.
func TestBrowserDraftStagingKeepsOriginalUntilCommitValidation(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group \"candidate stage?\" description staged\n", "")
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("no draft sequence: %v", s.report.findings)
	}
	sequence := &sequences[0]
	editor, err := editorcli.NewEditorFromContent([]byte(sequence.base), "-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := editor.Close(); err != nil {
			t.Error(err)
		}
	})
	before := editor.OriginalContent()
	if err := loadCorpusStep(editor, &sequence.steps[0]); err != nil {
		t.Fatalf("shared editor refused the intermediate draft: %v", err)
	}
	if editor.OriginalContent() != before {
		t.Fatal("staging changed committed content")
	}
	candidate := editor.WorkingContent()
	tree, err := config.NewParser(s.schema).Parse(candidate)
	if err != nil {
		t.Fatal(err)
	}
	group := tree.GetContainer("bgp").GetList("group")["candidate stage?"]
	if group == nil {
		t.Fatal("accepted intermediate draft was dropped")
	}
	if description, _ := group.Get("description"); description != "staged" {
		t.Errorf("draft description = %q", description)
	}
	s.judgeSequence(sequence)
	if len(s.report.findings) != 0 {
		t.Errorf("accepted staging was treated as publication: %v", s.report.findings)
	}
	if s.report.stagedDrafts != 1 {
		t.Errorf("accepted staged drafts = %d, want 1", s.report.stagedDrafts)
	}
	commit := &sweepContext{t: t, schema: s.schema, report: &sweepReport{hostVerdicts: map[string]int{}}}
	commit.checkBody(sweepCarrier{source: "attempted commit", body: candidate})
	if !strings.Contains(strings.Join(commit.report.findings, "\n"), "config-bgp-resolve") {
		t.Errorf("full publication validator accepted the engine-invalid candidate: %v", commit.report.findings)
	}
}

// TestBrowserCommitBoundaryChecksAppliedCandidate follows parsed commit input
// after a load and requires full validation of that exact candidate snapshot.
func TestBrowserCommitBoundaryChecksAppliedCandidate(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group \"invalid at commit?\" description staged\n", "")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, []byte("action=fill:id=terminal-input:value=commit now\naction=press:id=terminal-input:key=Enter\n")...)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	s.browserSequences(path, map[string]bool{})
	if !strings.Contains(strings.Join(s.report.findings, "\n"), "config-bgp-resolve") {
		t.Errorf("parsed commit boundary was not fully validated: %v", s.report.findings)
	}
}

// TestBrowserCommitBoundaryCountsValidatedCandidate pairs the invalid-commit
// control with a submitted, fully valid candidate counted as a configuration.
func TestBrowserCommitBoundaryCountsValidatedCandidate(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group valid-commit description published\n", "")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, []byte("action=fill:id=terminal-input:value=commit now\naction=press:id=terminal-input:key=Enter\n")...)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("submitted commit lost its preceding load: %v", s.report.findings)
	}
	if s.report.zeConfigs != 1 {
		t.Errorf("submitted commit configurations = %d, want 1", s.report.zeConfigs)
	}
	if len(s.report.findings) != 0 {
		t.Errorf("fully valid submitted candidate was refused: %v", s.report.findings)
	}
}

// TestBrowserFilledCommitIsNotSubmitted proves that typing commit without an
// Enter action cannot turn a staged intermediate draft into a committed config.
func TestBrowserFilledCommitIsNotSubmitted(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group \"still staged?\" description staged\n", "")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, []byte("action=fill:id=terminal-input:value=commit now\n")...)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("draft sequence = %v; findings = %v", sequences, s.report.findings)
	}
	s.judgeSequence(&sequences[0])
	if s.report.zeConfigs != 0 {
		t.Error("unsubmitted text was treated as a commit")
	}
	if s.report.stagedDrafts != 1 {
		t.Errorf("accepted staged drafts = %d, want 1", s.report.stagedDrafts)
	}
	if len(s.report.findings) != 0 {
		t.Errorf("unsubmitted text caused publication validation: %v", s.report.findings)
	}
}

// TestBrowserInvalidTypedDraftIsNotCounted requires an invalid typed value to
// remain a refused load, outside the accepted-draft count.
func TestBrowserInvalidTypedDraftIsNotCounted(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp router-id invalid-address\n", "")
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("invalid load was not retained for judgment: %v", s.report.findings)
	}
	s.judgeSequence(&sequences[0])
	if s.report.stagedDrafts != 0 {
		t.Error("a refused typed value was counted as an accepted draft")
	}
	if len(s.report.findings) == 0 {
		t.Error("invalid typed load lost its refusal")
	}
}
