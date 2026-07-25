package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

func TestTranscriptWriterRecordsCommands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	tw := NewTranscriptWriter(f, "admin", "10.0.0.1:2222")
	tw.Record("show version", `{"version":"1.0"}`)
	tw.Record("peer list", `{"peers":{}}`)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "] > show version") {
		t.Error("transcript missing 'show version' command")
	}
	if !strings.Contains(content, `{"version":"1.0"}`) {
		t.Error("transcript missing show version output")
	}
	if !strings.Contains(content, "] > peer list") {
		t.Error("transcript missing 'peer list' command")
	}
	if !strings.Contains(content, `{"peers":{}}`) {
		t.Error("transcript missing peer list output")
	}
}

func TestTranscriptWriterHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	tw := NewTranscriptWriter(f, "operator", "router.example.com:2222")
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# Ze CLI Transcript") {
		t.Error("header missing transcript title")
	}
	if !strings.Contains(content, "# User: operator") {
		t.Error("header missing username")
	}
	if !strings.Contains(content, "# Host: router.example.com:2222") {
		t.Error("header missing remote host")
	}
	if !strings.Contains(content, "# Started: ") {
		t.Error("header missing timestamp")
	}
}

func TestTranscriptWriterBestEffort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	tw := NewTranscriptWriter(f, "admin", "10.0.0.1:2222")
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// Recording after file close should not panic or return error.
	tw.Record("show version", "output")
}

func TestTranscriptWriterDisabled(t *testing.T) {
	var tw *TranscriptWriter
	// Nil writer must be a no-op.
	tw.Record("show version", "output")
	if err := tw.Close(); err != nil {
		t.Errorf("nil writer Close returned error: %v", err)
	}
}

func TestTranscriptWriterNilFile(t *testing.T) {
	tw := NewTranscriptWriter(nil, "admin", "10.0.0.1:2222")
	if tw != nil {
		t.Error("expected nil writer for nil file")
	}
}

func TestTranscriptWrapsExecutor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	tw := NewTranscriptWriter(f, "admin", "10.0.0.1:2222")

	called := false
	base := func(input string) (string, error) {
		called = true
		return "response-for-" + input, nil
	}

	wrapped := WrapExecutorWithTranscript(base, tw)
	output, err := wrapped("show version")
	if err != nil {
		t.Fatalf("wrapped executor returned error: %v", err)
	}
	if output != "response-for-show version" {
		t.Errorf("wrapped executor returned %q, want %q", output, "response-for-show version")
	}
	if !called {
		t.Error("base executor was not called")
	}

	if closeErr := tw.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	content := string(data)
	if !strings.Contains(content, "] > show version") {
		t.Error("transcript missing command from wrapped executor")
	}
	if !strings.Contains(content, "response-for-show version") {
		t.Error("transcript missing output from wrapped executor")
	}
}

func TestTranscriptWrapsExecutorNilWriter(t *testing.T) {
	base := func(input string) (string, error) {
		return "ok", nil
	}
	wrapped := WrapExecutorWithTranscript(base, nil)
	output, err := wrapped("test")
	if err != nil {
		t.Fatal(err)
	}
	if output != "ok" {
		t.Errorf("got %q, want %q", output, "ok")
	}
}

func TestTranscriptEnabled(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"yes", true},
		{"enabled", true},
		{"false", false},
		{"0", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Setenv("ZE_CLI_TRANSCRIPT", tt.value)
		env.ResetCache()
		if got := TranscriptEnabled(); got != tt.want {
			t.Errorf("TranscriptEnabled() with %q = %v, want %v", tt.value, got, tt.want)
		}
	}
}
