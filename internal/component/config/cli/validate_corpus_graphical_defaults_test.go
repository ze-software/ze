//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- shared native import defaults.

package cli

import (
	"os"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/web"
)

// TestBrowserGraphicalUnselectedChoicesUseNativeDefaults supplies no select
// actions, so the real native default authority must govern replay composition.
func TestBrowserGraphicalUnselectedChoicesUseNativeDefaults(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "unused", "")
	location, operation := web.GraphicalImportDefaults()
	body := "group default-probe { description defaults-imported; }"
	var wantPath []string
	switch location {
	case "absolute":
		body = "bgp { " + body + " }"
	case "relative":
		wantPath = []string{"bgp"}
	default:
		t.Fatalf("native default location %q has no shared load grammar", location)
	}
	workflow := "option=server:kind=daemon\naction=open:path=/show/bgp/\n" +
		"action=fill:id=graphical-import-paste:value=" + body + "\n" +
		"action=click:id=graphical-import-paste-submit\n"
	if err := os.WriteFile(path, []byte(workflow), 0o600); err != nil {
		t.Fatal(err)
	}
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("unselected native choices produced %d sequences: %v", len(sequences), s.report.findings)
	}
	step := &sequences[0].steps[0]
	if !slices.Equal(step.path, wantPath) {
		t.Errorf("replayed default scope = %v, native authority requires %v", step.path, wantPath)
	}
	if step.replace != (operation == "replace") {
		t.Errorf("replayed replace = %v for native default %q", step.replace, operation)
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
	group := bgp.GetList("group")["default-probe"]
	if group == nil {
		t.Fatal("native defaults lost the supplied positive import")
	}
	if description, _ := group.Get("description"); description != "defaults-imported" {
		t.Errorf("default import description = %q", description)
	}
	_, retainedRouter := bgp.Get("router-id")
	if retainedRouter != (operation == "merge") {
		t.Errorf("baseline router retained = %v for native default %q", retainedRouter, operation)
	}
	s.judgeSequence(&sequences[0])
	if s.report.stagedDrafts != 1 {
		t.Errorf("default import drafts = %d", s.report.stagedDrafts)
	}
	if len(s.report.findings) != 0 {
		t.Fatalf("native defaults refused: %v", s.report.findings)
	}
}
