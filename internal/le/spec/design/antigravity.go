// Design: docs/contributing/spec-workflow.md -- the independent design judge
// Related: judge.go -- the judge action that checks this evidence
//
// antigravity.go resolves an Antigravity (agy) judge's identity and file-read
// proof directly from the conversation's own transcript under
// $ANTIGRAVITY_APP_DATA_DIR/brain/$ANTIGRAVITY_CONVERSATION_ID/.system_generated/logs/.
// Antigravity does not run Claude Code's PostToolUse Read hook, so the judge
// derives whole-door and section digests from the verbatim read_file results
// recorded in the conversation transcript, and refuses when this conversation
// itself wrote or edited the spec.
package specdesign

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/doc/readrecord"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// Antigravity environment variables exposed to agent commands.
const (
	agyConversationIDEnv = "ANTIGRAVITY_CONVERSATION_ID"
	agyAppDataDirEnv     = "ANTIGRAVITY_APP_DATA_DIR"
	agyFingerprintPrefix = "antigravity+"
)

// Antigravity transcript step types and tool names.
const (
	agyStepPlanner = "PLANNER_RESPONSE"
	agyStepGeneric = "GENERIC"
	agyStatusDone  = "DONE"
	agyToolRead    = "read_file"
	agyToolView    = "view_file"
	agyToolWrite   = "write_file"
	agyToolEdit    = "edit_file"
)

// antigravityReadHeaders match the two line headers read_file writes before
// numbered file lines in transcript_full.jsonl.
var (
	agyShowingLinesPattern = regexp.MustCompile(`^Showing lines (\d+)-(\d+) of (\d+)$`)
	agyPartialReadPattern  = regexp.MustCompile(`^Partial read \(lines (\d+)-(\d+) of (\d+)\)\. Showing bytes (\d+)-(\d+) of (\d+)`)
)

// agyTranscriptEntry is the subset of one Antigravity transcript JSONL line
// needed to pair tool calls with their results.
type agyTranscriptEntry struct {
	StepIndex int           `json:"step_index"`
	Type      string        `json:"type"`
	Status    string        `json:"status"`
	Content   string        `json:"content"`
	ToolCalls []agyToolCall `json:"tool_calls"`
}

// agyToolCall is one tool invocation recorded on a PLANNER_RESPONSE step.
type agyToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// antigravityReads holds the verified reads and spec-edit flag derived from
// one Antigravity conversation transcript for a specific checkout.
type antigravityReads struct {
	// wholeDigests maps checkout-relative path -> set of WholeDigest values
	// observed from complete whole-file reads in this conversation.
	wholeDigests map[string]map[string]bool
	// sections maps checkout-relative path -> set of "<digest>\t<heading>"
	// entries observed from complete sections returned in this conversation.
	sections map[string]map[string]bool
	// editedSpec is true when this conversation invoked write_file or
	// edit_file on the spec being judged.
	editedSpec bool
}

// antigravityTranscriptPath locates the conversation's transcript file under
// appDataDir. It prefers transcript_full.jsonl and falls back to
// transcript.jsonl only when transcript_full.jsonl does not exist.
func antigravityTranscriptPath(appDataDir, conversationID string) (string, error) {
	logsDir := filepath.Join(appDataDir, "brain", conversationID, ".system_generated", "logs")
	full := filepath.Join(logsDir, "transcript_full.jsonl")
	info, err := os.Stat(full)
	if err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("antigravity transcript %s is a directory", full)
		}
		return full, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat antigravity transcript %s: %w", full, err)
	}
	compact := filepath.Join(logsDir, "transcript.jsonl")
	info, err = os.Stat(compact)
	if err != nil {
		return "", fmt.Errorf("locate antigravity transcript (%s or %s): %w", full, compact, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("antigravity transcript %s is a directory", compact)
	}
	return compact, nil
}

// resolveAntigravityCaller builds a judgeCaller from Antigravity's environment
// variables. It returns (caller, nil, true) when running inside Antigravity,
// (zero, refusal, true) when Antigravity is detected but unusable, and
// (zero, nil, false) when not running inside Antigravity.
func resolveAntigravityCaller() (judgeCaller, []string, bool) {
	convID := os.Getenv(agyConversationIDEnv)
	if convID == "" {
		return judgeCaller{}, nil, false
	}
	if !lepath.ValidSessionID(convID) {
		return judgeCaller{}, []string{
			"refused: " + agyConversationIDEnv + " is not a valid session id: " + lepath.IDRule(),
			"  fix: run the judge from an Antigravity agent shell with a valid conversation id",
		}, true
	}
	appDataDir := os.Getenv(agyAppDataDirEnv)
	if appDataDir == "" {
		return judgeCaller{}, []string{
			"refused: " + agyAppDataDirEnv + " is not set, so the Antigravity transcript cannot be located",
			"  why: in Antigravity, the judge verifies file reads from this conversation's transcript",
			"  fix: run the judge from an Antigravity agent shell where " + agyAppDataDirEnv + " is set",
		}, true
	}
	transcript, err := antigravityTranscriptPath(appDataDir, convID)
	if err != nil {
		return judgeCaller{}, []string{
			"refused: cannot locate the Antigravity transcript: " + err.Error(),
			"  why: in Antigravity, the judge verifies file reads from this conversation's transcript",
			"  fix: ensure this Antigravity conversation has written its transcript, then run the judge again",
		}, true
	}
	return judgeCaller{
		harness:        judgeHarnessAntigravity,
		fingerprint:    agyFingerprintPrefix + convID,
		transcriptPath: transcript,
	}, nil, true
}

// loadAntigravityReads parses transcriptPath and extracts every verified file
// read confined to root, plus whether this conversation edited specRelative.
// A transcript that cannot be opened or holds malformed JSON is an error,
// never an empty read set.
func loadAntigravityReads(root, transcriptPath, specRelative string) (antigravityReads, error) {
	file, err := os.Open(transcriptPath) //nolint:gosec // path derived from validated conversation id under ANTIGRAVITY_APP_DATA_DIR
	if err != nil {
		return antigravityReads{}, fmt.Errorf("open antigravity transcript %s: %w", transcriptPath, err)
	}
	defer file.Close() //nolint:errcheck // read-only

	var plannerSteps []agyTranscriptEntry
	genericByStep := map[int]agyTranscriptEntry{}
	decoder := json.NewDecoder(file)
	for {
		var entry agyTranscriptEntry
		err := decoder.Decode(&entry)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return antigravityReads{}, fmt.Errorf("parse antigravity transcript %s: %w", transcriptPath, err)
		}
		switch entry.Type {
		case agyStepPlanner:
			if len(entry.ToolCalls) > 0 {
				plannerSteps = append(plannerSteps, entry)
			}
		case agyStepGeneric:
			genericByStep[entry.StepIndex] = entry
		}
	}

	canonicalRoot := canonicalCheckoutPath(root)
	reads := antigravityReads{
		wholeDigests: map[string]map[string]bool{},
		sections:     map[string]map[string]bool{},
	}
	for _, step := range plannerSteps {
		for index, call := range step.ToolCalls {
			rawPath, hasPath := agyArgString(call.Args, "Path")
			if !hasPath {
				continue
			}
			if rawPath == "" {
				continue
			}
			rel, inside := relativeToCheckout(root, canonicalRoot, rawPath)
			if !inside {
				continue
			}
			switch call.Name {
			case agyToolWrite, agyToolEdit:
				if rel == specRelative {
					reads.editedSpec = true
				}
			case agyToolRead, agyToolView:
				result, found := genericByStep[step.StepIndex+1+index]
				if !found {
					continue
				}
				if result.Status != agyStatusDone {
					continue
				}
				recordAntigravityRead(&reads, rel, result.Content)
			}
		}
	}
	return reads, nil
}

// recordAntigravityRead parses one read_file result body and records its
// whole-file digest (when complete) and complete section digests on reads.
func recordAntigravityRead(reads *antigravityReads, docPath, body string) {
	text, whole, reachesEnd, ok := parseAntigravityRead(body)
	if !ok {
		return
	}
	if whole {
		if reads.wholeDigests[docPath] == nil {
			reads.wholeDigests[docPath] = map[string]bool{}
		}
		reads.wholeDigests[docPath][readrecord.WholeDigest(text)] = true
	}
	complete := readrecord.CompleteSections(text, reachesEnd)
	if len(complete) == 0 {
		return
	}
	if reads.sections[docPath] == nil {
		reads.sections[docPath] = map[string]bool{}
	}
	for _, section := range complete {
		reads.sections[docPath][section.Digest()+"\t"+section.Heading] = true
	}
}

// isTimestampLine reports whether line is a leading read_file timestamp header.
func isTimestampLine(line string) bool {
	if strings.HasPrefix(line, "Created At:") {
		return true
	}
	return strings.HasPrefix(line, "Completed At:")
}

// parseIntAtLeast parses raw as a decimal integer at least minValue.
func parseIntAtLeast(raw string, minValue int) (int, bool) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	if value < minValue {
		return 0, false
	}
	return value, true
}

// parseAntigravityRead extracts the verbatim lines returned by Antigravity's
// read_file tool from a GENERIC transcript step's content. It handles both
// complete ranges ("Showing lines S-E of T") and byte-truncated ranges
// ("Partial read (lines S-E of T). Showing bytes B0-B1 of BT..."). Anything
// else returns ok false so unreadable or unrecognized output never counts.
func parseAntigravityRead(content string) (text string, whole, reachesEnd, ok bool) {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, "\r")
	}
	headerIndex := 0
	for headerIndex < len(lines) && isTimestampLine(lines[headerIndex]) {
		headerIndex++
	}
	if headerIndex >= len(lines) {
		return "", false, false, false
	}
	header := lines[headerIndex]
	body := lines[headerIndex+1:]
	if match := agyShowingLinesPattern.FindStringSubmatch(header); match != nil {
		return parseShowingLines(match, body)
	}
	if match := agyPartialReadPattern.FindStringSubmatch(header); match != nil {
		return parsePartialRead(match, body)
	}
	return "", false, false, false
}

// parseShowingLines decodes a complete line range "Showing lines S-E of T".
func parseShowingLines(match, body []string) (text string, whole, reachesEnd, ok bool) {
	start, valid := parseIntAtLeast(match[1], 1)
	if !valid {
		return "", false, false, false
	}
	end, valid := parseIntAtLeast(match[2], start)
	if !valid {
		return "", false, false, false
	}
	total, valid := parseIntAtLeast(match[3], end)
	if !valid {
		return "", false, false, false
	}
	count := end - start + 1
	if len(body) < count {
		return "", false, false, false
	}
	extracted := make([]string, 0, count)
	for offset := range count {
		prefix := strconv.Itoa(start+offset) + ": "
		value, found := strings.CutPrefix(body[offset], prefix)
		if !found {
			return "", false, false, false
		}
		extracted = append(extracted, value)
	}
	return strings.Join(extracted, "\n"), start == 1 && end == total, end == total, true
}

// parsePartialRead decodes a byte-windowed "Partial read (lines S-E of T).
// Showing bytes B0-B1 of BT..." response. A truncated tail line is dropped so
// a mid-line byte cut never counts as a complete line.
func parsePartialRead(match, body []string) (text string, whole, reachesEnd, ok bool) {
	start, valid := parseIntAtLeast(match[1], 1)
	if !valid {
		return "", false, false, false
	}
	end, valid := parseIntAtLeast(match[2], start)
	if !valid {
		return "", false, false, false
	}
	total, valid := parseIntAtLeast(match[3], end)
	if !valid {
		return "", false, false, false
	}
	byteStart, valid := parseIntAtLeast(match[4], 0)
	if !valid {
		return "", false, false, false
	}
	byteEnd, valid := parseIntAtLeast(match[5], byteStart)
	if !valid {
		return "", false, false, false
	}
	byteTotal, valid := parseIntAtLeast(match[6], byteEnd)
	if !valid {
		return "", false, false, false
	}

	lineIndex := 0
	lineNum := start
	if byteStart > 0 {
		foundStart := false
		for lineIndex < len(body) {
			num, found := leadingLineNumber(body[lineIndex])
			if found && num >= start && num <= end {
				lineNum = num
				foundStart = true
				break
			}
			lineIndex++
		}
		if !foundStart {
			return "", false, false, false
		}
	}

	var extracted []string
	for lineIndex < len(body) && lineNum <= end {
		prefix := strconv.Itoa(lineNum) + ": "
		value, found := strings.CutPrefix(body[lineIndex], prefix)
		if !found {
			break
		}
		extracted = append(extracted, value)
		lineIndex++
		lineNum++
	}
	if byteEnd < byteTotal && len(extracted) > 0 {
		extracted = extracted[:len(extracted)-1]
	}
	if len(extracted) == 0 {
		return "", false, false, false
	}
	completeBytes := byteStart == 0 && byteEnd == byteTotal
	return strings.Join(extracted, "\n"),
		completeBytes && start == 1 && end == total,
		byteEnd == byteTotal && end == total,
		true
}

// leadingLineNumber parses a leading "<digits>: " prefix from one read_file
// body line.
func leadingLineNumber(line string) (int, bool) {
	digits, _, found := strings.Cut(line, ": ")
	if !found {
		return 0, false
	}
	if digits == "" {
		return 0, false
	}
	return parseIntAtLeast(digits, 1)
}

// agyArgString reads a string argument from an Antigravity tool call,
// unquoting compact transcript.jsonl strings that carry an extra JSON quote
// layer.
func agyArgString(args map[string]any, key string) (string, bool) {
	raw, ok := args[key].(string)
	if !ok {
		return "", false
	}
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		if unquoted, err := strconv.Unquote(raw); err == nil {
			return unquoted, true
		}
	}
	return raw, true
}

// canonicalCheckoutPath resolves root to an absolute symlink-resolved path so
// worktrees and symlinked checkouts compare by physical identity.
func canonicalCheckoutPath(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = filepath.Clean(root)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// relativeToCheckout resolves rawPath against root and returns its slash path
// relative to canonicalRoot when it lies inside the checkout.
func relativeToCheckout(root, canonicalRoot, rawPath string) (string, bool) {
	target := rawPath
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	canonicalTarget := canonicalCheckoutPath(target)
	rel, err := filepath.Rel(canonicalRoot, canonicalTarget)
	if err != nil {
		return "", false
	}
	if !filepath.IsLocal(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// doorRead reports whether reads holds a whole-file read of door matching its
// current content on disk. A door that cannot be read from disk is an error,
// never counted as read.
func (r antigravityReads) doorRead(root, door string) (bool, error) {
	digests := r.wholeDigests[door]
	if len(digests) == 0 {
		return false, nil
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(door))) //nolint:gosec // checkout-relative door path
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read door %s: %w", door, err)
	}
	return digests[readrecord.WholeDigest(string(body))], nil
}

// sectionRead reports whether reads holds a complete read of section on page
// with its current digest.
func (r antigravityReads) sectionRead(page string, section readrecord.Section) bool {
	recorded := r.sections[page]
	if len(recorded) == 0 {
		return false
	}
	return recorded[section.Digest()+"\t"+section.Heading]
}
