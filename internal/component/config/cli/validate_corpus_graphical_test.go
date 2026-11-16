//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- authenticated native form replay.

package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestBrowserGraphicalFileUsesSelectedCandidate replays the actual browser case,
// including its authenticated owner, selected slash key and partial file.
func TestBrowserGraphicalFileUsesSelectedCandidate(t *testing.T) {
	s, _ := browserCorpusCase(t, "merge", "unused", "")
	s.root = filepath.Join("..", "..", "..", "..")
	path := filepath.Join(s.root, "test/web/graphical-import-file.wb")
	loaded := map[string]bool{}
	sequences := s.browserSequences(path, loaded)
	if len(sequences) != 1 {
		t.Fatalf("native file compositions = %d, want 1: %v", len(sequences), s.report.findings)
	}
	step := &sequences[0].steps[0]
	if !slices.Equal(step.path, []string{"bgp", "group", "graphical/file"}) || !step.replace {
		t.Fatalf("native form scope = %v, replace = %v", step.path, step.replace)
	}
	fixture := filepath.Join(s.root, "test/web/fixtures/graphical-import-relative.conf")
	if step.source != fixture || !loaded[fixture] {
		t.Fatalf("selected physical input = %q, loaded = %v", step.source, loaded)
	}
	composed, err := s.load(sequences[0].base, step)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.NewParser(s.schema).Parse(composed)
	if err != nil {
		t.Fatal(err)
	}
	bgp := tree.GetContainer("bgp")
	group := bgp.GetList("group")["graphical/file"]
	if group == nil {
		t.Fatal("selected slash-key group was lost")
	}
	if description, _ := group.Get("description"); description != "file-subtree-import" {
		t.Errorf("imported description = %q", description)
	}
	if group.GetContainer("session") != nil {
		t.Error("selected replacement retained the omitted remote ASN")
	}
	retained := bgp.GetList("group")["retained-import"]
	if retained == nil {
		t.Fatal("selected replacement removed an unrelated candidate group")
	}
	if description, _ := retained.Get("description"); description != "unrelated-candidate" {
		t.Errorf("preceding candidate mutation = %q", description)
	}
	s.judgeSequence(&sequences[0])
	if len(s.report.findings) != 0 || s.report.stagedDrafts != 1 {
		t.Fatalf("native form judgment: drafts=%d findings=%v", s.report.stagedDrafts, s.report.findings)
	}
}

// TestBrowserGraphicalPasteDerivesFormChoices keeps all four native form
// combinations tied to the parsed selection and the shared editor load.
func TestBrowserGraphicalPasteDerivesFormChoices(t *testing.T) {
	for _, location := range []string{"absolute", "relative"} {
		for _, operation := range []string{"merge", "replace"} {
			t.Run(location+"/"+operation, func(t *testing.T) {
				s, path := browserCorpusCase(t, "merge", "unused", "")
				body := "group imported { description pasted; }"
				if location == "absolute" {
					body = "bgp { " + body + " }"
				}
				browserGraphicalWorkflow(t, path, "action=login:user=admin:password=admin-fixture\n"+
					"action=open:path=/show/bgp/\n"+
					"action=click:id=graphical-actions-toggle\n"+
					"action=click:id=graphical-edit-toggle\n"+
					"action=click:id=graphical-import-toggle\n"+
					"action=fill:id=graphical-import-paste:value="+body+"\n"+
					"action=select:id=graphical-import-paste-location:value="+location+"\n"+
					"action=select:id=graphical-import-paste-action:value="+operation+"\n"+
					"action=click:id=graphical-import-paste-submit\n")
				sequences := s.browserSequences(path, map[string]bool{})
				if len(sequences) != 1 {
					t.Fatalf("native paste composition missing: %v", s.report.findings)
				}
				step := &sequences[0].steps[0]
				wantPath := []string(nil)
				if location == "relative" {
					wantPath = []string{"bgp"}
				}
				if !slices.Equal(step.path, wantPath) || step.replace != (operation == "replace") {
					t.Fatalf("native choices lost: path=%v replace=%v", step.path, step.replace)
				}
				composed, err := s.load(sequences[0].base, step)
				if err != nil {
					t.Fatal(err)
				}
				tree, err := config.NewParser(s.schema).Parse(composed)
				if err != nil {
					t.Fatal(err)
				}
				bgp := tree.GetContainer("bgp")
				if bgp.GetList("group")["imported"] == nil {
					t.Fatal("native paste did not import its positive content")
				}
				_, router := bgp.Get("router-id")
				if router != (operation == "merge") {
					t.Errorf("baseline router-id present=%v for %s", router, operation)
				}
			})
		}
	}
}

// TestBrowserAuthenticatedOwnerKeepsCandidateAcrossPages follows the same
// authenticated owner from CLI to the graphical editor and back to the CLI.
func TestBrowserAuthenticatedOwnerKeepsCandidateAcrossPages(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group imported description physical-input\n", "")
	browserGraphicalWorkflow(t, path,
		"action=login:user=admin:password=admin-fixture\naction=open:path=/cli\n"+
			"action=fill:id=terminal-input:value=set bgp group owner description original-owner\naction=press:id=terminal-input:key=Enter\n"+
			"action=open:path=/show/bgp/group/owner/\n"+
			"action=click:id=graphical-actions-toggle\n"+
			"action=open:path=/cli\n"+
			"action=fill:id=terminal-input:value=load file absolute merge label.conf\naction=press:id=terminal-input:key=Enter\n"+
			"action=upload:id=cli-load-file:file=fragment.conf\naction=click:id=cli-load-apply\n")
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("authenticated load missing: %v", s.report.findings)
	}
	tree, err := config.NewParser(s.schema).Parse(sequences[0].base)
	if err != nil {
		t.Fatal(err)
	}
	groups := tree.GetContainer("bgp").GetList("group")
	if groups["owner"] == nil {
		t.Fatal("page navigation lost the authenticated owner's candidate")
	}
	if description, _ := groups["owner"].Get("description"); description != "original-owner" {
		t.Errorf("resumed owner description = %q", description)
	}
}

// TestBrowserAuthenticatedBadPasswordKeepsStandaloneValidation requires a
// declared identity's actual password to authenticate before load composition.
func TestBrowserAuthenticatedBadPasswordKeepsStandaloneValidation(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group forbidden description denied\n", "")
	browserGraphicalWorkflow(t, path,
		"action=login:user=admin:password=incorrect-fixture\naction=open:path=/cli\n"+
			"action=fill:id=terminal-input:value=load file absolute merge label.conf\naction=press:id=terminal-input:key=Enter\n"+
			"action=upload:id=cli-load-file:file=fragment.conf\naction=click:id=cli-load-apply\n")
	loaded := map[string]bool{}
	if sequences := s.browserSequences(path, loaded); len(sequences) != 0 {
		t.Fatal("incorrect password composed a candidate mutation")
	}
	if !strings.Contains(strings.Join(s.report.findings, "\n"), "authentication failed") {
		t.Fatalf("the declared password was not checked: %v", s.report.findings)
	}
	if loaded[filepath.Join(s.root, "fragment.conf")] {
		t.Fatal("failed authentication consumed the standalone input oracle")
	}
}

// TestBrowserAuthenticatedRepeatedLoginIsUnsupported preserves the runner's
// real login-form constraint: an authenticated page offers no second login.
func TestBrowserAuthenticatedRepeatedLoginIsUnsupported(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group forbidden description denied\n", "")
	browserGraphicalWorkflow(t, path,
		"action=login:user=admin:password=admin-fixture\n"+
			"action=login:user=other:password=other-fixture\naction=open:path=/cli\n"+
			"action=fill:id=terminal-input:value=load file absolute merge label.conf\naction=press:id=terminal-input:key=Enter\n"+
			"action=upload:id=cli-load-file:file=fragment.conf\naction=click:id=cli-load-apply\n")
	loaded := map[string]bool{}
	if sequences := s.browserSequences(path, loaded); len(sequences) != 0 {
		t.Fatal("impossible repeated login silently switched editor ownership")
	}
	if !strings.Contains(strings.Join(s.report.findings, "\n"), "already authenticated") {
		t.Fatalf("the browser login-form limitation was hidden: %v", s.report.findings)
	}
	if loaded[filepath.Join(s.root, "fragment.conf")] {
		t.Fatal("unsupported second login consumed the standalone input oracle")
	}
}

// TestBrowserAuthenticatedReaderCannotComposeMutation requires actual policy
// refusal and preserves the physical input's independent validation oracle.
func TestBrowserAuthenticatedReaderCannotComposeMutation(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group forbidden description denied\n", "")
	browserGraphicalWorkflow(t, path,
		"action=login:user=noc:password=reader-fixture\naction=open:path=/cli\n"+
			"action=fill:id=terminal-input:value=load file absolute merge label.conf\naction=press:id=terminal-input:key=Enter\n"+
			"action=upload:id=cli-load-file:file=fragment.conf\naction=click:id=cli-load-apply\n")
	loaded := map[string]bool{}
	if sequences := s.browserSequences(path, loaded); len(sequences) != 0 {
		t.Fatal("read-only owner composed a candidate mutation")
	}
	if !strings.Contains(strings.Join(s.report.findings, "\n"), "authorization denied") {
		t.Fatalf("declared edit denial was not evaluated: %v", s.report.findings)
	}
	if loaded[filepath.Join(s.root, "fragment.conf")] {
		t.Fatal("denied load consumed the standalone input oracle")
	}
}

func browserGraphicalWorkflow(t *testing.T, path, actions string) {
	t.Helper()
	body := "option=server:kind=daemon\n" +
		"option=auth:user=admin:password=admin-fixture:role=admin\n" +
		"option=auth:user=other:password=other-fixture:role=admin\n" +
		"option=auth:user=noc:password=reader-fixture:role=read-only\n" + actions
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
