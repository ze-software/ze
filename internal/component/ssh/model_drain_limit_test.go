// Design: docs/guide/config-editor.md -- a storage call that stops answering ends the model join.
// Related: history_shutdown_test.go -- the join itself, for storage that answers.

package ssh

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	charmssh "charm.land/ssh"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSSHStopEndsItsWaitOnStorageThatStoppedAnswering is the hold behind the
// limit: a real model's history write never returns while Stop runs.
//
// GOAL: the daemon's stop continues once storage has stopped answering, says why,
// and the session's write still lands if the storage answers later.
// METHOD: the fake clock of a synctest bubble, so the limit passes only when
// every goroutine is durably blocked, which is the permanent hold itself.
// PREVENTS: models.wait with no bound, which holds Server.Stop, and with it the
// plugin server's stop hooks and everything after them, until a second signal.
func TestSSHStopEndsItsWaitOnStorageThatStoppedAnswering(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	memory := storage.CreateMemory()
	defer memory.Close() //nolint:errcheck // The test's own store.
	if err := server.wish.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		store := &historyShutdownStore{Storage: memory, entered: make(chan struct{}), release: make(chan struct{})}
		var release sync.Once
		unblock := func() { release.Do(func() { close(store.release) }) }
		server.SetSessionModelFactory(func(username, _ string, _ plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
			model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
			model.SetHistory(cli.NewHistory(store, username))
			initial := model
			model.SetInput("show held")
			model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			return initial, nil
		})
		ended := make(chan struct{})
		session := &historySession{ctx: &historySessionContext{Context: context.Background()}}
		go func() {
			defer close(ended)
			server.sessionCloseMiddleware()(func(sess charmssh.Session) { server.teaHandler(sess) })(session)
		}()
		defer func() { unblock(); <-ended }()
		synctest.Wait()
		select {
		case <-store.entered:
		default:
			t.Fatal("the real model never reached storage")
		}

		began := time.Now()
		err := server.Stop(context.Background())
		if !errors.Is(err, errModelDrainOverdue) {
			t.Errorf("Stop over a held write = %v, want errModelDrainOverdue", err)
		}
		if waited := time.Since(began); waited != modelDrainLimitDefault {
			t.Errorf("Stop waited %s, want exactly the limit %s", waited, modelDrainLimitDefault)
		}
		select {
		case <-ended:
			t.Error("the session finished while its write was still held")
		default:
		}

		// A write that answers late still lands while the store stays open.
		unblock()
		<-ended
		if got := cli.NewHistory(memory, "operator").Load(cli.ModeOperational.String()); !slices.Equal(got, []string{"show held"}) {
			t.Errorf("history after the late write = %v", got)
		}
	})
}

// TestModelDrainLimitLeavesRoomInTheTeardownMargin holds the limit to the budget it
// spends from.
//
// GOAL: a full drain plus storage that stopped answering still ends inside the
// supervisor's stop timeout, so every later stop hook and the plugin cleanup run
// before any SIGKILL.
// METHOD: the limit plus the other bounds teardown spends in series after the drain,
// against the margin the systemd unit grants. Each bound is a copy, named at its
// source, because they live in packages this one does not import.
// PREVENTS: a limit picked on its own. 30 seconds took the teardown past the margin,
// and systemd killed ze and its external plugins before their cleanup.
func TestModelDrainLimitLeavesRoomInTheTeardownMargin(t *testing.T) {
	margin := rpc.DrainTeardownMarginSeconds * time.Second
	others := []time.Duration{
		3 * time.Second,         // apiCtx over apiShutdowns, cmd/ze/hub/main.go.
		3 * time.Second,         // txShutdownGrace, plugin/server/reload.go.
		3 * time.Second,         // The SSH stop hook's network ctx, cmd/ze/hub/infra_setup.go.
		3 * time.Second,         // pluginStopGrace, plugin/process/manager.go.
		1500 * time.Millisecond, // pluginExitGrace plus the 500ms group wait, same file.
		3 * time.Second,         // stopCtx over eng.Stop, cmd/ze/hub/main.go.
	}
	spent := modelDrainLimitDefault
	for _, other := range others {
		spent += other
	}
	if room := margin - spent; room < 5*time.Second {
		t.Errorf("teardown spends %s of the %s margin, leaving %s, want at least 5s of room", spent, margin, room)
	}
}

// TestSSHStartRefusesAGenerationBesideAnOverdueOwner holds the one promise the
// bounded join would otherwise break.
//
// GOAL: while a session the last Stop gave up on is still running, Start refuses,
// and it starts once that session has finished.
// METHOD: an owner admitted by hand and a one-millisecond limit, so Stop gives up
// on it at once.
// PREVENTS: a new generation writing beside the old one, whose late write could
// land over what the new generation already saved.
func TestSSHStartRefusesAGenerationBesideAnOverdueOwner(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	models := server.models
	if !models.admit() {
		t.Fatal("an open generation refused admission")
	}
	server.modelDrainLimit = time.Millisecond
	if err := server.Stop(context.Background()); !errors.Is(err, errModelDrainOverdue) {
		t.Fatalf("Stop over a running owner = %v, want errModelDrainOverdue", err)
	}

	if err := server.Start(t.Context(), nil, nil); !errors.Is(err, errModelDrainOverdue) {
		t.Errorf("Start beside a running owner = %v, want errModelDrainOverdue", err)
	}

	models.finish()
	if err := server.Start(t.Context(), nil, nil); err != nil {
		t.Errorf("Start after the owner finished = %v", err)
	}
}

// TestModelSessionsWait pins each answer of the bounded join directly.
//
// GOAL: wait answers true exactly when every admitted owner finished.
// METHOD: a synctest bubble, whose clock makes the limit deterministic.
// PREVENTS: a seal with nothing admitted waiting out the limit, which would add
// the limit to every clean stop, and a finish the join never observes.
func TestModelSessionsWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		idle := &modelSessions{}
		idle.seal()
		began := time.Now()
		if !idle.wait(time.Hour) {
			t.Error("wait with nothing admitted = false")
		}
		if waited := time.Since(began); waited != 0 {
			t.Errorf("wait with nothing admitted took %s", waited)
		}

		held := &modelSessions{}
		if !held.admit() {
			t.Fatal("an open generation refused admission")
		}
		held.seal()
		if held.admit() {
			t.Error("a sealed generation admitted an owner")
		}
		if held.wait(time.Minute) {
			t.Error("wait over a running owner = true")
		}
		held.finish()
		if !held.wait(time.Hour) {
			t.Error("wait after the owner finished = false")
		}

		finishing := &modelSessions{}
		if !finishing.admit() {
			t.Fatal("an open generation refused admission")
		}
		finishing.seal()
		go func() {
			time.Sleep(time.Second)
			finishing.finish()
		}()
		began = time.Now()
		if !finishing.wait(time.Hour) {
			t.Error("wait over an owner that finished = false")
		}
		if waited := time.Since(began); waited != time.Second {
			t.Errorf("wait took %s, want the owner's one second", waited)
		}
	})
}
