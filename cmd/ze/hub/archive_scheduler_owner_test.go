// Design: docs/guide/config-archive.md -- scheduled reads finish before their store owner returns.
package hub

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// archiveHeldRead borrows the real storage implementation and holds one
// scheduled active-version read until the test lets that borrower finish.
type archiveHeldRead struct {
	storage.Storage
	entered  chan struct{}
	release  <-chan struct{}
	finished chan struct{}
}

func (s *archiveHeldRead) ReadVersion(name, stamp string) ([]byte, error) {
	close(s.entered)
	defer close(s.finished)
	<-s.release
	return s.Storage.ReadVersion(name, stamp)
}

// TestArchiveSchedulerOwnerJoinsActiveRead reaches the actual boot archive's
// stored read. Both shutdown and reload replacement must wait for that read;
// after release they finish, and repeating stop has no remaining worker.
func TestArchiveSchedulerOwnerJoinsActiveRead(t *testing.T) {
	for _, operation := range []string{"shutdown", "reload"} {
		t.Run(operation, func(t *testing.T) {
			stopArchiveScheduler()
			dir := t.TempDir()
			path := filepath.Join(dir, "router.conf")
			base := newTestStore(t, dir)
			stamp := "20261117-200000.000"
			require.NoError(t, base.WriteVersion(path, validConfig, mustParseReloadStamp(t, stamp)))
			setConfigPointer(t, base, path, zefs.KeyConfigActive, stamp)
			release := make(chan struct{})
			held := &archiveHeldRead{
				Storage: base, entered: make(chan struct{}), release: release, finished: make(chan struct{}),
			}
			store := storage.BindConfigSource(held, path, storage.ConfigSourceStored)
			schema, err := zeconfig.YANGSchema()
			require.NoError(t, err)
			parser := zeconfig.NewParser(schema)
			tree, err := parser.Parse(`system {
	archive scheduled { location "file://` + filepath.Join(dir, "archives") + `"; trigger hourly; }
}`)
			require.NoError(t, err)
			emptyTree, err := parser.Parse("system {}")
			require.NoError(t, err)
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(stopArchiveScheduler)
			t.Cleanup(unblock)
			startArchiveScheduler(tree, path, store, nil)
			select {
			case <-held.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("the scheduled boot archive did not reach its stored active-version read")
			}
			attempted, stopped := make(chan struct{}), make(chan struct{})
			go func() {
				close(attempted)
				if operation == "reload" {
					startArchiveScheduler(emptyTree, path, store, nil)
				} else {
					stopArchiveScheduler()
				}
				close(stopped)
			}()
			<-attempted
			select {
			case <-stopped:
				t.Fatal("archive owner returned while its stored configuration read was still held")
			case <-time.After(200 * time.Millisecond):
				// The controlled read cannot finish before unblock; this bound
				// gives a cancel-only owner time to expose its premature return.
			}
			unblock()
			select {
			case <-stopped:
			case <-time.After(3 * time.Second):
				t.Fatal("archive owner did not return after its stored read finished")
			}
			select {
			case <-held.finished:
			default:
				t.Fatal("archive owner returned without joining its stored read")
			}
			stopArchiveScheduler()
		})
	}
}
