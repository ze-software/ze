// Design: docs/architecture/core-design.md -- one definition of the demo manifest contract
// Overview: manifest.go -- the contracts a render writes and verifies
// Related: types.go -- Manifest, Demo and ArtifactEntry, which this answers
package terminaldemo

import (
	"errors"
	"fmt"
)

// Published answers the checked-in demo definitions and the artifacts a render
// generated from them, for a consumer that PUBLISHES a demo rather than
// records one.
//
// Both manifests MUST exist and MUST declare the current schema. An absent or
// empty artifact manifest is refused rather than read as an empty set: a
// publisher that took the empty answer would render a page whose demonstration
// had silently disappeared, which is the one failure a reader cannot see.
//
// root is the checkout, whose demos/terminal/manifest.json states what a demo
// is. artifactRoot is the directory a render wrote its media into, whose own
// manifest.json states each file's size and digest. Nothing is verified here;
// the caller checks each asset it publishes.
func Published(root, artifactRoot string) (Manifest, map[string]ArtifactEntry, error) {
	engine := New(Options{Root: root, ArtifactRoot: artifactRoot})
	var manifest Manifest
	if err := readJSON(engine.demoRoot, &manifest); err != nil {
		return Manifest{}, nil, fmt.Errorf("terminal demo source manifest: %w", err)
	}
	if manifest.Schema != manifestSchema {
		return Manifest{}, nil, fmt.Errorf("terminal demo source manifest must use schema %d", manifestSchema)
	}
	if manifest.GalleryPage == "" {
		return Manifest{}, nil, errors.New("terminal demo source manifest states no gallery-page")
	}
	var generated artifactManifest
	if err := readJSON(artifactRoot, &generated); err != nil {
		return Manifest{}, nil, fmt.Errorf("terminal demo artifact manifest in %s: %w", artifactRoot, err)
	}
	if generated.Schema != manifestSchema {
		return Manifest{}, nil, fmt.Errorf("terminal demo artifact manifest must use schema %d", manifestSchema)
	}
	if len(generated.Demos) == 0 {
		return Manifest{}, nil, fmt.Errorf("terminal demo artifact manifest in %s names no demo", artifactRoot)
	}
	return manifest, generated.Demos, nil
}
