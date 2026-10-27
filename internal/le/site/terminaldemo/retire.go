// Design: docs/contributing/gh-pages.md -- retired recordings own only their declared media.
package siteterminaldemo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// RetireDeleted removes previously declared media for IDs absent from the source
// manifest. It MUST run after a site build seeds its media and before pages read it.
func (e *Engine) RetireDeleted() error {
	return e.withLock(func() error {
		var generated artifactManifest
		if err := readJSON(e.artifactRoot, &generated); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// No inventory means this tree owns no retired media.
				return nil
			}
			return err
		}
		if generated.Schema != manifestSchema {
			return fmt.Errorf("cannot retire media from unsupported artifact schema %d", generated.Schema)
		}
		var manifest Manifest
		if err := readJSON(e.demoRoot, &manifest); err != nil {
			return err
		}
		if manifest.Schema != manifestSchema {
			return fmt.Errorf("cannot retire media from unsupported source schema %d", manifest.Schema)
		}
		if len(manifest.Demos) == 0 {
			return fmt.Errorf("cannot retire media from an empty source manifest")
		}
		indexed := make(map[string]Demo, len(manifest.Demos))
		for _, demo := range manifest.Demos {
			if !demoIDPattern(demo.ID) {
				return fmt.Errorf("source manifest has unsafe id %q", demo.ID)
			}
			if _, exists := indexed[demo.ID]; exists {
				return fmt.Errorf("source manifest repeats id %q", demo.ID)
			}
			indexed[demo.ID] = demo
		}
		before := len(generated.Demos)
		if err := e.retireDeletedAssets(&generated, indexed); err != nil {
			return err
		}
		if len(generated.Demos) == before {
			return nil
		}
		return e.writeArtifactManifest(generated)
	})
}

// retireDeletedAssets validates the entire deletion set before removing anything.
// The artifact inventory, not a filename glob, is the ownership record.
func (e *Engine) retireDeletedAssets(generated *artifactManifest, indexed map[string]Demo) error {
	var retired []string
	var paths []string
	for id, entry := range generated.Demos {
		if _, exists := indexed[id]; exists {
			continue
		}
		if !demoIDPattern(id) {
			return fmt.Errorf("retired demo has unsafe id %q", id)
		}
		for kind, asset := range entry.Assets {
			extension, err := retiredExtension(kind)
			if err != nil {
				return err
			}
			if asset.Path != id+extension {
				return fmt.Errorf("retired demo %s does not own %q: expected %s%s", id, asset.Path, id, extension)
			}
			paths = append(paths, asset.Path)
		}
		retired = append(retired, id)
	}
	if len(retired) == 0 {
		return nil
	}
	root, err := os.OpenRoot(e.artifactRoot)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // The removal result owns the verdict.
	// Resolve every retained declaration before deleting anything. Noncanonical
	// spelling is normalized; symlink ownership is refused rather than guessed.
	protected := make(map[string]struct{})
	base, err := filepath.EvalSymlinks(e.artifactRoot)
	if err != nil {
		return fmt.Errorf("resolve artifact root before retirement: %w", err)
	}
	for id := range indexed {
		for _, asset := range generated.Demos[id].Assets {
			name, err := retainedAssetPath(root, base, asset.Path)
			if err != nil {
				return fmt.Errorf("retained demo %s ownership: %w", id, err)
			}
			protected[name] = struct{}{}
		}
	}
	for _, name := range paths {
		if _, retained := protected[name]; retained {
			return fmt.Errorf("retired asset %q is also owned by a current demo", name)
		}
	}
	slices.Sort(paths)
	for _, name := range paths {
		if err := root.Remove(name); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("retire %s: %w", name, err)
			}
		}
	}
	for _, id := range retired {
		delete(generated.Demos, id)
	}
	return nil
}

func retainedAssetPath(root *os.Root, base, declared string) (string, error) {
	name := filepath.FromSlash(declared)
	if !filepath.IsLocal(name) {
		return "", fmt.Errorf("unsafe asset path %q", declared)
	}
	name = filepath.Clean(name)
	info, err := root.Stat(name)
	if err != nil {
		return "", fmt.Errorf("resolve retained asset %q: %w", declared, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("retained asset %q is not a regular file", declared)
	}
	expected := filepath.Join(base, name)
	resolved, err := filepath.EvalSymlinks(expected)
	if err != nil {
		return "", fmt.Errorf("resolve retained asset %q: %w", declared, err)
	}
	if resolved != expected {
		return "", fmt.Errorf("retained asset %q has aliased ownership through a symlink", declared)
	}
	return name, nil
}

func retiredExtension(kind string) (string, error) {
	switch kind {
	case "cast":
		return ".cast", nil
	case transcriptLabel:
		return textExtension, nil
	case "video":
		return ".webm", nil
	case "poster":
		return ".webp", nil
	default:
		return "", fmt.Errorf("retired demo has unknown asset kind %q", kind)
	}
}
