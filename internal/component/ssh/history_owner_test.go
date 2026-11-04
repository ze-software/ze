// Design: docs/guide/config-editor.md -- real SSH sessions drain daemon-owned history.

package ssh

import (
	"io"
	"io/fs"
	"os"
	"slices"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

type sshHistoryStore struct {
	storage.Storage
	entered   chan struct{}
	release   chan struct{}
	published chan struct{}
	once      sync.Once
}

func (s *sshHistoryStore) WriteFile(key string, data []byte, perm fs.FileMode) error {
	s.once.Do(func() { close(s.entered); <-s.release })
	err := s.Storage.WriteFile(key, data, perm)
	close(s.published)
	return err
}

// TestSSHPTYDrainsHistoryAtSessionEnd runs an authenticated PTY session against
// the real server and actual Model owner, proving eventual cleanup and exact
// publication after disconnect. TestSSHSessionCleanupDrainsBeforeTranscriptClose
// separately discriminates cleanup ordering at a causally admitted boundary.
func TestSSHPTYDrainsHistoryAtSessionEnd(t *testing.T) {
	testSSHHistoryDrain(t, true)
}

// TestSSHNoPTYDrainsHistoryAtSessionEnd uses the same real shell channel without
// requesting a PTY. Refusing the program must still discharge model ownership.
func TestSSHNoPTYDrainsHistoryAtSessionEnd(t *testing.T) {
	testSSHHistoryDrain(t, false)
}

func testSSHHistoryDrain(t *testing.T, pty bool) {
	t.Helper()
	store := &sshHistoryStore{Storage: storage.CreateMemory(), entered: make(chan struct{}), release: make(chan struct{}), published: make(chan struct{})}
	defer store.Storage.Close() //nolint:errcheck // Borrowed store outlives session/model cleanup.
	var release sync.Once
	defer release.Do(func() { close(store.release) })
	reader, transcript, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()     //nolint:errcheck // Test pipe.
	defer transcript.Close() //nolint:errcheck // Rescue if model construction fails.
	transcriptEnded := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, reader)
		transcriptEnded <- err
	}()
	server := answerServer(t, func(string) (*plugin.Response, error) { return plugin.NewResponse(plugin.StatusDone, nil), nil })
	created := make(chan cli.Model, 1)
	server.SetSessionModelFactory(func(username, remoteAddr string, _ plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
		model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
		model.SetHistory(cli.NewHistory(store, username))
		model.SetTranscript(cli.NewTranscriptWriter(transcript, username, remoteAddr))
		// teaHandler registers this initial value as io.Closer. A later Update
		// copy submits the command, exactly the aliasing the owner must handle.
		initial := model
		model.SetInput("show history-owner")
		_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		created <- initial
		return initial, nil
	})
	client, err := gossh.Dial("tcp", server.Address(), &gossh.ClientConfig{
		User: "operator", Auth: []gossh.AuthMethod{gossh.Password("read-pass")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // Per-test server key.
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // Test transport cleanup.
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close() //nolint:errcheck // Test channel cleanup.
	session.Stdout, session.Stderr = io.Discard, io.Discard
	if pty {
		if err := session.RequestPty("xterm", 24, 80, gossh.TerminalModes{gossh.ECHO: 0}); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.Shell(); err != nil {
		t.Fatal(err)
	}
	initial := <-created
	defer initial.Close() //nolint:errcheck // Rescue, after releasing the gate below.
	defer release.Do(func() { close(store.release) })
	<-store.entered
	if err := session.Close(); err != nil {
		t.Error(err)
	}
	if err := client.Close(); err != nil {
		t.Error(err)
	}
	select {
	case err := <-transcriptEnded:
		t.Fatalf("transcript closed before held history drained: %v", err)
	default:
	}
	release.Do(func() { close(store.release) })
	if err := <-transcriptEnded; err != nil {
		t.Fatal(err)
	}
	// EOF observes real owner completion on this authenticated transport path.
	// The causal middleware regression, not client-close scheduling, proves
	// transcript-versus-history ordering while the writer remains held.
	select {
	case <-store.published:
	default:
		t.Fatal("SSH transcript closed before history publication finished")
	}
	if got := cli.NewHistory(store, "operator").Load(cli.ModeOperational.String()); !slices.Equal(got, []string{"show history-owner"}) {
		t.Errorf("SSH owner history = %v", got)
	}
	if err := store.Storage.WriteFile("meta/borrowed-after-ssh", []byte("usable"), 0); err != nil {
		t.Fatal(err)
	}
}
