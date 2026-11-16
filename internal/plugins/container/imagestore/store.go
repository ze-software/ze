// Design: docs/architecture/container/image-store.md -- the store, the seam it fills and the commands it answers
// Related: register.go -- the registration that installs newStore

package imagestore

import (
	"context"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
)

// The store and the image commands must agree on the seam. Compile-time
// checks, so a drift fails the build rather than refusing every command.
var (
	_ container.Store           = (*Store)(nil)
	_ container.ImageOperations = (*Store)(nil)
)

// Store is the image store of one run of the container plugin.
//
// Wiring stub of the store's first phase: every operation answers that the
// store is not implemented in this build, and Resolve answers an error, never
// an absent image, because no fetch could bring one. Each later phase of
// spec-container-2-image-store replaces one group of these answers. Safe for
// concurrent use: it holds no state.
type Store struct {
	// notices is what the store tells the runtime: an image present, a
	// restart on a new generation. Nothing sends one before the fetch phase.
	notices container.StoreNotices
}

// newStore builds the store for one run of the plugin. It starts nothing:
// the plugin calls it from its engine entry, never from a declaration query.
func newStore(notices container.StoreNotices) container.Store {
	return &Store{notices: notices}
}

// notImplemented is the refusal of every operation of the wiring stub.
func notImplemented(operation string) error {
	return opererr.New(
		operation+" was not run: the image store is not implemented in this build",
		"this build registers the store's commands and seam before the store itself",
		"ze show doctor",
	)
}

// Resolve answers an error for every container: the stub holds no image and
// can fetch none, so an absent answer would wait for a fetch that never ends.
func (s *Store) Resolve(_ context.Context, request container.ResolveRequest) (container.Resolution, error) {
	return nil, opererr.New(
		"container "+request.Instance+" has no image: the image store is not implemented in this build",
		"this build registers the store's commands and seam before the store itself",
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

func (s *Store) ListImages(context.Context) ([]container.ImageRow, error) {
	return nil, notImplemented("show container image list")
}

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
