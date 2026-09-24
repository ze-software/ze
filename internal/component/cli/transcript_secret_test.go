// Design: docs/architecture/ssh/fixit-bcrypt-hash-credential.md -- credential-token redaction for logs
// Related: transcript.go -- Record, the one choke point every transcript wiring shares

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/tacacs/yang"
)

// The transcript is a FILE on the operator's disk, so a credential typed at
// the prompt outlives the session that typed it. These tests drive the
// executor wrapper the four wirings install, not Record on its own, and read
// the bytes that reached the file.
//
// Each assertion checks a distinctive TAIL of the value as well as the whole
// of it. A writer that truncates or reformats can drop the whole value and
// still publish a piece of it (plan/journal/secret-echoed-to-the-client.md,
// 2026-08-15).
const (
	transcriptFakeCredential = "DUMMY-VALUE-NOT-A-CREDENTIAL-4471bc"
	transcriptFakeTail       = "4471bc"
)

// newTranscriptUnderTest opens a real file and returns the writer over it plus
// a reader for what was written, which is what the operator would later read.
func newTranscriptUnderTest(t *testing.T) (*TranscriptWriter, func() string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.log")
	f, err := os.Create(path) //nolint:gosec // the path is the test's own temp directory
	if err != nil {
		t.Fatalf("create transcript: %v", err)
	}
	tw := NewTranscriptWriter(f, "operator", "local")
	if tw == nil {
		t.Fatal("NewTranscriptWriter returned nil for a real file")
	}
	return tw, func() string {
		if closeErr := tw.Close(); closeErr != nil {
			t.Fatalf("close transcript: %v", closeErr)
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // the path is the test's own temp directory
		if readErr != nil {
			t.Fatalf("read transcript: %v", readErr)
		}
		return string(body)
	}
}

// TestTranscriptNeverRecordsACredential drives the wrapped executor with a set
// command that carries a plaintext password and requires that neither the
// value nor its tail reaches the transcript file.
func TestTranscriptNeverRecordsACredential(t *testing.T) {
	tw, contents := newTranscriptUnderTest(t)

	executor := WrapExecutorWithTranscript(func(string) (CommandOutput, error) {
		return CommandOutput{Text: "ok"}, nil
	}, tw)
	if executor == nil {
		t.Fatal("WrapExecutorWithTranscript returned nil for a real writer")
	}

	command := "set system authentication user admin plaintext-password " + transcriptFakeCredential
	if _, err := executor(command); err != nil {
		t.Fatalf("executor: %v", err)
	}

	body := contents()
	if strings.Contains(body, transcriptFakeCredential) {
		t.Error("the transcript file holds the whole credential")
	}
	if strings.Contains(body, transcriptFakeTail) {
		t.Error("the transcript file holds the tail of the credential")
	}
	if !strings.Contains(body, "plaintext-password <redacted>") {
		t.Errorf("the transcript does not name the leaf that was set, got:\n%s", body)
	}
}

// TestTranscriptStillRecordsACommandWithNoCredential is the opposite polarity.
// A writer that redacted every line would satisfy the test above and destroy
// the transcript's reason to exist.
func TestTranscriptStillRecordsACommandWithNoCredential(t *testing.T) {
	tw, contents := newTranscriptUnderTest(t)

	executor := WrapExecutorWithTranscript(func(string) (CommandOutput, error) {
		return CommandOutput{Text: "peer 1 established"}, nil
	}, tw)

	if _, err := executor("show bgp summary | json"); err != nil {
		t.Fatalf("executor: %v", err)
	}

	body := contents()
	if !strings.Contains(body, "show bgp summary | json") {
		t.Errorf("the transcript lost the command, got:\n%s", body)
	}
	if !strings.Contains(body, "peer 1 established") {
		t.Errorf("the transcript lost the answer, got:\n%s", body)
	}
}

// A quoted TACACS+ key contains several words but remains one secret. The
// transcript uses the schema, while the executor receives the original line.
func TestTranscriptRedactsQuotedTacacsKey(t *testing.T) {
	writer, contents := newTranscriptUnderTest(t)
	var executed string
	executor := WrapExecutorWithTranscript(func(command string) (CommandOutput, error) {
		executed = command
		return CommandOutput{Text: "ok"}, nil
	}, writer)
	input := `set system authentication tacacs server 192.0.2.1 key "private first middle tail-value"`
	if _, err := executor(input); err != nil {
		t.Fatal(err)
	}
	if executed != input {
		t.Fatal("transcript redaction changed the executed command")
	}
	recorded := contents()
	for _, part := range []string{"private", "middle", "tail-value"} {
		if strings.Contains(recorded, part) {
			t.Fatalf("transcript leaked a shared secret word: %s", recorded)
		}
	}
	if !strings.Contains(recorded, "192.0.2.1 key <redacted>") {
		t.Fatalf("transcript lost the updated configuration path: %s", recorded)
	}
}
