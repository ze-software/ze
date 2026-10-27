//go:build docker

// Design: docs/architecture/testing/interop.md -- streamed build identity and ownership.
package interoplab

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDockerBuildStreamsProgressAndRetainsIdentity builds two scratch images on
// the real daemon. Retagging must not change the first image's measured label;
// releasing its owned tag must leave the second image under the cache tag.
func TestDockerBuildStreamsProgressAndRetainsIdentity(t *testing.T) {
	if _, err := exec.LookPath(DockerExecutable); err != nil {
		if os.Getenv("ZE_REQUIRE_DOCKER") == "1" {
			t.Fatal("Docker is required; install Docker before running this proof")
		}
		t.Skip("Docker is not installed")
	}
	ctx := testDeadlineContext(t)
	docker := NewDocker()
	if err := docker.Probe(ctx); err != nil {
		t.Fatal(err)
	}
	cacheTag := "ze-progress-test:" + strings.ToLower(rand.Text())
	cleanup := func(image ImageResult) {
		t.Helper()
		if err := docker.ReleaseImage(context.WithoutCancel(ctx), image); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { cleanup(ImageResult{retainedTag: cacheTag}) })
	directory := t.TempDir()
	dockerfile := filepath.Join(directory, "Dockerfile")
	var output bytes.Buffer
	var images []ImageResult
	for _, marker := range []string{"first", "second"} {
		if err := os.WriteFile(dockerfile, []byte("FROM scratch\nLABEL ze.progress.marker="+marker+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		image, err := docker.Build(ctx, ImageBuild{
			Tag: cacheTag, Dockerfile: dockerfile, Context: directory, Progress: &output,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cleanup(image) })
		images = append(images, image)
	}
	if output.Len() == 0 {
		t.Fatal("real Docker builds emitted no progress")
	}
	if images[0].Reference == images[1].Reference {
		t.Fatal("two different image configurations answered the same immutable identity")
	}
	markerOf := func(reference string) string {
		t.Helper()
		result, err := docker.command(ctx, dockerCommandTimeout, DockerExecutable,
			dockerImageObject, dockerInspectVerb, dockerFormatFlag, `{{index .Config.Labels "ze.progress.marker"}}`, reference)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(result.Stdout)
	}
	if marker := markerOf(images[0].Reference); marker != "first" {
		t.Fatalf("retag changed first immutable image: marker=%q", marker)
	}
	for _, image := range images {
		if err := docker.ReleaseImage(ctx, image); err != nil {
			t.Fatal(err)
		}
		present, err := docker.ImagePresent(ctx, image.retainedTag)
		if err != nil || present {
			t.Fatalf("owned tag survives release: present=%v error=%v", present, err)
		}
	}
	if marker := markerOf(cacheTag); marker != "second" {
		t.Fatalf("owned-tag release changed cache image: marker=%q", marker)
	}
}
