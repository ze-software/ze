// Design: docs/contributing/gh-pages.md -- measured carrier and observation boundaries.
package siteterminaldemo

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCarrierFilesRejectUnsafePathsAndChangedBytes checks the artifact admission boundary.
func TestCarrierFilesRejectUnsafePathsAndChangedBytes(t *testing.T) {
	root := t.TempDir()
	report := CarrierReport{Duration: 2, Width: 640, Height: 480, Assets: make(map[string]AssetMetadata)}
	for kind, name := range map[string]string{"svg": "hero.svg", "video": "hero.mp4", "thumbnail": "hero.png"} {
		data := []byte("candidate-" + kind)
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		report.Assets[kind] = AssetMetadata{Path: name, Bytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}
	}
	if err := verifyCarrierFiles(root, &report); err != nil {
		t.Fatal(err)
	}
	original := report.Assets["svg"]
	for _, path := range []string{"../hero.svg", "/hero.svg", "nested/hero.svg", "hero.png", ""} {
		asset := original
		asset.Path = path
		report.Assets["svg"] = asset
		if err := verifyCarrierFiles(root, &report); err == nil {
			t.Fatalf("unsafe carrier path accepted: %q", path)
		}
	}
	report.Assets["svg"] = original
	if err := os.WriteFile(filepath.Join(root, "hero.svg"), []byte(strings.Repeat("x", int(original.Bytes))), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCarrierFiles(root, &report); err == nil {
		t.Fatal("same-size carrier mutation accepted")
	}
}

// TestCarrierProbeRequiresObservedCandidates prevents an unknown GitHub result from selecting a fallback.
func TestCarrierProbeRequiresObservedCandidates(t *testing.T) {
	report := CarrierReport{Assets: map[string]AssetMetadata{
		"svg": {SHA256: "svg", Bytes: 11}, "video": {SHA256: "video", Bytes: 12}, "thumbnail": {SHA256: "thumbnail", Bytes: 13},
	}}
	observation := func(hash string) CarrierObservation {
		return CarrierObservation{SHA256: hash, Result: "fail", Evidence: "Retained README browser observation", LimitBytes: 100,
			LimitSource: "https://docs.github.com/en/get-started/writing-on-github/working-with-advanced-formatting/attaching-files"}
	}
	probe := CarrierProbe{ObservedAt: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), SVG: observation("svg"), Video: observation("video"), Thumbnail: observation("thumbnail")}
	probe.Thumbnail.Result = "pass"
	if got, err := selectCarrier(&report, &probe); err != nil || got != "thumbnail" {
		t.Fatalf("observed fallback: %q, %v", got, err)
	}
	probe.SVG.Result = "pass"
	if got, err := selectCarrier(&report, &probe); err != nil || got != "svg" {
		t.Fatalf("observed SVG: %q, %v", got, err)
	}
	probe.SVG.LimitBytes = 10
	if _, err := selectCarrier(&report, &probe); err == nil {
		t.Fatal("oversized successful SVG accepted")
	}
	probe.SVG.LimitBytes = 11
	if _, err := selectCarrier(&report, &probe); err != nil {
		t.Fatalf("exact size limit refused: %v", err)
	}
	probe.SVG.Result = "unobserved"
	if _, err := selectCarrier(&report, &probe); err == nil {
		t.Fatal("unobserved SVG silently selected thumbnail")
	}
	probe.SVG.Result = "fail"
	probe.Video.Result = "pass"
	probe.Video.URL = "https://example.com/video.mp4"
	if _, err := selectCarrier(&report, &probe); err == nil {
		t.Fatal("non-GitHub video treated as GitHub-playable")
	}
	probe.Video.URL = "https://github.com/user-attachments/assets/observed-id"
	if got, err := selectCarrier(&report, &probe); err != nil || got != "video" {
		t.Fatalf("observed video: %q, %v", got, err)
	}
	probe.Video.SHA256 = "other-recording"
	if _, err := selectCarrier(&report, &probe); err == nil {
		t.Fatal("probe for a different artifact accepted")
	}
}

// TestReadmeHeroReplacementPreservesUnownedContent checks marker confinement and malformed input.
func TestReadmeHeroReplacementPreservesUnownedContent(t *testing.T) {
	original := "before\n<!-- terminal-demo-hero:start -->\nold\n<!-- terminal-demo-hero:end -->\nafter\n"
	got, err := replaceReadmeHero(original, "new\n")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(original, "old", "new", 1)
	if got != want {
		t.Fatalf("unowned README changed: %q", got)
	}
	for _, input := range []string{"no markers", original + "<!-- terminal-demo-hero:start -->", "<!-- terminal-demo-hero:end --><!-- terminal-demo-hero:start -->"} {
		if _, err := replaceReadmeHero(input, "new"); err == nil {
			t.Fatalf("malformed markers accepted: %q", input)
		}
	}
}

// TestCarrierFilesRefusesEscapingSymlink protects the input media root.
func TestCarrierFilesRefusesEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.svg")
	data := []byte("outside")
	if err := os.WriteFile(outside, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	report := CarrierReport{Duration: 1, Width: 1, Height: 1, Assets: make(map[string]AssetMetadata)}
	for kind, name := range map[string]string{"svg": "hero.svg", "video": "hero.mp4", "thumbnail": "hero.png"} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
		report.Assets[kind] = AssetMetadata{Path: name, Bytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}
	}
	if err := verifyCarrierFiles(root, &report); err != nil {
		t.Fatal("valid fixture refused:", err)
	}
	if err := os.Remove(filepath.Join(root, "hero.svg")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "hero.svg")); err != nil {
		t.Fatal(err)
	}
	if err := verifyCarrierFiles(root, &report); err == nil {
		t.Fatal("escaping media symlink accepted")
	}
}
