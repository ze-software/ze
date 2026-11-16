// Design: docs/architecture/appliance/docker-host.md -- shared image publication retention

package dockerhost

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDockerHostBuildPreservesSharedEntries builds through the recording
// builder, then reopens each old shared image, static input and staging output.
// Holding the oldest image open models another run without relying on its age
// as a substitute for ownership or continued use.
func TestDockerHostBuildPreservesSharedEntries(t *testing.T) {
	recorder := newRecordingBuilder(t)
	req := buildRequest{repo: fakeRepo(t), arch: "arm64", pins: fullPins(t), sizeGiB: imageSizeGiBDefault}
	images := filepath.Join(recorder.cacheDir, req.arch)
	entries := make(map[string]string)
	for index, name := range []string{"open", "comparison", "previous", "newer", "static-shared", ".copytree-other-run"} {
		path := filepath.Join(images, name, imageFileName)
		want := "shared artifact " + name
		writeFile(t, path, []byte(want))
		modified := time.Now().Add(time.Duration(index-72) * time.Hour)
		if err := os.Chtimes(filepath.Dir(path), modified, modified); err != nil {
			t.Fatal(err)
		}
		entries[path] = want
		if name == "open" {
			open, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := open.Close(); err != nil {
					t.Errorf("close held shared image: %v", err)
				}
			})
		}
	}

	built, err := recorder.build(context.Background(), req)
	if err != nil {
		t.Fatalf("publish image: %v", err)
	}
	if !built.Built {
		t.Fatal("fixture reused an image instead of reaching publication")
	}
	if _, err := os.Stat(built.Path); err != nil {
		t.Errorf("published image cannot be reopened: %v", err)
	}
	for path, want := range entries {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("shared artifact cannot be reopened at %s: %v", path, err)
			continue
		}
		if string(body) != want {
			t.Errorf("shared artifact at %s = %q, want %q", path, body, want)
		}
	}
}
