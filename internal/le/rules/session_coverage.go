// Design: docs/architecture/core-design.md -- measure whether routed rules were read
// Overview: actions.go -- the rules command surface
// Related: rules.go -- the shared rule corpus predicate
//
// session_coverage.go ports internal/le/rules/coverage.go. It measures one
// transcript. The gate-map report measures hook bindings instead, so the two
// analyses share the rule corpus and nothing else.
package rules

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/spec/specpath"
)

const ruleCoverageReportRel = "tmp/rule-coverage/report.ndjson"

var safeTranscriptSession = regexp.MustCompile(`\A[A-Za-z0-9._-]+\z`)

// TranscriptFiles is the observation recovered from one transcript. Written
// and RulesRead are sets expressed as sorted slices.
type TranscriptFiles struct {
	Written   []string
	RulesRead []string
}

// TranscriptSource is the model/transcript boundary. A caller injects it, so
// the rules package never imports or starts the former Python running-model
// helper. Files MUST return empty sets with an error when it cannot observe the
// transcript; the report speaks and remains advisory.
type TranscriptSource interface {
	TranscriptPath(root string) string
	Files(root, path string) (TranscriptFiles, error)
}

// NativeTranscriptSource reads Claude JSONL transcripts without a subprocess.
type NativeTranscriptSource struct{}

// TranscriptPath answers this session's transcript, or an empty path when it
// cannot identify one. An explicit session id never falls back to a neighbor.
func (NativeTranscriptSource) TranscriptPath(root string) string {
	dir := transcriptDirectory(root)
	if dir == "" {
		return ""
	}
	rawSession, hasSession := os.LookupEnv("CLAUDE_CODE_SESSION_ID")
	if hasSession {
		if rawSession == "." || rawSession == ".." || !safeTranscriptSession.MatchString(rawSession) {
			return ""
		}
		return existingTranscript(filepath.Join(dir, rawSession+".jsonl"))
	}
	if os.Getenv("CLAUDE_CODE_FORK_SUBAGENT") != "" {
		paths, err := lepath.ResolveSession(root, false)
		if err != nil {
			return ""
		}
		return existingTranscript(filepath.Join(dir, paths.ID+".jsonl"))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var newest string
	var newestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, statErr := entry.Info()
		if statErr != nil || !info.Mode().IsRegular() {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest = path
			newestTime = info.ModTime()
		}
	}
	return newest
}

func transcriptDirectory(root string) string {
	absolute, err := filepath.Abs(root)
	if err != nil {
		absolute = root
	}
	slug := strings.NewReplacer("/", "-", ".", "-").Replace(absolute)
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects", slug)
}

func existingTranscript(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return path
}

// Files recovers files written and direct rule-file reads. Main-thread and
// sidechain entries intentionally share the same observation.
func (NativeTranscriptSource) Files(root, path string) (TranscriptFiles, error) {
	file, err := os.Open(path) // #nosec G304 -- the caller supplies its own transcript path
	if err != nil {
		return TranscriptFiles{Written: []string{}, RulesRead: []string{}}, err
	}
	defer func() { _ = file.Close() }()

	written := make(map[string]bool)
	rulesRead := make(map[string]bool)
	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadString('\n')
		if line != "" {
			observeTranscriptLine(line, root, written, rulesRead)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return TranscriptFiles{
				Written: sessionCoverageSortedKeys(written), RulesRead: sessionCoverageSortedKeys(rulesRead),
			}, readErr
		}
	}
	return TranscriptFiles{
		Written: sessionCoverageSortedKeys(written), RulesRead: sessionCoverageSortedKeys(rulesRead),
	}, nil
}

func observeTranscriptLine(line, root string, written, rulesRead map[string]bool) {
	if !strings.Contains(line, `"tool_use"`) {
		return
	}
	var entry struct {
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal([]byte(line), &entry); err != nil || len(entry.Message) == 0 {
		return
	}
	var message struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(entry.Message, &message); err != nil {
		return
	}
	var content []json.RawMessage
	if err := json.Unmarshal(message.Content, &content); err != nil {
		return
	}
	for _, raw := range content {
		var block struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal(raw, &block); err != nil || block.Type != "tool_use" {
			continue
		}
		var input struct {
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal(block.Input, &input); err != nil || input.FilePath == "" {
			continue
		}
		rel := transcriptRelativePath(input.FilePath, root)
		switch block.Name {
		case "Write", "Edit", "MultiEdit", "NotebookEdit":
			written[rel] = true
		case "Read":
			if directRulePath(rel) {
				rulesRead[filepath.Base(filepath.FromSlash(rel))] = true
			}
		}
	}
}

func transcriptRelativePath(path, root string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return path
	}
	absolute = resolveSymlinkPrefix(absolute)
	absoluteRoot = resolveSymlinkPrefix(absoluteRoot)
	rel, err := filepath.Rel(absoluteRoot, absolute)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return path
	}
	return filepath.ToSlash(rel)
}

// resolveSymlinkPrefix resolves symlinks in the longest existing prefix of an
// absolute path, then rejoins any trailing components that do not exist yet.
// filepath.EvalSymlinks refuses the whole path when the leaf has not been
// created on disk, which desyncs a not-yet-existing edit target from a root
// directory that does exist wherever the two sit behind different symlinks
// (macOS resolves a temp directory under /var to /private/var). Without this,
// the mismatch sends Rel outside root and every file kind keyed off a
// repo-relative prefix (ai/rules/, plan/spec-, docs/) silently stops matching.
func resolveSymlinkPrefix(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	dir := filepath.Dir(path)
	if dir == path {
		return path
	}
	return filepath.Join(resolveSymlinkPrefix(dir), filepath.Base(path))
}

func directRulePath(path string) bool {
	if !strings.HasPrefix(path, rulesRel+"/") || !strings.HasSuffix(path, ".md") {
		return false
	}
	name := strings.TrimPrefix(path, rulesRel+"/")
	if strings.Contains(name, "/") || name == "TRIGGERS.md" || name == "CORE.md" || name == "INDEX.md" {
		return false
	}
	return true
}

// SessionCoverageOptions is the closed coverage-report argument contract.
type SessionCoverageOptions struct {
	Quiet      bool
	Transcript string
	Session    string
	RulesDir   string
	NoAppend   bool
}

// sessionCoverageReport is the same report set rule_coverage.py emitted.
type sessionCoverageReport struct {
	BlockingTotal    int      `json:"blocking-total"`
	AlwaysOnExcluded int      `json:"always-on-excluded"`
	AlwaysOnRules    []string `json:"always-on-rules"`
	Touched          int      `json:"touched"`
	Kinds            []string `json:"kinds"`
	RulesRead        []string `json:"rules-read"`
	Matched          []string `json:"matched"`
	Missed           []string `json:"missed"`
	Unmatchable      int      `json:"unmatchable"`
	UnmatchableRules []string `json:"unmatchable-rules"`

	quiet      bool
	changed    bool
	reportPath string
}

// Text renders the detector's default or quiet report. A repeated quiet report
// returns an empty string while its NDJSON row still persists.
func (r sessionCoverageReport) Text() string {
	if r.quiet {
		if !r.changed {
			return ""
		}
		var tb textbuf.Buffer
		return tb.Str("rule-coverage: ").Int(int64(len(r.Missed))).Str(" of ").
			Int(int64(r.BlockingTotal)).Str(" matched blocking rule(s) unread -> ").
			Str(r.reportPath).String()
	}

	var tb textbuf.Buffer
	if len(r.Missed) > 0 {
		tb.Str("rule-coverage: ").Int(int64(len(r.Missed))).
			Str(" blocking rule(s) matched this session's files but were never read:\n")
		for _, name := range r.Missed {
			tb.Str("  - ").Str(rulesRel).Byte('/').Str(name).Byte('\n')
		}
	} else {
		tb.Str("rule-coverage: 0 missed of ").Int(int64(len(r.Matched))).
			Str(" blocking rule(s) matched by ").Int(int64(r.Touched)).
			Str(" touched file(s)\n")
	}
	tb.Str("rule-coverage: ").Int(int64(r.Unmatchable)).Str(" of ").
		Int(int64(r.BlockingTotal)).
		Str(" blocking rules have action-shaped triggers that no file type can match, ").
		Str("so this count UNDER-reports; silence is not proof of coverage\n")
	tb.Str("rule-coverage: ").Int(int64(r.AlwaysOnExcluded)).
		Str(" always-on rule(s) sit outside that total; ai/rules/CORE.md carries their directives and ").
		Str("CLAUDE.md imports it, so no session Reads them and none is ever counted missed\n")
	return tb.Str("rule-coverage: report ").Str(r.reportPath).String()
}

type coverageRule struct {
	name     string
	trigger  string
	severity string
	alwaysOn bool
}

type fileKind struct {
	name     string
	matches  func(string) bool
	keywords []string
}

var coverageFileKinds = []fileKind{
	{name: "go", matches: func(path string) bool { return strings.HasSuffix(path, ".go") }, keywords: []string{
		"go", ".go", "goroutine", "wire-encoding", "wire encoding", "encoding path", "buffer", "pool",
		"allocation", "string-building", "fmt.sprintf", "hot path", "exported", "protocol-implementing",
		"protocol behavior", "wire format", "import", "function", "functions", "error", "errors",
	}},
	{name: "go-test", matches: func(path string) bool { return strings.HasSuffix(path, "_test.go") }, keywords: []string{testKeyword, testsKeyword, "tdd", "test-first"}},
	{name: "ci-test", matches: func(path string) bool { return strings.HasSuffix(path, ".ci") || strings.HasSuffix(path, ".et") }, keywords: []string{
		"functional test", "functional tests", ".ci", "user-facing behavior", "user-visible behavior", "test", "tests",
	}},
	{name: "yang", matches: func(path string) bool { return strings.HasSuffix(path, ".yang") }, keywords: []string{
		"yang", "config leaf", "config option", "config surface", "config content", "schema",
	}},
	{name: "docs", matches: func(path string) bool { return strings.HasPrefix(path, "docs/") && strings.HasSuffix(path, ".md") }, keywords: []string{
		"documentation", "docs", "doc", "prose", "user-visible behavior", "comment", "comments",
	}},
	{name: "spec", matches: func(path string) bool {
		return specpath.IsSpec(path) || strings.HasPrefix(path, "plan/design-")
	}, keywords: []string{
		"spec", "specs", "acceptance criterion", "acceptance criteria",
	}},
	{name: "learned", matches: func(path string) bool { return strings.HasPrefix(path, "plan/learned/") }, keywords: []string{"learned", "learned summary"}},
	{name: "rule", matches: func(path string) bool { return strings.HasPrefix(path, rulesRel+"/") && strings.HasSuffix(path, ".md") }, keywords: []string{"rule", "rules", "ai/rules/*.md"}},
	{name: "python", matches: func(path string) bool { return strings.HasSuffix(path, ".py") }, keywords: []string{"script", "scripts", "python", "tool", "gate", "hook", "test", "tests"}},
	{name: "shell", matches: func(path string) bool {
		return strings.HasSuffix(path, ".sh") || strings.HasPrefix(path, ".claude/hooks/")
	}, keywords: []string{"shell", "bash", "hook", "hooks"}},
	{name: "make", matches: func(path string) bool {
		return strings.HasSuffix(path, ".mk") || filepath.Base(filepath.FromSlash(path)) == "Makefile"
	}, keywords: []string{"make target", "makefile", "build", "gate"}},
}

// RunSessionCoverage runs the detector with an injected transcript source and
// clock. Diagnostics go to errOut so hook callers can preserve fail-speak
// behavior without redirecting process-global stderr.
func RunSessionCoverage(root string, options SessionCoverageOptions, source TranscriptSource,
	now func() time.Time, errOut io.Writer) (*sessionCoverageReport, int) {
	rulesDir := options.RulesDir
	if rulesDir == "" {
		rulesDir = filepath.Join(root, "ai", "rules")
	}
	if info, err := os.Stat(rulesDir); err != nil || !info.IsDir() {
		var tb textbuf.Buffer
		_, _ = fmt.Fprintln(errOut, tb.Str("rule-coverage: rule directory ").Str(rulesDir).
			Str(" does not exist; nothing to match against").String())
		return nil, 0
	}

	transcript := options.Transcript
	if transcript == "" {
		transcript = source.TranscriptPath(root)
	}
	if info, err := os.Stat(transcript); transcript == "" || err != nil || !info.Mode().IsRegular() {
		shown := transcript
		if shown == "" {
			shown = "none resolved"
		}
		var tb textbuf.Buffer
		_, _ = fmt.Fprintln(errOut, tb.Str("rule-coverage: no readable session transcript (").Str(shown).
			Str("); reporting nothing rather than guessing which rules were consulted").String())
		return nil, 0
	}

	rules, err := loadCoverageRules(rulesDir, errOut)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return nil, 1
	}
	files, err := source.Files(root, transcript)
	if err != nil {
		var tb textbuf.Buffer
		_, _ = fmt.Fprintln(errOut, tb.Str("rule-coverage: cannot read the session transcript ").Str(transcript).
			Str(": ").Err(err).Str("; reporting nothing rather than guessing which rules were consulted").String())
		files = TranscriptFiles{Written: []string{}, RulesRead: []string{}}
	}
	report := analyzeSessionCoverage(rules, files)

	session := options.Session
	if session == "" {
		session = "unknown"
	}
	reportPath := filepath.Join(root, filepath.FromSlash(ruleCoverageReportRel))
	var prior []string
	hadPrior := false
	if options.Quiet {
		prior, hadPrior = previousCoverageMissed(reportPath, session)
	}
	if !options.NoAppend {
		if err := appendCoverageReport(reportPath, session, report, now()); err != nil {
			var tb textbuf.Buffer
			_, _ = fmt.Fprintln(errOut, tb.Str("rule-coverage: cannot record report at ").Str(reportPath).
				Str(": ").Err(err).Str("; the analysis below still stands, only the accumulated evidence is lost").String())
		}
	}
	report.quiet = options.Quiet
	report.changed = !hadPrior || !sameStrings(prior, report.Missed)
	report.reportPath = reportPath
	if len(report.Missed) > 0 {
		return &report, 1
	}
	return &report, 0
}

func loadCoverageRules(rulesDir string, errOut io.Writer) ([]coverageRule, error) {
	parsed, err := loadRulesAllowEmpty(rulesDir)
	if err != nil {
		return nil, err
	}
	alwaysOn := loadAlwaysOnRules(rulesDir, errOut)
	rules := make([]coverageRule, 0, len(parsed))
	for i := range parsed {
		rule := &parsed[i]
		rules = append(rules, coverageRule{
			name:     rule.Name,
			trigger:  rule.Trigger,
			severity: rule.Severity,
			alwaysOn: alwaysOn[rule.Name],
		})
	}
	return rules, nil
}

func loadAlwaysOnRules(rulesDir string, errOut io.Writer) map[string]bool {
	path := filepath.Join(rulesDir, "CORE.md")
	raw, err := os.ReadFile(path) // #nosec G304 -- rulesDir is the selected corpus
	if err != nil {
		var tb textbuf.Buffer
		_, _ = fmt.Fprintln(errOut, tb.Str("rule-coverage: cannot read ").Str(path).Str(": ").Err(err).
			Str("; excluding no always-on rule, so any always-on rule its triggers match will be reported").String())
		return map[string]bool{}
	}
	members := make(map[string]bool)
	prefix := "`" + rulesRel + "/"
	for rawLine := range strings.SplitSeq(string(raw), "\n") {
		line := strings.TrimRight(rawLine, " \t\r\n\v\f")
		if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, ".md`") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(line, prefix), "`")
		stem := strings.TrimSuffix(name, ".md")
		if stem == "" || strings.ContainsAny(stem, "/`") {
			continue
		}
		members[name] = true
	}
	if len(members) == 0 {
		var tb textbuf.Buffer
		_, _ = fmt.Fprintln(errOut, tb.Str("rule-coverage: ").Str(path).
			Str(" is readable but carries no `").Str(rulesRel).
			Str("/<name>.md` line; excluding no always-on rule. The generator (internal/le/rules.RenderArtifacts) has most likely changed shape").String())
	}
	return members
}

func analyzeSessionCoverage(rules []coverageRule, files TranscriptFiles) sessionCoverageReport {
	written := make(map[string]bool, len(files.Written))
	for _, path := range files.Written {
		written[path] = true
	}
	read := make(map[string]bool, len(files.RulesRead))
	for _, name := range files.RulesRead {
		read[name] = true
	}
	kinds := kindsForCoverage(written)
	report := sessionCoverageReport{
		AlwaysOnRules:    []string{},
		Kinds:            sessionCoverageSortedKeys(kinds),
		RulesRead:        sessionCoverageSortedKeys(read),
		Matched:          []string{},
		Missed:           []string{},
		UnmatchableRules: []string{},
		Touched:          len(written),
	}
	for _, rule := range rules {
		if rule.severity != "blocking" {
			continue
		}
		if rule.alwaysOn {
			report.AlwaysOnRules = append(report.AlwaysOnRules, rule.name)
			continue
		}
		report.BlockingTotal++
		trigger := strings.ToLower(rule.trigger)
		hitAny := false
		matched := false
		for _, kind := range coverageFileKinds {
			if !matchesCoverageKeyword(trigger, kind.keywords) {
				continue
			}
			hitAny = true
			if kinds[kind.name] {
				matched = true
				break
			}
		}
		if !hitAny {
			report.UnmatchableRules = append(report.UnmatchableRules, rule.name)
			continue
		}
		if matched {
			report.Matched = append(report.Matched, rule.name)
			if !read[rule.name] {
				report.Missed = append(report.Missed, rule.name)
			}
		}
	}
	report.AlwaysOnExcluded = len(report.AlwaysOnRules)
	report.Unmatchable = len(report.UnmatchableRules)
	sort.Strings(report.AlwaysOnRules)
	sort.Strings(report.Matched)
	sort.Strings(report.Missed)
	sort.Strings(report.UnmatchableRules)
	return report
}

func kindsForCoverage(written map[string]bool) map[string]bool {
	kinds := make(map[string]bool)
	for path := range written {
		for _, kind := range coverageFileKinds {
			if kind.matches(path) {
				kinds[kind.name] = true
			}
		}
	}
	return kinds
}

func matchesCoverageKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		at := 0
		for at <= len(text)-len(keyword) {
			found := strings.Index(text[at:], keyword)
			if found < 0 {
				break
			}
			found += at
			beforeOK := found == 0 || !asciiAlphaNumeric(text[found-1])
			after := found + len(keyword)
			afterOK := after == len(text) || !asciiAlphaNumeric(text[after])
			if beforeOK && afterOK {
				return true
			}
			at = found + 1
		}
	}
	return false
}

func asciiAlphaNumeric(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
}

func previousCoverageMissed(path, session string) ([]string, bool) {
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed report path under the checkout
	if err != nil {
		return nil, false
	}
	lines := strings.Split(string(raw), "\n")
	for _, line := range slices.Backward(lines) {
		var row struct {
			Session string   `json:"session"`
			Missed  []string `json:"missed"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		if row.Session == session {
			if row.Missed == nil {
				row.Missed = []string{}
			}
			return row.Missed, true
		}
	}
	return nil, false
}

type coverageRecord struct {
	Timestamp   string   `json:"ts"`
	Session     string   `json:"session"`
	Touched     int      `json:"touched"`
	Kinds       []string `json:"kinds"`
	Matched     int      `json:"matched"`
	Read        int      `json:"read"`
	Missed      []string `json:"missed"`
	Unmatchable int      `json:"unmatchable"`
	AlwaysOn    int      `json:"always-on"`
}

func appendCoverageReport(path, session string, report sessionCoverageReport, now time.Time) error {
	row := coverageRecord{
		Timestamp:   now.Format("2006-01-02T15:04:05"),
		Session:     session,
		Touched:     report.Touched,
		Kinds:       report.Kinds,
		Matched:     len(report.Matched),
		Read:        len(report.RulesRead),
		Missed:      report.Missed,
		Unmatchable: report.Unmatchable,
		AlwaysOn:    report.AlwaysOnExcluded,
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		return err
	}
	// 0o777 is deliberate: every agent account on this host appends to one report.
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil { //nolint:gosec // shared across agent accounts, see above
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666) //nolint:gosec // fixed report path, and 0o666 lets every agent account append to it
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, writeErr := file.Write(encoded)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func sessionCoverageSortedKeys[V any](set map[string]V) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for index := range leftCopy {
		if leftCopy[index] != rightCopy[index] {
			return false
		}
	}
	return true
}

func coverageReportAnswer(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	options := SessionCoverageOptions{
		Quiet:      args.Has("quiet"),
		Transcript: args["transcript"],
		Session:    args["session"],
		RulesDir:   args["rules-dir"],
		NoAppend:   args.Has("no-append"),
	}
	return RunSessionCoverage(root, options, NativeTranscriptSource{}, time.Now, os.Stderr)
}

// testKeyword is the trigger word a task description uses for test work.
const testKeyword = "test"

// testsKeyword is the plural trigger word a task description uses for test work.
const testsKeyword = "tests"
