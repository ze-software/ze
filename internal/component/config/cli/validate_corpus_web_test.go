//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- browser load composition.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/test/labconfig"
)

// TestSweepBrowserUploadUsesDaemonComposition reads the real browser upload
// carrier and its deliberately partial fragment through the repository sweep.
// The merge must be validated over the same complete base the daemon runs.
func TestSweepBrowserUploadUsesDaemonComposition(t *testing.T) {
	report := runSweep(t, ruleNone, sweepKeep{
		"test/web/cli-load-browser-file.wb", "test/web/fixtures/cli-load.conf",
	})
	if report.sequences != 1 {
		t.Fatalf("browser carrier produced %d load sequences, want 1; findings: %v", report.sequences, report.findings)
	}
	if report.steps != 1 {
		t.Errorf("browser carrier produced %d load steps, want 1", report.steps)
	}
	for _, finding := range report.findings {
		if strings.Contains(finding, "cli-load") {
			t.Error(finding)
		}
	}
}

// TestBrowserUploadUsesPhysicalFile binds the browser chooser's selected bytes,
// not the informational filename typed in the load command.
func TestBrowserUploadUsesPhysicalFile(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group corpus-file description uploaded\n", "")
	loaded := map[string]bool{}
	sequences := s.browserSequences(path, loaded)
	if len(sequences) != 1 {
		t.Fatalf("sequences = %v, findings = %v", sequences, s.report.findings)
	}
	step := sequences[0].steps[0]
	if step.source != filepath.Join(s.root, "fragment.conf") {
		t.Errorf("selected source = %q", step.source)
	}
	if step.body != "set bgp group corpus-file description uploaded\n" {
		t.Errorf("selected bytes = %q", step.body)
	}
	if !loaded[filepath.Join(s.root, "fragment.conf")] {
		t.Error("the successfully composed physical file was not recorded")
	}
	s.judgeSequence(&sequences[0])
	if len(s.report.findings) != 0 {
		t.Errorf("valid browser merge refused: %v", s.report.findings)
	}
}

// TestBrowserUploadInvalidFragmentIsRefused keeps schema failures visible when
// the uploaded file is validated as a merge instead of a standalone config.
func TestBrowserUploadInvalidFragmentIsRefused(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp router-id invalid-address\n", "")
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("no composed invalid fragment: %v", s.report.findings)
	}
	s.judgeSequence(&sequences[0])
	if len(s.report.findings) == 0 {
		t.Fatal("invalid uploaded router-id passed corpus validation")
	}
	if !strings.Contains(strings.Join(s.report.findings, "\n"), "fragment.conf") {
		t.Errorf("refusal lost its physical source: %v", s.report.findings)
	}
}

// TestBrowserUploadReplaceDoesNotInheritBaseline proves a replacement is not
// judged as a merge that borrows mandatory settings from the old configuration.
func TestBrowserUploadReplaceDoesNotInheritBaseline(t *testing.T) {
	s, path := browserCorpusCase(t, "replace", "set bgp group replacement description incomplete\n", "")
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("no replacement sequence: %v", s.report.findings)
	}
	composed, err := s.load(sequences[0].base, &sequences[0].steps[0])
	if err != nil {
		t.Fatalf("shared editor refused the replacement draft: %v", err)
	}
	tree, err := config.NewParser(s.schema).Parse(composed)
	if err != nil {
		t.Fatal(err)
	}
	bgp := tree.GetContainer("bgp")
	if _, present := bgp.Get("router-id"); present {
		t.Error("replacement inherited the baseline router-id")
	}
	if session := bgp.GetContainer("session"); session != nil {
		if asn := session.GetContainer("asn"); asn != nil {
			if _, present := asn.Get("local"); present {
				t.Error("replacement inherited the baseline local ASN")
			}
		}
	}
	if bgp.GetList("peer")["peer1"] != nil {
		t.Error("replacement inherited the baseline peer")
	}
	group := bgp.GetList("group")["replacement"]
	if group == nil {
		t.Fatal("replacement draft lost its supplied group")
	}
	if value, _ := group.Get("description"); value != "incomplete" {
		t.Errorf("replacement description = %q", value)
	}
	s.judgeSequence(&sequences[0])
	if len(s.report.findings) != 0 {
		t.Errorf("accepted replacement draft was refused: %v", s.report.findings)
	}
	if s.report.stagedDrafts != 1 {
		t.Errorf("accepted replacement drafts = %d, want 1", s.report.stagedDrafts)
	}
	s.checkBody(sweepCarrier{source: "replacement publication", body: composed})
	for _, field := range []string{"bgp/router-id", "bgp/session/asn/local"} {
		if !strings.Contains(strings.Join(s.report.findings, "\n"), field) {
			t.Errorf("publication did not name its missing %s: %v", field, s.report.findings)
		}
	}
}

// TestBrowserUploadUnknownCompositionKeepsStandaloneValidation refuses a
// preceding graphical mutation instead of silently assuming the daemon base.
func TestBrowserUploadUnknownCompositionKeepsStandaloneValidation(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group unknown description input\n", "action=click:id=unknown-mutation\n")
	loaded := map[string]bool{}
	if sequences := s.browserSequences(path, loaded); len(sequences) != 0 {
		t.Errorf("unknown composition produced a sequence: %v", sequences)
	}
	if len(s.report.findings) == 0 {
		t.Error("unknown composition was silently accepted")
	}
	if loaded[filepath.Join(s.root, "fragment.conf")] {
		t.Error("unknown composition removed the standalone validation oracle")
	}
}

// TestBrowserUploadUnknownPageKeepsStandaloneValidation refuses a path-bearing
// CLI handoff rather than guessing which context or candidate it established.
func TestBrowserUploadUnknownPageKeepsStandaloneValidation(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group unknown-page description input\n", "action=open:path=/cli?path=bgp\n")
	loaded := map[string]bool{}
	if sequences := s.browserSequences(path, loaded); len(sequences) != 0 {
		t.Errorf("unknown page produced a sequence: %v", sequences)
	}
	if !strings.Contains(strings.Join(s.report.findings, "\n"), "/cli?path=bgp") {
		t.Errorf("unknown page was silently accepted: %v", s.report.findings)
	}
	if loaded[filepath.Join(s.root, "fragment.conf")] {
		t.Error("unknown page removed the standalone validation oracle")
	}
}

// TestBrowserPasteUsesRelativeCommandContext replays the parsed edit and set
// actions before loading a hierarchical body at the resulting context.
func TestBrowserPasteUsesRelativeCommandContext(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "unused", "")
	body := "option=server:kind=daemon\naction=open:path=/cli\n" +
		"action=fill:id=terminal-input:value=edit bgp\naction=press:id=terminal-input:key=Enter\n" +
		"action=fill:id=terminal-input:value=set router-id 192.0.2.123\naction=press:id=terminal-input:key=Enter\n" +
		"action=fill:id=terminal-input:value=load terminal relative merge\naction=press:id=terminal-input:key=Enter\n" +
		"action=fill:id=cli-load-text:value=group corpus-paste { description pasted; }\n" +
		"action=click:id=cli-load-apply\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	sequences := s.browserSequences(path, map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("paste sequence = %v; findings = %v", sequences, s.report.findings)
	}
	composed, err := s.load(sequences[0].base, &sequences[0].steps[0])
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.NewParser(s.schema).Parse(composed)
	if err != nil {
		t.Fatal(err)
	}
	bgp := tree.GetContainer("bgp")
	if router, _ := bgp.Get("router-id"); router != "192.0.2.123" {
		t.Errorf("preceding set was lost: %q", router)
	}
	group := bgp.GetList("group")["corpus-paste"]
	if group == nil {
		t.Fatal("relative paste did not create its group under bgp")
	}
	if description, _ := group.Get("description"); description != "pasted" {
		t.Errorf("pasted description = %q", description)
	}
	s.judgeSequence(&sequences[0])
	if len(s.report.findings) != 0 {
		t.Errorf("relative paste refused: %v", s.report.findings)
	}
}

// TestBrowserCorpusBaselineRetainsPluginOverlay reads the shared producer as a
// tree and checks the runtime baseline plus the workflow's declared plugin.
func TestBrowserCorpusBaselineRetainsPluginOverlay(t *testing.T) {
	s, _ := browserCorpusCase(t, "merge", "unused", "")
	tree, err := config.NewParser(s.schema).Parse(string(labconfig.WebDaemon([]string{"fakeenrich"})))
	if err != nil {
		t.Fatal(err)
	}
	bgp := tree.GetContainer("bgp")
	if value, _ := bgp.Get("router-id"); value != "1.2.3.4" {
		t.Errorf("runtime router-id = %q", value)
	}
	if value, _ := bgp.GetContainer("session").GetContainer("asn").Get("local"); value != "65000" {
		t.Errorf("runtime local ASN = %q", value)
	}
	if bgp.GetList("peer")["peer1"] == nil {
		t.Error("runtime peer is missing")
	}
	peer := bgp.GetList("peer")["peer1"]
	if peer != nil {
		filter := peer.GetContainer("filter")
		if filter == nil {
			t.Fatal("dummy peer has no explicit routing policy")
		}
		for _, direction := range []string{"import", "export"} {
			policy := filter.GetContainer(direction)
			if policy == nil {
				t.Fatalf("dummy peer has no explicit %s policy", direction)
			}
			if final, _ := policy.Get("final"); final != "reject" {
				t.Errorf("dummy peer %s final action = %q, want reject", direction, final)
			}
		}
	}
	endpoints := tree.GetContainer("bus").GetList("endpoint")
	if endpoints["fakeenrich"] == nil {
		t.Fatal("parsed workflow bus endpoint overlay is missing")
	}
	if value, _ := endpoints["fakeenrich"].Get("internal"); value != "fakeenrich" {
		t.Errorf("endpoint internal = %q", value)
	}
}

// TestSweepBrowserRuleStillChecksStandaloneFragment proves the existing
// exclusion discriminator restores the standalone mandatory-field findings.
func TestSweepBrowserRuleStillChecksStandaloneFragment(t *testing.T) {
	report := runSweep(t, ruleLoadSequence, sweepKeep{
		"test/web/cli-load-browser-file.wb", "test/web/fixtures/cli-load.conf",
	})
	if report.sequences != 0 {
		t.Error("disabled composition still judged a sequence")
	}
	if !strings.Contains(strings.Join(report.findings, "\n"), "router-id") {
		t.Errorf("standalone fragment no longer exposes its missing baseline: %v", report.findings)
	}
}

func browserCorpusCase(t *testing.T, operation, fragment, before string) (*sweepContext, string) {
	t.Helper()
	diagnostic.ResetForTest()
	diagnostic.RegisterBuiltinCodes()
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "fragment.conf"), []byte(fragment), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "scenario.wb")
	body := "option=server:kind=daemon\naction=open:path=/cli\n" + before +
		"action=fill:id=terminal-input:value=load file absolute " + operation + " label-only.conf\n" +
		"action=press:id=terminal-input:key=Enter\n" +
		"action=upload:id=cli-load-file:file=fragment.conf\n" +
		"action=click:id=cli-load-apply\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return &sweepContext{t: t, root: root, schema: schema, report: &sweepReport{
		hostVerdicts: map[string]int{}, kinds: map[string]int{},
	}}, path
}
