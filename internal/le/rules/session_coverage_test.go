// VALIDATES: spec-le-is-a-ze-binary AC-5 and AC-11. The native transcript
// miss-detector preserves rule_coverage.py's observations, report, and exit
// contract without passing through the gate-map measurement.
// PREVENTS: crediting digest or point reads, muting rules cited by CORE.md,
// silently losing an observation failure, and repeating unchanged Stop output.

package rules

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type coverageFixture struct {
	root       string
	rulesDir   string
	transcript string
}

func newCoverageFixture(t *testing.T) coverageFixture {
	t.Helper()
	root := t.TempDir()
	rulesDir := filepath.Join(root, "ai", "rules")
	if err := os.MkdirAll(rulesDir, 0o750); err != nil {
		t.Fatalf("fixture rule directory: %v", err)
	}
	return coverageFixture{
		root:       root,
		rulesDir:   rulesDir,
		transcript: filepath.Join(root, "session.jsonl"),
	}
}

func (f coverageFixture) writeRule(t *testing.T, name, trigger, severity string) {
	t.Helper()
	text := "# " + name + "\n**When:** " + trigger + "\n**Severity:** " + severity +
		"\n\n## Directives\n- do it\n"
	if err := os.WriteFile(filepath.Join(f.rulesDir, name), []byte(text), 0o600); err != nil {
		t.Fatalf("fixture rule %s: %v", name, err)
	}
}

func (f coverageFixture) writeCore(t *testing.T, names ...string) {
	t.Helper()
	var text strings.Builder
	text.WriteString("# Ze Rules -- Always-On Core\n\n")
	for _, name := range names {
		text.WriteString("## " + name + "\n`ai/rules/" + name + "`\n**When:** always\n\n")
		text.WriteString("Fix the root cause (`ai/rules/completion.md`), never record it.\n\n")
	}
	if err := os.WriteFile(filepath.Join(f.rulesDir, "CORE.md"), []byte(text.String()), 0o600); err != nil {
		t.Fatalf("fixture CORE.md: %v", err)
	}
}

type transcriptCall struct {
	tool string
	path string
}

func (f coverageFixture) writeTranscript(t *testing.T, calls ...transcriptCall) {
	t.Helper()
	var out bytes.Buffer
	for _, call := range calls {
		row := map[string]any{
			"type": "assistant",
			"message": map[string]any{
				"role": "assistant",
				"content": []any{map[string]any{
					"type":  "tool_use",
					"name":  call.tool,
					"input": map[string]any{"file_path": call.path},
				}},
			},
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			t.Fatalf("fixture transcript row: %v", err)
		}
		out.Write(encoded)
		out.WriteByte('\n')
	}
	if err := os.WriteFile(f.transcript, out.Bytes(), 0o600); err != nil {
		t.Fatalf("fixture transcript: %v", err)
	}
}

func (f coverageFixture) analyse(t *testing.T) (sessionCoverageReport, string) {
	t.Helper()
	var errOut bytes.Buffer
	rules, err := loadCoverageRules(f.rulesDir, &errOut)
	if err != nil {
		t.Fatalf("loadCoverageRules: %v", err)
	}
	files, err := (NativeTranscriptSource{}).Files(f.root, f.transcript)
	if err != nil {
		t.Fatalf("NativeTranscriptSource.Files: %v", err)
	}
	return analyseSessionCoverage(rules, files), errOut.String()
}

func containsCoverageName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func TestSessionCoverageReportsOnlyUnreadMatchedBlockingRules(t *testing.T) {
	fixture := newCoverageFixture(t)
	fixture.writeRule(t, "performance.md", "writing any wire-encoding path", "blocking")
	fixture.writeRule(t, "advisory-one.md", "writing any wire-encoding path", "advisory")
	fixture.writeTranscript(t, transcriptCall{"Edit", filepath.Join(fixture.root, "internal", "wire.go")})

	report, _ := fixture.analyse(t)
	if len(report.Missed) != 1 || report.Missed[0] != "performance.md" {
		t.Fatalf("missed = %v, want performance.md", report.Missed)
	}
	if report.BlockingTotal != 1 || containsCoverageName(report.Matched, "advisory-one.md") {
		t.Errorf("blocking total/matched = %d/%v", report.BlockingTotal, report.Matched)
	}

	fixture.writeTranscript(t,
		transcriptCall{"Edit", filepath.Join(fixture.root, "internal", "wire.go")},
		transcriptCall{"Read", filepath.Join(fixture.rulesDir, "performance.md")},
	)
	report, _ = fixture.analyse(t)
	if len(report.Missed) != 0 || !containsCoverageName(report.RulesRead, "performance.md") {
		t.Errorf("direct read did not clear the miss: read=%v missed=%v", report.RulesRead, report.Missed)
	}
}

func TestSessionCoverageCreditsNeitherDigestNorPointReads(t *testing.T) {
	fixture := newCoverageFixture(t)
	fixture.writeRule(t, "performance.md", "writing any wire-encoding path", "blocking")
	point := filepath.Join(fixture.rulesDir, "points", "plugins", "directives", "performance.md")
	if err := os.MkdirAll(filepath.Dir(point), 0o750); err != nil {
		t.Fatalf("fixture point directory: %v", err)
	}
	if err := os.WriteFile(point, []byte("body\n"), 0o600); err != nil {
		t.Fatalf("fixture point: %v", err)
	}
	fixture.writeTranscript(t,
		transcriptCall{"Edit", filepath.Join(fixture.root, "internal", "wire.go")},
		transcriptCall{"Read", filepath.Join(fixture.rulesDir, "TRIGGERS.md")},
		transcriptCall{"Read", point},
	)

	report, _ := fixture.analyse(t)
	if len(report.RulesRead) != 0 {
		t.Errorf("digest or point read was credited: %v", report.RulesRead)
	}
	if len(report.Missed) != 1 || report.Missed[0] != "performance.md" {
		t.Errorf("missed = %v, want performance.md", report.Missed)
	}
}

func TestSessionCoverageCoreMembershipIsAStandaloneArtifactPath(t *testing.T) {
	fixture := newCoverageFixture(t)
	fixture.writeRule(t, "spec-no-code.md", "writing or editing a spec", "blocking")
	fixture.writeRule(t, "completion.md", "when creating or updating a spec", "blocking")
	fixture.writeCore(t, "spec-no-code.md")
	fixture.writeTranscript(t, transcriptCall{"Edit", filepath.Join(fixture.root, "plan", "spec-thing.md")})

	report, _ := fixture.analyse(t)
	if len(report.AlwaysOnRules) != 1 || report.AlwaysOnRules[0] != "spec-no-code.md" {
		t.Fatalf("always-on = %v, inline citation must not count", report.AlwaysOnRules)
	}
	if len(report.Missed) != 1 || report.Missed[0] != "completion.md" {
		t.Errorf("missed = %v, cited routed rule is still owed", report.Missed)
	}
	if report.BlockingTotal != 1 || report.AlwaysOnExcluded != 1 {
		t.Errorf("measured/excluded totals = %d/%d", report.BlockingTotal, report.AlwaysOnExcluded)
	}
}

func TestSessionCoverageReadsTheNativeCoreRendererShape(t *testing.T) {
	fixture := newCoverageFixture(t)
	core := []Rule{{
		Name:       "spec-no-code.md",
		Stem:       "spec-no-code",
		Path:       "ai/rules/spec-no-code.md",
		Title:      "No Code in Specs",
		Meta:       []MetaPair{{Key: "When", Value: "writing or editing a spec"}, {Key: "Severity", Value: "blocking"}},
		Body:       []string{"", "## Directives", "", "Specs MUST NOT contain code."},
		Trigger:    "writing or editing a spec",
		Severity:   "blocking",
		CoreReason: "the ladder itself",
	}}
	if err := os.WriteFile(filepath.Join(fixture.rulesDir, "CORE.md"), []byte(buildCore(core)), 0o600); err != nil {
		t.Fatalf("fixture rendered core: %v", err)
	}
	var errOut bytes.Buffer
	members := loadAlwaysOnRules(fixture.rulesDir, &errOut)
	if len(members) != 1 || !members["spec-no-code.md"] {
		t.Errorf("native buildCore round trip = %v; stderr=%q", members, errOut.String())
	}
}

func TestSessionCoverageMissingOrChangedCoreExcludesNothingAndSpeaks(t *testing.T) {
	for _, test := range []struct {
		name          string
		core          string
		wantPhrase    string
		wantGenerator bool
	}{
		{name: "missing", wantPhrase: "cannot read"},
		{name: "changed shape", core: "# Core\n\n- [rule](ai/rules/spec-no-code.md)\n",
			wantPhrase: "readable but carries no", wantGenerator: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCoverageFixture(t)
			fixture.writeRule(t, "spec-no-code.md", "writing or editing a spec", "blocking")
			if test.core != "" {
				if err := os.WriteFile(filepath.Join(fixture.rulesDir, "CORE.md"), []byte(test.core), 0o600); err != nil {
					t.Fatalf("fixture CORE.md: %v", err)
				}
			}
			fixture.writeTranscript(t, transcriptCall{"Edit", filepath.Join(fixture.root, "plan", "spec-thing.md")})

			report, diagnostic := fixture.analyse(t)
			if len(report.Missed) != 1 || report.AlwaysOnExcluded != 0 {
				t.Errorf("safe fallback = missed %v, excluded %d", report.Missed, report.AlwaysOnExcluded)
			}
			if !strings.Contains(diagnostic, test.wantPhrase) || !strings.Contains(diagnostic, "excluding no always-on rule") {
				t.Errorf("diagnostic = %q", diagnostic)
			}
			if test.wantGenerator && !strings.Contains(diagnostic, "internal/le/rules.RenderArtifacts") {
				t.Errorf("changed CORE diagnostic does not name its producer: %q", diagnostic)
			}
		})
	}
}

func TestSessionCoveragePublishesTheUnmatchableBlindSpot(t *testing.T) {
	fixture := newCoverageFixture(t)
	fixture.writeRule(t, "never-destroy-work.md",
		"before deleting, reverting, or overwriting any file holding uncommitted work", "blocking")
	fixture.writeTranscript(t, transcriptCall{"Edit", filepath.Join(fixture.root, "internal", "wire.go")})

	report, _ := fixture.analyse(t)
	if len(report.Missed) != 0 || report.Unmatchable != 1 ||
		!containsCoverageName(report.UnmatchableRules, "never-destroy-work.md") {
		t.Errorf("unmatchable report = missed %v, count %d, rules %v",
			report.Missed, report.Unmatchable, report.UnmatchableRules)
	}
	if !strings.Contains(report.Text(), "UNDER-reports") {
		t.Errorf("text hides the blind spot: %q", report.Text())
	}
}

func TestSessionCoverageTextPreservesTheHumanReportContract(t *testing.T) {
	report := sessionCoverageReport{
		BlockingTotal:    2,
		AlwaysOnExcluded: 1,
		AlwaysOnRules:    []string{"core.md"},
		Touched:          1,
		Kinds:            []string{"go"},
		Matched:          []string{"performance.md"},
		Missed:           []string{"performance.md"},
		Unmatchable:      1,
		UnmatchableRules: []string{"action.md"},
		reportPath:       "tmp/x.ndjson",
	}
	want := "rule-coverage: 1 blocking rule(s) matched this session's files but were never read:\n" +
		"  - ai/rules/performance.md\n" +
		"rule-coverage: 1 of 2 blocking rules have action-shaped triggers that no file type can match, " +
		"so this count UNDER-reports; silence is not proof of coverage\n" +
		"rule-coverage: 1 always-on rule(s) sit outside that total; ai/rules/CORE.md carries their directives and " +
		"CLAUDE.md imports it, so no session Reads them and none is ever counted missed\n" +
		"rule-coverage: report tmp/x.ndjson"
	if got := report.Text(); got != want {
		t.Errorf("missed report =\n%q\nwant\n%q", got, want)
	}

	report.Missed = []string{}
	report.Matched = []string{}
	report.Touched = 0
	if got := report.Text(); !strings.HasPrefix(got,
		"rule-coverage: 0 missed of 0 blocking rule(s) matched by 0 touched file(s)\n") {
		t.Errorf("clean report = %q", got)
	}
}

type fixedTranscriptSource struct {
	path  string
	files TranscriptFiles
	err   error
}

func (s fixedTranscriptSource) TranscriptPath(string) string { return s.path }

func (s fixedTranscriptSource) Files(string, string) (TranscriptFiles, error) {
	return s.files, s.err
}

func TestRunSessionCoverageAppendsTheNDJSONContract(t *testing.T) {
	fixture := newCoverageFixture(t)
	fixture.writeRule(t, "performance.md", "writing any wire-encoding path", "blocking")
	fixture.writeTranscript(t)
	source := fixedTranscriptSource{
		files: TranscriptFiles{Written: []string{"internal/wire.go"}, RulesRead: []string{}},
	}
	var errOut bytes.Buffer
	report, code := RunSessionCoverage(fixture.root, SessionCoverageOptions{
		Transcript: fixture.transcript,
		Session:    "sess-1",
		RulesDir:   fixture.rulesDir,
	}, source, func() time.Time { return time.Date(2026, 8, 27, 12, 34, 56, 0, time.Local) }, &errOut)
	if code != 1 || report == nil || len(report.Missed) != 1 {
		t.Fatalf("RunSessionCoverage = report %#v, code %d", report, code)
	}

	raw, err := os.ReadFile(filepath.Join(fixture.root, filepath.FromSlash(ruleCoverageReportRel)))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var row coverageRecord
	if err := json.Unmarshal(bytes.TrimSpace(raw), &row); err != nil {
		t.Fatalf("decode report %q: %v", raw, err)
	}
	if row.Timestamp != "2026-08-27T12:34:56" || row.Session != "sess-1" ||
		row.Touched != 1 || row.Matched != 1 || row.Read != 0 ||
		len(row.Missed) != 1 || row.Missed[0] != "performance.md" {
		t.Errorf("NDJSON row = %#v", row)
	}
	for _, key := range []string{"\"unmatchable\"", "\"always-on\"", "\"kinds\""} {
		if !bytes.Contains(raw, []byte(key)) {
			t.Errorf("NDJSON row lacks %s: %s", key, raw)
		}
	}
}

func TestRunSessionCoverageQuietPrintsOnlyAChangedMissSetButAlwaysAppends(t *testing.T) {
	fixture := newCoverageFixture(t)
	fixture.writeRule(t, "performance.md", "writing any wire-encoding path", "blocking")
	fixture.writeTranscript(t)
	source := fixedTranscriptSource{files: TranscriptFiles{Written: []string{"internal/wire.go"}}}
	options := SessionCoverageOptions{
		Quiet:      true,
		Transcript: fixture.transcript,
		Session:    "sess-quiet",
		RulesDir:   fixture.rulesDir,
	}
	now := func() time.Time { return time.Date(2026, 8, 27, 1, 2, 3, 0, time.Local) }

	first, firstCode := RunSessionCoverage(fixture.root, options, source, now, &bytes.Buffer{})
	second, secondCode := RunSessionCoverage(fixture.root, options, source, now, &bytes.Buffer{})
	if firstCode != 1 || secondCode != 1 {
		t.Fatalf("quiet miss codes = %d, %d", firstCode, secondCode)
	}
	if first.Text() == "" || !strings.Contains(first.Text(), "1 of 1 matched blocking rule(s) unread") {
		t.Errorf("first quiet text = %q", first.Text())
	}
	if second.Text() != "" {
		t.Errorf("repeated quiet text = %q, want silence", second.Text())
	}

	raw, err := os.ReadFile(filepath.Join(fixture.root, filepath.FromSlash(ruleCoverageReportRel)))
	if err != nil {
		t.Fatalf("read accumulated report: %v", err)
	}
	if lines := bytes.Count(raw, []byte{'\n'}); lines != 2 {
		t.Errorf("report rows = %d, want 2", lines)
	}

	source.files.RulesRead = []string{"performance.md"}
	changed, changedCode := RunSessionCoverage(fixture.root, options, source, now, &bytes.Buffer{})
	if changedCode != 0 || changed.Text() == "" || !strings.Contains(changed.Text(), "0 of 1") {
		t.Errorf("changed quiet answer = code %d, text %q", changedCode, changed.Text())
	}
}

func TestRunSessionCoverageObservationFailuresSpeakAndNeverExitTwo(t *testing.T) {
	fixture := newCoverageFixture(t)
	fixture.writeRule(t, "performance.md", "writing any wire-encoding path", "blocking")
	fixture.writeTranscript(t)

	var unreadableErr bytes.Buffer
	report, code := RunSessionCoverage(fixture.root, SessionCoverageOptions{
		Transcript: fixture.transcript,
		RulesDir:   fixture.rulesDir,
		NoAppend:   true,
	}, fixedTranscriptSource{err: errors.New("permission denied")}, time.Now, &unreadableErr)
	if code != 0 || report == nil || len(report.Missed) != 0 {
		t.Errorf("unreadable observation = report %#v, code %d", report, code)
	}
	if !strings.Contains(unreadableErr.String(), "cannot read the session transcript") ||
		!strings.Contains(unreadableErr.String(), "permission denied") {
		t.Errorf("unreadable diagnostic = %q", unreadableErr.String())
	}

	var absentErr bytes.Buffer
	report, code = RunSessionCoverage(fixture.root, SessionCoverageOptions{
		Transcript: filepath.Join(fixture.root, "absent.jsonl"),
		RulesDir:   fixture.rulesDir,
		NoAppend:   true,
	}, fixedTranscriptSource{}, time.Now, &absentErr)
	if code != 0 || report != nil {
		t.Errorf("absent transcript = report %#v, code %d", report, code)
	}
	if !strings.Contains(absentErr.String(), "no readable session transcript") {
		t.Errorf("absent diagnostic = %q", absentErr.String())
	}

	var missingRulesErr bytes.Buffer
	report, code = RunSessionCoverage(fixture.root, SessionCoverageOptions{
		Transcript: fixture.transcript,
		RulesDir:   filepath.Join(fixture.root, "no-rules"),
		NoAppend:   true,
	}, fixedTranscriptSource{}, time.Now, &missingRulesErr)
	if code != 0 || report != nil || !strings.Contains(missingRulesErr.String(), "nothing to match against") {
		t.Errorf("missing rules = report %#v, code %d, stderr %q", report, code, missingRulesErr.String())
	}
}

func TestNativeTranscriptSourceSkipsMalformedRowsAndCountsSidechains(t *testing.T) {
	fixture := newCoverageFixture(t)
	absoluteWrite := filepath.Join(fixture.root, "internal", "wire.go")
	absoluteRule := filepath.Join(fixture.rulesDir, "performance.md")
	fixture.writeTranscript(t,
		transcriptCall{"Edit", absoluteWrite},
		transcriptCall{"Read", absoluteRule},
	)
	file, err := os.OpenFile(fixture.transcript, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open transcript: %v", err)
	}
	sidechain, err := json.Marshal(map[string]any{
		"isSidechain": true,
		"message": map[string]any{"content": []any{map[string]any{
			"type":  "tool_use",
			"name":  "Edit",
			"input": map[string]any{"file_path": filepath.Join(fixture.root, "docs", "side.md")},
		}}},
	})
	if err != nil {
		t.Fatalf("sidechain transcript row: %v", err)
	}
	sidechain = append(sidechain, '\n')
	if _, err := file.Write(sidechain); err != nil {
		_ = file.Close()
		t.Fatalf("append sidechain row: %v", err)
	}
	if _, err := file.WriteString("not json but says \\\"tool_use\\\"\n"); err != nil {
		_ = file.Close()
		t.Fatalf("append malformed row: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close transcript: %v", err)
	}

	files, err := (NativeTranscriptSource{}).Files(fixture.root, fixture.transcript)
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(files.Written) != 2 || files.Written[0] != "docs/side.md" ||
		files.Written[1] != "internal/wire.go" {
		t.Errorf("written = %v", files.Written)
	}
	if len(files.RulesRead) != 1 || files.RulesRead[0] != "performance.md" {
		t.Errorf("rules read = %v", files.RulesRead)
	}
}

func TestNativeTranscriptSourceResolvesOnlyTheNamedSafeSession(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "session_1.safe")
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "")
	dir := transcriptDirectory(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("transcript directory: %v", err)
	}
	want := filepath.Join(dir, "session_1.safe.jsonl")
	if err := os.WriteFile(want, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("transcript fixture: %v", err)
	}
	if got := (NativeTranscriptSource{}).TranscriptPath(root); got != want {
		t.Errorf("TranscriptPath = %q, want %q", got, want)
	}
	for _, unsafe := range []string{".", "..", "../neighbor"} {
		t.Setenv("CLAUDE_CODE_SESSION_ID", unsafe)
		if got := (NativeTranscriptSource{}).TranscriptPath(root); got != "" {
			t.Errorf("unsafe direct session %q selected %q", unsafe, got)
		}
	}
}

func TestSessionCoverageReportHasTheLegacyMachineReadableSet(t *testing.T) {
	raw, err := json.Marshal(sessionCoverageReport{
		BlockingTotal:    3,
		AlwaysOnExcluded: 1,
		AlwaysOnRules:    []string{"core.md"},
		Touched:          2,
		Kinds:            []string{"go"},
		RulesRead:        []string{"read.md"},
		Matched:          []string{"read.md", "missed.md"},
		Missed:           []string{"missed.md"},
		Unmatchable:      1,
		UnmatchableRules: []string{"action.md"},
	})
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	want := []string{
		"blocking-total", "always-on-excluded", "always-on-rules", "touched",
		"kinds", "rules-read", "matched", "missed", "unmatchable", "unmatchable-rules",
	}
	if len(fields) != len(want) {
		t.Fatalf("machine-readable fields = %v", fields)
	}
	for _, name := range want {
		if _, ok := fields[name]; !ok {
			t.Errorf("machine-readable report lacks %q: %s", name, raw)
		}
	}
	clean := analyseSessionCoverage(nil, TranscriptFiles{Written: []string{}, RulesRead: []string{}})
	cleanRaw, err := json.Marshal(clean)
	if err != nil {
		t.Fatalf("marshal empty report: %v", err)
	}
	for _, field := range []string{"always-on-rules", "kinds", "rules-read", "matched", "missed", "unmatchable-rules"} {
		needle := []byte("\"" + field + "\":[]")
		if !bytes.Contains(cleanRaw, needle) {
			t.Errorf("empty report encodes %s as null: %s", field, cleanRaw)
		}
	}
}

func TestRulesCoverageReportIsADistinctKeywordAction(t *testing.T) {
	listing := Actions()
	for _, row := range listing.Actions {
		if row.Verb != "coverage-report" {
			continue
		}
		if !row.Writes {
			t.Errorf("coverage-report row = %#v", row)
		}
		fixture := newCoverageFixture(t)
		fixture.writeRule(t, "performance.md", "writing any wire-encoding path", "blocking")
		fixture.writeCore(t, "unrelated.md")
		fixture.writeTranscript(t, transcriptCall{"Edit", filepath.Join(fixture.root, "internal", "wire.go")})
		t.Setenv("ZE_REPO_ROOT", fixture.root)
		answer, code := Answer([]string{
			"coverage-report",
			"quiet",
			"transcript", fixture.transcript,
			"session", "action-session",
			"rules-dir", fixture.rulesDir,
			"no-append",
		})
		report, ok := answer.(*sessionCoverageReport)
		if !ok || report == nil || code != 1 || len(report.Missed) != 1 {
			t.Fatalf("coverage-report answer = %#v, code %d", answer, code)
		}
		if !strings.Contains(report.Text(), "1 of 1 matched blocking rule(s) unread") {
			t.Errorf("coverage-report quiet text = %q", report.Text())
		}
		if _, err := os.Stat(filepath.Join(fixture.root, filepath.FromSlash(ruleCoverageReportRel))); !os.IsNotExist(err) {
			t.Errorf("no-append wrote a report: %v", err)
		}
		return
	}
	t.Fatal("rules action list has no coverage-report action")
}
