// Design: docs/architecture/plugin/rib-storage-design.md -- graph terminal for AS path topology
// Overview: rib_pipeline.go -- pipeline terminal dispatch
// Related: rib_attr_format.go -- formatASPath for pool-based AS path extraction

package rib

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/graph"
)

// graphTerminal drains the upstream pipeline, extracts AS paths from RouteItems,
// builds an AS topology graph, and renders it as Unicode box-drawing text.
// The rendered text is stored in PipelineMeta.JSON (following the prefix-summary precedent).
type graphTerminal struct {
	upstream PipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newGraphTerminal(upstream PipelineIterator) *graphTerminal {
	return &graphTerminal{upstream: upstream}
}

func (gt *graphTerminal) Next() (RouteItem, bool) {
	if !gt.drained {
		gt.drain()
	}
	return RouteItem{}, false
}

func (gt *graphTerminal) drain() {
	gt.drained = true

	var paths [][]uint32
	count := 0

	for {
		item, ok := gt.upstream.Next()
		if !ok {
			break
		}
		count++

		asPath := extractASPathFromItem(item)
		if len(asPath) > 0 {
			paths = append(paths, asPath)
		}
	}

	gt.meta.Count = count

	g := graph.BuildGraphFromPaths(paths)
	if len(g.Nodes) == 0 {
		return
	}

	if len(g.Nodes) > graph.MaxNodes {
		gt.meta.JSON = fmt.Sprintf("graph too many nodes (%d, limit %d)\n", len(g.Nodes), graph.MaxNodes)
		return
	}

	gt.meta.JSON = graph.RenderText(g)
}

func (gt *graphTerminal) Meta() PipelineMeta {
	if !gt.drained {
		gt.drain()
	}
	return gt.meta
}
