//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- repository config corpus
// Related: validate_corpus_test.go -- declared refusals and editor load sequences

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestSweepDeclaredRefusalHostVerdicts judges the actual mirror reload carrier
// with the schemas of both hosts. Only Linux can assert its reference refusal.
func TestSweepDeclaredRefusalHostVerdicts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		goos     string
		off      sweepRule
		findings int
		hosts    int
		refused  int
	}{
		{"darwin", "darwin", ruleNone, 0, 1, 0},
		{"linux", "linux", ruleNone, 0, 0, 1},
		{"host rule disabled", "darwin", ruleHostVerdict, 1, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSweepHostContext(t, tc.goos, tc.off)
			path := filepath.Join(s.root, "test", "reload", "iface-mirror-target-undeclared-refused.ci")
			carriers, _ := s.ciCarriers(path)
			var selected int
			for i := range carriers {
				carrier := &carriers[i]
				if !strings.HasSuffix(carrier.source, ":config2.conf") {
					continue
				}
				selected++
				if carrier.refused != "config-yang-reference" {
					t.Fatalf("fixture declares %q, want config-yang-reference", carrier.refused)
				}
				s.judgeCarrier(carrier)
			}
			if selected != 1 {
				t.Fatalf("selected %d mirror carriers, want 1", selected)
			}
			if got := len(s.report.findings); got != tc.findings {
				t.Errorf("findings = %d, want %d: %v", got, tc.findings, s.report.findings)
			}
			if got := s.report.hostVerdicts[diagnostic.CodeConfigNodeOtherOS]; got != tc.hosts {
				t.Errorf("host verdicts = %d, want %d", got, tc.hosts)
			}
			if got := s.report.refused; got != tc.refused {
				t.Errorf("asserted refusals = %d, want %d", got, tc.refused)
			}
		})
	}
}

// TestSweepDeclaredRefusalKeepsConfigErrors distinguishes a host-limited parse
// from a parse that also reports a real error, and counts no unobserved refusal.
func TestSweepDeclaredRefusalKeepsConfigErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		hosts    int
		accepted bool
	}{
		{"accepted", "bgp { router-id 203.0.113.1; session { asn { local 65000; } } }", 0, true},
		{"nonhost", "bgp { router-id invalid; }", 0, false},
		{"mixed", refusedRouteRefreshFile + "\n" + xfrmConfig, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSweepHostContext(t, "darwin", ruleNone)
			if tc.accepted {
				if result := runValidation(tc.body, tc.name); !result.Valid {
					t.Fatalf("accepted input was refused: %v", result.Diagnostics)
				}
			}
			if tc.hosts > 0 {
				result := runValidation(tc.body, tc.name)
				if findDiagnostic(result, diagnostic.CodeConfigNodeOtherOS) == nil {
					t.Fatal("mixed input did not produce a host verdict")
				}
				var configErrors int
				for _, d := range result.Diagnostics {
					if d.Severity != diagnostic.SeverityError {
						continue
					}
					if !isHostVerdict(d.Code) {
						configErrors++
					}
				}
				if configErrors == 0 {
					t.Fatal("mixed input did not produce a nonhost error")
				}
			}
			s.judgeCarrier(&sweepCarrier{source: tc.name, body: tc.body, refused: "config-yang-reference"})
			if len(s.report.findings) != 1 {
				t.Errorf("findings = %v, want one mismatched-refusal finding", s.report.findings)
			}
			if s.report.refused != 0 {
				t.Errorf("asserted refusals = %d, want 0", s.report.refused)
			}
			if got := s.report.hostVerdicts[diagnostic.CodeConfigNodeOtherOS]; got != tc.hosts {
				t.Errorf("host verdicts = %d, want %d", got, tc.hosts)
			}
		})
	}
}

// TestSweepMatchedRefusalCountsLaterHostVerdicts uses the parser's scalar
// recovery to place a matched refusal before a structural host verdict.
func TestSweepMatchedRefusalCountsLaterHostVerdicts(t *testing.T) {
	s := newSweepHostContext(t, "darwin", ruleNone)
	body := refusedRouteRefreshFile + "\n" + xfrmConfig
	result := runValidation(body, "matched.conf")
	if len(result.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %v, want one scalar refusal then one host verdict", result.Diagnostics)
	}
	matched := result.Diagnostics[0]
	if matched.Severity != diagnostic.SeverityError {
		t.Fatalf("first diagnostic = %v, want an error", matched)
	}
	if isHostVerdict(matched.Code) {
		t.Fatalf("first diagnostic = %v, want a config refusal", matched)
	}
	if result.Diagnostics[1].Code != diagnostic.CodeConfigNodeOtherOS {
		t.Fatalf("second diagnostic = %v, want the later host verdict", result.Diagnostics[1])
	}
	s.judgeCarrier(&sweepCarrier{source: "matched.conf", body: body, refused: matched.Code})
	if len(s.report.findings) != 0 {
		t.Errorf("matched refusal produced findings: %v", s.report.findings)
	}
	if s.report.refused != 1 {
		t.Errorf("asserted refusals = %d, want 1", s.report.refused)
	}
	if got := s.report.hostVerdicts[diagnostic.CodeConfigNodeOtherOS]; got != 1 {
		t.Errorf("host verdicts = %d, want 1", got)
	}
}

// TestSweepLoadPreservesHostVerdict follows the actual demo fragment through
// ParseLoad and requires both its typed refusal and the unchanged candidate.
func TestSweepLoadPreservesHostVerdict(t *testing.T) {
	s := newSweepHostContext(t, "darwin", ruleNone)
	step := sweepXFRMLoad(t, s.root)
	editor, err := editorcli.NewEditorFromContent([]byte("bgp { router-id 203.0.113.1; }"), "-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := editor.Close(); err != nil {
			t.Error(err)
		}
	})
	before := editor.WorkingContent()
	err = loadCorpusStep(editor, &step)
	if _, ok := errors.AsType[*config.NodeOfAnotherOSError](err); !ok {
		t.Errorf("load error = %v, want typed NodeOfAnotherOSError", err)
	}
	if got := editor.WorkingContent(); got != before {
		t.Errorf("refused load changed candidate from %q to %q", before, got)
	}
}

// TestSweepLoadHostVerdicts keeps the real editor's refused-step state while a
// later valid draft still stages. A host rule never hides ordinary parse errors.
func TestSweepLoadHostVerdicts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		off      sweepRule
		nonhost  bool
		findings int
		hosts    int
	}{
		{"host verdict", ruleNone, false, 0, 1},
		{"host rule disabled", ruleHostVerdict, false, 1, 0},
		{"nonhost refusal", ruleNone, true, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSweepHostContext(t, "darwin", tc.off)
			step := sweepXFRMLoad(t, s.root)
			if tc.nonhost {
				step = loadStep{source: "invalid.conf", body: "bgp { router-id invalid; }"}
			}
			seq := &loadSequence{source: tc.name, base: "bgp { router-id 203.0.113.1; }", draft: true,
				steps: []loadStep{step, {source: "valid.conf", body: "bgp { router-id 203.0.113.2; }"}}}
			if got := s.runningAfter(seq, 1); got != seq.base {
				t.Errorf("refused step changed running config to %q", got)
			}
			s.judgeSequence(seq)
			if got := len(s.report.findings); got != tc.findings {
				t.Errorf("findings = %d, want %d: %v", got, tc.findings, s.report.findings)
			}
			if got := s.report.hostVerdicts[diagnostic.CodeConfigNodeOtherOS]; got != tc.hosts {
				t.Errorf("host verdicts = %d, want %d", got, tc.hosts)
			}
			if s.report.stagedDrafts != 1 {
				t.Errorf("staged drafts = %d, want the one valid load", s.report.stagedDrafts)
			}
			if s.report.refused != 0 {
				t.Errorf("asserted refusals = %d, want 0", s.report.refused)
			}
		})
	}
}

func newSweepHostContext(t *testing.T, goos string, off sweepRule) *sweepContext {
	t.Helper()
	restore := config.SetSchemaOSForTest(goos)
	t.Cleanup(restore)
	diagnostic.ResetForTest()
	diagnostic.RegisterBuiltinCodes()
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	return &sweepContext{t: t, root: filepath.Join("..", "..", "..", ".."), schema: schema, off: off,
		report: &sweepReport{hostVerdicts: map[string]int{}}}
}

func sweepXFRMLoad(t *testing.T, root string) loadStep {
	t.Helper()
	path := filepath.Join(root, "demos", "terminal", "topics", "ipsec", "4-xfrm.conf")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return loadStep{source: path, body: string(body)}
}
