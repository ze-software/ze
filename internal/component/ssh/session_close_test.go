// Design: docs/features/cli-commands.md -- daemon-owned interactive configuration.

package ssh

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestSSHPTYClosesModelAtSessionEnd drives review finding I5: the daemon owns
// a PTY session's model and its transcript, so the SSH server closes the
// model once the session's program returns. The transcript file is closed
// when a write to it fails with os.ErrClosed.
func TestSSHPTYClosesModelAtSessionEnd(t *testing.T) {
	transcript, err := os.Create(filepath.Join(t.TempDir(), "transcript.log"))
	require.NoError(t, err)
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	server.SetSessionModelFactory(
		func(username, remoteAddr string, _ plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
			model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
			model.SetTranscript(cli.NewTranscriptWriter(transcript, username, remoteAddr))
			return model, nil
		},
	)

	client, err := gossh.Dial("tcp", server.Address(), &gossh.ClientConfig{
		User:            "operator",
		Auth:            []gossh.AuthMethod{gossh.Password("read-pass")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test server key is generated per run.
		Timeout:         5 * time.Second,
	})
	require.NoError(t, err)
	session, err := client.NewSession()
	require.NoError(t, err)
	var screen synchronizedBuffer
	session.Stdout = &screen
	session.Stderr = &screen
	require.NoError(t, session.RequestPty("xterm", 24, 80, gossh.TerminalModes{gossh.ECHO: 0}))
	require.NoError(t, session.Shell())
	require.Eventually(t, func() bool {
		return strings.Contains(screen.String(), "welcome to ze!")
	}, 5*time.Second, 10*time.Millisecond, "PTY model did not render its initial view")
	_, err = transcript.WriteString("open while the session runs\n")
	require.NoError(t, err, "the transcript is open for the session's lifetime")

	require.NoError(t, session.Close())
	require.NoError(t, client.Close())
	require.Eventually(t, func() bool {
		_, writeErr := transcript.WriteString("x")
		return errors.Is(writeErr, os.ErrClosed)
	}, 5*time.Second, 10*time.Millisecond, "the session's end did not close the model's transcript")
}

// TestSSHNoPTYClosesModelAtSessionEnd covers review round-2 I3: a session
// without a PTY (`ssh -T`) is refused by the bubbletea middleware before it
// hands the session on, so the close middleware must sit OUTSIDE it. The
// model's transcript is still closed when the session ends.
func TestSSHNoPTYClosesModelAtSessionEnd(t *testing.T) {
	transcript, err := os.Create(filepath.Join(t.TempDir(), "transcript.log"))
	require.NoError(t, err)
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	server.SetSessionModelFactory(
		func(username, remoteAddr string, _ plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
			model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
			model.SetTranscript(cli.NewTranscriptWriter(transcript, username, remoteAddr))
			return model, nil
		},
	)

	client, err := gossh.Dial("tcp", server.Address(), &gossh.ClientConfig{
		User:            "operator",
		Auth:            []gossh.AuthMethod{gossh.Password("read-pass")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test server key is generated per run.
		Timeout:         5 * time.Second,
	})
	require.NoError(t, err)
	session, err := client.NewSession()
	require.NoError(t, err)
	var screen synchronizedBuffer
	session.Stdout = &screen
	session.Stderr = &screen
	require.NoError(t, session.Shell())
	// The server ends the session itself: no PTY, no program.
	_ = session.Wait()
	require.Contains(t, screen.String(), "no active terminal")
	require.Eventually(t, func() bool {
		_, writeErr := transcript.WriteString("x")
		return errors.Is(writeErr, os.ErrClosed)
	}, 5*time.Second, 10*time.Millisecond, "a no-PTY session's end did not close the model's transcript")
	require.NoError(t, client.Close())
}
