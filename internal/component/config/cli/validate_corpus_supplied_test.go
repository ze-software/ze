//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- supplied browser inspection carriers.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBrowserSuppliedInspectionClassifiesPhysicalInputs reads the native
// read-only workflows through the current browser entry point. Submitted
// inspection files are accounted for without becoming loads or publications.
func TestBrowserSuppliedInspectionClassifiesPhysicalInputs(t *testing.T) {
	for _, test := range []struct {
		workflow string
		files    []string
	}{
		{workflow: "graphical-supplied-file.wb", files: []string{"graphical-supplied-before.conf"}},
		{workflow: "graphical-supplied-compare.wb", files: []string{
			"graphical-supplied-before.conf", "graphical-supplied-after.conf",
		}},
	} {
		t.Run(test.workflow, func(t *testing.T) {
			s, _ := browserCorpusCase(t, "merge", "unused", "")
			s.root = filepath.Join("..", "..", "..", "..")
			consumed := map[string]bool{}
			path := filepath.Join(s.root, "test", "web", test.workflow)
			sequences := s.browserSequences(path, consumed)
			for _, file := range test.files {
				physical := filepath.Join(s.root, "test", "web", "fixtures", file)
				if !consumed[physical] {
					t.Errorf("submitted inspection input %s was left for publication validation", file)
				}
			}
			if len(s.report.findings) != 0 {
				t.Errorf("native read-only inspection findings: %v", s.report.findings)
			}
			if len(sequences) != 0 {
				t.Errorf("read-only inspection produced candidate load sequences: %v", sequences)
			}
			if s.report.stagedDrafts != 0 {
				t.Errorf("read-only inspection counted %d draft loads", s.report.stagedDrafts)
			}
			if s.report.zeConfigs != 0 {
				t.Errorf("read-only inspection counted %d published configurations", s.report.zeConfigs)
			}
		})
	}
}

// TestSweepBrowserSuppliedInspectionKeepsItsReadContract drives discovery and
// judgment together: an input accepted by its submitted inspection producer
// must not be judged again as a complete configuration intended for publication.
func TestSweepBrowserSuppliedInspectionKeepsItsReadContract(t *testing.T) {
	report := runSweep(t, ruleNone, sweepKeep{
		"test/web/graphical-supplied-file.wb",
		"test/web/fixtures/graphical-supplied-before.conf",
	})
	if len(report.findings) != 0 {
		t.Fatalf("native supplied inspection acquired publication findings: %v", report.findings)
	}
	if report.zeConfigs != 0 {
		t.Errorf("inspection-only sweep counted %d published configurations", report.zeConfigs)
	}
	if report.stagedDrafts != 0 {
		t.Errorf("inspection-only sweep counted %d draft loads", report.stagedDrafts)
	}
	if report.sequences != 0 {
		t.Errorf("inspection-only sweep counted %d load sequences", report.sequences)
	}
}

// TestBrowserSuppliedInspectionUnsupportedSelectionKeepsStandalone requires
// the native descriptor's offered selection and successful authentication
// before submitted file bytes can leave standalone validation.
func TestBrowserSuppliedInspectionUnsupportedSelectionKeepsStandalone(t *testing.T) {
	for _, test := range []struct {
		name     string
		password string
		command  string
	}{
		{name: "unknown command", password: "reader-fixture", command: "show config supplied unknown"},
		{name: "mutation command", password: "reader-fixture", command: "delete config"},
		{name: "failed authentication", password: "incorrect-fixture", command: "show config supplied dump"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, path := browserCorpusCase(t, "merge", validConfig, "")
			browserGraphicalWorkflow(t, path,
				"action=login:user=noc:password="+test.password+"\n"+
					"action=open:path=/show/\n"+
					"action=select:id=graphical-supplied-command:value="+test.command+"\n"+
					"action=upload:id=graphical-supplied-file:file=fragment.conf\n"+
					"action=click:id=graphical-supplied-submit\n")
			consumed := map[string]bool{}
			if sequences := s.browserSequences(path, consumed); len(sequences) != 0 {
				t.Fatalf("unsupported inspection composed a candidate: %v", sequences)
			}
			if len(s.report.findings) == 0 {
				t.Error("unsupported submitted inspection has no classification finding")
			}
			if consumed[filepath.Join(s.root, "fragment.conf")] {
				t.Error("unsupported inspection consumed the standalone input oracle")
			}
		})
	}
}

// TestBrowserSuppliedInspectionUnsubmittedFileKeepsStandalone proves an upload
// alone is not read authority; only an actual parsed submission consumes it.
func TestBrowserSuppliedInspectionUnsubmittedFileKeepsStandalone(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", validConfig, "")
	browserGraphicalWorkflow(t, path,
		"action=login:user=noc:password=reader-fixture\naction=open:path=/show/\n"+
			"action=select:id=graphical-supplied-command:value=show config supplied dump\n"+
			"action=upload:id=graphical-supplied-file:file=fragment.conf\n")
	consumed := map[string]bool{}
	if sequences := s.browserSequences(path, consumed); len(sequences) != 0 {
		t.Fatalf("unsubmitted file composed a candidate: %v", sequences)
	}
	if consumed[filepath.Join(s.root, "fragment.conf")] {
		t.Error("unsubmitted file was classified as inspected")
	}
}

// TestBrowserSuppliedInspectionPasteReachesItsOwner distinguishes a successful
// native paste from dropping its bytes or consuming a malformed configuration.
func TestBrowserSuppliedInspectionPasteReachesItsOwner(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		read bool
	}{
		{name: "accepted", body: "bgp { router-id 203.0.113.253; peer peer1 { connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; } } session { asn { remote 65533; local 65533; } } } }", read: true},
		{name: "malformed", body: "bgp {", read: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, path := browserCorpusCase(t, "merge", "unused", "")
			browserGraphicalWorkflow(t, path,
				"action=login:user=noc:password=reader-fixture\naction=open:path=/show/\n"+
					"action=select:id=graphical-supplied-command:value=show config supplied dump\n"+
					"action=fill:id=graphical-supplied-content:value="+test.body+"\n"+
					"action=click:id=graphical-supplied-submit\n")
			if sequences := s.browserSequences(path, map[string]bool{}); len(sequences) != 0 {
				t.Fatalf("paste inspection composed a candidate: %v", sequences)
			}
			if test.read {
				if len(s.report.findings) != 0 {
					t.Fatalf("the real inspection owner refused valid paste: %v", s.report.findings)
				}
				if s.report.kinds[".wb"] != 1 {
					t.Errorf("accepted native paste inspections = %d, want 1", s.report.kinds[".wb"])
				}
			} else {
				if len(s.report.findings) == 0 {
					t.Error("malformed paste was never refused by its inspection owner")
				}
				if s.report.kinds[".wb"] != 0 {
					t.Error("malformed paste counted as a successful native inspection")
				}
			}
		})
	}
}

// TestBrowserSuppliedInspectionCombinedBudgetDeclaration accepts a real native
// combined-input refusal only with its exact adjacent problem declaration.
// Both independently selected files remain under the single-input limit.
func TestBrowserSuppliedInspectionCombinedBudgetDeclaration(t *testing.T) {
	for _, declaration := range []string{
		"expect=element:text=The supplied inputs are too large.\n",
		"",
		"expect=element:text=195,840\n",
		"expect=element:id=graphical-supplied-submit\nexpect=element:text=The supplied inputs are too large.\n",
	} {
		t.Run(declaration, func(t *testing.T) {
			s, path := browserCorpusCase(t, "merge", "unused", "")
			body := validConfig + "\n# " + strings.Repeat("x", 100_000) + "\n"
			for _, file := range []string{"single.conf", "before.conf", "after.conf"} {
				if err := os.WriteFile(filepath.Join(s.root, file), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			browserGraphicalWorkflow(t, path,
				"action=login:user=noc:password=reader-fixture\naction=open:path=/show/\n"+
					"action=select:id=graphical-supplied-command:value=show config supplied dump\n"+
					"action=upload:id=graphical-supplied-file:file=single.conf\n"+
					"action=click:id=graphical-supplied-submit\n"+
					"action=select:id=graphical-supplied-command:value=show config supplied diff\n"+
					"action=upload:id=graphical-supplied-before-file:file=before.conf\n"+
					"action=upload:id=graphical-supplied-after-file:file=after.conf\n"+
					"action=click:id=graphical-supplied-submit\n"+declaration)
			consumed := map[string]bool{}
			if sequences := s.browserSequences(path, consumed); len(sequences) != 0 {
				t.Fatalf("oversized inspection composed a candidate: %v", sequences)
			}
			if !consumed[filepath.Join(s.root, "single.conf")] {
				t.Error("the same physical bytes were not accepted by a single-input inspection")
			}
			declared := declaration == "expect=element:text=The supplied inputs are too large.\n"
			if declared {
				if len(s.report.findings) != 0 {
					t.Errorf("declared native size refusal was misclassified: %v", s.report.findings)
				}
				if s.report.refused != 1 {
					t.Errorf("asserted native size refusals = %d, want 1", s.report.refused)
				}
			} else if len(s.report.findings) == 0 {
				t.Error("missing, partial or nonadjacent declaration concealed the size refusal")
			}
			for _, file := range []string{"before.conf", "after.conf"} {
				if consumed[filepath.Join(s.root, file)] != declared {
					t.Errorf("%s consumed = %v, declared = %v", file, consumed[filepath.Join(s.root, file)], declared)
				}
			}
		})
	}
}

// TestBrowserSuppliedInspectionDeniedMutationKeepsStandalone requires the
// reader's preceding candidate mutation to be refused before inspection can
// consume its physical file under an invented candidate history.
func TestBrowserSuppliedInspectionDeniedMutationKeepsStandalone(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", validConfig, "")
	browserGraphicalWorkflow(t, path,
		"action=login:user=noc:password=reader-fixture\naction=open:path=/cli\n"+
			"action=fill:id=terminal-input:value=set system host forbidden-reader-mutation\n"+
			"action=press:id=terminal-input:key=Enter\naction=open:path=/show/\n"+
			"action=select:id=graphical-supplied-command:value=show config supplied dump\n"+
			"action=upload:id=graphical-supplied-file:file=fragment.conf\n"+
			"action=click:id=graphical-supplied-submit\n")
	consumed := map[string]bool{}
	if sequences := s.browserSequences(path, consumed); len(sequences) != 0 {
		t.Fatalf("denied reader mutation composed a candidate: %v", sequences)
	}
	if !strings.Contains(strings.Join(s.report.findings, "\n"), "authorization denied") {
		t.Errorf("the reader's mutation policy was not applied: %v", s.report.findings)
	}
	if consumed[filepath.Join(s.root, "fragment.conf")] {
		t.Error("denied preceding mutation consumed the standalone input oracle")
	}
}

// TestBrowserSuppliedInspectionMalformedFileKeepsStandalone proves the actual
// submitted owner must accept physical bytes before they leave full validation.
func TestBrowserSuppliedInspectionMalformedFileKeepsStandalone(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "bgp {", "")
	browserGraphicalWorkflow(t, path,
		"action=login:user=noc:password=reader-fixture\naction=open:path=/show/\n"+
			"action=select:id=graphical-supplied-command:value=show config supplied dump\n"+
			"action=upload:id=graphical-supplied-file:file=fragment.conf\n"+
			"action=click:id=graphical-supplied-submit\n")
	consumed := map[string]bool{}
	if sequences := s.browserSequences(path, consumed); len(sequences) != 0 {
		t.Fatalf("malformed inspection composed a candidate: %v", sequences)
	}
	if len(s.report.findings) == 0 {
		t.Error("malformed physical bytes were not refused by their owner")
	}
	if consumed[filepath.Join(s.root, "fragment.conf")] {
		t.Error("malformed input lost its standalone validation oracle")
	}
}
