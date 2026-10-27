// Design: docs/contributing/gh-pages.md -- measured README carriers from verified recordings.
package siteterminaldemo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// HeroDemo is the one recording selected for the homepage and README.
// Select it after its manifest entry exists and before recording: this source
// participates in recording digests. Publication still requires verified media.
const HeroDemo = "showcase"

// CarrierReport contains measured local artifacts, not a GitHub rendering verdict.
type CarrierReport struct {
	Demo             string                   `json:"demo"`
	Title            string                   `json:"title"`
	Description      string                   `json:"description"`
	Destination      string                   `json:"destination"`
	CastSHA256       string                   `json:"cast-sha256"`
	Duration         float64                  `json:"duration-seconds"`
	FPS              int                      `json:"frames-per-second"`
	Width            int                      `json:"width"`
	Height           int                      `json:"height"`
	ThumbnailSeconds float64                  `json:"thumbnail-seconds"`
	Renderer         json.RawMessage          `json:"renderer"`
	Video            json.RawMessage          `json:"video"`
	Assets           map[string]AssetMetadata `json:"assets"`
	Observation      string                   `json:"github-observation"`
	Choice           string                   `json:"choice"`
	Probe            *CarrierProbe            `json:"probe,omitempty"`
}

func runReadmeCarrier(args leaction.Arguments) (any, int) {
	return signalRooted(func(ctx context.Context) (any, int) {
		engine, _, err := renderEngine(ctx, false)
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		var report CarrierReport
		err = engine.withLock(func() error {
			if probe := args.One("probe"); probe != "" {
				return engine.updateReadmeCarrier(args.One("output"), probe, &report)
			}
			return engine.prepareReadmeCarrier(args.One("output"), &report)
		})
		if err != nil {
			leaction.ReportError(err)
			return report, 1
		}
		return report, 0
	})
}

// verifiedHero checks both recording freshness and the exact owned asset paths.
// Callers MUST hold the engine lock while consuming its artifact inventory.
func (e *Engine) verifiedHero() (Demo, ArtifactEntry, Renderer, error) {
	manifest, indexed, err := e.loadManifest()
	if err != nil {
		return Demo{}, ArtifactEntry{}, Renderer{}, err
	}
	demo, exists := indexed[HeroDemo]
	if !exists {
		return Demo{}, ArtifactEntry{}, Renderer{}, fmt.Errorf("README hero %q has no manifest entry", HeroDemo)
	}
	if demo.Kind != terminalLabel {
		return Demo{}, ArtifactEntry{}, Renderer{}, fmt.Errorf("README hero %s must be a terminal recording", HeroDemo)
	}
	generated, err := e.loadArtifactManifest(manifest)
	if err != nil {
		return Demo{}, ArtifactEntry{}, Renderer{}, err
	}
	entry, exists := generated.Demos[HeroDemo]
	if !exists {
		return Demo{}, ArtifactEntry{}, Renderer{}, fmt.Errorf("README hero %s has not been recorded", HeroDemo)
	}
	expected, err := e.assetPaths(demo)
	if err != nil {
		return Demo{}, ArtifactEntry{}, Renderer{}, err
	}
	for kind, asset := range entry.Assets {
		if asset.Path != filepath.Base(expected[kind]) {
			return Demo{}, ArtifactEntry{}, Renderer{}, fmt.Errorf("README hero %s has unsafe %s asset %q", HeroDemo, kind, asset.Path)
		}
	}
	if err := e.verifyAssets(manifest, indexed, []string{HeroDemo}, "", false); err != nil {
		return Demo{}, ArtifactEntry{}, Renderer{}, err
	}
	return demo, entry, manifest.Renderer, nil
}

func (e *Engine) prepareReadmeCarrier(output string, report *CarrierReport) error {
	demo, entry, renderer, err := e.verifiedHero()
	if err != nil {
		return err
	}
	if err := e.requireRendererImage(Manifest{Renderer: renderer}); err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	// A fresh output directory prevents overwriting a previous probe or foreign files.
	if err := os.Mkdir(output, 0o750); err != nil {
		return fmt.Errorf("create fresh carrier directory: %w", err)
	}
	cast := entry.Assets["cast"]
	if err := copyRootAsset(e.artifactRoot, cast.Path, filepath.Join(output, "hero.cast")); err != nil {
		return err
	}
	*report = CarrierReport{Demo: demo.ID, Title: demo.Title, Description: demo.Description,
		Destination: "https://ze-software.net/demos/terminal/#" + demo.Anchor, CastSHA256: cast.SHA256,
		Observation: "unobserved", Choice: "unselected"}
	if err := writeCarrierJSON(filepath.Join(output, "input.json"), report); err != nil {
		return err
	}
	root, err := filepath.Abs(e.root)
	if err != nil {
		return err
	}
	args := []string{"--entrypoint", "node", "--user", strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())}
	if !strings.HasSuffix(renderer.Platform, "/native") {
		args = append(args, "--platform", renderer.Platform)
	}
	result, err := e.docker.RunOneShot(e.ctx, interoplab.OneShotContainer{
		Image: renderer.Image, Network: "none", Arguments: args,
		Mounts:  []interoplab.Mount{{Source: root, Target: "/source", ReadOnly: true}, {Source: output, Target: "/output"}},
		Command: []string{"/source/demos/terminal/readme-carrier.cjs"}, Timeout: demoContainerTimeout,
	})
	// RunOneShot has stopped the container before returning. Forced removal
	// bypasses the JavaScript finally, so the host MUST release its frames too.
	cleanupErr := cleanupCarrierFrames(output)
	if err != nil {
		return errors.Join(fmt.Errorf("render README candidates: %w: %s", err, result.Stderr), cleanupErr)
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	if err := readCarrierJSON(filepath.Join(output, "carrier.json"), report); err != nil {
		return err
	}
	return verifyCarrierFiles(output, report)
}

func cleanupCarrierFrames(output string) error {
	root, err := os.OpenRoot(output)
	if err != nil {
		return fmt.Errorf("open carrier output for frame cleanup: %w", err)
	}
	defer root.Close() //nolint:errcheck // Frame removal owns the verdict.
	info, err := root.Lstat("frames")
	if errors.Is(err, os.ErrNotExist) {
		// JavaScript already removed the frames, or failed before creating them.
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect carrier frames: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("carrier frames is not an owned directory; refusing cleanup")
	}
	if err := root.RemoveAll("frames"); err != nil {
		return fmt.Errorf("remove carrier frames: %w", err)
	}
	return nil
}

func copyRootAsset(rootPath, name, destination string) error {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // The copy result owns the verdict.
	input, err := root.Open(name)
	if err != nil {
		return err
	}
	defer input.Close()                                                               //nolint:errcheck // The copy result owns the verdict.
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) // #nosec G302 -- Public README media.
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func readCarrierJSON(name string, value any) error {
	data, err := readRootFile(filepath.Dir(name), filepath.Base(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func writeCarrierJSON(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(name, append(data, '\n'), 0o644) // #nosec G306 -- Public carrier measurements.
}

func verifyCarrierFiles(output string, report *CarrierReport) error {
	if report.Duration <= 0 {
		return fmt.Errorf("carrier duration must be measured and positive")
	}
	if report.Width <= 0 {
		return fmt.Errorf("carrier width must be measured and positive")
	}
	if report.Height <= 0 {
		return fmt.Errorf("carrier height must be measured and positive")
	}
	for kind, name := range map[string]string{"svg": "hero.svg", "video": "hero.mp4", "thumbnail": "hero.png"} {
		asset, exists := report.Assets[kind]
		if !exists {
			return fmt.Errorf("carrier is missing %s metadata", kind)
		}
		if asset.Path != name {
			return fmt.Errorf("unsafe %s carrier path %q", kind, asset.Path)
		}
		info, err := os.Stat(filepath.Join(output, name))
		if err != nil {
			return err
		}
		if info.Size() != asset.Bytes {
			return fmt.Errorf("%s carrier size changed", kind)
		}
		if asset.Bytes <= 0 {
			return fmt.Errorf("%s carrier is empty", kind)
		}
		digest, err := hashFile(filepath.Join(output, name))
		if err != nil {
			return err
		}
		if digest != asset.SHA256 {
			return fmt.Errorf("%s carrier digest changed", kind)
		}
	}
	return nil
}
