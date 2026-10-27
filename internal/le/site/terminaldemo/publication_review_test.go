// Design: docs/contributing/gh-pages.md -- publication refuses unproven media and ownership.
package siteterminaldemo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// carrierReviewFixture supplies a fresh, hash-verified hero without invoking a renderer.
func carrierReviewFixture(t *testing.T) (*demoFixture, *Engine) {
	t.Helper()
	fixture := newDemoFixture(t)
	var log bytes.Buffer
	engine := fixture.engine(&log)
	manifest, _, err := engine.loadManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Demos[0].ID = HeroDemo
	writeJSON(t, filepath.Join(engine.demoRoot, "manifest.json"), manifest)
	entry := ArtifactEntry{Assets: make(map[string]AssetMetadata)}
	for kind, extension := range map[string]string{"cast": ".cast", "transcript": ".txt"} {
		name := HeroDemo + extension
		mustWrite(t, filepath.Join(fixture.artifacts, name), "fixture "+kind)
		digest, err := hashFile(filepath.Join(fixture.artifacts, name))
		if err != nil {
			t.Fatal(err)
		}
		entry.Assets[kind] = AssetMetadata{Path: name, Bytes: int64(len("fixture " + kind)), SHA256: digest}
	}
	entry.SourceSHA256, err = engine.sourceDigest(manifest.Demos[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.writeArtifactManifest(artifactManifest{Schema: manifestSchema, Renderer: manifest.Renderer, Demos: map[string]ArtifactEntry{HeroDemo: entry}}); err != nil {
		t.Fatal(err)
	}
	return fixture, engine
}

// TestVideoCarrierDoesNotPublishRejectedThumbnail exercises selection through README and image publication.
func TestVideoCarrierDoesNotPublishRejectedThumbnail(t *testing.T) {
	fixture, engine := carrierReviewFixture(t)
	mustMkdir(t, filepath.Join(fixture.root, "docs", "demos"))
	mustWrite(t, filepath.Join(fixture.root, "README.md"), "before\n<!-- terminal-demo-hero:start -->\nold\n<!-- terminal-demo-hero:end -->\nafter\n")
	demo, entry, _, err := engine.verifiedHero()
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	report := CarrierReport{Demo: demo.ID, CastSHA256: entry.Assets["cast"].SHA256, Duration: 1, Width: 640, Height: 480, Assets: make(map[string]AssetMetadata)}
	for kind, name := range map[string]string{"svg": "hero.svg", "video": "hero.mp4", "thumbnail": "hero.png"} {
		mustWrite(t, filepath.Join(output, name), kind)
		digest, err := hashFile(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}
		report.Assets[kind] = AssetMetadata{Path: name, Bytes: int64(len(kind)), SHA256: digest}
	}
	writeJSON(t, filepath.Join(output, "carrier.json"), report)
	observation := func(kind, result string) CarrierObservation {
		return CarrierObservation{SHA256: report.Assets[kind].SHA256, Result: result, Evidence: "retained fixture observation", LimitBytes: 100,
			LimitSource: "https://docs.github.com/en/get-started/writing-on-github/working-with-advanced-formatting/attaching-files"}
	}
	probe := CarrierProbe{ObservedAt: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), SVG: observation("svg", "fail"), Video: observation("video", "pass"), Thumbnail: observation("thumbnail", "fail")}
	probe.Video.URL = "https://github.com/user-attachments/assets/observed-video"
	probePath := filepath.Join(output, "probe.json")
	writeJSON(t, probePath, probe)
	if err := engine.updateReadmeCarrier(output, probePath, &report); err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(fixture.root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), probe.Video.URL) {
		t.Fatal("passing video was not published")
	}
	if !strings.Contains(string(readme), "[Watch the recording and read its transcript]("+report.Destination+")") {
		t.Fatal("textual player destination was lost")
	}
	if strings.Contains(string(readme), "[![") {
		t.Fatal("video layout embeds a rejected image")
	}
	if _, err := os.Stat(filepath.Join(fixture.root, "docs", "demos", HeroDemo+"-readme.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected thumbnail was published: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "hero.png")); err != nil {
		t.Fatal("local candidate was removed:", err)
	}
}

// TestRetirementPreflightsRetainedAliases rejects uncertain ownership before any file changes.
func TestRetirementPreflightsRetainedAliases(t *testing.T) {
	for _, kind := range []string{"dot-path", "symlink", "directory-symlink", "missing", "escaping-symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			for _, name := range []string{"old.cast", "old.txt", "foreign.webm"} {
				mustWrite(t, filepath.Join(root, name), "keep "+name)
			}
			mustWrite(t, filepath.Join(outside, "outside.cast"), "outside")
			retained := "./old.cast"
			switch kind {
			case "dot-path":
			case "symlink":
				retained = "live.cast"
				if err := os.Symlink("old.cast", filepath.Join(root, retained)); err != nil {
					t.Fatal(err)
				}
			case "directory-symlink":
				retained = "alias/old.cast"
				if err := os.Symlink(".", filepath.Join(root, "alias")); err != nil {
					t.Fatal(err)
				}
			case "missing":
				retained = "missing.cast"
			case "escaping-symlink":
				retained = "live.cast"
				if err := os.Symlink(filepath.Join(outside, "outside.cast"), filepath.Join(root, retained)); err != nil {
					t.Fatal(err)
				}
			}
			generated := artifactManifest{Demos: map[string]ArtifactEntry{
				"old":  {Assets: map[string]AssetMetadata{"cast": {Path: "old.cast"}, "transcript": {Path: "old.txt"}}},
				"live": {Assets: map[string]AssetMetadata{"cast": {Path: retained}}},
			}}
			engine := New(Options{Root: root, ArtifactRoot: root})
			if err := engine.retireDeletedAssets(&generated, map[string]Demo{"live": {ID: "live"}}); err == nil {
				t.Fatal("uncertain retained ownership admitted")
			}
			if len(generated.Demos) != 2 {
				t.Fatal("inventory partially retired")
			}
			for _, name := range []string{"old.cast", "old.txt", "foreign.webm"} {
				data, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || string(data) != "keep "+name {
					t.Fatalf("%s changed: %q, %v", name, data, err)
				}
			}
			data, err := os.ReadFile(filepath.Join(outside, "outside.cast"))
			if err != nil || string(data) != "outside" {
				t.Fatalf("outside changed: %q, %v", data, err)
			}
		})
	}
}

type carrierStoppedRenderer struct {
	fixture *demoFixture
	t       *testing.T
	output  string
	failure error
}

func (renderer *carrierStoppedRenderer) Run(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	if len(command.Arguments) > 1 {
		if command.Arguments[1] == "run" {
			mustWrite(renderer.t, filepath.Join(renderer.output, "frames", "000000.png"), "temporary frame")
			mustWrite(renderer.t, filepath.Join(renderer.output, "hero.mp4"), "partial candidate")
			mustWrite(renderer.t, filepath.Join(renderer.output, "diagnostic.txt"), "renderer diagnostic")
			return interoplab.ProcessResult{ExitCode: 137, Stderr: "renderer stopped"}, renderer.failure
		}
	}
	return renderer.fixture.Run(ctx, command)
}

// TestCarrierHostCleansFramesAfterRendererStops reaches cleanup through conversion failure and cancellation.
func TestCarrierHostCleansFramesAfterRendererStops(t *testing.T) {
	for _, failure := range []error{context.Canceled, errors.New("renderer failed")} {
		t.Run(failure.Error(), func(t *testing.T) {
			fixture, engine := carrierReviewFixture(t)
			output := filepath.Join(t.TempDir(), "candidates")
			engine.docker = interoplab.NewFixtureDocker(&carrierStoppedRenderer{fixture: fixture, t: t, output: output, failure: failure})
			var report CarrierReport
			if err := engine.prepareReadmeCarrier(output, &report); err == nil {
				t.Fatal("failed conversion accepted")
			}
			if _, err := os.Stat(filepath.Join(output, "frames")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("frames survived: %v", err)
			}
			for name, want := range map[string]string{"hero.mp4": "partial candidate", "diagnostic.txt": "renderer diagnostic"} {
				data, err := os.ReadFile(filepath.Join(output, name))
				if err != nil || string(data) != want {
					t.Fatalf("%s changed: %q, %v", name, data, err)
				}
			}
		})
	}
}

// TestCarrierFrameCleanupRejectsSymlinkRoot never follows a replacement frames directory outside its root.
func TestCarrierFrameCleanupRejectsSymlinkRoot(t *testing.T) {
	output := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "keep.png"), "outside")
	if err := os.Symlink(outside, filepath.Join(output, "frames")); err != nil {
		t.Fatal(err)
	}
	if err := cleanupCarrierFrames(output); err == nil {
		t.Fatal("symlink frames directory admitted")
	}
	data, err := os.ReadFile(filepath.Join(outside, "keep.png"))
	if err != nil || string(data) != "outside" {
		t.Fatalf("outside frame changed: %q, %v", data, err)
	}
}

// TestCarrierFrameCleanupDoesNotFollowChildSymlinks removes only frame entries and is idempotent.
func TestCarrierFrameCleanupDoesNotFollowChildSymlinks(t *testing.T) {
	output := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(output, "frames", "000000.png"), "frame")
	mustWrite(t, filepath.Join(output, "hero.svg"), "candidate")
	mustWrite(t, filepath.Join(outside, "keep.png"), "outside")
	if err := os.Symlink(outside, filepath.Join(output, "frames", "foreign")); err != nil {
		t.Fatal(err)
	}
	if err := cleanupCarrierFrames(output); err != nil {
		t.Fatal(err)
	}
	if err := cleanupCarrierFrames(output); err != nil {
		t.Fatal("second cleanup:", err)
	}
	for name, want := range map[string]string{
		filepath.Join(output, "hero.svg"):  "candidate",
		filepath.Join(outside, "keep.png"): "outside",
	} {
		data, err := os.ReadFile(name)
		if err != nil || string(data) != want {
			t.Fatalf("%s changed: %q, %v", name, data, err)
		}
	}
}
