//go:build docker

// Design: docs/contributing/gh-pages.md -- renderer image construction and tag ownership.
package siteterminaldemo

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestBuildImagePublishesManifestImageAndReleasesOwnedTag drives Engine.BuildImage
// against real Docker with a minimal, uniquely labeled scratch Dockerfile. It
// observes the published image and its tags, not the command list used to build it.
func TestBuildImagePublishesManifestImageAndReleasesOwnedTag(t *testing.T) {
	if _, err := exec.LookPath(dockerProgram); err != nil {
		if os.Getenv("ZE_REQUIRE_DOCKER") == "1" {
			t.Fatal("Docker is required; install Docker before running this proof")
		}
		t.Skip("Docker is not installed")
	}
	fixture := newDemoFixture(t)
	var output bytes.Buffer
	engine := fixture.engine(&output)
	docker := interoplab.NewDocker()
	engine.docker = docker
	engine.lookup = exec.LookPath
	engine.ctx = t.Context()
	manifest, _, err := engine.loadManifest()
	if err != nil {
		t.Fatal(err)
	}
	marker := strings.ToLower(rand.Text())
	manifest.Renderer.Image = "ze-renderer-progress-test:" + marker
	writeJSON(t, filepath.Join(engine.demoRoot, "manifest.json"), manifest)
	mustWrite(t, filepath.Join(engine.demoRoot, "Dockerfile"), "FROM scratch\n"+
		"ARG FRR_VERSION\nARG KEEPALIVED_VERSION\nARG KEEPALIVED_SHA256\n"+
		"LABEL ze.renderer.frr=$FRR_VERSION ze.renderer.keepalived=$KEEPALIVED_VERSION "+
		"ze.renderer.keepalived.sha256=$KEEPALIVED_SHA256 ze.renderer.marker="+marker+"\n")
	// This unique manifest tag belongs to the test. Never remove the shared
	// renderer image in the checkout's actual manifest or another run's tags.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 30*time.Second)
		defer cancel()
		if err := docker.RemoveImageTagForTest(ctx, manifest.Renderer.Image); err != nil {
			t.Errorf("release test's manifest tag: %v", err)
			return
		}
		if _, err := docker.InspectImage(ctx, manifest.Renderer.Image); !errors.Is(err, interoplab.ErrImageAbsent) {
			t.Fatalf("released manifest tag still inspects: %v", err)
		}
	})
	// The daemon's answer for a tag nobody built is absence, not a failure,
	// which is what lets the render preflight name the build action.
	if _, err := docker.InspectImage(t.Context(), manifest.Renderer.Image); !errors.Is(err, interoplab.ErrImageAbsent) {
		t.Fatalf("unbuilt manifest tag inspected as %v, want ErrImageAbsent", err)
	}
	report, err := engine.BuildImage()
	if err != nil {
		t.Fatal(err)
	}
	if report.Mode != rendererImageBuildMode {
		t.Fatalf("report mode = %q", report.Mode)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	image, err := docker.InspectImage(ctx, manifest.Renderer.Image)
	if err != nil {
		t.Fatalf("manifest tag was not published: %v", err)
	}
	if image.Labels["ze.renderer.marker"] != marker {
		t.Fatalf("published image did not come from the demo Dockerfile: %+v", image.Labels)
	}
	arguments, err := rendererPeerArguments(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, argument := range arguments {
		name, want, found := strings.Cut(argument, "=")
		if !found {
			t.Fatalf("peer argument has no value: %q", argument)
		}
		label := rendererPinLabels[name]
		if image.Labels[label] != want {
			t.Errorf("published image label %s = %q, want canonical pin %q", label, image.Labels[label], want)
		}
	}
	// The render preflight reads the same labels: an image this build just
	// published from the checkout's pins is accepted.
	if err := engine.requireRendererImage(manifest); err != nil {
		t.Fatalf("render preflight refused the image this build published: %v", err)
	}
	if !strings.HasPrefix(image.ID, "sha256:") {
		t.Fatalf("published reference is not immutable: %q", image.ID)
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(image.ID, "sha256:"))
	if err != nil || len(digest) != 32 {
		t.Fatalf("published image identity is invalid: %q (%v)", image.ID, err)
	}
	if len(image.RepoTags) != 1 || image.RepoTags[0] != manifest.Renderer.Image {
		t.Fatalf("renderer did not release its owned tag or keep its manifest tag: %v", image.RepoTags)
	}
	if image.Labels["ze.lab.kind"] != "image" {
		t.Fatalf("renderer bypassed Docker owner labels: %+v", image.Labels)
	}
}
