package topology

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	// The BGP suite registers the ze image every topology Ze node runs; the
	// le binary links it through the integration actions.
	_ "github.com/ze-software/ze/internal/le/interoplab/bgp"
)

// VALIDATES: every topology scenario directory has a declaration and a
// checker, every declaration has a directory, each plan passes the core's
// link validation, and every image a node names is registered (AC-9 at plan
// time).
// Method: Discover the committed scenario root against the checker map, then
// build each plan and validate its links and images.
// PREVENTS: a directory and its register file silently disagreeing, and a
// link or image fault surfacing only once Docker runs.
func TestTopologyCatalogDiscovers(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	sources, err := interoplab.Discover(filepath.Join(root, filepath.FromSlash(scenarioDirectory)), "", checkers())
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != len(scenarios) {
		t.Fatalf("discovered %d scenario directories, declared %d", len(sources), len(scenarios))
	}
	for _, source := range sources {
		plan := scenarioPlan(root, source, scenarios[source.Name], "fixture")
		if err := interoplab.ValidateLinks(source.Name, plan.Links, plan.Peers); err != nil {
			t.Fatal(err)
		}
		images := make([]string, 0, len(plan.Peers))
		for index := range plan.Peers {
			images = append(images, plan.Peers[index].Image)
		}
		if _, _, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root}, images); err != nil {
			t.Fatalf("%s: %v", source.Name, err)
		}
	}
}
