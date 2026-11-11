//go:build docker

// Design: docs/architecture/testing/interop.md -- the build context is a tar Ze writes.
package interoplab

import (
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// VALIDATES: a lab image builds, against the real daemon, from a context that
// holds a symlink to a directory outside it (the checkout's `cache`) and a
// symlink inside it. Method: Build from a context under t.TempDir, so a run
// with TMPDIR on a 9p share (a QEMU guest on a macOS host) reads the context
// through the share, the route that failed.
// PREVENTS: BuildKit walking the context itself, which on a macOS host's 9p
// share fails "failed to xattr <ctx>/cache: too many levels of symbolic links"
// (plan/journal/container-cannot-see-migrated-scratch.md, 2026-10-21).
func TestDockerBuildSendsAContextHoldingSymlinks(t *testing.T) {
	if _, err := exec.LookPath(DockerExecutable); err != nil {
		if os.Getenv("ZE_REQUIRE_DOCKER") == "1" {
			t.Fatalf("docker is not installed, and ZE_REQUIRE_DOCKER=1 says this run must exercise it. Fix: install Docker")
		}
		t.Skip("docker is not installed; install Docker to run this test")
	}
	ctx := testDeadlineContext(t)
	docker := NewDocker()
	if err := docker.Probe(ctx); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	outside := t.TempDir()
	files := map[string]string{
		"Dockerfile":    "FROM scratch\nCOPY payload /payload\n",
		".dockerignore": "cache/\n",
		"payload":       "payload\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "entry"), []byte("cache entry\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "cache")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("payload", filepath.Join(directory, "link")); err != nil {
		t.Fatal(err)
	}
	tag := "ze-interop-symlink-context-test:" + rand.Text()
	t.Cleanup(func() {
		if err := docker.ReleaseImage(context.Background(), ImageResult{retainedTag: tag}); err != nil {
			t.Error(err)
		}
	})
	image, err := docker.Build(ctx, ImageBuild{
		Name: "symlink-context", Tag: tag,
		Dockerfile: filepath.Join(directory, "Dockerfile"), Context: directory, Required: true,
	})
	t.Cleanup(func() {
		if err := docker.ReleaseImage(context.Background(), image); err != nil {
			t.Error(err)
		}
	})
	if err != nil {
		t.Fatalf("build from a context holding symlinks (context %s): %v", directory, err)
	}
	if _, err := docker.command(ctx, dockerCommandTimeout, DockerExecutable, "image", "inspect", image.retainedTag); err != nil {
		t.Fatalf("the built image is not retained as %s: %v", image.retainedTag, err)
	}
}
