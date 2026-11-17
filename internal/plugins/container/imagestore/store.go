// Design: docs/architecture/container/image-store.md -- the store, the seam it fills and the commands it answers
// Related: register.go -- the registration that installs newStore
// Related: layout.go -- the store root every operation opens

package imagestore

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/container"
)

// The store and the image commands must agree on the seam. Compile-time
// checks, so a drift fails the build rather than refusing every command.
var (
	_ container.Store           = (*Store)(nil)
	_ container.ImageOperations = (*Store)(nil)
	_ container.StoreChecker    = (*Store)(nil)
)

// storeLog is the store's logger. It reports what no caller is waiting for:
// a store root that could not be prepared when the plugin started.
var storeLog = slogutil.LazyLogger("container.store")

// Store is the image store of one run of the container plugin.
//
// The store lands in phases. The layout is in place: the store root under
// the data directory, its folders and modes, staging emptied at start,
// publication by one rename, and the image listing. Every other operation
// answers that it is not implemented in this build, and Resolve answers an
// error, never an absent image, because no fetch could bring one yet.
//
// No handle is kept between operations: each opens the store root, checks
// it, and closes it, so a store root that changed mode or owner while Ze ran
// is refused at the next operation. Safe for concurrent use.
type Store struct {
	// notices is what the store tells the runtime: an image present, a
	// restart on a new generation. Nothing sends one before the fetch phase.
	notices container.StoreNotices
	// path is the absolute path of the store root. pathErr replaces it when
	// no data directory resolved. Both are set at construction, exactly one
	// of them, and never change.
	path    string
	pathErr error
	// mu guards prepared, which is true once a preparation of this run
	// succeeded. Staging is emptied by that preparation and never again in
	// the run: emptying it later would remove the work of a fetch in flight.
	mu       sync.Mutex
	prepared bool
}

// newStore builds the store for one run of the plugin over the data
// directory Ze resolves. It starts no worker: the plugin calls it from its
// engine entry, never from a declaration query.
func newStore(notices container.StoreNotices) container.Store {
	return openStore(paths.DefaultDataDir(), notices)
}

// openStore builds the store over dataDir and prepares its root: the
// folders are created where they are absent and staging is emptied, once in
// this run. A root that cannot be prepared now is not fatal to the plugin:
// every operation tries again and answers the refusal it meets.
func openStore(dataDir string, notices container.StoreNotices) *Store {
	s := &Store{notices: notices}
	s.path, s.pathErr = storePath(dataDir)
	if err := s.withRoot(func(*storeRoot) error { return nil }); err != nil {
		storeLog().Warn("container: the image store could not be prepared at start",
			"error", err, "code", container.CodeStoreUnwritable)
	}
	return s
}

// storePath answers the absolute path of the store root under dataDir. An
// empty dataDir is no data directory: the binary sits in no known layout and
// the override is not set, so the store has no place and says so.
func storePath(dataDir string) (string, error) {
	if dataDir == "" {
		return "", opererr.New(
			"the container store has no data directory",
			"ze derives it from the location of its binary, which matches no known layout, and ze.data.dir is not set",
			"export ZE_DATA_DIR=/var/lib/ze, then start ze again",
		)
	}
	absolute, err := filepath.Abs(dataDir)
	if err != nil {
		return "", opererr.Wrap(err,
			"the container store's data directory "+dataDir+" cannot be made absolute",
			"the store hands the runtime absolute folders",
			"export ZE_DATA_DIR=/var/lib/ze, then start ze again",
		)
	}
	return filepath.Join(absolute, storeFolder), nil
}

// withRoot opens the store root, hands it to use, and closes it. The first
// open of the run that succeeds also prepares the root.
func (s *Store) withRoot(use func(*storeRoot) error) error {
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	useErr := use(root)
	closeErr := root.close()
	if closeErr == nil {
		return useErr
	}
	return errors.Join(useErr, closeErr)
}

// openRoot opens the store root, preparing it when no preparation of this
// run succeeded yet. The caller MUST close the answer.
func (s *Store) openRoot() (*storeRoot, error) {
	if s.pathErr != nil {
		return nil, s.pathErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prepared {
		return openStoreRoot(s.path)
	}
	root, err := prepareStoreRoot(s.path)
	if err != nil {
		return nil, err
	}
	s.prepared = true
	return root, nil
}

// CheckStore answers whether the store root can be used now, for the doctor
// check container-store: the refusal the next operation would meet, or nil.
// It opens and checks the root and changes nothing: unlike an operation, it
// never prepares the root, because doctor only reads.
func (s *Store) CheckStore() error {
	if s.pathErr != nil {
		return s.pathErr
	}
	root, err := openStoreRoot(s.path)
	if err != nil {
		return err
	}
	if err := root.close(); err != nil {
		return storeCannotOpen(s.path, err)
	}
	return nil
}

// notImplemented is the refusal of every operation a later phase of the
// store brings.
func notImplemented(operation string) error {
	return opererr.New(
		operation+" was not run: this operation of the image store is not implemented in this build",
		"this build holds the store's layout and its image listing, and not yet this operation",
		"ze show doctor",
	)
}

// Resolve answers an error for every container: the store can fetch no
// image yet, so an absent answer would wait for a fetch that never ends.
func (s *Store) Resolve(_ context.Context, request container.ResolveRequest) (container.Resolution, error) {
	return nil, opererr.New(
		"container "+request.Instance+" has no image: the image store cannot fetch one in this build",
		"this build holds the store's layout and its image listing, and not yet the fetch and the resolve",
		"ze show doctor",
	)
}

// Hold answers a release that frees nothing: Resolve never answers Ready, so
// no resolution exists to hold.
func (s *Store) Hold(string, container.Ready) func() {
	return func() {}
}

func (s *Store) FetchImage(context.Context, string) (container.FetchAnswer, error) {
	return container.FetchAnswer{}, notImplemented("create container image")
}

// ListImages answers one row for each published image. Until the fetch
// phase no fetch is in flight and none failed, so every row reads present.
func (s *Store) ListImages(context.Context) ([]container.ImageRow, error) {
	var rows []container.ImageRow
	err := s.withRoot(func(root *storeRoot) error {
		var listErr error
		rows, listErr = root.images()
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// StoreTotals is not answered yet: its caps-enforced field needs the quota
// probe of the capped-storage phase, and a guessed false would read as a
// fact about the filesystem.
func (s *Store) StoreTotals(context.Context) (container.StoreTotals, error) {
	return container.StoreTotals{}, notImplemented("show container store")
}

func (s *Store) ListVolumes(context.Context) ([]container.VolumeRow, error) {
	return nil, notImplemented("show container volume list")
}

func (s *Store) UpdateImage(context.Context, string) (container.UpdateAnswer, error) {
	return container.UpdateAnswer{}, notImplemented("update container image")
}

func (s *Store) RollbackImage(context.Context, string) (container.RollbackAnswer, error) {
	return container.RollbackAnswer{}, notImplemented("update container image rollback")
}

func (s *Store) DeleteImage(context.Context, string) (container.DeleteAnswer, error) {
	return container.DeleteAnswer{}, notImplemented("delete container image")
}

func (s *Store) DeleteAllImages(context.Context) (container.DeleteAnswer, error) {
	return container.DeleteAnswer{}, notImplemented("delete container image all")
}

func (s *Store) DeleteVolume(context.Context, string, string) (container.VolumeDeleteAnswer, error) {
	return container.VolumeDeleteAnswer{}, notImplemented("delete container volume")
}

func (s *Store) ClearLayer(context.Context, string) (container.ClearAnswer, error) {
	return container.ClearAnswer{}, notImplemented("clear container layer")
}
