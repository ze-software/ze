// Design: docs/architecture/testing/interop.md -- the persistent image cache.
// Related: docker_image_cache.go -- the cache entry, its key and its load.
package interoplab

import (
	"context"
	"errors"
	"fmt"
)

// ErrImageNotCached says the persistent image cache holds no build of an
// image's current inputs. A caller that only boots the image answers it with
// the one command that builds it.
var ErrImageNotCached = errors.New("the image cache holds no build of this image's current inputs")

// LoadCached answers build from the persistent image cache and never builds
// it. A test or lab that boots a reusable image calls it instead of Build: the
// owner's rule is that a test boots the cached image and, on a cold cache,
// refuses and names the one command that builds it, which runs on its own
// outside any test deadline. A cold cache answers an error wrapping
// ErrImageNotCached. A run that holds the entry's lock is building it, so
// LoadCached waits for that build, bounded by the build's timeout, and loads
// what it saved.
//
// The answer carries a run-owned tag beside build.Tag, as Build's does: the
// caller MUST release it with ReleaseImage once the image is started or no
// longer needed. Safe for concurrent use: the entry's lock serializes runs.
func (d *Docker) LoadCached(ctx context.Context, build ImageBuild) (ImageResult, error) {
	image := ImageResult{Name: build.Name, Tag: build.Tag}
	if !build.Cache {
		return image, errors.New("image " + build.Name + " is not kept in the image cache, so it cannot be loaded from it")
	}
	timeout := max(build.Timeout, d.buildTimeout)
	if timeout <= 0 {
		return image, errors.New("Docker build timeout must be positive")
	}
	entry, err := d.imageCacheEntry(ctx, build)
	if err != nil {
		return image, fmt.Errorf("image %s cannot be looked up in the image cache: %w", build.Name, err)
	}
	lock, err := entry.lock(ctx, timeout, func() {
		imageCacheNotice(build, "waits for another run that builds the same image (key "+entry.key+")")
	})
	if err != nil {
		return image, fmt.Errorf("image %s: the image cache entry %s cannot be locked: %w", build.Name, entry.key, err)
	}
	defer lock.Close() //nolint:errcheck // Closing drops the hold; nothing was written through it.

	loaded, err := d.loadCachedImage(ctx, build, entry, timeout)
	if errors.Is(err, errImageCacheMiss) {
		return image, fmt.Errorf("image %s (key %s): %w", build.Name, entry.key, ErrImageNotCached)
	}
	if err != nil {
		return image, err
	}
	imageCacheNotice(build, "loaded from the image cache, key "+entry.key+" ("+loaded.Reference+")")
	return loaded, nil
}
