//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- declared browser import refusal.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	webtesting "github.com/ze-software/ze/internal/component/web/testing"
)

// TestBrowserGraphicalDeclaredRefusalRecoversRealPaste reads the live journey:
// the malformed paste must be refused atomically, then only its corrected paste
// may count as a staged draft and reach the submitted publication boundary.
func TestBrowserGraphicalDeclaredRefusalRecoversRealPaste(t *testing.T) {
	s, _ := browserCorpusCase(t, "merge", "unused", "")
	s.root = filepath.Join("..", "..", "..", "..")
	path := filepath.Join(s.root, "test", "web", "graphical-import-paste.wb")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := webtesting.ParseWBFile(string(body))
	if err != nil {
		t.Fatal(err)
	}
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("accepted paste compositions = %d, want only the recovered paste: %v", len(sequences), s.report.findings)
	}
	sequence := &sequences[0]
	editor, err := editorcli.NewEditorFromContent([]byte(sequence.base), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := editor.Close(); err != nil {
			t.Errorf("close refusal editor: %v", err)
		}
	})
	before := editor.WorkingContent()
	malformed := ""
	for _, action := range workflow.Actions {
		if action.Kind == "fill" {
			if action.Values["id"] == "graphical-import-paste" {
				malformed = action.Values["value"]
				break
			}
		}
	}
	if malformed == "" {
		t.Fatal("the real recovery journey has no malformed paste")
	}
	if err := loadCorpusStep(editor, &loadStep{body: malformed}); err == nil {
		t.Fatal("the shared editor accepted the journey's malformed paste")
	}
	if editor.WorkingContent() != before {
		t.Fatal("refusing the malformed paste changed the recovery candidate")
	}
	composed, err := s.load(sequence.base, &sequence.steps[0])
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.NewParser(s.schema).Parse(composed)
	if err != nil {
		t.Fatal(err)
	}
	if host, _ := tree.GetContainer("system").Get("host"); host != "graphical-paste-import" {
		t.Errorf("recovered host = %q", host)
	}
	if s.report.refused != 1 {
		t.Errorf("actual declared refusals = %d, want 1", s.report.refused)
	}
	s.judgeSequence(sequence)
	if s.report.stagedDrafts != 1 {
		t.Errorf("staged drafts = %d, want only the successful recovery", s.report.stagedDrafts)
	}
	if s.report.zeConfigs == 0 {
		t.Error("recovery never reached the real submitted commit's full validation")
	}
	if len(s.report.findings) != 0 {
		t.Fatalf("declared refusal or later recovery was misjudged: %v", s.report.findings)
	}
}

// TestBrowserGraphicalRefusalDeclarationMustMatch preserves findings when the
// actual import error is not declared immediately after that parsed submission.
func TestBrowserGraphicalRefusalDeclarationMustMatch(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	body, err := os.ReadFile(filepath.Join(root, "test", "web", "graphical-import-paste.wb"))
	if err != nil {
		t.Fatal(err)
	}
	declaration := "expect=element:text=The configuration could not be imported.\n"
	submit := "action=click:id=graphical-import-paste-submit\n"
	if strings.Count(string(body), declaration) != 1 {
		t.Fatal("the real journey no longer has its one declared import refusal")
	}
	for _, control := range []struct {
		name string
		body string
	}{
		{name: "undeclared", body: strings.Replace(string(body), declaration, "", 1)},
		{name: "wrong-text", body: strings.Replace(string(body), declaration, "expect=element:text=unrelated failure\n", 1)},
		{name: "partial-text", body: strings.Replace(string(body), declaration, "expect=element:text=configuration\n", 1)},
		{name: "wrong-kind", body: strings.Replace(string(body), declaration, "expect=html:contains=The configuration could not be imported.\n", 1)},
		{name: "extra-qualifier", body: strings.Replace(string(body), declaration, "expect=element:text=The configuration could not be imported.:id=unrelated-error\n", 1)},
		{name: "preceding-expectation", body: strings.Replace(string(body), submit+declaration, declaration+submit, 1)},
		{name: "intervening-action", body: strings.Replace(string(body), declaration, "action=wait:ms=1\n"+declaration, 1)},
	} {
		t.Run(control.name, func(t *testing.T) {
			s, path := browserCorpusCase(t, "merge", "unused", "")
			if err := os.WriteFile(path, []byte(control.body), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, sequence := range s.browserSequences(path, map[string]bool{}) {
				s.judgeSequence(&sequence)
			}
			if len(s.report.findings) == 0 {
				t.Fatal("an unmatched refusal declaration hid the actual import failure")
			}
		})
	}
}
