//go:build integration

// Design: docs/contributing/terminal-demos.md -- actual deterministic browser posters.
package siteterminaldemo

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/sessionpath"
)

// TestWebConfigPosterUsesAuthoredInputs executes the real login/edit/save/diff/
// confirm/readback journey and CDP WebP producer three times. Equal authored
// inputs must yield equal image bytes; changing only hostname operands must
// change the image. No canonical journey, presentation script or publisher is
// edited, and every run starts and joins its own daemon and browser.
func TestWebConfigPosterUsesAuthoredInputs(t *testing.T) {
	if os.Getenv("ZE_DEMO_POSTER_PROOF_CHILD") != "1" {
		runWebConfigPosterInRenderer(t)
		return
	}
	for _, executable := range []string{"ze", "node", "agent-browser"} {
		if _, err := exec.LookPath(executable); err != nil {
			t.Fatalf("actual poster proof requires %s: %v", executable, err)
		}
	}
	// ZE_DEMO_ROOT is still the container root the parent set; it changes below.
	if err := posterZeIsOwned(demoBinary("ze")); err != nil {
		t.Fatal(err)
	}
	source, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	if !sessionpath.IsRepoRoot(source) {
		t.Fatalf("poster proof requires the main checkout, not %s", source)
	}
	authored, err := os.ReadFile(filepath.Join(source, recordedWebConfigJourney))
	if err != nil {
		t.Fatal(err)
	}
	// The parent explicitly mounts this child's TMPDIR; no host session
	// identity crosses the renderer container boundary.
	scratch := os.TempDir()
	root, err := os.MkdirTemp(scratch, "web-config-poster-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("retained actual poster evidence: %s", root)
			return
		}
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	artifacts := filepath.Join(root, "demos", "terminal", "artifacts")
	if err := os.MkdirAll(artifacts, 0o750); err != nil {
		t.Fatal(err)
	}
	// Link the current producer inputs, never export or copy the checkout. The
	// producer's output and private daemon stores remain under this owned root.
	producerLink := filepath.Join(root, "demos", "terminal", "web-config")
	if err := os.Symlink(filepath.Join(source, "demos", "terminal", "web-config"), producerLink); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(producerLink); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("ZE_DEMO_ROOT", root)
	t.Setenv("ZE_DEMO_STATE_ROOT", filepath.Join(root, "state"))
	capture := func(name, hostname string) []byte {
		t.Helper()
		journey, err := webtesting.ParseWBFile(string(authored))
		if err != nil {
			t.Fatal(err)
		}
		if hostname != "edge-demo" {
			replaced := false
			for index := range journey.Actions {
				action := &journey.Actions[index]
				if action.Kind == "fill" && action.Values["id"] == "field-hostname" {
					if action.Values["value"] != "edge-demo" {
						t.Fatal("canonical hostname fill changed; reassess the negative control")
					}
					action.Values["value"] = hostname
					replaced = true
				}
			}
			if !replaced {
				t.Fatal("canonical journey has no hostname fill for the negative control")
			}
			for index := range journey.Expects {
				for key, value := range journey.Expects[index].Values {
					journey.Expects[index].Values[key] = strings.ReplaceAll(value, "edge-demo", hostname)
				}
			}
		}
		if err := runParsedWebConfigJourney(journey, true); err != nil {
			t.Fatalf("%s actual poster journey: %v", name, err)
		}
		image, err := os.ReadFile(filepath.Join(artifacts, "web-config.webp"))
		if err != nil {
			t.Fatal(err)
		}
		if len(image) < 20 || string(image[:4]) != "RIFF" || string(image[8:12]) != "WEBP" {
			t.Fatalf("%s is not an actual WebP capture", name)
		}
		if int(binary.LittleEndian.Uint32(image[4:8]))+8 != len(image) {
			t.Fatalf("%s WebP container length does not match captured bytes", name)
		}
		// Retain the already-read payload, not a later reopen of the producer's
		// mutable output. Failed comparisons leave all three immutable inputs.
		path := filepath.Join(root, name+".webp")
		if err := os.WriteFile(path, image, 0o400); err != nil {
			t.Fatal(err)
		}
		t.Logf("actual poster capture: %s", path)
		return image
	}
	first := capture("first", "edge-demo")
	second := capture("same-input", "edge-demo")
	if !bytes.Equal(first, second) {
		t.Error("equal authored inputs produced different actual WebP poster bytes")
	}
	changed := capture("hostname-only", "edge-demo-negative")
	if bytes.Equal(first, changed) {
		t.Error("changing only the committed hostname did not change the actual WebP poster")
	}
}

// runWebConfigPosterInRenderer uses the existing pinned renderer and ownership
// contract, not a host daemon or the browser-less QEMU integration guest.
func runWebConfigPosterInRenderer(t *testing.T) {
	t.Helper()
	source, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	if !sessionpath.IsRepoRoot(source) {
		t.Fatalf("poster proof requires the main checkout, not %s", source)
	}
	artifacts, err := os.MkdirTemp(sessionpath.DefaultScratchRoot(), "web-config-poster-renderer-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("retained isolated poster evidence: %s", artifacts)
			return
		}
		if err := os.RemoveAll(artifacts); err != nil {
			t.Error(err)
		}
	})
	ctx := t.Context()
	if deadline, bounded := t.Deadline(); bounded {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline.Add(-runner.TeardownGraceTimeout))
		defer cancel()
	}
	engine := New(Options{Root: source, ArtifactRoot: artifacts, Context: ctx, Output: os.Stdout})
	manifest, indexed, err := engine.loadManifest()
	if err != nil {
		t.Fatal(err)
	}
	demo, ok := indexed["web-config"]
	if !ok {
		t.Fatal("renderer manifest has no web-config demo")
	}
	if err := engine.preflightContainers(manifest); err != nil {
		t.Fatal(err)
	}
	grants, err := engine.demoGrants(&demo)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const insideRoot = "/src/demos/terminal/artifacts"
	scratch := filepath.Join(artifacts, "demos", "terminal", "artifacts", "scratch")
	if err := os.MkdirAll(scratch, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(artifacts, "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	// The child drives a ze built here from the tree under test, never the
	// hand-built shared tmp/terminal-demos/bin copy. It sits where the child's
	// demoBinary("ze") resolves under ZE_DEMO_ROOT=insideRoot, and the child
	// refuses any other ze it finds first on PATH (posterZeIsOwned).
	if err := buildPosterZe(ctx, source, filepath.Join(artifacts, "tmp", "terminal-demos", "bin", "ze")); err != nil {
		t.Fatal(err)
	}
	container := engine.containerCommand(manifest.Renderer, grants, "/bin/sh", "-c",
		"export PATH="+insideRoot+"/tmp/terminal-demos/bin:$PATH; cd /src/internal/le/site/terminaldemo && exec /run/web-config-poster.test -test.v -test.run '^TestWebConfigPosterUsesAuthoredInputs$'")
	for index := 1; index < len(container.Arguments); index++ {
		if container.Arguments[index-1] == "--volume" &&
			container.Arguments[index] != artifacts+":"+insideRoot {
			container.Arguments[index] += ":ro"
		}
	}
	container.Arguments = append(container.Arguments,
		"--volume", executable+":/run/web-config-poster.test:ro",
		"--env", "ZE_DEMO_POSTER_PROOF_CHILD=1",
		"--env", "ZE_DEMO_ROOT="+insideRoot,
		"--env", "TMPDIR="+filepath.Join(insideRoot, "demos", "terminal", "artifacts", "scratch"))
	if err := engine.withLock(func() error { return engine.runDemoContainer(&demo, container) }); err != nil {
		t.Fatal(err)
	}
}

// buildPosterZe builds the ze the poster child drives, from the tree under
// test, at the path the child resolves as demoBinary("ze") inside the
// renderer container (output is that path on the host side of the mount).
// It is buildCommand's recipe (zeBuildCommand), so tags, flags and target
// match `./le site terminal-demo build binaries`. Go's build cache makes a
// rebuild of an unchanged tree cheap, and building every run is what makes a
// binary older than the tree impossible: the shared tmp/terminal-demos/bin
// copy is rebuilt only by hand and was five days old when it judged the poster.
func buildPosterZe(ctx context.Context, root, output string) error {
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return fmt.Errorf("poster ze toolchain: %w", err)
	}
	arch, err := rendererGOARCH()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return fmt.Errorf("poster ze directory: %w", err)
	}
	command, _ := zeBuildCommand(root, toolchain, arch, demoTags(toolchain), output)
	process := exec.CommandContext(ctx, command.Args[0], command.Args[1:]...) // #nosec G204 -- the fixed Go toolchain build recipe of this package.
	process.Dir = command.Dir
	process.Env = command.Env
	process.Stdout = os.Stderr
	process.Stderr = os.Stderr
	if err := process.Run(); err != nil {
		return fmt.Errorf("build the poster ze from the tree under test (%s): %w", strings.Join(command.Args, " "), err)
	}
	return nil
}
