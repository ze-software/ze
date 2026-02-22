// Design: docs/architecture/testing/ci-format.md — output capture, saving, and parsing
// Related: runner.go — Runner struct and lifecycle
// Related: runner_exec.go — test execution that produces output

package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// testOutput holds captured output for saving.
type testOutput struct {
	peerStdout   string
	peerStderr   string
	clientStdout string
	clientStderr string
}

// writeExpectFile writes expected messages to a temp file.
func (r *Runner) writeExpectFile(rec *Record) (string, error) {
	f, err := os.CreateTemp("", "ze-functional-*.expect")
	if err != nil {
		return "", err
	}

	// Write options
	for _, opt := range rec.Options {
		if _, err := fmt.Fprintln(f, opt); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return "", err
		}
	}

	// Write expects
	for _, exp := range rec.Expects {
		if _, err := fmt.Fprintln(f, exp); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return "", err
		}
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}

	return f.Name(), nil
}

// extractReceivedMessages parses peer output for received raw messages.
func extractReceivedMessages(output string) []string {
	var messages []string

	// Look for "msg  recv" lines followed by hex
	// Format: "msg  recv    FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0029:02:..."
	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		if strings.Contains(line, "msg  recv") || strings.Contains(line, "msg recv") {
			// Extract hex after the prefix
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				hex := parts[len(parts)-1]
				// Clean up hex (remove colons)
				hex = strings.ReplaceAll(hex, ":", "")
				if len(hex) >= 38 { // Minimum BGP message
					messages = append(messages, hex)
				}
			}
		}
	}

	return messages
}

// extractMismatchIndices tries to find which message mismatched.
func extractMismatchIndices(output string) (expected, received int) {
	// Default to first message
	expected = 1
	received = 0

	// Try to parse "message N mismatch" patterns
	re := regexp.MustCompile(`message\s+(\d+)`)
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		if n, err := fmt.Sscanf(matches[1], "%d", &expected); err == nil && n > 0 {
			received = expected - 1
		}
	}

	return
}

// sanitizeFilename removes/replaces characters unsafe for filenames.
func sanitizeFilename(name string) string {
	// Replace path separators and other unsafe chars with underscore
	result := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '_'
		default:
			return r
		}
	}, name)
	// Truncate if too long
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// saveTestOutput saves test outputs to files when SaveDir is set.
func (r *Runner) saveTestOutput(rec *Record, out *testOutput, saveDir string) error {
	if saveDir == "" {
		return nil
	}

	// Create test directory (nick-name for easy identification)
	dirName := fmt.Sprintf("%s-%s", rec.Nick, sanitizeFilename(rec.Name))
	testDir := filepath.Join(saveDir, dirName)
	if err := os.MkdirAll(testDir, 0o700); err != nil {
		return fmt.Errorf("create save dir: %w", err)
	}

	// Write output files
	files := map[string]string{
		"peer-stdout.log":   out.peerStdout,
		"peer-stderr.log":   out.peerStderr,
		"client-stdout.log": out.clientStdout,
		"client-stderr.log": out.clientStderr,
	}

	for name, content := range files {
		path := filepath.Join(testDir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	// Write expected.txt (from .ci file)
	var expected strings.Builder
	for _, exp := range rec.Expects {
		expected.WriteString(exp)
		expected.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(testDir, "expected.txt"), []byte(expected.String()), 0o600); err != nil {
		return fmt.Errorf("write expected.txt: %w", err)
	}

	// Write received.txt (from peer output)
	var received strings.Builder
	for _, raw := range rec.ReceivedRaw {
		received.WriteString(raw)
		received.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(testDir, "received.txt"), []byte(received.String()), 0o600); err != nil {
		return fmt.Errorf("write received.txt: %w", err)
	}

	return nil
}
