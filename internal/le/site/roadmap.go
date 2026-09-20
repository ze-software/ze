// Design: docs/contributing/gh-pages.md -- the roadmap publishes one immutable inventory.
// Detail: docs.go renders its Markdown with the shared shell and mirror.
package site

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/internal/le/spec/roadmap"
)

const (
	roadmapName        = "roadmap"
	roadmapDestination = "project/roadmap/index.html"
	roadmapDataFile    = "data/release-roadmap.json"
)

func init() {
	registerProducer(Producer{Name: roadmapName, Render: renderRoadmap})
}

// renderRoadmap collects once so JSON, HTML, and Markdown name the same tree
// even if HEAD changes during rendering. No local generated index is read.
func renderRoadmap(paths Paths) ([]string, error) {
	snapshot, err := roadmap.Collect(context.Background(), paths.Repository, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("release roadmap: %w", err)
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode release roadmap: %w", err)
	}
	if err := writeNamedArtifact(paths.Output, roadmapDataFile, string(data)+"\n"); err != nil {
		return nil, err
	}
	renderer, err := newDocsRenderer(paths)
	if err != nil {
		return nil, err
	}
	page := sitePage{
		Source:  "plan/README.md",
		Dest:    roadmapDestination,
		Desc:    "Release work items from committed specs, an inventory preview pending owner classification.",
		Journey: "Inventory preview",
	}
	source := roadmap.Markdown(&snapshot, false) + "\n[Snapshot JSON](../../" + roadmapDataFile + ")\n"
	if err := renderer.renderSource(page, []byte(source)); err != nil {
		return nil, err
	}
	return []string{"/" + page.destinationDirectory() + "/"}, nil
}
