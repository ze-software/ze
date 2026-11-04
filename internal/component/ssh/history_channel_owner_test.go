// Design: docs/guide/config-editor.md -- each SSH channel owns its actual model closer.
// Related: history_shutdown_test.go -- transport metadata adapters and gated real storage.

package ssh

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"
	charmssh "charm.land/ssh"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestSSHChannelsSharingConnectionCloseTheirOwnModels models the vendor's real
// shared connection Context while driving actual sessionCloseMiddleware,
// teaHandler, cli.Model, History and transcript files. Channel A must finish its
// own cleanup after A's writer releases even while B is still active and held.
// A connection-context closer slot instead closes B and leaves A orphaned.
func TestSSHChannelsSharingConnectionCloseTheirOwnModels(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	fileA, err := os.Create(filepath.Join(t.TempDir(), "channel-a-transcript"))
	if err != nil {
		t.Fatal(err)
	}
	defer fileA.Close() //nolint:errcheck // Rescue after actual model ownership is exercised.
	fileB, err := os.Create(filepath.Join(t.TempDir(), "channel-b-transcript"))
	if err != nil {
		t.Fatal(err)
	}
	defer fileB.Close() //nolint:errcheck // Rescue after actual model ownership is exercised.
	transcriptA := cli.NewTranscriptWriter(fileA, "operator", "shared-connection")
	transcriptB := cli.NewTranscriptWriter(fileB, "operator", "shared-connection")
	memory := storage.CreateMemory()
	defer memory.Close() //nolint:errcheck // Both real models borrow this one store.
	// Network channels belong outside synctest; the model barrier is exercised
	// and joined inside the bubble where these session owners are admitted.
	if err := server.wish.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	synctest.Test(t, func(t *testing.T) {
		defer func() {
			if err := server.Stop(context.Background()); err != nil {
				t.Errorf("stop channel fixture: %v", err)
			}
		}()
		shared := &historySessionContext{Context: context.Background()}
		writerA := &historyShutdownStore{Storage: memory, entered: make(chan struct{}), release: make(chan struct{})}
		writerB := &historyShutdownStore{Storage: memory, entered: make(chan struct{}), release: make(chan struct{})}
		install := func(writer *historyShutdownStore, transcript *cli.TranscriptWriter, input string) <-chan cli.Model {
			created := make(chan cli.Model, 1)
			server.SetSessionModelFactory(func(username, _ string, _ plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
				model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
				model.SetHistory(cli.NewHistory(writer, username))
				model.SetTranscript(transcript)
				initial := model
				model.SetInput(input)
				model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				created <- initial
				return initial, nil
			})
			return created
		}

		disconnectedA := make(chan struct{})
		readyA := make(chan struct{})
		endedA := make(chan struct{})
		var disconnectA, releaseA sync.Once
		createdA := install(writerA, transcriptA, "show channel-a")
		go func() {
			defer close(endedA)
			server.sessionCloseMiddleware()(func(sess charmssh.Session) {
				server.teaHandler(sess)
				close(readyA)
				<-disconnectedA
			})(&historySession{ctx: shared})
		}()
		<-readyA
		modelA := <-createdA
		defer func() {
			disconnectA.Do(func() { close(disconnectedA) })
			releaseA.Do(func() { close(writerA.release) })
			<-endedA
			modelA.Close() //nolint:errcheck // Rescue also joins the orphan in the intended pre-fix red.
		}()

		disconnectedB := make(chan struct{})
		readyB := make(chan struct{})
		endedB := make(chan struct{})
		var disconnectB, releaseB sync.Once
		createdB := install(writerB, transcriptB, "show channel-b")
		go func() {
			defer close(endedB)
			server.sessionCloseMiddleware()(func(sess charmssh.Session) {
				server.teaHandler(sess)
				close(readyB)
				<-disconnectedB
			})(&historySession{ctx: shared})
		}()
		<-readyB
		modelB := <-createdB
		defer func() {
			disconnectB.Do(func() { close(disconnectedB) })
			releaseB.Do(func() { close(writerB.release) })
			<-endedB
			modelB.Close() //nolint:errcheck // Cleanup is idempotent for history; transcript may already be closed.
		}()

		synctest.Wait()
		select {
		case <-writerA.entered:
		default:
			t.Fatal("channel A's accepted command never reached its writer")
		}
		select {
		case <-writerB.entered:
		default:
			t.Fatal("channel B's accepted command never reached its writer")
		}
		disconnectA.Do(func() { close(disconnectedA) })
		synctest.Wait()
		select {
		case <-endedA:
			t.Error("channel A cleanup returned before its held publication")
		default:
		}
		releaseA.Do(func() { close(writerA.release) })
		synctest.Wait()
		select {
		case <-endedA:
		default:
			t.Error("channel A cleanup waits on channel B's history: shared connection context chose the wrong model closer")
		}
		if _, err := fileA.WriteString("must be closed after A cleanup"); !errors.Is(err, os.ErrClosed) {
			t.Errorf("channel A transcript remains open after its own history drained: %v", err)
		}
		if _, err := fileB.WriteString("B remains active while A closes\n"); err != nil {
			t.Errorf("channel A cleanup closed active channel B's transcript: %v", err)
		}

		disconnectB.Do(func() { close(disconnectedB) })
		releaseB.Do(func() { close(writerB.release) })
		<-endedB
		<-endedA
		writerA.mu.Lock()
		writesA := slices.Clone(writerA.writes)
		writerA.mu.Unlock()
		writerB.mu.Lock()
		writesB := slices.Clone(writerB.writes)
		writerB.mu.Unlock()
		if want := []historyShutdownWrite{{key: "meta/history/operator/operational", payload: "show channel-a"}}; !slices.Equal(writesA, want) {
			t.Errorf("channel A actual publications = %v, want %v", writesA, want)
		}
		if want := []historyShutdownWrite{{key: "meta/history/operator/operational", payload: "show channel-b"}}; !slices.Equal(writesB, want) {
			t.Errorf("channel B actual publications = %v, want %v", writesB, want)
		}
		if got := cli.NewHistory(memory, "operator").Load(cli.ModeOperational.String()); !slices.Equal(got, []string{"show channel-b"}) {
			t.Errorf("ordered final same-user snapshot = %v, want channel B", got)
		}
	})
}
