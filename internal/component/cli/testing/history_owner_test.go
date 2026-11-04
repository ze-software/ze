// Design: docs/guide/config-editor.md -- headless model/editor/store shutdown order.

package testing

import (
	"context"
	"io/fs"
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
)

type headlessHistoryStore struct {
	storage.Storage
	entered     chan struct{}
	release     chan struct{}
	closeCalled chan struct{}
	once        sync.Once
	closed      sync.Once
	payload     []byte
	closes      int
}

func (s *headlessHistoryStore) WriteFile(key string, data []byte, perm fs.FileMode) error {
	s.once.Do(func() { close(s.entered); <-s.release })
	return s.Storage.WriteFile(key, data, perm)
}

func (s *headlessHistoryStore) Close() error {
	s.closes++
	s.closed.Do(func() { close(s.closeCalled) })
	data, err := s.Storage.ReadFile("meta/history/headless-owner/config")
	if err == nil {
		s.payload = slices.Clone(data)
	}
	return s.Storage.Close()
}

// TestHeadlessHistoryOwnerDrainsBeforeEditorClose calls the actual headless close
// with a real editor owning its store. The stopped signal proves close started;
// the store may not close while history is held, and captures the exact final
// payload at closure. No special headless history flush path is involved.
func TestHeadlessHistoryOwnerDrainsBeforeEditorClose(t *testing.T) {
	memory := storage.CreateMemory()
	defer memory.Close() //nolint:errcheck // Rescue after real owner cleanup.
	if err := memory.WriteFile("headless.conf", []byte("set system host-name headless\n"), 0); err != nil {
		t.Fatal(err)
	}
	store := &headlessHistoryStore{Storage: memory}
	hm, err := newHeadlessModel(store, "headless.conf")
	if err != nil {
		t.Fatal(err)
	}
	hm.editor.OwnStore()
	synctest.Test(t, func(t *testing.T) {
		store.entered = make(chan struct{})
		store.release = make(chan struct{})
		store.closeCalled = make(chan struct{})
		hm.Model().SetHistory(cli.NewHistory(store, "headless-owner"))
		var release sync.Once
		defer release.Do(func() { close(store.release) })
		hm.typeText("show")
		hm.pressEnter()
		synctest.Wait()
		select {
		case <-store.entered:
		default:
			t.Fatal("headless Enter never published history")
		}
		ended := make(chan error, 1)
		joined := make(chan struct{})
		go func() { defer close(joined); ended <- hm.close() }()
		defer func() { release.Do(func() { close(store.release) }); <-joined }()
		synctest.Wait()
		select {
		case <-hm.stopped:
		default:
			t.Error("headless close never started")
		}
		select {
		case <-store.closeCalled:
			t.Error("editor/store released before history drained")
		default:
		}
		select {
		case err := <-ended:
			t.Fatalf("headless owner returned before writer release: %v", err)
		default:
		}
		release.Do(func() { close(store.release) })
		if err := <-ended; err != nil {
			t.Fatal(err)
		}
		if string(store.payload) != "show" {
			t.Errorf("payload at store close = %q, want show", store.payload)
		}
		if store.closes != 1 {
			t.Errorf("store closed %d times, want editor alone", store.closes)
		}
	})
}

// TestHeadlessHistoryRestartDrainsBeforeReload runs the actual .et runner's
// restart path in both modes, preserving exact Up/Down and persisted recall.
func TestHeadlessHistoryRestartDrainsBeforeReload(t *testing.T) {
	for _, mode := range []string{"config", "operational"} {
		t.Run(mode, func(t *testing.T) {
			prefix := "tmpfs=owner.conf:terminator=END\nset system host-name restart\nEND\noption=file:path=owner.conf\n"
			if mode == "operational" {
				prefix = "option=mode:value=operational\n"
			}
			content := prefix + "option=history:store\ninput=type:text=show\ninput=enter\ninput=type:text=show | compare\ninput=enter\nrestart=editor\ninput=up\nexpect=input:value=show | compare\ninput=up\nexpect=input:value=show\ninput=down\nexpect=input:value=show | compare\n"
			result := RunETContentIn(context.Background(), content, t.TempDir())
			if !result.Passed {
				t.Fatalf("actual %s restart: %+v", mode, result)
			}
		})
	}
}
