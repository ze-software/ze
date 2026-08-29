// Design: docs/architecture/core-design.md -- native spec lifecycle support
// Related: review.go -- review artifact model enforcement

// Package specsession owns spec claims, state paths, review artifacts, and the
// transcript facts those contracts use.
package specsession

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/le/lepath"
)

const transcriptTailBytes = 1_048_576

var safeSessionPattern = regexp.MustCompile(`\A[A-Za-z0-9._-]+\z`)

// transcriptDir answers Claude's per-project transcript directory for root.
func transcriptDir(root string) string {
	if project := os.Getenv("CLAUDE_PROJECT_DIR"); project != "" {
		root = project
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	slug := strings.NewReplacer("/", "-", ".", "-").Replace(absolute)
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects", slug)
}

// TranscriptPath answers this session's transcript, or an empty path when it
// cannot identify one without borrowing a neighboring session.
func TranscriptPath(root string) string {
	raw, present := os.LookupEnv("CLAUDE_CODE_SESSION_ID")
	dir := transcriptDir(root)
	if dir == "" {
		return ""
	}
	if sid := safeSessionID(raw); sid != "" {
		return existingTranscript(dir, sid)
	}
	if os.Getenv("CLAUDE_CODE_FORK_SUBAGENT") != "" {
		session, err := lepath.ResolveSession(root, false)
		if err != nil {
			return ""
		}
		return existingTranscript(dir, session.ID)
	}
	if present {
		return ""
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	latest := ""
	var latestTime int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		mtime := info.ModTime().UnixNano()
		if latest == "" {
			latest = path
			latestTime = mtime
			continue
		}
		if mtime > latestTime {
			latest = path
			latestTime = mtime
		}
	}
	return latest
}

// CurrentModel answers the model on the last main-thread assistant message in
// this session's transcript. An empty answer means that the model is unreadable.
func CurrentModel(root string) string {
	return RunningModel(TranscriptPath(root))
}

// RunningModel reads path and answers the model on its last main-thread
// assistant message. An empty path is an explicit unreadable answer and never
// triggers transcript discovery.
func RunningModel(path string) string {
	if path == "" {
		return ""
	}
	file, err := os.Open(path) //nolint:gosec // the caller or TranscriptPath selected the transcript
	if err != nil {
		return ""
	}
	defer file.Close() //nolint:errcheck // read-only
	info, err := file.Stat()
	if err != nil {
		return ""
	}
	start := int64(0)
	if info.Size() > transcriptTailBytes {
		start = info.Size() - transcriptTailBytes
	}
	body := make([]byte, int(info.Size()-start))
	if _, err := file.ReadAt(body, start); err != nil {
		return ""
	}
	if start > 0 {
		newline := bytes.IndexByte(body, '\n')
		if newline < 0 {
			return ""
		}
		body = body[newline+1:]
	}
	for end := len(body); end > 0; {
		lineStart := bytes.LastIndexByte(body[:end], '\n')
		line := body[lineStart+1 : end]
		end = lineStart
		if !bytes.Contains(line, []byte(`"model"`)) {
			continue
		}
		var record struct {
			Sidechain bool `json:"isSidechain"`
			Message   struct {
				Model string `json:"model"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &record) != nil {
			continue
		}
		if record.Sidechain {
			continue
		}
		if record.Message.Model == "" {
			continue
		}
		return record.Message.Model
	}
	return ""
}

// IsReviewTier reports whether model is in the review model family.
func IsReviewTier(model string) bool {
	if model == "" {
		return false
	}
	return strings.Contains(model, "opus-5")
}

func existingTranscript(dir, sid string) string {
	path := filepath.Join(dir, sid+".jsonl")
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if !info.Mode().IsRegular() {
		return ""
	}
	return path
}

func safeSessionID(value string) string {
	if value == "" {
		return ""
	}
	if value == "." {
		return ""
	}
	if value == ".." {
		return ""
	}
	if !safeSessionPattern.MatchString(value) {
		return ""
	}
	return value
}
