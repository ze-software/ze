//go:build docker

// Design: docs/architecture/testing/interop.md -- run-owned image retention.
package interoplab

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// VALIDATES: AC-3 and AC-14: replacing a shared tag does not change or remove
// an image that a run selected, and a failed suite setup releases the tag of
// an earlier successful build. Method: two builds onto one cache tag, each run
// through RunOneShot by its retained reference, against the real daemon.
// PREVENTS: a retag or a release removing an image another run still uses.
// Tagged docker so `./le test integration docker-peers` runs it with
// ZE_REQUIRE_DOCKER=1, where an absent docker binary fails rather than skips.
func TestDockerBuildRetainsImageAcrossRetag(t *testing.T) {
	if _, err := exec.LookPath(DockerExecutable); err != nil {
		if os.Getenv("ZE_REQUIRE_DOCKER") == "1" {
			t.Fatalf("docker is not installed, and ZE_REQUIRE_DOCKER=1 says this run must exercise it. Fix: install Docker")
		}
		t.Skip("docker is not installed; install Docker to run this test")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	docker := NewDocker()
	if err := docker.Probe(ctx); err != nil {
		t.Fatal(err)
	}
	cacheTag := "ze-interop-retention-test:" + rand.Text()
	cleanup := func(image ImageResult) {
		t.Helper()
		if err := docker.ReleaseImage(context.Background(), image); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { cleanup(ImageResult{retainedTag: cacheTag}) })
	directory := t.TempDir()
	dockerfile := filepath.Join(directory, "Dockerfile")
	var images []ImageResult
	for _, marker := range []string{"first", "second"} {
		content := "FROM alpine:3.21\nCMD [\"echo\", \"" + marker + "\"]\n"
		if err := os.WriteFile(dockerfile, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		image, err := docker.Build(ctx, ImageBuild{
			Name: marker, Tag: cacheTag, Dockerfile: dockerfile, Context: directory,
			Timeout: time.Minute, Required: true,
		})
		t.Cleanup(func() { cleanup(image) })
		if err != nil {
			t.Fatal(err)
		}
		images = append(images, image)
	}
	for index, marker := range []string{"first", "second"} {
		result, err := docker.RunOneShot(ctx, OneShotContainer{
			Image: images[index].Reference, Arguments: []string{"--network", "none"}, Timeout: dockerCommandTimeout,
		})
		if err != nil {
			t.Fatalf("run selected %s image after retag: %v", marker, err)
		}
		if strings.TrimSpace(result.Stdout) != marker {
			t.Fatalf("selected %s image produced %q", marker, result.Stdout)
		}
	}
	for _, image := range images {
		if err := docker.ReleaseImage(ctx, image); err != nil {
			t.Fatal(err)
		}
		_, err := docker.command(ctx, dockerCommandTimeout,
			DockerExecutable, "image", "inspect", image.retainedTag)
		var commandErr *commandError
		if !errors.As(err, &commandErr) || !strings.Contains(commandErr.Stderr, "No such image: "+image.retainedTag) {
			t.Fatalf("released tag remains or inspection failed for another reason: %v", err)
		}
	}
	result, err := docker.RunOneShot(ctx, OneShotContainer{
		Image: cacheTag, Arguments: []string{"--network", "none"}, Timeout: dockerCommandTimeout,
	})
	if err != nil || strings.TrimSpace(result.Stdout) != "second" {
		t.Fatalf("releasing run-owned tags changed the shared cache image: stdout=%q error=%v", result.Stdout, err)
	}

	// A later setup failure must release an earlier successful build as well.
	// The suite runs no Ze, so a stand-in answers the kernel check Suite.Run
	// makes before the first build; it sits in the checkout so a Docker VM that
	// mounts only the home directory sees it.
	standIn, err := filepath.Abs(filepath.Join("testdata", "kernel-all-present", "ze"))
	if err != nil {
		t.Fatal(err)
	}
	report := (Suite{
		Docker:   docker,
		StagedZe: standIn,
		Images: []ImageBuild{
			{Name: "probe", Tag: cacheTag, Dockerfile: dockerfile, Context: directory, Required: true},
			{Name: "invalid-without-tag"},
		},
		Scenarios: []ScenarioPlan{{Source: ScenarioSource{Name: "unreached"}}},
	}).Run(ctx)
	if report.Code != 1 || report.SetupError == "" || len(report.Images) != 1 || len(report.CleanupErrors) != 0 {
		t.Fatalf("failed setup did not retain its result and complete cleanup: %+v", report)
	}
	_, err = docker.command(ctx, dockerCommandTimeout,
		DockerExecutable, "image", "inspect", report.Images[0].retainedTag)
	var commandErr *commandError
	if !errors.As(err, &commandErr) || !strings.Contains(commandErr.Stderr, "No such image: "+report.Images[0].retainedTag) {
		t.Fatalf("failed suite did not release its image tag: %v", err)
	}
}
