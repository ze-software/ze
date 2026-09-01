// Design: docs/architecture/testing/verify-freshness-scope.md -- compact verification failure index
// Related: status.go -- the certificate written beside the failure index
package verifyengine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/runlog"
)

const (
	lintStage        = "lint"
	lintLineLimit    = 100
	failureLineLimit = 80
)

// FailureSummary is one stage block appended to the verification failure index.
type FailureSummary struct {
	Stage    string   `json:"stage"`
	FullLog  string   `json:"full-log"`
	KeyLines []string `json:"key-lines"`
	Missing  bool     `json:"missing"`
}

// Text renders the established failure-index block.
func (s FailureSummary) Text() string {
	var text strings.Builder
	fmt.Fprintf(&text, "\n### Stage: %s\n", s.Stage)
	fmt.Fprintf(&text, "Full log: %s\n\n", s.FullLog)
	text.WriteString("Key lines:\n")
	for _, line := range s.KeyLines {
		text.WriteString(line)
		text.WriteByte('\n')
	}
	return text.String()
}

// Summarize reads one stage log and extracts the lines its reader needs first.
//
// Which lines those are belongs to internal/le/runlog, so this index and a
// quiet job (internal/le/job) name the same lines.
func Summarize(stage, logPath string) (FailureSummary, error) {
	summary := FailureSummary{Stage: stage, FullLog: logPath}
	file, err := os.Open(logPath) //nolint:gosec // the path is a verification artifact under the checkout root
	if os.IsNotExist(err) {
		summary.Missing = true
		summary.KeyLines = []string{fmt.Sprintf("(stage log missing: %s)", logPath)}
		return summary, nil
	}
	if err != nil {
		return FailureSummary{}, fmt.Errorf("read stage log %q: %w", logPath, err)
	}
	defer file.Close() //nolint:errcheck // the log is only read

	if stage == lintStage {
		head, err := runlog.Head(file, lintLineLimit)
		if err != nil {
			return FailureSummary{}, fmt.Errorf("read stage log %q: %w", logPath, err)
		}
		summary.KeyLines = head
		return summary, nil
	}

	key, err := runlog.Key(file, failureLineLimit)
	if err != nil {
		return FailureSummary{}, fmt.Errorf("read stage log %q: %w", logPath, err)
	}
	summary.KeyLines = key
	if len(summary.KeyLines) == 0 {
		summary.KeyLines = []string{"(no obvious FAIL lines found; see full log)"}
	}
	return summary, nil
}

// AppendSummary appends one complete block with one write. Concurrent stage
// failures therefore cannot interleave the lines of their blocks.
func AppendSummary(failuresPath, stage, logPath string) (FailureSummary, error) {
	summary, err := Summarize(stage, logPath)
	if err != nil {
		return FailureSummary{}, err
	}
	if err := os.MkdirAll(filepath.Dir(failuresPath), 0o750); err != nil {
		return FailureSummary{}, fmt.Errorf("create failure index directory: %w", err)
	}
	file, err := os.OpenFile(failuresPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600) //nolint:gosec // the path is a verification artifact under the checkout root
	if err != nil {
		return FailureSummary{}, fmt.Errorf("open failure index %q: %w", failuresPath, err)
	}
	block := summary.Text()
	written, writeErr := file.WriteString(block)
	closeErr := file.Close()
	if writeErr != nil {
		return FailureSummary{}, fmt.Errorf("append failure index %q: %w", failuresPath, writeErr)
	}
	if written != len(block) {
		return FailureSummary{}, fmt.Errorf("append failure index %q: short write", failuresPath)
	}
	if closeErr != nil {
		return FailureSummary{}, fmt.Errorf("close failure index %q: %w", failuresPath, closeErr)
	}
	return summary, nil
}
