// Design: docs/architecture/container/image-store.md -- the store, the seam it fills and the commands it answers
// Related: register.go -- the registration that installs newStore
// Related: layout.go -- the store root every operation opens

package imagestore

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
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
// The store lands in phases. In place: the store root under the data
// directory, its folders and modes, staging emptied at start, the fetch on
// one worker with its queue, its kept blobs and its free-space rule, layer
// application, publication by one rename and the binding of the reference,
// the image listing with the fetches in flight and the failed ones, and the
// resolve of a container to its image, its writable folder and its current
// generation. Every other operation answers that it is not implemented in
// this build.
//
// No handle is kept between operations: each opens the store root, checks
// it, and closes it, so a store root that changed mode or owner while Ze ran
// is refused at the next operation. Safe for concurrent use.
type Store struct {
	// notices is what the store tells the runtime: an image present, a
	// restart on a new generation. A fetch sends the first.
	notices container.StoreNotices
	// engine is the link to the engine's state store, which holds the
	// bindings and the generation records. It is the process's link, set at
	// construction and never changed; a field so the package's tests can hand
	// the store a fake engine.
	engine *engineLink
	// fetches is the queue of the one worker that runs every fetch.
	fetches fetchQueue
	// clientConfig answers how the registry a reference names is reached.
	// space answers the space of the store's filesystem. answerWait bounds
	// how long the fetch command waits for its fetch. largestImage bounds
	// the expanded bytes of one image. Each is set at construction and never
	// changes; they are fields so the package's tests can point them at a
	// test registry, a filesystem of a chosen size and a shorter wait.
	clientConfig func(registry.Reference) (registry.Config, error)
	space        func(path string) (diskspace.Blocks, error)
	answerWait   time.Duration
	largestImage uint64
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
	// heldMu guards held, the digest each container holds from its Hold to
	// its release, by container name.
	heldMu sync.Mutex
	held   map[string]string
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
	s := &Store{
		notices:      notices,
		engine:       &receivedEngine,
		clientConfig: engineClientConfig,
		space:        diskspace.Stat,
		answerWait:   fetchAnswerWaitDefault,
		largestImage: largestImageDefault,
	}
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

// Hold records that instance holds the image of resolved, for the listing's
// used-by column, until the release it answers is called. It protects
// nothing from removal, which is safe only while no operation of the store
// removes an image, a writable layer or a volume: delete, clear, update and
// rollback each answer "not implemented in this build". The first of them to
// land MUST refuse to remove what a hold names.
func (s *Store) Hold(instance string, resolved container.Ready) func() {
	s.heldMu.Lock()
	defer s.heldMu.Unlock()
	if s.held == nil {
		s.held = map[string]string{}
	}
	s.held[instance] = resolved.Digest
	var once sync.Once
	return func() {
		once.Do(func() {
			s.heldMu.Lock()
			defer s.heldMu.Unlock()
			// A later hold of the same container on another digest is not
			// this hold's to release.
			if s.held[instance] == resolved.Digest {
				delete(s.held, instance)
			}
		})
	}
}

// holders answers the containers that hold digest, sorted.
func (s *Store) holders(digest string) []string {
	s.heldMu.Lock()
	defer s.heldMu.Unlock()
	names := []string{}
	for instance, held := range s.held {
		if held == digest {
			names = append(names, instance)
		}
	}
	slices.Sort(names)
	return names
}

// ListImages answers one row for each published image, present, then one for
// each fetch in flight, fetching with the bytes it received, and one for the
// last failed fetch of each reference, failed with its sentence and its code.
func (s *Store) ListImages(context.Context) ([]container.ImageRow, error) {
	// The queue is read before the folders. A fetch publishes its image and
	// only then ends, so a fetch read as in flight is either not published
	// yet or published by the time the folders are read, and its row is
	// dropped then: the image shows once, never twice and never not at all.
	queued := s.fetches.rows()
	var rows []container.ImageRow
	err := s.withRoot(func(root *storeRoot) error {
		var listErr error
		rows, listErr = root.images()
		return listErr
	})
	if err != nil {
		return nil, err
	}
	published := make(map[string]struct{}, len(rows))
	for i := range rows {
		published[rows[i].Digest] = struct{}{}
		rows[i].UsedBy = s.holders(rows[i].Digest)
	}
	for i := range queued {
		if _, ok := published[queued[i].resolved]; ok {
			continue
		}
		rows = append(rows, queued[i].row)
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
