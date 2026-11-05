// Design: docs/guide/config-editor.md -- local editor program drains before store release.

package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/yang"
)

type editorHistoryStore struct {
	storage.Storage
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *editorHistoryStore) WriteFile(key string, data []byte, perm fs.FileMode) error {
	s.once.Do(func() { close(s.entered); <-s.release })
	return s.Storage.WriteFile(key, data, perm)
}

// TestEditorProgramOwnerClosesHistory constructs real loose-file and backup
// editors, then runs the production program owner with a borrowed terminal
// runner. Normal completion, program failure and cancellation must all wait for
// history, with no final command result delivered to Update.
func TestEditorProgramOwnerClosesHistory(t *testing.T) {
	for _, backup := range []bool{false, true} {
		t.Run(map[bool]string{false: "loose", true: "backup"}[backup], func(t *testing.T) {
			for _, tc := range []struct {
				name string
				err  error
			}{
				{name: "normal"},
				{name: "program refusal", err: errors.New("terminal program refused")},
				{name: "canceled", err: context.Canceled},
			} {
				t.Run(tc.name, func(t *testing.T) {
					store := storage.CreateMemory()
					defer store.Close() //nolint:errcheck // Store outlives model and editor.
					var ed *editor.Editor
					var err error
					if backup {
						if err := store.WriteFile("owner.conf", []byte(backupTestConfig), 0); err != nil {
							t.Fatal(err)
						}
						ed, err = editor.NewOfflineSessionEditor(store, "owner.conf", "program-owner")
					} else {
						path := filepath.Join(t.TempDir(), "owner.conf")
						if err := os.WriteFile(path, []byte(backupTestConfig), 0o600); err != nil {
							t.Fatal(err)
						}
						ed, err = editor.NewLooseFileEditor(nil, path)
					}
					if err != nil {
						t.Fatal(err)
					}
					defer ed.Close() //nolint:errcheck // Editor outlives the program owner.
					model, err := editor.NewModel(ed, editor.FilesystemAuthorityOperatorLocal)
					if err != nil {
						t.Fatal(err)
					}
					synctest.Test(t, func(t *testing.T) {
						writer := &editorHistoryStore{Storage: store, entered: make(chan struct{}), release: make(chan struct{})}
						history := editor.NewHistory(writer, "program-owner")
						model.SetHistory(history)
						var release sync.Once
						ended := make(chan int, 1)
						joined := make(chan struct{})
						go func() {
							defer close(joined)
							ended <- runEditorProgramWith(&model, func(model *editor.Model) error {
								model.SetInput("show")
								next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
								updated, ok := next.(editor.Model)
								if !ok {
									t.Errorf("Update returned %T", next)
									return errors.New("wrong model")
								}
								*model = updated
								return tc.err
							})
						}()
						defer func() { release.Do(func() { close(writer.release) }); <-joined }()
						synctest.Wait()
						select {
						case <-writer.entered:
						default:
							t.Fatal("program never admitted history")
						}
						select {
						case code := <-ended:
							t.Fatalf("program owner returned %d before history drained", code)
						default:
						}
						release.Do(func() { close(writer.release) })
						want := exitOK
						if tc.err != nil {
							want = exitError
						}
						if got := <-ended; got != want {
							t.Errorf("exit = %d, want %d", got, want)
						}
						if got := history.Load(editor.ModeConfig.String()); !slices.Equal(got, []string{"show"}) {
							t.Errorf("program history = %v", got)
						}
						if err := store.WriteFile("meta/store-still-open", []byte("yes"), 0); err != nil {
							t.Fatal(err)
						}
					})
				})
			}
		})
	}
}

// editorCloseStore observes the actual ed.Close obligation on constructor/tree
// refusal, with no model, worker or terminal constructed first.
type editorCloseStore struct {
	storage.Storage
	closed bool
}

func (s *editorCloseStore) Close() error { s.closed = true; return s.Storage.Close() }

// TestEditorConstructorRefusalReleasesOwnedStore calls both real local owners
// after making their shared schema refuse. The editor owns its real store;
// each pre-program error path must release it without starting terminal work.
func TestEditorConstructorRefusalReleasesOwnedStore(t *testing.T) {
	for _, backup := range []bool{false, true} {
		t.Run(map[bool]string{false: "loose", true: "backup"}[backup], func(t *testing.T) {
			store := &editorCloseStore{Storage: storage.CreateMemory()}
			defer store.Storage.Close() //nolint:errcheck // Rescue on assertion failure.
			configPath := filepath.Join(t.TempDir(), "owner.conf")
			var ed *editor.Editor
			var err error
			if backup {
				if err := store.WriteFile("owner.conf", []byte(backupTestConfig), 0); err != nil {
					t.Fatal(err)
				}
				ed, err = editor.NewOfflineSessionEditor(store, "owner.conf", "constructor-owner")
			} else {
				if err := os.WriteFile(configPath, []byte(backupTestConfig), 0o600); err != nil {
					t.Fatal(err)
				}
				ed, err = editor.NewLooseFileEditor(store, configPath)
			}
			if err != nil {
				t.Fatal(err)
			}
			ed.OwnStore()
			restore := yang.RegisterModuleForTest("ze-history-refusal.yang", `module ze-history-refusal {
 namespace "urn:ze:history-refusal"; prefix refusal;
 import ze-extensions { prefix ze; }
 leaf refusal { type string; ze:hepl "Undeclared extension."; }
}`)
			defer restore()
			var code int
			if backup {
				code = runBackupModel(ed)
			} else {
				code = runEditor(ed, configPath, "constructor-owner")
			}
			if code != exitError {
				t.Errorf("constructor refusal exit = %d", code)
			}
			if !store.closed {
				t.Error("constructor refusal leaked editor-owned store")
			}
		})
	}
}
