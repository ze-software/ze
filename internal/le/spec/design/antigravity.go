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
	"slices"
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
	partials := map[agyPartialKey]map[int]agyPartialChunk{}
	lineRuns := map[agyLineFileKey]map[int]string{}
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
				collectAntigravityChunks(rel, result.Content, partials, lineRuns)
			}
		}
	}
	recordStitchedPartials(&reads, partials, lineRuns)
	recordStitchedLineRuns(&reads, lineRuns)
	return reads, nil
}

// agyPartialKey identifies one byte-windowed Partial read range of one file so
// ByteOffset continuation chunks can be stitched back into the full range.
type agyPartialKey struct {
	docPath   string
	start     int
	end       int
	total     int
	byteTotal int
}

// agyPartialChunk holds one raw byte slice returned by a Partial read.
type agyPartialChunk struct {
	byteStart int
	byteEnd   int
	rawBody   string
}

// agyLineFileKey identifies one file and its line count for stitching
// contiguous complete line-range reads.
type agyLineFileKey struct {
	docPath string
	total   int
}

// recordAntigravityRead parses one read_file result body and records its
// whole-file digest (when complete) and complete section digests on reads.
func recordAntigravityRead(reads *antigravityReads, docPath, body string) {
	text, whole, reachesEnd, ok := parseAntigravityRead(body)
	if !ok {
		return
	}
	recordParsedRead(reads, docPath, text, whole, reachesEnd)
}

// recordParsedRead adds whole-file and complete-section digests from one
// verified text span onto reads.
func recordParsedRead(reads *antigravityReads, docPath, text string, whole, reachesEnd bool) {
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

// collectAntigravityChunks saves Partial read byte-offset chunks and complete
// Showing lines ranges for post-scan stitching across multi-chunk reads.
func collectAntigravityChunks(docPath, content string, partials map[agyPartialKey]map[int]agyPartialChunk, lineRuns map[agyLineFileKey]map[int]string) {
	header, rawBody, ok := splitAntigravityHeader(content)
	if !ok {
		return
	}
	if match := agyPartialReadPattern.FindStringSubmatch(header); match != nil {
		key, chunk, valid := parsePartialChunkMeta(docPath, match, rawBody)
		if !valid {
			return
		}
		if partials[key] == nil {
			partials[key] = map[int]agyPartialChunk{}
		}
		partials[key][chunk.byteStart] = chunk
		return
	}
	if match := agyShowingLinesPattern.FindStringSubmatch(header); match != nil {
		start, end, total, valid := parseLineBounds(match)
		if !valid {
			return
		}
		extracted, _, _, valid := parseNumberedLines(start, end, total, strings.Split(rawBody, "\n"))
		if !valid {
			return
		}
		storeLineRun(lineRuns, docPath, total, start, extracted)
	}
}

// recordStitchedPartials stitches contiguous Partial read byte-offset chains
// covering [0, byteTotal) back into complete numbered line ranges.
func recordStitchedPartials(reads *antigravityReads, partials map[agyPartialKey]map[int]agyPartialChunk, lineRuns map[agyLineFileKey]map[int]string) {
	for key, byOffset := range partials {
		var stitched strings.Builder
		pos := 0
		for pos < key.byteTotal {
			chunk, found := byOffset[pos]
			if !found {
				break
			}
			if chunk.byteEnd <= pos {
				break
			}
			stitched.WriteString(chunk.rawBody)
			pos = chunk.byteEnd
		}
		if pos != key.byteTotal {
			continue
		}
		extracted, whole, reachesEnd, valid := parseNumberedLines(key.start, key.end, key.total, strings.Split(stitched.String(), "\n"))
		if !valid {
			continue
		}
		recordParsedRead(reads, key.docPath, strings.Join(extracted, "\n"), whole, reachesEnd)
		storeLineRun(lineRuns, key.docPath, key.total, key.start, extracted)
	}
}

// recordStitchedLineRuns merges contiguous verified line intervals for the same
// file and line count so sections exceeding read_file's byte cap can be read
// across adjacent line-range calls.
func recordStitchedLineRuns(reads *antigravityReads, lineRuns map[agyLineFileKey]map[int]string) {
	for key, lines := range lineRuns {
		if len(lines) == 0 {
			continue
		}
		nums := make([]int, 0, len(lines))
		for num := range lines {
			nums = append(nums, num)
		}
		slices.Sort(nums)
		runStart := nums[0]
		runLines := []string{lines[runStart]}
		prev := runStart
		flush := func(start, end int, textLines []string) {
			recordParsedRead(reads, key.docPath, strings.Join(textLines, "\n"), start == 1 && end == key.total, end == key.total)
		}
		for _, num := range nums[1:] {
			if num == prev+1 {
				runLines = append(runLines, lines[num])
				prev = num
				continue
			}
			flush(runStart, prev, runLines)
			runStart = num
			prev = num
			runLines = []string{lines[num]}
		}
		flush(runStart, prev, runLines)
	}
}

// storeLineRun records verified line texts keyed by 1-based line number.
func storeLineRun(lineRuns map[agyLineFileKey]map[int]string, docPath string, total, start int, extracted []string) {
	key := agyLineFileKey{docPath: docPath, total: total}
	if lineRuns[key] == nil {
		lineRuns[key] = map[int]string{}
	}
	for offset, line := range extracted {
		lineRuns[key][start+offset] = line
	}
}

// splitAntigravityHeader skips leading timestamp lines and separates the
// read_file header line from the normalized body that follows it.
func splitAntigravityHeader(content string) (header, rawBody string, ok bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	rest := normalized
	for {
		line, after, hasNL := strings.Cut(rest, "\n")
		if isTimestampLine(line) {
			if !hasNL {
				return "", "", false
			}
			rest = after
			continue
		}
		if !hasNL {
			return "", "", false
		}
		return line, after, true
	}
}

// parseLineBounds extracts start, end and total from a Showing lines or
// Partial read regexp match.
func parseLineBounds(match []string) (start, end, total int, ok bool) {
	start, valid := parseIntAtLeast(match[1], 1)
	if !valid {
		return 0, 0, 0, false
	}
	end, valid = parseIntAtLeast(match[2], start)
	if !valid {
		return 0, 0, 0, false
	}
	total, valid = parseIntAtLeast(match[3], end)
	if !valid {
		return 0, 0, 0, false
	}
	return start, end, total, true
}

// parsePartialChunkMeta decodes one Partial read header and body into its
// stitching key and byte-window chunk.
func parsePartialChunkMeta(docPath string, match []string, rawBody string) (agyPartialKey, agyPartialChunk, bool) {
	start, end, total, valid := parseLineBounds(match)
	if !valid {
		return agyPartialKey{}, agyPartialChunk{}, false
	}
	byteStart, valid := parseIntAtLeast(match[4], 0)
	if !valid {
		return agyPartialKey{}, agyPartialChunk{}, false
	}
	byteEnd, valid := parseIntAtLeast(match[5], byteStart)
	if !valid {
		return agyPartialKey{}, agyPartialChunk{}, false
	}
	byteTotal, valid := parseIntAtLeast(match[6], byteEnd)
	if !valid {
		return agyPartialKey{}, agyPartialChunk{}, false
	}
	if byteEnd <= byteStart {
		return agyPartialKey{}, agyPartialChunk{}, false
	}
	if byteTotal == 0 {
		return agyPartialKey{}, agyPartialChunk{}, false
	}
	return agyPartialKey{
		docPath:   docPath,
		start:     start,
		end:       end,
		total:     total,
		byteTotal: byteTotal,
	}, agyPartialChunk{
		byteStart: byteStart,
		byteEnd:   byteEnd,
		rawBody:   rawBody,
	}, true
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
	header, rawBody, valid := splitAntigravityHeader(content)
	if !valid {
		return "", false, false, false
	}
	body := strings.Split(rawBody, "\n")
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
	start, end, total, valid := parseLineBounds(match)
	if !valid {
		return "", false, false, false
	}
	extracted, whole, reachesEnd, valid := parseNumberedLines(start, end, total, body)
	if !valid {
		return "", false, false, false
	}
	return strings.Join(extracted, "\n"), whole, reachesEnd, true
}

// parseNumberedLines validates that body starts with exact "<n>: " lines for
// start..end and returns the stripped line texts.
func parseNumberedLines(start, end, total int, body []string) (extracted []string, whole, reachesEnd, ok bool) {
	count := end - start + 1
	if len(body) < count {
		return nil, false, false, false
	}
	extracted = make([]string, 0, count)
	for offset := range count {
		prefix := strconv.Itoa(start+offset) + ": "
		value, found := strings.CutPrefix(body[offset], prefix)
		if !found {
			return nil, false, false, false
		}
		extracted = append(extracted, value)
	}
	return extracted, start == 1 && end == total, end == total, true
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
