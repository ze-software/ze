package interoplab

import (
	"bytes"
	"errors"
	"testing"
)

// VALIDATES: LoadCached builds nothing. On a cold cache it answers
// ErrImageNotCached and runs no build; once Build has filled the entry, it
// loads it and answers a cached image with the shared tag.
// PREVENTS: a test that boots a reusable image compiling it inside its own
// deadline, which the owner's rule forbids (ai/META-DECISIONS.md, "Test systems
// and labs").
func TestLoadCachedRefusesAColdCacheAndLoadsAFilledOne(t *testing.T) {
	cacheDir := t.TempDir()
	daemon := newCacheDaemon(t)
	docker := daemon.docker(cacheDir)
	var progress bytes.Buffer
	build := cachedBuild(cacheContext(t), &progress)

	if _, err := docker.LoadCached(t.Context(), build); !errors.Is(err, ErrImageNotCached) {
		t.Fatalf("a cold cache answered %v, want ErrImageNotCached", err)
	}
	if len(daemon.builds) != 0 {
		t.Fatalf("a cold-cache load ran %d builds, want none", len(daemon.builds))
	}

	if _, err := docker.Build(t.Context(), build); err != nil {
		t.Fatalf("Build: %v", err)
	}
	image, err := docker.LoadCached(t.Context(), build)
	if err != nil {
		t.Fatalf("LoadCached after Build: %v", err)
	}
	if !image.Cached {
		t.Fatalf("a load answered cached=%v", image.Cached)
	}
	if daemon.tags[build.Tag] != image.Reference {
		t.Fatalf("the shared tag names %s, want the loaded %s", daemon.tags[build.Tag], image.Reference)
	}
	if daemon.countVerb("image", "load") != 1 {
		t.Fatalf("ran %d loads, want 1", daemon.countVerb("image", "load"))
	}
	// The one recipe build Build ran, and the relabel FROM the loaded image.
	if len(daemon.builds) != 2 {
		t.Fatalf("ran %d builds, want the recipe's and the load's relabel", len(daemon.builds))
	}
}

// VALIDATES: an image that is not kept in the cache is refused, not built.
func TestLoadCachedRefusesAnUncachedImage(t *testing.T) {
	daemon := newCacheDaemon(t)
	docker := daemon.docker(t.TempDir())
	build := cachedBuild(cacheContext(t), nil)
	build.Cache = false
	if _, err := docker.LoadCached(t.Context(), build); err == nil {
		t.Fatal("an image outside the cache was answered")
	}
	if len(daemon.builds) != 0 {
		t.Fatalf("ran %d builds, want none", len(daemon.builds))
	}
}
