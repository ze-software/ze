// Design: docs/architecture/appliance/docker-host.md -- a test boots the cached image and never builds one
// Related: build.go -- builder.cachedImage, imageEntry

package dockerhost

import (
	"context"
	"os"
	"strings"
	"testing"
)

// VALIDATES: the lookup a test or a lab uses never builds the image: on a cold
// cache it refuses naming the image's path and, on its last line, the one
// command that builds it; after a build it answers that same image.
// PREVENTS: a test that builds a docker-host image inside its own deadline,
// where a kill discards the build and the next run builds the same key again.
func TestDockerHostCachedImageRefusesAColdImage(t *testing.T) {
	recorder := newRecordingBuilder(t)
	req := buildRequest{repo: fakeRepo(t), arch: "arm64", pins: fullPins(t), sizeGiB: imageSizeGiBDefault}

	_, err := recorder.cachedImage(req)
	if err == nil {
		t.Fatal("a cold image cache was answered, want a refusal")
	}
	dir, _, keyErr := recorder.imageEntry(req)
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	message := err.Error()
	if !strings.Contains(message, dir) {
		t.Errorf("refusal does not name the image's cache entry %s:\n%s", dir, message)
	}
	lines := strings.Split(message, "\n")
	if want := "fix: ./le setup docker host build arch arm64"; lines[len(lines)-1] != want {
		t.Errorf("refusal's last line = %q, want %q", lines[len(lines)-1], want)
	}
	if len(recorder.containerRuns) != 0 {
		t.Errorf("the lookup ran %d builder containers, want none", len(recorder.containerRuns))
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Errorf("the lookup created the image's cache entry %s: %v", dir, statErr)
	}

	built, err := recorder.build(context.Background(), req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	image, err := recorder.cachedImage(req)
	if err != nil {
		t.Fatalf("a warm image cache was refused: %v", err)
	}
	if image != built.Path {
		t.Errorf("warm lookup = %q, want the built image %q", image, built.Path)
	}
}
