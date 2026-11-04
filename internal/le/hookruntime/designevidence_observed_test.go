// Design: docs/contributing/spec-workflow.md -- source evidence binds observed tool content
// VALIDATES: registered read hooks and the design gate preserve selected subjects,
// refuse unknown paths, confine canonical identities, and compare observed ranges.
// PREVENTS: replacement prose or a post-read filesystem mutation manufacturing freshness.
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const evidenceProbeSession = "observed-evidence"
const evidenceProbeSpec = "plan/post-release/engineering/spec-observed-evidence.md"
const evidenceProbeA = "internal/le/producer_a.go"
const evidenceProbeB = "internal/le/producer_b.go"

func observedReadPayload(path, content string, start, lines, total int) map[string]any {
	return map[string]any{
		"session_id": evidenceProbeSession, "tool_name": "Read", "tool_use_id": "read-observed-source",
		"tool_input": map[string]any{"file_path": path, "offset": start, "limit": lines},
		"tool_response": map[string]any{"type": "text", "file": map[string]any{
			"filePath": path, "content": content, "startLine": start, "numLines": lines, "totalLines": total,
		}},
	}
}

func recordObservedRead(t *testing.T, root, path, content string, start, lines, total int) {
	t.Helper()
	code, _, message := runHook(t, root, "mark-source-read", observedReadPayload(path, content, start, lines, total))
	if code != 0 {
		t.Fatalf("record observed read = %d: %s", code, message)
	}
}

func observedDesignGate(t *testing.T, root, proposed string, want int) string {
	t.Helper()
	code, message, found := Probe("writeDesignEvidence", Payload{
		SessionID: evidenceProbeSession, ToolName: toolWrite,
		ToolInput: map[string]any{"file_path": evidenceProbeSpec, "content": proposed},
	}, root)
	if !found || code != want {
		t.Fatalf("design gate = %d, found %v, want %d: %s", code, found, want, message)
	}
	return message
}

func TestDesignEvidenceRetainsSelectedSubjects(t *testing.T) {
	for _, test := range []struct {
		name     string
		current  string
		proposed string
	}{
		{"replace-current-producer", "## Current Behavior\n`" + evidenceProbeA + "`\n", "## Files to Modify\n`" + evidenceProbeB + "`\n"},
		{"remove-current-producer", "## Files to Modify\n`" + evidenceProbeA + "`\n", "# Revised design\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeHookFixture(t, root, evidenceProbeA, "package le\n")
			writeHookFixture(t, root, evidenceProbeB, "package other\n")
			writeHookFixture(t, root, evidenceProbeSpec, test.current)
			recordObservedRead(t, root, filepath.Join(root, evidenceProbeB), "package other\n", 1, 2, 2)
			message := observedDesignGate(t, root, test.proposed, 2)
			if !strings.Contains(message, evidenceProbeA) {
				t.Fatalf("refusal does not name the selected producer: %s", message)
			}
			recordObservedRead(t, root, filepath.Join(root, evidenceProbeA), "package le\n", 1, 2, 2)
			observedDesignGate(t, root, test.proposed, 0)
		})
	}
}

func TestDesignEvidenceRefusesUnknownSubjects(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify"} {
		for _, failure := range []string{"missing", "permission-denied", "not-a-directory"} {
			t.Run(heading+"/"+failure, func(t *testing.T) {
				root := t.TempDir()
				writeHookFixture(t, root, evidenceProbeB, "package other\n")
				recordObservedRead(t, root, filepath.Join(root, evidenceProbeB), "package other\n", 1, 2, 2)
				path := evidenceProbeA
				switch failure {
				case "permission-denied":
					writeHookFixture(t, root, path, "package le\n")
					dir := filepath.Dir(filepath.Join(root, path))
					if err := os.Chmod(dir, 0); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.Chmod(dir, 0o750); err != nil {
							t.Error(err)
						}
					})
					if _, err := os.Stat(filepath.Join(root, path)); !os.IsPermission(err) {
						t.Skip("the current user can traverse a mode-000 directory; permission-denied fixture is unavailable")
					}
				case "not-a-directory":
					writeHookFixture(t, root, "internal/blocked", "not a directory\n")
					path = "internal/blocked/producer.go"
				}
				message := observedDesignGate(t, root, "## "+heading+"\n`"+path+"`\n", 2)
				if !strings.Contains(message, path) {
					t.Fatalf("unknown producer is not named: %s", message)
				}
			})
		}
	}
}

func TestDesignEvidenceDistinguishesNewFiles(t *testing.T) {
	root := t.TempDir()
	writeHookFixture(t, root, evidenceProbeB, "package other\n")
	recordObservedRead(t, root, filepath.Join(root, evidenceProbeB), "package other\n", 1, 2, 2)
	proposal := "## Files to Create\n`" + evidenceProbeA + "`\n"
	observedDesignGate(t, root, proposal, 0)
	writeHookFixture(t, root, evidenceProbeA, "package le\n")
	observedDesignGate(t, root, proposal, 2)
	recordObservedRead(t, root, filepath.Join(root, evidenceProbeA), "package le\n", 1, 2, 2)
	observedDesignGate(t, root, proposal, 0)
}

func TestDesignEvidenceCanonicalCheckoutIdentity(t *testing.T) {
	t.Run("outside-symlink-read", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		writeHookFixture(t, outside, "producer.go", "package outside\n")
		if err := os.MkdirAll(filepath.Join(root, "internal"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "internal", "le")); err != nil {
			t.Fatal(err)
		}
		recordObservedRead(t, root, filepath.Join(root, "internal/le/producer.go"), "package outside\n", 1, 2, 2)
		observedDesignGate(t, root, "# A subjectless design\n", 2)
	})
	t.Run("outside-symlink-subject", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		writeHookFixture(t, root, evidenceProbeB, "package other\n")
		writeHookFixture(t, outside, "producer.go", "package outside\n")
		if err := os.Symlink(outside, filepath.Join(root, "internal", "escaped")); err != nil {
			t.Fatal(err)
		}
		recordObservedRead(t, root, filepath.Join(root, evidenceProbeB), "package other\n", 1, 2, 2)
		observedDesignGate(t, root, "## Files to Modify\n`internal/escaped/producer.go`\n", 2)
	})
	t.Run("normalized-subject", func(t *testing.T) {
		root := t.TempDir()
		writeHookFixture(t, root, evidenceProbeA, "package le\n")
		recordObservedRead(t, root, root+"/internal/le/../le/producer_a.go", "package le\n", 1, 2, 2)
		observedDesignGate(t, root, "## Files to Modify\n`internal/le/../le/producer_a.go`\n", 0)
	})
	t.Run("internal-symlink-alias", func(t *testing.T) {
		root := t.TempDir()
		writeHookFixture(t, root, evidenceProbeA, "package le\n")
		if err := os.Symlink(filepath.Join(root, "internal/le"), filepath.Join(root, "internal", "alias")); err != nil {
			t.Fatal(err)
		}
		recordObservedRead(t, root, filepath.Join(root, "internal/alias/producer_a.go"), "package le\n", 1, 2, 2)
		observedDesignGate(t, root, "## Files to Modify\n`"+evidenceProbeA+"`\n", 0)
	})
	t.Run("revalidation-does-not-follow-new-escape", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		writeHookFixture(t, root, evidenceProbeA, "package le\n")
		writeHookFixture(t, outside, "producer_a.go", "package le\n")
		recordObservedRead(t, root, filepath.Join(root, evidenceProbeA), "package le\n", 1, 2, 2)
		if err := os.Remove(filepath.Join(root, evidenceProbeA)); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(root, "internal/le")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "internal/le")); err != nil {
			t.Fatal(err)
		}
		observedDesignGate(t, root, "# A subjectless design\n", 2)
	})
}

func TestDesignEvidenceUsesObservedToolContent(t *testing.T) {
	for _, test := range []struct {
		name     string
		current  string
		observed string
		start    int
		lines    int
		total    int
		want     int
	}{
		{"unchanged-whole-read", "package le\n", "package le\n", 1, 2, 2, 0},
		{"unchanged-native-zero-offset", "package le\n", "package le\n", 0, 2, 2, 0},
		{"returned-A-checkout-B", "package le\nfunc B() {}\n", "package le\nfunc A() {}\n", 1, 3, 3, 2},
		{"unchanged-range", "package le\nfunc A() {}\n", "func A() {}", 2, 1, 3, 0},
		{"returned-range-A-checkout-B", "package le\nfunc B() {}\n", "func A() {}", 2, 1, 3, 2},
		{"unchanged-native-normalization", "\xef\xbb\xbfpackage le\r\n", "package le\n", 1, 2, 2, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeHookFixture(t, root, evidenceProbeA, test.current)
			recordObservedRead(t, root, filepath.Join(root, evidenceProbeA), test.observed, test.start, test.lines, test.total)
			observedDesignGate(t, root, "## Files to Modify\n`"+evidenceProbeA+"`\n", test.want)
		})
	}
	for _, test := range []string{"no-response", "LSP-location-only", "mismatched-response-path", "malformed-range"} {
		t.Run(test, func(t *testing.T) {
			root := t.TempDir()
			writeHookFixture(t, root, evidenceProbeA, "package le\n")
			payload := observedReadPayload(filepath.Join(root, evidenceProbeA), "package le\n", 1, 2, 2)
			kind := "mark-source-read"
			switch test {
			case "no-response":
				delete(payload, "tool_response")
			case "LSP-location-only":
				payload["tool_name"] = "LSP"
				payload["tool_input"] = map[string]any{"filePath": filepath.Join(root, evidenceProbeA), "operation": "hover", "line": 1, "character": 1}
				payload["tool_response"] = map[string]any{"filePath": filepath.Join(root, evidenceProbeA), "operation": "hover", "result": "package le"}
				kind = "mark-lsp-invoked"
			case "mismatched-response-path":
				payload["tool_response"].(map[string]any)["file"].(map[string]any)["filePath"] = filepath.Join(root, evidenceProbeB)
			case "malformed-range":
				payload["tool_response"].(map[string]any)["file"].(map[string]any)["numLines"] = 20
			}
			if code, _, message := runHook(t, root, kind, payload); code != 0 {
				t.Fatalf("record unusable event = %d: %s", code, message)
			}
			observedDesignGate(t, root, "## Files to Modify\n`"+evidenceProbeA+"`\n", 2)
		})
	}
}
