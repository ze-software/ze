// Design: docs/architecture/core-design.md -- RFC-tagged evidence is one owned surface
// Related: goscope.go -- the canonical tagged-unit boundaries this action exposes
package rfc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// TaggedScopeMaxBytes is the largest stdin request the action accepts. It is
// the command-wide whole-stdin cap, so replacing the Python hook cannot turn
// one pre-write check into an unbounded allocation.
const TaggedScopeMaxBytes = cliio.MaxStdinBytes

// taggedScopeActionRequest is the proposed full-file content and the edit hunks that
// selected it. Operation is "write" or "edit". Edit requires at least one hunk;
// Write carries none.
type taggedScopeActionRequest struct {
	Operation string     `json:"operation"`
	Content   *string    `json:"content"`
	Hunks     []EditHunk `json:"hunks,omitempty"`
}

// TaggedScopeChange is one RFC-tagged unit whose behaviour the proposed content
// changes.
type TaggedScopeChange struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// taggedScopeActionReport is the actionable pre-write decision for one proposed file.
// Scope is the exact widened old text rfc_tagged_scope.py would have returned;
// null means the old helper returned None.
type taggedScopeActionReport struct {
	Path       string              `json:"path"`
	Carrier    bool                `json:"carrier"`
	Allowed    bool                `json:"allowed"`
	Decision   string              `json:"decision"`
	Reason     string              `json:"reason"`
	Message    string              `json:"message"`
	Resolution ScopeKind           `json:"resolution,omitempty"`
	Scope      *string             `json:"scope"`
	Changes    []TaggedScopeChange `json:"changes"`
}

// Text renders the actionable message carried by the structured answer.
func (r taggedScopeActionReport) Text() string { return r.Message + "\n" }

// evaluateTaggedScope compares the proposed full-file content with path in
// tree. A missing path is a new Write and has no old RFC claim to weaken.
func evaluateTaggedScope(tree, path string, request taggedScopeActionRequest) (taggedScopeActionReport, error) {
	absolute, relative, err := taggedScopePath(tree, path)
	if err != nil {
		return taggedScopeError(path, err), err
	}
	if err := validateTaggedScopeRequest(request); err != nil {
		return taggedScopeError(relative, err), err
	}
	report := taggedScopeActionReport{
		Path: relative, Carrier: IsTagCarrier(relative), Allowed: true,
		Decision: "allow", Scope: nil, Changes: []TaggedScopeChange{},
	}
	if !report.Carrier {
		return taggedScopeAllowed(report, "not-carrier", "path is not an RFC evidence carrier"), nil
	}

	old, exists, err := readTaggedScopeExisting(absolute, TaggedScopeMaxBytes)
	if err != nil {
		err = fmt.Errorf("read existing file: %w", err)
		return taggedScopeError(relative, err), err
	}
	if !exists && request.Operation == "edit" {
		err = errors.New("edit target does not exist")
		return taggedScopeError(relative, err), err
	}
	if !utf8.Valid(old) || bytes.IndexByte(old, 0) >= 0 {
		err = errors.New("existing content is not valid UTF-8 text")
		return taggedScopeError(relative, err), err
	}

	oldText, newText := string(old), *request.Content
	if request.Operation == "edit" {
		if scope, held := tagScope(relative, oldText, request.Hunks, changedTagRE); held {
			report.Scope = &scope
		}
	}
	if oldText == newText {
		return taggedScopeAllowed(report, "unchanged", "proposed content is unchanged"), nil
	}
	if !changedTagRE.MatchString(oldText) {
		return taggedScopeAllowed(report, "no-tags", "existing content carries no RFC requirement tag"), nil
	}

	report.Resolution, report.Changes = changedTaggedUnits(relative, oldText, newText)
	if len(report.Changes) == 0 {
		return taggedScopeAllowed(report, "unchanged-behaviour", "no RFC-tagged test behaviour changed"), nil
	}
	report.Allowed = false
	report.Decision = "block"
	report.Reason = "changed-rfc-test"
	report.Message = taggedScopeBlockedMessage(relative, report.Changes)
	return report, nil
}

func validateTaggedScopeRequest(request taggedScopeActionRequest) error {
	if request.Content == nil {
		return errors.New("stdin request has no content field")
	}
	if !utf8.ValidString(*request.Content) || strings.IndexByte(*request.Content, 0) >= 0 {
		return errors.New("proposed content is not valid UTF-8 text")
	}
	switch request.Operation {
	case "write":
		if len(request.Hunks) != 0 {
			return errors.New("write request carries edit hunks")
		}
	case "edit":
		if len(request.Hunks) == 0 {
			return errors.New("edit request carries no hunks")
		}
		for _, hunk := range request.Hunks {
			if hunk.Old == "" {
				return errors.New("edit request carries an empty old hunk")
			}
		}
	default:
		return fmt.Errorf("unknown operation %q; expected write or edit", request.Operation)
	}
	return nil
}

func taggedScopePath(tree, path string) (string, string, error) {
	if path == "" {
		return "", "", errors.New("path is empty")
	}
	root, err := filepath.Abs(tree)
	if err != nil {
		return "", "", fmt.Errorf("resolve checkout root: %w", err)
	}
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(root, filepath.FromSlash(path))
	}
	absolute = filepath.Clean(absolute)
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return "", "", fmt.Errorf("resolve path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", filepath.ToSlash(relative), errors.New("path is outside the checkout")
	}
	return absolute, filepath.ToSlash(relative), nil
}

func changedTaggedUnits(path, oldText, newText string) (ScopeKind, []TaggedScopeChange) {
	if ScopeReader(path) != ScopeGo || tagOutsideFunction(oldText) {
		tags := ChangedTags(path, oldText, newText)
		if len(tags) == 0 {
			return ScopeFile, nil
		}
		return ScopeFile, []TaggedScopeChange{{Name: fileScopeName(path), Tags: tags}}
	}

	oldOrder, oldUnits := namedFunctionTexts(oldText)
	_, newUnits := namedFunctionTexts(newText)
	changes := make([]TaggedScopeChange, 0)
	for _, name := range oldOrder {
		oldUnit := oldUnits[name]
		if strings.TrimSpace(oldUnit) == "" {
			continue
		}
		tags := ChangedTags(path, oldUnit, newUnits[name])
		if len(tags) == 0 {
			continue
		}
		shown := name
		if shown == "" {
			shown = fileScopeName(path)
		}
		changes = append(changes, TaggedScopeChange{Name: shown, Tags: tags})
	}
	return ScopeFunc, changes
}

func tagOutsideFunction(content string) bool {
	spans := goFuncSpans(content)
	for _, match := range changedTagRE.FindAllStringIndex(content, -1) {
		inside := false
		for _, one := range spans {
			if one.begin <= match[0] && match[0] < one.end {
				inside = true
				break
			}
		}
		if !inside {
			return true
		}
	}
	return false
}

func namedFunctionTexts(content string) ([]string, map[string]string) {
	order := make([]string, 0)
	units := make(map[string]string)
	for _, unit := range FunctionUnits(content) {
		if previous, found := units[unit.Name]; found {
			units[unit.Name] = previous + "\n" + unit.Text
			continue
		}
		order = append(order, unit.Name)
		units[unit.Name] = unit.Text
	}
	return order, units
}

func fileScopeName(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func readTaggedScopeExisting(path string, limit int64) ([]byte, bool, error) {
	file, err := os.Open(path) //nolint:gosec // path is confined to the checkout before this helper
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer file.Close() //nolint:errcheck // the read result is authoritative; close cannot change it
	body, err := readTaggedScopeBytes(file, limit)
	if err != nil {
		return nil, true, err
	}
	return body, true, nil
}

func readTaggedScopeBytes(reader io.Reader, limit int64) ([]byte, error) {
	if limit < 0 {
		return nil, errors.New("stdin limit is negative")
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("content exceeds %d bytes", limit)
	}
	return body, nil
}

func readTaggedScopeRequest(reader io.Reader, limit int64) (taggedScopeActionRequest, error) {
	body, err := readTaggedScopeBytes(reader, limit)
	if err != nil {
		return taggedScopeActionRequest{}, fmt.Errorf("read proposed content from stdin: %w", err)
	}
	if !utf8.Valid(body) {
		return taggedScopeActionRequest{}, errors.New("stdin request is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var request taggedScopeActionRequest
	if err := decoder.Decode(&request); err != nil {
		return taggedScopeActionRequest{}, fmt.Errorf("decode stdin request: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return taggedScopeActionRequest{}, errors.New("decode stdin request: trailing content")
	}
	if err := validateTaggedScopeRequest(request); err != nil {
		return taggedScopeActionRequest{}, err
	}
	return request, nil
}

func taggedScopeAllowed(report taggedScopeActionReport, reason, message string) taggedScopeActionReport {
	report.Reason = reason
	report.Message = "allowed " + report.Path + ": " + message
	return report
}

func taggedScopeBlockedMessage(path string, changes []TaggedScopeChange) string {
	var out textbuf.Buffer
	out.Str("blocked ").Str(path).Str(": RFC-tagged test behaviour changed")
	for _, change := range changes {
		out.Str("\n  ").Str(change.Name).Str(": ").Join(change.Tags, ", ")
	}
	return out.Str("\nRecord the owner's approval for each named unit in test/rfc-changed.md before writing.").String()
}

func taggedScopeError(path string, err error) taggedScopeActionReport {
	return taggedScopeActionReport{
		Path: path, Allowed: false, Decision: "error", Reason: "cannot-judge",
		Message: "cannot judge " + path + ": " + err.Error(), Changes: []TaggedScopeChange{},
	}
}

func taggedScopeAnswer(args leaction.Arguments) (any, int) {
	path, held := args["path"]
	if !held {
		err := errors.New("rfc tagged-scope requires path <path>")
		return taggedScopeError("", err), 2
	}
	root, err := lepath.Root()
	if err != nil {
		return taggedScopeError(path, err), 2
	}
	reader, err := cliio.OpenReader(cliio.StdinToken)
	if err != nil {
		return taggedScopeError(path, err), 2
	}
	defer reader.Close() //nolint:errcheck // stdin close is a no-op; the process exits after the action
	request, err := readTaggedScopeRequest(reader, TaggedScopeMaxBytes)
	if err != nil {
		return taggedScopeError(path, err), 2
	}
	report, err := evaluateTaggedScope(root, path, request)
	if err != nil {
		return report, 2
	}
	if !report.Allowed {
		return report, 1
	}
	return report, 0
}
