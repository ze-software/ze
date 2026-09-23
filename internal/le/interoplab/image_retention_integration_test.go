//go:build integration

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

// Replacing a shared tag must not change or remove an image that a run selected.
func TestDockerBuildRetainsImageAcrossRetag(t *testing.T) {
	if _, err := exec.LookPath(dockerExecutable); err != nil {
		t.Skip("Docker is required for the image-retention integration test")
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
		if err := docker.releaseImage(context.Background(), image); err != nil {
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
		result, err := docker.command(ctx, dockerCommandTimeout,
			dockerExecutable, "run", "--rm", "--network", "none", images[index].Reference)
		if err != nil {
			t.Fatalf("run selected %s image after retag: %v", marker, err)
		}
		if strings.TrimSpace(result.Stdout) != marker {
			t.Fatalf("selected %s image produced %q", marker, result.Stdout)
		}
	}
	for _, image := range images {
		if err := docker.releaseImage(ctx, image); err != nil {
			t.Fatal(err)
		}
		_, err := docker.command(ctx, dockerCommandTimeout,
			dockerExecutable, "image", "inspect", image.retainedTag)
		var commandErr *commandError
		if !errors.As(err, &commandErr) || !strings.Contains(commandErr.Stderr, "No such image: "+image.retainedTag) {
			t.Fatalf("released tag remains or inspection failed for another reason: %v", err)
		}
	}
	result, err := docker.command(ctx, dockerCommandTimeout,
		dockerExecutable, "run", "--rm", "--network", "none", cacheTag)
	if err != nil || strings.TrimSpace(result.Stdout) != "second" {
		t.Fatalf("releasing run-owned tags changed the shared cache image: stdout=%q error=%v", result.Stdout, err)
	}

	// A later setup failure must release an earlier successful build as well.
	report := (Suite{
		Docker: docker,
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
		dockerExecutable, "image", "inspect", report.Images[0].retainedTag)
	var commandErr *commandError
	if !errors.As(err, &commandErr) || !strings.Contains(commandErr.Stderr, "No such image: "+report.Images[0].retainedTag) {
		t.Fatalf("failed suite did not release its image tag: %v", err)
	}
}
