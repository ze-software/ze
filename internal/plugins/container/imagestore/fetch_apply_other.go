// Design: docs/architecture/container/image-store.md -- the fetch: application of the checked blobs
// Related: fetch_steps.go -- the fetch that calls it
// Related: fetch_apply_linux.go -- the application on Linux

//go:build !linux

package imagestore

import (
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// applyLayers refuses: the applier resolves every entry with Linux's
// descriptor-relative calls, and containers run on Linux only, so no image
// is applied where none could run.
func applyLayers(_ *storeRoot, _ string, reference registry.Reference, _ []registry.Layer, _ []registry.Digest, _ uint64, _ *imageFacts) error {
	return &fetchError{code: container.CodeImageRefused, err: opererr.New(
		"image "+reference.String()+" was not applied: containers run on Linux only",
		"the store applies a layer with Linux's descriptor-relative calls, and no image it applied could run on this system",
		container.FixShowDoctor,
	)}
}
