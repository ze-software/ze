// Design: docs/guide/configuration.md -- CLI transcript ownership and contents
// Related: main.go -- Run serves local commands before loading credentials

package client

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/core/env"
)

const (
	localTranscriptCommand = "show schema protocol | raw"
	localTranscriptAnswer  = `{"protocol":"Hub Architecture","version":"1.0"}`
)

// TestCLILocalOneShotRecordsTranscript calls the public CLI entry with no
// credentials or daemon. The real local-data handler must still print its
// answer and record that exact command and answer once in the configured home.
// VALIDATES: spec-web-cli-config-one-off-defects AC-16.
// PREVENTS: the pre-credential local return bypassing transcript setup.
func TestCLILocalOneShotRecordsTranscript(t *testing.T) {
	dataHome := configureOneShotTranscript(t, true)
	var code int
	var stdout string
	stderr := captureOutput(t, true, func() {
		stdout = captureOutput(t, false, func() {
			code = Run([]string{"--remote", "127.0.0.1:1", "-c", localTranscriptCommand})
		})
	})
	require.Equal(t, 0, code, "local execution must need neither credentials nor a daemon: %s", stderr)
	require.Empty(t, stderr)
	require.Equal(t, localTranscriptAnswer+"\n", stdout)
	assertOneShotTranscript(t, dataHome, localTranscriptCommand, localTranscriptAnswer)
}

// TestCLILocalOneShotLeavesTranscriptDisabled keeps recording opt-in while
// proving that disabling it preserves the same useful local answer.
func TestCLILocalOneShotLeavesTranscriptDisabled(t *testing.T) {
	dataHome := configureOneShotTranscript(t, false)
	var code int
	stdout := captureOutput(t, false, func() {
		code = Run([]string{"--remote", "127.0.0.1:1", "-c", localTranscriptCommand})
	})
	require.Equal(t, 0, code)
	require.Equal(t, localTranscriptAnswer+"\n", stdout)
	_, err := os.Stat(filepath.Join(dataHome, "ze", "transcripts"))
	require.True(t, errors.Is(err, os.ErrNotExist), "disabled recording created a transcript directory: %v", err)
}

// TestCLIDaemonOneShotStillRecordsTranscript drives the existing stream seam
// through runOne. Recording the local answer must preserve the daemon route's
// exact rendered answer and its one-command transcript lifecycle.
func TestCLIDaemonOneShotStillRecordsTranscript(t *testing.T) {
	dataHome := configureOneShotTranscript(t, true)
	const input = "show bgp peer list | raw"
	const answer = `{"peers":[{"name":"transcript-control"}]}`
	dispatched := false
	var received string
	client := &cliClient{
		creds: sshclient.Credentials{Host: "127.0.0.1", Port: "1"},
		stream: func(_ sshclient.Credentials, command string, output io.Writer) (sshclient.Answer, error) {
			dispatched = true
			received = command
			_, err := io.WriteString(output, answer)
			return sshclient.Answer{}, err
		},
	}
	var code int
	stdout := captureOutput(t, false, func() {
		code = client.runOne(input, "")
	})
	require.Equal(t, 0, code)
	require.True(t, dispatched, "the control must reach the daemon transport")
	require.Equal(t, input, received)
	require.Equal(t, answer+"\n", stdout)
	assertOneShotTranscript(t, dataHome, input, answer)
}

// configureOneShotTranscript confines recording and credentials to this test.
// Empty stdin makes a misplaced credential prompt fail instead of waiting for
// input; local commands must finish before that credential path is reached.
func configureOneShotTranscript(t *testing.T, enabled bool) string {
	t.Helper()
	for _, setting := range []string{"ze.cli.transcript", "ze.config.dir", "ze.ssh.username", "ze.ssh.password"} {
		for _, key := range []string{setting, strings.ReplaceAll(setting, ".", "_"), strings.ToUpper(strings.ReplaceAll(setting, ".", "_"))} {
			t.Setenv(key, "")
		}
	}
	t.Setenv("ze.config.dir", t.TempDir())
	if enabled {
		t.Setenv("ze.cli.transcript", "true")
	} else {
		t.Setenv("ze.cli.transcript", "false")
	}
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	stdin, err := os.Open(os.DevNull)
	require.NoError(t, err)
	previous := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() {
		os.Stdin = previous
		require.NoError(t, stdin.Close())
	})
	return dataHome
}

// assertOneShotTranscript checks the single file the public command created.
// The timestamp is variable; everything after it must match the exact command
// and answer bytes, with no extra recording or rerendering.
func assertOneShotTranscript(t *testing.T, dataHome, command, answer string) {
	t.Helper()
	directory := filepath.Join(dataHome, "ze", "transcripts")
	entries, err := os.ReadDir(directory)
	require.NoError(t, err, "the enabled public command must create its transcript")
	require.Len(t, entries, 1, "one CLI invocation must create one transcript file")
	content, err := os.ReadFile(filepath.Join(directory, entries[0].Name()))
	require.NoError(t, err)
	_, record, found := strings.Cut(string(content), "] > ")
	require.True(t, found, "transcript has no command record: %s", content)
	expected := command + "\n"
	if answer != "" {
		expected += answer + "\n"
	}
	expected += "\n"
	require.Equal(t, expected, record)
}
