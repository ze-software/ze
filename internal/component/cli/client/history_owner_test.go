// Design: docs/guide/config-editor.md -- attached model ownership on every terminal exit.

package client

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"

	unicli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/statestore"
)

type attachedHistoryStore struct {
	storage.Storage
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *attachedHistoryStore) WriteFile(key string, data []byte, perm fs.FileMode) error {
	s.once.Do(func() { close(s.entered); <-s.release })
	return s.Storage.WriteFile(key, data, perm)
}

// TestAttachedModelOwnerClosesHistory exercises the actual attached owner, not
// a fake closer. Its borrowed terminal runner admits real Update/Enter history
// and then completes, refuses TTY startup, fails its program, or is canceled.
// The owner cannot return until the held write drains, even without a result.
func TestAttachedModelOwnerClosesHistory(t *testing.T) {
	t.Setenv("ZE_CLI_TRANSCRIPT", "false")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	// Resolve the registered schema outside synctest; no filesystem initialization
	// is smuggled into the deterministic lifetime observation.
	warm, err := newAttachedModel(attachedTestDispatch, unicli.CommandExecutor(attachedTestDispatch), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := warm.Close(); err != nil {
		t.Fatal(err)
	}
	for _, editor := range []bool{false, true} {
		t.Run(map[bool]string{false: "command", true: "editor"}[editor], func(t *testing.T) {
			var ed *unicli.Editor
			if editor {
				ed = attachedTestEditor(t)
			}
			for _, tc := range []struct {
				name string
				err  error
			}{
				{name: "normal"},
				{name: "tty refusal", err: &fs.PathError{Op: "open", Path: "/dev/tty", Err: fs.ErrPermission}},
				{name: "program failure", err: errors.New("program failed")},
				{name: "canceled", err: context.Canceled},
			} {
				t.Run(tc.name, func(t *testing.T) {
					synctest.Test(t, func(t *testing.T) {
						store := &attachedHistoryStore{Storage: storage.CreateMemory(), entered: make(chan struct{}), release: make(chan struct{})}
						defer store.Storage.Close() //nolint:errcheck // Borrowed store outlives the model owner.
						previous := statestore.Store()
						statestore.SetStore(store.Storage)
						defer statestore.SetStore(previous)
						var history *unicli.History
						var release sync.Once
						ended := make(chan int, 1)
						joined := make(chan struct{})
						go func() {
							defer close(joined)
							ended <- runInteractiveWithProgram(attachedTestDispatch, ed, func(model *unicli.Model) error {
								history = unicli.NewHistory(store, "attached-owner")
								model.SetHistory(history)
								model.SetInput("show owner")
								next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
								updated, ok := next.(unicli.Model)
								if !ok {
									t.Errorf("Update returned %T", next)
									return errors.New("wrong model")
								}
								*model = updated
								return tc.err
							})
						}()
						defer func() { release.Do(func() { close(store.release) }); <-joined }()
						synctest.Wait()
						select {
						case <-store.entered:
						default:
							t.Fatal("attached owner never admitted history")
						}
						select {
						case code := <-ended:
							t.Fatalf("attached owner returned %d before drain", code)
						default:
						}
						release.Do(func() { close(store.release) })
						want := 0
						if tc.err != nil {
							want = 1
						}
						if got := <-ended; got != want {
							t.Errorf("exit = %d, want %d", got, want)
						}
						if got := history.Load(unicli.ModeOperational.String()); !slices.Equal(got, []string{"show owner"}) {
							t.Errorf("attached history reload = %v", got)
						}
						if err := store.Storage.WriteFile("meta/borrowed-open", []byte("yes"), 0); err != nil {
							t.Fatal(err)
						}
					})
				})
			}
		})
	}
}

// TestAttachedModelConstructorRefusalOwnsNoHistory uses the real schema refusal
// before model/history acquisition; the terminal runner MUST never be called.
func TestAttachedModelConstructorRefusalOwnsNoHistory(t *testing.T) {
	restore := yang.RegisterModuleForTest("ze-probe.yang", probeHeplModule)
	defer func() { restore(); resetYANGStateForTest() }()
	resetYANGStateForTest()
	called := false
	code := runInteractiveWithProgram(attachedTestDispatch, nil, func(*unicli.Model) error { called = true; return nil })
	if code != 1 {
		t.Errorf("schema refusal exit = %d, want 1", code)
	}
	if called {
		t.Error("terminal started after constructor refusal")
	}
}
