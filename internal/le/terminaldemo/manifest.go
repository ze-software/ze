// Design: docs/architecture/core-design.md -- terminal-demo manifests are the source and artifact contracts
// Overview: types.go -- the pipeline types and external boundary
// Related: render.go -- the producer of the artifacts verified here

package terminaldemo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const hashChunkBytes = 1024 * 1024

var demoIDPattern = func(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	previousDash := false
	for _, character := range value {
		if character == '-' {
			if previousDash {
				return false
			}
			previousDash = true
			continue
		}
		previousDash = false
		if character >= 'a' && character <= 'z' {
			continue
		}
		if character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

func (e *Engine) loadManifest() (Manifest, map[string]Demo, error) {
	var manifest Manifest
	if err := readJSON(e.demoRoot, "manifest.json", &manifest); err != nil {
		return Manifest{}, nil, err
	}
	indexed, err := e.validateContract(manifest)
	if err != nil {
		return Manifest{}, nil, err
	}
	return manifest, indexed, nil
}

func readJSON(rootPath, name string, value any) error {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // The decoded file owns the read verdict.
	handle, err := root.Open(name)
	if err != nil {
		return err
	}
	defer handle.Close() //nolint:errcheck // The decoder reports the read result.
	decoder := json.NewDecoder(handle)
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%s: expected one JSON value", filepath.Join(rootPath, name))
		}
		return err
	}
	return nil
}

func readRootFile(rootPath, name string) ([]byte, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // The rooted read owns the verdict.
	return root.ReadFile(name)
}

func (e *Engine) validateContract(manifest Manifest) (map[string]Demo, error) {
	if manifest.Schema != manifestSchema {
		return nil, errors.New("manifest.json: unsupported schema")
	}
	rendererFields := []struct {
		name  string
		value string
	}{
		{"name", manifest.Renderer.Name},
		{"version", manifest.Renderer.Version},
		{"image", manifest.Renderer.Image},
		{platformLabel, manifest.Renderer.Platform},
	}
	for _, field := range rendererFields {
		if field.value == "" {
			return nil, fmt.Errorf("manifest.json: renderer.%s is required", field.name)
		}
	}
	if manifest.GalleryPage == "" {
		return nil, errors.New("manifest.json: gallery-page is required")
	}
	if !regularFile(filepath.Join(e.root, "docs", manifest.GalleryPage)) {
		return nil, fmt.Errorf("manifest.json: gallery page does not exist: %s", manifest.GalleryPage)
	}
	if len(manifest.Demos) == 0 {
		return nil, errors.New("manifest.json: demos must be a non-empty list")
	}

	indexed := make(map[string]Demo, len(manifest.Demos))
	for index := range manifest.Demos {
		demo := &manifest.Demos[index]
		if !demoIDPattern(demo.ID) {
			return nil, fmt.Errorf("manifest.json: invalid demo id %q", demo.ID)
		}
		if _, exists := indexed[demo.ID]; exists {
			return nil, fmt.Errorf("manifest.json: duplicate demo id %s", demo.ID)
		}
		if err := e.validateDemo(demo); err != nil {
			return nil, err
		}
		indexed[demo.ID] = *demo
	}
	return indexed, nil
}

func (e *Engine) validateDemo(demo *Demo) error {
	required := []struct {
		name  string
		value string
	}{
		{"title", demo.Title},
		{"description", demo.Description},
		{"page", demo.Page},
		{"anchor", demo.Anchor},
		{platformLabel, demo.Platform},
		{"kind", demo.Kind},
		{"engine", demo.Engine},
		{sourceLabel, demo.Source},
		{rendererValidateMode, demo.Validate},
	}
	for _, field := range required {
		if field.value == "" {
			return fmt.Errorf("manifest.json: %s.%s is required", demo.ID, field.name)
		}
	}
	extensions, err := assetExtensions(demo.Kind)
	if err != nil {
		return fmt.Errorf("manifest.json: %s.kind is unsupported", demo.ID)
	}
	_, hasCast := extensions["cast"]
	if hasCast {
		if demo.Duration != "" {
			return fmt.Errorf("manifest.json: %s.duration is read from the cast, so the manifest must not state it", demo.ID)
		}
	}
	if !hasCast {
		if demo.Duration == "" {
			return fmt.Errorf("manifest.json: %s.duration is required", demo.ID)
		}
	}
	checks := []struct {
		path string
		kind string
	}{
		{filepath.Join(e.root, "docs", demo.Page), "page"},
		{filepath.Join(e.demoRoot, demo.Source), sourceLabel},
		{filepath.Join(e.demoRoot, filepath.Dir(demo.Source), transcriptFilename), transcriptLabel},
		{filepath.Join(e.demoRoot, demo.Validate), "validator"},
	}
	for _, check := range checks {
		if regularFile(check.path) {
			continue
		}
		relative, err := filepath.Rel(e.root, check.path)
		if err != nil {
			relative = check.path
		}
		return fmt.Errorf("manifest.json: %s does not exist: %s", check.kind, filepath.ToSlash(relative))
	}
	return nil
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func assetExtensions(kind string) (map[string]string, error) {
	switch kind {
	case terminalLabel:
		return map[string]string{"cast": ".cast", transcriptLabel: textExtension}, nil
	case "browser":
		return map[string]string{"video": ".webm", "poster": ".png", transcriptLabel: textExtension}, nil
	default:
		return nil, fmt.Errorf("unsupported kind %q", kind)
	}
}

func (e *Engine) assetPaths(demo Demo) (map[string]string, error) {
	extensions, err := assetExtensions(demo.Kind)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", demo.ID, err)
	}
	paths := make(map[string]string, len(extensions))
	for name, extension := range extensions {
		var buffer textbuf.Buffer
		filename := buffer.Str(demo.ID).Str(extension).String()
		paths[name] = filepath.Join(e.artifactRoot, filename)
	}
	return paths, nil
}

func assetNames(kind string) ([]string, error) {
	switch kind {
	case terminalLabel:
		return []string{"cast", transcriptLabel}, nil
	case "browser":
		return []string{"video", "poster", transcriptLabel}, nil
	default:
		return nil, fmt.Errorf("unsupported kind %q", kind)
	}
}

func (e *Engine) sourceDigest(demo Demo) (string, error) {
	shared := []string{
		filepath.Join(e.demoRoot, "common.tape"),
		filepath.Join(e.demoRoot, "cards.sh"),
		filepath.Join(e.demoRoot, "Dockerfile"),
		filepath.Join(e.demoRoot, "container-entrypoint.sh"),
		filepath.Join(e.demoRoot, "demo-lock.sh"),
		filepath.Join(e.demoRoot, "validate-common.sh"),
		filepath.Join(e.demoRoot, "pty-session.py"),
		filepath.Join(e.demoRoot, "render.py"),
		filepath.Join(e.demoRoot, "screen.py"),
	}
	files := append([]string(nil), shared...)
	sourceDir := filepath.Join(e.demoRoot, filepath.Dir(demo.Source))
	err := filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	contract := renderContract(false, demo)
	return e.digestPaths(contract, files)
}

func (e *Engine) definitionDigest(demo Demo) (string, error) {
	files := []string{filepath.Join(e.demoRoot, "common.tape"), filepath.Join(e.demoRoot, demo.Source)}
	contract := renderContract(true, demo)
	return e.digestPaths(contract, files)
}

func renderContract(includeKind bool, demo Demo) []byte {
	var buffer textbuf.Buffer
	buffer.Byte('{')
	if includeKind {
		buffer.Str(`"kind": `).Str(strconv.Quote(demo.Kind)).Str(", ")
	}
	buffer.Str(`"privileged": `).Bool(demo.isPrivileged()).Str(", ").
		Str(`"realtime": `).Bool(demo.isRealtime()).Str(", ").
		Str(`"source": `).Str(strconv.Quote(demo.Source)).Byte('}')
	return append([]byte(nil), buffer.Bytes()...)
}

func (e *Engine) digestPaths(contract []byte, paths []string) (string, error) {
	sort.Strings(paths)
	root, err := os.OpenRoot(e.root)
	if err != nil {
		return "", err
	}
	defer root.Close() //nolint:errcheck // Each rooted read owns its verdict.
	digest := sha256.New()
	digest.Write(contract)  //nolint:errcheck // A hash writer cannot fail.
	digest.Write([]byte{0}) //nolint:errcheck // A hash writer cannot fail.
	for _, path := range paths {
		relative, err := filepath.Rel(e.root, path)
		if err != nil {
			return "", err
		}
		digest.Write([]byte(filepath.ToSlash(relative))) //nolint:errcheck // A hash writer cannot fail.
		digest.Write([]byte{0})                          //nolint:errcheck // A hash writer cannot fail.
		data, err := root.ReadFile(relative)
		if err != nil {
			return "", err
		}
		digest.Write(data)      //nolint:errcheck // A hash writer cannot fail.
		digest.Write([]byte{0}) //nolint:errcheck // A hash writer cannot fail.
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func hashFile(path string) (string, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	defer root.Close() //nolint:errcheck // The file read owns the verdict.
	handle, err := root.Open(filepath.Base(path))
	if err != nil {
		return "", err
	}
	defer handle.Close() //nolint:errcheck // The read result owns the verdict.
	digest := sha256.New()
	buffer := make([]byte, hashChunkBytes)
	for {
		readBytes, readErr := handle.Read(buffer)
		if readBytes > 0 {
			digest.Write(buffer[:readBytes]) //nolint:errcheck // A hash writer cannot fail.
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func (e *Engine) loadArtifactManifest(manifest Manifest) (ArtifactManifest, error) {
	var generated ArtifactManifest
	err := readJSON(e.artifactRoot, "manifest.json", &generated)
	if err == nil {
		if generated.Schema == manifestSchema && generated.Demos != nil {
			return generated, nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ArtifactManifest{}, err
	}
	return ArtifactManifest{Schema: manifestSchema, Renderer: manifest.Renderer, Demos: map[string]ArtifactEntry{}}, nil
}

func (e *Engine) writeArtifactManifest(generated ArtifactManifest) error {
	if err := os.MkdirAll(e.artifactRoot, 0o755); err != nil { // #nosec G301 -- Public website artifacts must be traversable by the web server.
		return err
	}
	data, err := json.MarshalIndent(artifactJSONValue(generated), "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(e.artifactManifestPath, data, 0o644) // #nosec G306 -- The manifest is a public website artifact, not private data.
}

func artifactJSONValue(generated ArtifactManifest) map[string]any {
	demos := make(map[string]any, len(generated.Demos))
	for demoID, entry := range generated.Demos {
		assets := make(map[string]any, len(entry.Assets))
		for name, asset := range entry.Assets {
			assets[name] = map[string]any{
				"bytes":  asset.Bytes,
				"path":   asset.Path,
				"sha256": asset.SHA256,
			}
		}
		demos[demoID] = map[string]any{
			"assets":            assets,
			"binary-sha256":     entry.BinarySHA256,
			"definition-sha256": entry.DefinitionSHA256,
			"release":           entry.Release,
			"source-sha256":     entry.SourceSHA256,
		}
	}
	return map[string]any{
		"demos": demos,
		"renderer": map[string]any{
			"image":       generated.Renderer.Image,
			"name":        generated.Renderer.Name,
			platformLabel: generated.Renderer.Platform,
			"version":     generated.Renderer.Version,
		},
		"schema": generated.Schema,
	}
}

func (e *Engine) verifyAssets(manifest Manifest, indexed map[string]Demo, selected []string, release string, definitionOnly bool) error {
	var generated ArtifactManifest
	if err := readJSON(e.artifactRoot, "manifest.json", &generated); err != nil {
		return err
	}
	if generated.Schema != manifestSchema {
		return errors.New("generated manifest: unsupported schema")
	}
	if generated.Renderer != manifest.Renderer {
		return errors.New("generated manifest: renderer contract is stale")
	}
	if generated.Demos == nil {
		return errors.New("generated manifest: demos must be an object")
	}
	for _, demoID := range selected {
		entry, exists := generated.Demos[demoID]
		if !exists {
			return fmt.Errorf("generated manifest: missing %s", demoID)
		}
		if release != "" {
			if entry.Release != release {
				return fmt.Errorf("%s: rendered for '%s', expected '%s'", demoID, entry.Release, release)
			}
		}
		demo := indexed[demoID]
		var expectedDigest string
		var err error
		if definitionOnly {
			expectedDigest, err = e.definitionDigest(demo)
		} else {
			expectedDigest, err = e.sourceDigest(demo)
		}
		if err != nil {
			return err
		}
		gotDigest := entry.SourceSHA256
		stale := sourceLabel
		if definitionOnly {
			gotDigest = entry.DefinitionSHA256
			stale = "definition"
		}
		if gotDigest != expectedDigest {
			return fmt.Errorf("%s: %s changed since the last render", demoID, stale)
		}
		if entry.Assets == nil {
			return fmt.Errorf("%s: assets are missing", demoID)
		}
		expected, err := e.assetPaths(demo)
		if err != nil {
			return err
		}
		foreign := make([]string, 0)
		for name := range entry.Assets {
			if _, exists := expected[name]; !exists {
				foreign = append(foreign, name)
			}
		}
		if len(foreign) > 0 {
			sort.Strings(foreign)
			names := make([]string, 0, len(expected))
			for name := range expected {
				names = append(names, name)
			}
			sort.Strings(names)
			return fmt.Errorf("%s: a %s demo does not produce %s; its assets are %s", demoID, demo.Kind, strings.Join(foreign, ", "), strings.Join(names, ", "))
		}
		names, err := assetNames(demo.Kind)
		if err != nil {
			return err
		}
		for _, name := range names {
			asset, exists := entry.Assets[name]
			if !exists {
				return fmt.Errorf("%s: missing %s metadata", demoID, name)
			}
			if asset.Path == "" {
				return fmt.Errorf("%s: missing %s metadata", demoID, name)
			}
			published := filepath.Join(e.artifactRoot, filepath.FromSlash(asset.Path))
			info, err := os.Stat(published)
			if err != nil {
				return fmt.Errorf("%s: missing generated asset: %s", demoID, published)
			}
			digest, err := hashFile(published)
			if err != nil {
				return err
			}
			if info.Size() != asset.Bytes {
				return fmt.Errorf("%s: %s digest mismatch", demoID, name)
			}
			if digest != asset.SHA256 {
				return fmt.Errorf("%s: %s digest mismatch", demoID, name)
			}
		}
	}
	var buffer textbuf.Buffer
	if _, err := fmt.Fprintln(e.output, buffer.Str("Ze demo artifacts verified: ").
		Join(selected, ", ").String()); err != nil {
		return fmt.Errorf("write artifact verification result: %w", err)
	}
	return nil
}
