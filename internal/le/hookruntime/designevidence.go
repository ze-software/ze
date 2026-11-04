// Design: docs/contributing/spec-workflow.md -- subject-bound current source evidence
// Related: lifecycle.go -- hookSourceRead records reads before writeDesignEvidence
package hookruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// sourceRead binds one returned Read range to its canonical checkout path and
// tool event. Its zero value and older unbound markers prove nothing.
type sourceRead struct {
	Path       string `json:"path"`
	Digest     string `json:"digest"`
	StartLine  int    `json:"start_line"`
	NumLines   int    `json:"num_lines"`
	TotalLines int    `json:"total_lines"`
	Tool       string `json:"tool"`
	ToolUseID  string `json:"tool_use_id"`
}

func sourceKind(path string) bool {
	if filepath.Base(path) == "Makefile" {
		return true
	}
	switch filepath.Ext(path) {
	case ".go", ".sh", ".py", ".yang", ".mk", ".rs", ".js", ".ts", ".c", ".h", ".templ", ".proto":
		return true
	default:
		return false
	}
}

func sourceReadMarker(ctx context, path string) string {
	id := resolvedSessionID(ctx)
	if id == "" {
		return ""
	}
	fingerprint, err := lepath.AgentFingerprint(id, ctx.payload.AgentID)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256([]byte(path))
	return filepath.Join(ctx.root, "tmp", "session", ".source-read-"+hex.EncodeToString(digest[:])+"-"+fingerprint)
}

// recordSourceRead hashes the native Read result, never a later filesystem
// read. LSP's formatted navigation result carries no source content identity.
func recordSourceRead(ctx context) int {
	if ctx.tool != "Read" || ctx.payload.ToolUseID == "" {
		return 0
	}
	path, err := checkoutRelative(ctx)
	if err != nil || !sourceKind(path) {
		return 0
	}
	var response struct {
		Type string `json:"type"`
		File struct {
			Path       string  `json:"filePath"`
			Content    *string `json:"content"`
			StartLine  *int    `json:"startLine"`
			NumLines   int     `json:"numLines"`
			TotalLines int     `json:"totalLines"`
		} `json:"file"`
	}
	if err := json.Unmarshal(ctx.payload.ToolResponse, &response); err != nil || response.Type != "text" || response.File.Content == nil || response.File.StartLine == nil {
		return 0
	}
	returned := ctx
	returned.path = response.File.Path
	returnedPath, err := checkoutRelative(returned)
	if response.File.Path == "" || err != nil || returnedPath != path {
		return 0
	}
	record := sourceRead{
		Path: path, StartLine: max(1, *response.File.StartLine), NumLines: response.File.NumLines,
		TotalLines: response.File.TotalLines, Tool: ctx.tool, ToolUseID: ctx.payload.ToolUseID,
	}
	if *response.File.StartLine < 0 || !record.validRange() || strings.Count(*response.File.Content, "\n")+1 != record.NumLines {
		return 0
	}
	digest := sha256.Sum256([]byte(*response.File.Content))
	record.Digest = hex.EncodeToString(digest[:])
	body, err := json.Marshal(record)
	if err != nil {
		return 0
	}
	return writeMarkerFile(sourceReadMarker(ctx, path), string(body))
}

func (r sourceRead) validRange() bool {
	return r.StartLine >= 1 && r.NumLines >= 1 && r.TotalLines >= r.StartLine && r.NumLines <= r.TotalLines-r.StartLine+1
}

// designSubjects retains selected and proposed producer obligations. Only a
// genuinely absent Files to Create path is new; every other lookup error refuses.
func designSubjects(ctx context, repository *os.Root, texts ...string) ([]string, error) {
	var subjects []string
	for _, text := range texts {
		for _, heading := range []string{"## Current Behavior", "## Files to Modify", "## Files to Create"} {
			for _, named := range specRepoPaths(ctx.root, markdownSection(text, heading)) {
				subject := ctx
				subject.path = named
				path, err := checkoutRelative(subject)
				if err != nil {
					return nil, fmt.Errorf("%s: resolve producer: %w", named, err)
				}
				if !sourceKind(path) || strings.HasSuffix(path, "_test.go") {
					continue
				}
				info, err := repository.Stat(filepath.FromSlash(path))
				if err != nil {
					if heading == "## Files to Create" && os.IsNotExist(err) {
						continue
					}
					return nil, fmt.Errorf("%s: inspect producer: %w", named, err)
				}
				if !info.Mode().IsRegular() {
					return nil, fmt.Errorf("%s: producer is not a regular file", named)
				}
				if !slices.Contains(subjects, path) {
					subjects = append(subjects, path)
				}
			}
		}
	}
	return subjects, nil
}

// observedSourceRange applies the native Read text normalization to current
// UTF-8 source, then selects exactly the returned lines, including a final empty
// line when present. Unreturned lines are never hashed as observed evidence.
func observedSourceRange(body []byte, record sourceRead) (string, bool) {
	if !record.validRange() || !utf8.Valid(body) {
		return "", false
	}
	text := strings.TrimPrefix(string(body), "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\r")
	if strings.Count(text, "\n")+1 != record.TotalLines {
		return "", false
	}
	for line := 1; line < record.StartLine; line++ {
		_, text, _ = strings.Cut(text, "\n")
	}
	end := 0
	for line := range record.NumLines {
		if next := strings.IndexByte(text[end:], '\n'); next >= 0 {
			end += next
			if line+1 == record.NumLines {
				return text[:end], true
			}
			end++
		} else {
			return text, line+1 == record.NumLines
		}
	}
	return "", false
}

func currentSourceReads(ctx context, repository *os.Root) map[string]bool {
	reads := make(map[string]bool)
	entries, err := filepath.Glob(filepath.Join(ctx.root, "tmp", "session", ".source-read-*"))
	if err != nil {
		return reads
	}
	for _, marker := range entries {
		body, err := os.ReadFile(marker)
		if err != nil {
			continue
		}
		var record sourceRead
		if err := json.Unmarshal(body, &record); err != nil || record.Tool != "Read" || record.ToolUseID == "" || !record.validRange() {
			continue
		}
		if marker != sourceReadMarker(ctx, record.Path) {
			continue
		}
		source := ctx
		source.path = record.Path
		path, err := checkoutRelative(source)
		if err != nil || path != record.Path || !sourceKind(path) {
			continue
		}
		info, err := repository.Stat(filepath.FromSlash(path))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		body, err = repository.ReadFile(filepath.FromSlash(path))
		if err != nil {
			continue
		}
		observed, ok := observedSourceRange(body, record)
		if !ok {
			continue
		}
		digest := sha256.Sum256([]byte(observed))
		if record.Digest == hex.EncodeToString(digest[:]) {
			reads[path] = true
		}
	}
	return reads
}

func designEvidenceProblem(ctx context) *verdict {
	current, err := os.ReadFile(absolutePath(ctx))
	if err != nil && !os.IsNotExist(err) {
		return &verdict{2, "❌ BLOCKED: cannot read the design being edited: " + err.Error() + "\n  Make it readable, then retry."}
	}
	proposed, applied := proposedSpec(ctx, string(current))
	if !applied {
		return nil
	}
	repository, err := os.OpenRoot(ctx.root)
	if err != nil {
		return &verdict{2, "❌ BLOCKED: cannot open the design's source checkout: " + err.Error() +
			"\n  Make this checkout readable, then retry.\n  ai/rules/evidence.md"}
	}
	defer repository.Close() //nolint:errcheck // read-only checkout capability
	subjects, err := designSubjects(ctx, repository, string(current), proposed)
	if err != nil {
		return &verdict{2, "❌ BLOCKED: cannot establish design source subjects: " + err.Error() +
			"\n  Make each existing producer readable inside this checkout, then read it in this context and retry. Only genuinely new Files to Create paths may be absent.\n  ai/rules/evidence.md"}
	}
	reads := currentSourceReads(ctx, repository)
	if len(subjects) == 0 && len(reads) != 0 {
		return nil
	}
	if len(subjects) != 0 {
		missing := make([]string, 0, len(subjects))
		for _, path := range subjects {
			if !reads[path] {
				missing = append(missing, path)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		return &verdict{2, "❌ BLOCKED: read the producing source before writing a design/spec\n  Missing current subject evidence: " + strings.Join(missing, ", ") +
			"\n  Read the producing ranges in this context, then retry. The hook needs the Read tool's returned source content and range; a generic LSP result or another subject does not establish freshness.\n  ai/rules/evidence.md"}
	}
	return &verdict{2, "❌ BLOCKED: read the producing source before writing a design/spec\n  Read the implementation in this context, then retry. Unreadable source or a tool response without usable observed source content has no current evidence.\n  ai/rules/evidence.md"}
}
