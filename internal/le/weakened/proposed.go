// Design: .claude/hooks/pretool-writeedit.py -- pre-write weakening and RFC approval hatches
package weakened

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

const (
	ProposedInputLimit = 16 << 20
	proposedFileLimit  = 8 << 20
	rfcChangedLedger   = "test/rfc-changed.md"
)

// ProposedRequest is the bounded stdin contract for `le test-weakened proposed`.
// Fully reconstructed old/new values win over ToolInput. Base64 fields preserve
// arbitrary bytes and are decoded with the hook's invalid-UTF-8 replacement rule.
type ProposedRequest struct {
	Path      string            `json:"path"`
	Tool      string            `json:"tool"`
	Exists    *bool             `json:"exists,omitempty"`
	ToolInput ProposedToolInput `json:"tool-input,omitempty"`
	Old       *string           `json:"old,omitempty"`
	New       *string           `json:"new,omitempty"`
	OldBase64 *string           `json:"old-base64,omitempty"`
	NewBase64 *string           `json:"new-base64,omitempty"`
}

// ProposedToolInput carries the native Write/Edit/MultiEdit payload subset.
type ProposedToolInput struct {
	FilePath   string         `json:"file_path,omitempty"`
	Content    string         `json:"content,omitempty"`
	OldString  string         `json:"old_string,omitempty"`
	NewString  string         `json:"new_string,omitempty"`
	ReplaceAll bool           `json:"replace_all,omitempty"`
	Edits      []ProposedEdit `json:"edits,omitempty"`
}

// ProposedEdit is one MultiEdit hunk.
type ProposedEdit struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

// ProposedRFCChange is one owner-governed tagged unit changed by the proposal.
type ProposedRFCChange struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// ProposedLedger reports exactly which names one on-disk hatch accepts.
type ProposedLedger struct {
	Path     string   `json:"path"`
	Rows     int      `json:"rows"`
	Names    []string `json:"names"`
	Missing  []string `json:"missing,omitempty"`
	Matched  []Row    `json:"matched,omitempty"`
	Problems []string `json:"problems,omitempty"`
}

// ProposedReport is the structured pre-write verdict.
type ProposedReport struct {
	Path       string              `json:"path"`
	Tool       string              `json:"tool"`
	Weakened   []Finding           `json:"weakened,omitempty"`
	RFCChanges []ProposedRFCChange `json:"rfc-changes,omitempty"`
	Ledgers    []ProposedLedger    `json:"ledgers,omitempty"`
	Messages   []string            `json:"messages,omitempty"`
	Blocking   bool                `json:"blocking"`
	Notice     bool                `json:"notice"`
}

// ExitCode preserves the hook contract: a permitted edit or count-only notice
// is zero; a governed change without its exact row is two.
func (r ProposedReport) ExitCode() int {
	if r.Blocking {
		return 2
	}
	return 0
}

// Text renders the actionable hook diagnosis.
func (r ProposedReport) Text() string {
	var text textbuf.Buffer
	if !r.Blocking && !r.Notice {
		return text.Str("test-weakened proposed: clean for ").Str(r.Path).Byte('\n').String()
	}
	if r.Blocking {
		text.Str("BLOCKED: proposed test weakening in ").Str(r.Path).Byte('\n')
	} else {
		text.Str("notice: proposed edit lowers a test count in ").Str(r.Path).Byte('\n')
	}
	for _, message := range r.Messages {
		text.Str("  ").Str(message).Byte('\n')
	}
	for _, ledger := range r.Ledgers {
		for _, problem := range ledger.Problems {
			text.Str("  ").Str(problem).Byte('\n')
		}
		for _, name := range ledger.Missing {
			text.Str("  add first to ").Str(ledger.Path).Str(": | ").Str(name).
				Str(" | <what was approved or removed, and why> |\n")
		}
	}
	if r.Blocking {
		text.Str("  Fix the code by default. A test/weakened.md row is self-service; ").
			Str("it never substitutes for owner approval in test/rfc-changed.md.\n")
	}
	return text.String()
}

// Proposed reads one bounded JSON request and judges the bytes the edit would
// produce without requiring those bytes to exist on disk yet.
func Proposed(root string, input io.Reader) (ProposedReport, error) {
	request, err := decodeProposedRequest(input)
	if err != nil {
		return ProposedReport{}, err
	}
	rawPath := request.Path
	if rawPath == "" {
		rawPath = request.ToolInput.FilePath
	}
	path, err := proposedPath(root, rawPath)
	if err != nil {
		return ProposedReport{}, err
	}
	report := ProposedReport{Path: path, Tool: request.Tool}
	if request.Exists != nil && !*request.Exists {
		return report, nil
	}
	if request.Exists == nil && request.Tool == "Write" &&
		request.Old == nil && request.OldBase64 == nil {
		if _, currentErr := proposedCurrent(root, path); errors.Is(currentErr, os.ErrNotExist) {
			return report, nil
		}
	}
	oldText, newText, err := proposedTexts(root, path, request)
	if err != nil {
		return ProposedReport{}, err
	}
	hookTest := strings.HasSuffix(path, "_test.go") ||
		(strings.HasSuffix(path, ".ci") || strings.HasSuffix(path, ".et")) &&
			(strings.HasPrefix(path, "test/") || strings.Contains(path, "/test/"))
	taggedCarrier := rfc.IsTagCarrier(path) && strings.Contains(oldText, "RFC requirement:")
	if !hookTest && !taggedCarrier {
		return report, nil
	}
	packageName := filepath.Base(filepath.Dir(path))
	if filepath.Dir(path) == "." {
		packageName = ""
	}

	rfcChanges := proposedRFCChanges(path, oldText, newText)
	report.RFCChanges = rfcChanges
	if len(rfcChanges) != 0 {
		names := make([]string, 0, len(rfcChanges))
		for _, change := range rfcChanges {
			names = appendUniqueName(names, change.Name)
			report.Messages = append(report.Messages,
				"RFC-TAGGED test changed: "+change.Name+" ("+strings.Join(change.Tags, ", ")+")")
		}
		ledger := proposedLedger(root, rfcChangedLedger, path, packageName, names)
		report.Ledgers = append(report.Ledgers, ledger)
		if len(ledger.Missing) != 0 || len(ledger.Problems) != 0 {
			report.Blocking = true
		}
	}

	verdict := detect(oldText, newText, path)
	findings := proposedFindings(path, oldText, newText)
	report.Weakened = findings
	if len(verdict.blocking) != 0 {
		names := make([]string, 0, len(findings))
		for _, finding := range findings {
			names = appendUniqueName(names, finding.Name)
		}
		if len(names) == 0 {
			names = []string{strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
		}
		ledger := proposedLedger(root, ContractPath, path, packageName, names)
		report.Ledgers = append(report.Ledgers, ledger)
		if len(ledger.Missing) != 0 || len(ledger.Problems) != 0 {
			report.Blocking = true
		}
		for _, detail := range append(append([]string{}, verdict.blocking...), verdict.advisory...) {
			report.Messages = append(report.Messages, detail)
		}
	} else if len(verdict.advisory) != 0 {
		report.Notice = true
		for _, detail := range verdict.advisory {
			report.Messages = append(report.Messages, detail)
		}
	}
	return report, nil
}

func decodeProposedRequest(input io.Reader) (ProposedRequest, error) {
	limited := io.LimitReader(input, ProposedInputLimit+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return ProposedRequest{}, err
	}
	if len(content) > ProposedInputLimit {
		return ProposedRequest{}, fmt.Errorf("proposed JSON exceeds %d bytes", ProposedInputLimit)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var request ProposedRequest
	if err := decoder.Decode(&request); err != nil {
		return ProposedRequest{}, fmt.Errorf("decode proposed JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ProposedRequest{}, errors.New("proposed stdin must contain exactly one JSON object")
	}
	return request, nil
}

func proposedTexts(root, path string, request ProposedRequest) (string, string, error) {
	hasText := request.Old != nil || request.New != nil
	hasBase64 := request.OldBase64 != nil || request.NewBase64 != nil
	if hasText || hasBase64 {
		if hasText && hasBase64 || (request.Old == nil) != (request.New == nil) ||
			(request.OldBase64 == nil) != (request.NewBase64 == nil) {
			return "", "", errors.New("fully reconstructed input requires exactly one old/new or old-base64/new-base64 pair")
		}
		if hasText {
			return boundedProposedText(*request.Old, *request.New)
		}
		oldBytes, err := base64.StdEncoding.DecodeString(*request.OldBase64)
		if err != nil {
			return "", "", fmt.Errorf("decode old-base64: %w", err)
		}
		newBytes, err := base64.StdEncoding.DecodeString(*request.NewBase64)
		if err != nil {
			return "", "", fmt.Errorf("decode new-base64: %w", err)
		}
		return boundedProposedText(validUTF8(oldBytes), validUTF8(newBytes))
	}
	current, currentErr := proposedCurrent(root, path)
	switch request.Tool {
	case "Write":
		if currentErr != nil && !errors.Is(currentErr, os.ErrNotExist) {
			return "", "", currentErr
		}
		return boundedProposedText(current, request.ToolInput.Content)
	case "Edit":
		if currentErr == nil && request.ToolInput.OldString != "" && strings.Contains(current, request.ToolInput.OldString) {
			count := 1
			if request.ToolInput.ReplaceAll {
				count = -1
			}
			return boundedProposedText(current, strings.Replace(current, request.ToolInput.OldString, request.ToolInput.NewString, count))
		}
		return boundedProposedText(request.ToolInput.OldString, request.ToolInput.NewString)
	case "MultiEdit":
		if currentErr == nil {
			after := current
			for _, edit := range request.ToolInput.Edits {
				if edit.OldString == "" || !strings.Contains(after, edit.OldString) {
					return proposedJoinedHunks(request.ToolInput.Edits)
				}
				count := 1
				if edit.ReplaceAll {
					count = -1
				}
				after = strings.Replace(after, edit.OldString, edit.NewString, count)
			}
			return boundedProposedText(current, after)
		}
		return proposedJoinedHunks(request.ToolInput.Edits)
	default:
		return "", "", fmt.Errorf("unsupported tool %q; want Write, Edit, or MultiEdit", request.Tool)
	}
}

func proposedJoinedHunks(edits []ProposedEdit) (string, string, error) {
	oldParts := make([]string, len(edits))
	newParts := make([]string, len(edits))
	for index, edit := range edits {
		oldParts[index] = edit.OldString
		newParts[index] = edit.NewString
	}
	return boundedProposedText(strings.Join(oldParts, "\n"), strings.Join(newParts, "\n"))
}

func proposedCurrent(root, path string) (string, error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	content, readErr := repository.ReadFile(filepath.FromSlash(path))
	closeErr := repository.Close()
	if readErr != nil {
		return "", readErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return validUTF8(content), nil
}

func boundedProposedText(oldText, newText string) (string, string, error) {
	if len(oldText) > proposedFileLimit || len(newText) > proposedFileLimit {
		return "", "", fmt.Errorf("proposed old/new file exceeds %d bytes", proposedFileLimit)
	}
	return oldText, newText, nil
}

func validUTF8(content []byte) string {
	return strings.ToValidUTF8(string(content), "\uFFFD")
}

func proposedPath(root, raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n") {
		return "", errors.New("proposed path is empty or unsafe")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := raw
	if filepath.IsAbs(path) {
		path, err = filepath.Rel(absoluteRoot, filepath.Clean(path))
		if err != nil {
			return "", err
		}
	}
	path = filepath.Clean(path)
	if path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("proposed path is outside the checkout: %s", raw)
	}
	path = filepath.ToSlash(path)
	if path == ".git" || strings.HasPrefix(path, ".git/") {
		return "", fmt.Errorf("proposed path is a Git internal: %s", raw)
	}
	return path, nil
}

func proposedFindings(path, oldText, newText string) []Finding {
	packageName := filepath.Base(filepath.Dir(path))
	if filepath.Dir(path) == "." {
		packageName = ""
	}
	verdicts := weakenedUnits(path, oldText, newText)
	findings := make([]Finding, 0, len(verdicts))
	for _, verdict := range verdicts {
		findings = append(findings, Finding{
			Path: path, Package: packageName, Name: verdict.name,
			Details: append([]string(nil), verdict.details...),
		})
	}
	return findings
}

func proposedRFCChanges(path, oldText, newText string) []ProposedRFCChange {
	if !rfc.IsTagCarrier(path) {
		return nil
	}
	fallback := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if rfc.ScopeReader(path) != rfc.ScopeGo || proposedTagOutsideFunction(path, oldText) {
		tags := rfc.ChangedTags(path, oldText, newText)
		if len(tags) == 0 {
			return nil
		}
		return []ProposedRFCChange{{Name: fallback, Tags: tags}}
	}
	newByName := make(map[string][]string)
	for _, unit := range rfc.FunctionUnits(newText) {
		newByName[unit.Name] = append(newByName[unit.Name], unit.Text)
	}
	changes := make([]ProposedRFCChange, 0)
	for _, unit := range rfc.FunctionUnits(oldText) {
		newUnit := ""
		if len(newByName[unit.Name]) == 1 {
			newUnit = newByName[unit.Name][0]
		}
		tags := rfc.ChangedTags(path, unit.Text, newUnit)
		if len(tags) == 0 {
			continue
		}
		name := unit.Name
		if name == "" {
			name = fallback
		}
		changes = append(changes, ProposedRFCChange{Name: name, Tags: tags})
	}
	sort.Slice(changes, func(left, right int) bool { return changes[left].Name < changes[right].Name })
	return changes
}

func proposedTagOutsideFunction(path, content string) bool {
	for lineNumber, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, "RFC requirement:") {
			continue
		}
		if rfc.UnitAt(path, content, lineNumber+1).Scope == rfc.ScopeFile {
			return true
		}
	}
	return false
}

// proposedLedger checks names, the findings of the file at findingPath,
// against the ledger at path (test/weakened.md, or the RFC-changed ledger).
// Matching goes through rowMatches directly, not the exported RowMatches, so
// a path-scoped row (scopedRowMatches in weakened.go) is honoured here too:
// an edit inside a tree a scoped row already covers must not be reported as
// missing a row of its own.
func proposedLedger(root, path, findingPath, packageName string, names []string) ProposedLedger {
	ledger := ProposedLedger{Path: path, Names: append([]string(nil), names...)}
	repository, err := os.OpenRoot(root)
	if err != nil {
		ledger.Problems = []string{"cannot open checkout to read " + path + ": " + err.Error()}
		return ledger
	}
	content, readErr := repository.ReadFile(filepath.FromSlash(path))
	closeErr := repository.Close()
	if readErr != nil {
		ledger.Problems = []string{path + " does not exist yet or is unreadable; write it, header first: " + readErr.Error()}
		return ledger
	}
	if closeErr != nil {
		ledger.Problems = []string{"cannot close checkout after reading " + path + ": " + closeErr.Error()}
		return ledger
	}
	rows, problems := parseLedger(validUTF8(content), path)
	ledger.Rows = len(rows)
	ledger.Problems = problems
	if len(problems) != 0 {
		return ledger
	}
	for _, name := range names {
		matched := false
		for _, row := range rows {
			if rowMatches(row.Name, Finding{Path: findingPath, Package: packageName, Name: name}) {
				ledger.Matched = append(ledger.Matched, row)
				matched = true
				break
			}
		}
		if !matched {
			ledger.Missing = append(ledger.Missing, name)
		}
	}
	return ledger
}

func appendUniqueName(names []string, name string) []string {
	for _, held := range names {
		if held == name {
			return names
		}
	}
	return append(names, name)
}
