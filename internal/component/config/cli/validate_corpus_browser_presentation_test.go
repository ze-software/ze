//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- browser refusal recovery.

package cli

import (
	"path/filepath"
	"testing"

	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	webtesting "github.com/ze-software/ze/internal/component/web/testing"
)

// TestBrowserCorpusErrorDismissalsPermitRecoveredPaste isolates each native
// presentation action between an actual refused import and its corrected load.
// Each case must count only the successful draft, preserving the refusal.
func TestBrowserCorpusErrorDismissalsPermitRecoveredPaste(t *testing.T) {
	for _, test := range []struct {
		name   string
		action string
	}{
		{name: "drawer", action: "action=click:id=error-toggle"},
		{name: "toast", action: `action=click:text=button "x"`},
		{name: "settlement", action: `action=wait-html:contains=id="notification-area" class="notification-area"></div>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, path := browserCorpusCase(t, "merge", "unused", "")
			browserGraphicalWorkflow(t, path,
				"action=login:user=admin:password=admin-fixture\n"+
					"action=open:path=/show/\n"+
					"action=fill:id=graphical-import-paste:value=system { host \"unfinished; }\n"+
					"action=click:id=graphical-import-paste-submit\n"+
					"expect=element:text=load: the configuration input could not be parsed\n"+
					test.action+"\n"+
					"action=fill:id=graphical-import-paste:value=system { host recovered-paste; }\n"+
					"action=click:id=graphical-import-paste-submit\n")
			loaded := map[string]bool{}
			sequences := s.browserSequences(path, loaded)
			if len(s.report.findings) != 0 {
				t.Fatalf("presentation interrupted recovery: %v", s.report.findings)
			}
			if s.report.refused != 1 {
				t.Fatalf("matched native refusals = %d, want 1", s.report.refused)
			}
			if len(sequences) != 1 {
				t.Fatalf("recovered load sequences = %d, want 1", len(sequences))
			}
			sequence := &sequences[0]
			if len(sequence.steps) != 1 {
				t.Fatalf("recovered load steps = %d, want 1", len(sequence.steps))
			}
			base, err := config.NewParser(s.schema).Parse(sequence.base)
			if err != nil {
				t.Fatal(err)
			}
			if host, exists := base.GetContainer("system").Get("host"); exists {
				t.Fatalf("a refused paste added host %q to the next load's starting candidate", host)
			}
			content, err := s.load(sequence.base, &sequence.steps[0])
			if err != nil {
				t.Fatal(err)
			}
			tree, err := config.NewParser(s.schema).Parse(content)
			if err != nil {
				t.Fatal(err)
			}
			if host, _ := tree.GetContainer("system").Get("host"); host != "recovered-paste" {
				t.Fatalf("recovered host = %q, want recovered-paste", host)
			}
			if router, _ := tree.GetContainer("bgp").Get("router-id"); router != "1.2.3.4" {
				t.Fatalf("the recovered paste changed baseline router-id to %q", router)
			}
			s.judgeSequence(sequence)
			if s.report.stagedDrafts != 1 {
				t.Errorf("staged drafts = %d, want 1", s.report.stagedDrafts)
			}
			if s.report.zeConfigs != 0 {
				t.Errorf("unpublished recovery counted %d published configurations", s.report.zeConfigs)
			}
			if len(s.report.findings) != 0 {
				t.Errorf("recovered draft findings: %v", s.report.findings)
			}
			if len(loaded) != 0 {
				t.Errorf("presentation or paste consumed a physical standalone input: %v", loaded)
			}
		})
	}
}

// TestBrowserCorpusUnknownPresentationControlsStayRefused keeps the click
// allowlist exact and proves a refused control cannot consume a pending file,
// mutate the candidate or erase the command the operator has not submitted.
func TestBrowserCorpusUnknownPresentationControlsStayRefused(t *testing.T) {
	for _, action := range []string{
		"action=click:id=error-toggle-save",
		`action=click:text=button "Delete"`,
		`action=click:id=unrecognized:text=button "x"`,
		`action=click:id=:text=button "x"`,
	} {
		t.Run(action, func(t *testing.T) {
			workflow, err := webtesting.ParseWBFile(action + "\n")
			if err != nil {
				t.Fatal(err)
			}
			if len(workflow.Actions) != 1 {
				t.Fatalf("parsed actions = %d, want 1", len(workflow.Actions))
			}
			editor, err := editorcli.NewEditorFromContent([]byte(validConfig), "presentation-control")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := editor.Close(); err != nil {
					t.Errorf("close presentation-control editor: %v", err)
				}
			})
			before := editor.WorkingContent()
			pending := &browserCorpusLoad{source: "file", payload: true, step: loadStep{
				source: filepath.Join(t.TempDir(), "pending.conf"), body: "system { host pending; }",
			}}
			reader := &browserCorpus{editor: editor, context: t.Context(), input: "unsubmitted command", pending: pending}
			loaded := map[string]bool{}
			if err := reader.action(t.TempDir(), "presentation-control", &workflow.Actions[0], loaded); err == nil {
				t.Fatal("an unrecognized presentation control was accepted")
			}
			if editor.WorkingContent() != before {
				t.Fatal("the refused control changed the candidate")
			}
			if reader.pending != pending {
				t.Fatal("the refused control replaced the pending file")
			}
			if reader.input != "unsubmitted command" {
				t.Fatal("the refused control replaced the unsubmitted command")
			}
			if len(reader.sequences) != 0 {
				t.Fatal("the refused control created a load sequence")
			}
			if len(loaded) != 0 {
				t.Fatal("the refused control consumed the standalone file oracle")
			}
		})
	}
}
