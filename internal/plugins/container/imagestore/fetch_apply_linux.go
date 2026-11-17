// Design: docs/architecture/container/image-store.md -- the fetch: application of the checked blobs
// Related: fetch_steps.go -- the fetch that calls it
// Related: apply_linux.go -- the applier it opens on the staged root

//go:build linux

package imagestore

import (
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// applyLayers applies the kept blob of each of layers, in order, onto a new
// root under staging/staged, each checked against its digest in digests once
// decompressed, and records the applier's counts in facts. Every blob MUST
// have been checked against its own digest and size: it is in blobs/ only
// then. On an error the staged folder holds part of the image, and the
// caller removes it.
func applyLayers(root *storeRoot, staged string, reference registry.Reference, layers []registry.Layer, digests []registry.Digest, largestImage uint64, facts *imageFacts) error {
	applier, err := root.newLayerApplier(staged, defaultApplyLimits(largestImage))
	if err != nil {
		return err
	}
	for i, layer := range layers {
		if err := applyBlob(root, applier, layer, digests[i]); err != nil {
			return errors.Join(applyFailed(root, reference, err), applier.close())
		}
	}
	applier.countedFacts(facts)
	if err := applier.close(); err != nil {
		return storeCannotOpen(root.path, err)
	}
	return nil
}

// applyBlob applies the kept blob of layer.
func applyBlob(root *storeRoot, applier *layerApplier, layer registry.Layer, digest registry.Digest) error {
	blob, err := root.root.Open(blobPath(layer.Blob().Digest()))
	if err != nil {
		return err
	}
	applyErr := applier.apply(blob, layer.Compression(), digest)
	return errors.Join(applyErr, blob.Close())
}

// applyFailed maps a failed application to its doctor code and its fix. A
// refusal broke a rule of the application table. An entry failure is a write
// the system refused: the image broke no rule, so the store is what could
// not be written.
func applyFailed(root *storeRoot, reference registry.Reference, err error) error {
	text := reference.String()
	if refusal, ok := errors.AsType[*layerRefusal](err); ok {
		fix := container.FixFetch("REFERENCE")
		if refusal.rule == ruleExpandedBytes {
			fix = container.FixLargestImage
		}
		return &fetchError{code: container.CodeImageRefused, err: opererr.Wrap(refusal,
			"image "+text+" was refused",
			"the store applies no layer that breaks a rule of its application table, so nothing of the image is kept",
			fix,
		)}
	}
	if failure, ok := errors.AsType[*entryFailure](err); ok {
		return &fetchError{code: container.CodeStoreUnwritable, err: opererr.Wrap(failure,
			"image "+text+" could not be applied in the container store "+root.path,
			"the system refused a write the layer needed, so the image is not published",
			container.FixShowDoctor,
		)}
	}
	return &fetchError{code: container.CodeStoreUnwritable, err: opererr.Wrap(err,
		"image "+text+" could not be applied in the container store "+root.path,
		"a kept blob could not be read back, so the image is not published",
		container.FixFetch(text),
	)}
}
