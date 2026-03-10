// Design: docs/architecture/plugin/rib-storage-design.md — best-path pipeline for rib best commands
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_pipeline.go — iterator pipeline for show commands (scope, filters, terminals)
// Related: rib_commands.go — command handling and JSON responses
// Related: bestpath.go — best-path selection (gatherCandidates, SelectBest)
package rib

import (
	"encoding/json"
	"fmt"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
)

// --- Best-path source ---

// bestSource iterates over best-path results (one winner per prefix).
// Gathers unique (family, nlri) keys across matching peers, selects best per prefix,
// and yields the winning RouteItem with pool entry from the best peer.
// Caller must hold at least RLock on RIBManager.
type bestSource struct {
	items []RouteItem
	idx   int
	count int
}

func newBestSource(r *RIBManager, selector string) *bestSource {
	// Collect all unique (family, nlriKey) across matching peers.
	type routeKey struct {
		family  nlri.Family
		nlriKey string
		familyS string
		prefixS string
	}
	seen := make(map[string]routeKey) // "familyStr|nlriKey" → routeKey

	for peer, peerRIB := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		peerRIB.Iterate(func(family nlri.Family, nlriBytes []byte, _ *storage.RouteEntry) bool {
			fStr := formatFamily(family)
			pStr := formatNLRIAsPrefix(family, nlriBytes)
			key := fStr + "|" + string(nlriBytes)
			if _, ok := seen[key]; !ok {
				seen[key] = routeKey{family: family, nlriKey: string(nlriBytes), familyS: fStr, prefixS: pStr}
			}
			return true
		})
	}

	// For each unique prefix, gather candidates and select best.
	var items []RouteItem
	for _, rk := range seen {
		candidates := r.gatherCandidates(rk.family, []byte(rk.nlriKey))
		best := SelectBest(candidates)
		if best == nil {
			continue
		}

		item := RouteItem{
			Peer:      best.PeerAddr,
			Family:    rk.familyS,
			Prefix:    rk.prefixS,
			Direction: "received",
		}

		// Attach the pool entry from the winning peer for attribute access.
		if peerRIB := r.ribInPool[best.PeerAddr]; peerRIB != nil {
			if entry, ok := peerRIB.Lookup(rk.family, []byte(rk.nlriKey)); ok {
				item.InEntry = entry
			}
		}

		items = append(items, item)
	}

	// Sort by family then prefix for stable output.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Family != items[j].Family {
			return items[i].Family < items[j].Family
		}
		return items[i].Prefix < items[j].Prefix
	})

	return &bestSource{items: items}
}

func (s *bestSource) Next() (RouteItem, bool) {
	if s.idx >= len(s.items) {
		return RouteItem{}, false
	}
	item := s.items[s.idx]
	s.idx++
	s.count++
	return item, true
}

func (s *bestSource) Meta() PipelineMeta {
	return PipelineMeta{Count: s.count}
}

// --- Best-path pipeline builder ---

// bestPipeline builds and executes a pipeline from best-path source.
// Called by handleCommand for "rib best" with optional filter/terminal stages.
// Returns JSON string result with "best-path" top-level key.
func (r *RIBManager) bestPipeline(selector string, args []string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stages, errMsg := parseBestPipelineArgs(args)
	if errMsg != "" {
		data, _ := json.Marshal(map[string]any{"error": errMsg})
		return string(data)
	}

	source := newBestSource(r, selector)

	// Apply filter stages
	var current PipelineIterator = source
	for _, stage := range stages {
		current = stage.apply(current)
	}

	// If no terminal was specified, default to best-path json
	if !hasTerminal(stages) {
		bt := newBestJSONTerminal(current)
		meta := bt.Meta()
		return meta.JSON
	}

	// Execute terminal — drain it and return metadata
	meta := current.Meta()
	if meta.JSON != "" {
		return meta.JSON
	}

	// count terminal
	data, _ := json.Marshal(map[string]any{"count": meta.Count})
	return string(data)
}

// parseBestPipelineArgs parses args for rib best (no scope keyword, filters + terminals only).
// Returns (stages, errorMessage).
// Validates ordering: filters must precede terminals, and at most one terminal is allowed.
func parseBestPipelineArgs(args []string) ([]pipelineStage, string) {
	var stages []pipelineStage
	i := 0
	sawTerminal := false
	for i < len(args) {
		keyword := args[i]

		if filterKeywords[keyword] {
			if sawTerminal {
				return nil, fmt.Sprintf("filter after terminal: %s", keyword)
			}
			i++
			if i >= len(args) {
				return nil, fmt.Sprintf("%s requires a value", keyword)
			}
			if keyword == filterPath {
				if errMsg := validatePathPattern(args[i]); errMsg != "" {
					return nil, errMsg
				}
			}
			stages = append(stages, pipelineStage{kind: keyword, arg: args[i]})
			i++
			continue
		}

		if terminalKeywords[keyword] {
			if sawTerminal {
				return nil, "multiple terminals not allowed"
			}
			sawTerminal = true
			stages = append(stages, pipelineStage{kind: keyword, terminal: true})
			i++
			continue
		}

		return nil, fmt.Sprintf("unknown keyword: %s", keyword)
	}
	return stages, ""
}

// --- Best-path JSON terminal ---

// bestJSONTerminal drains upstream best-path items and serializes to best-path JSON format.
type bestJSONTerminal struct {
	upstream PipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newBestJSONTerminal(upstream PipelineIterator) *bestJSONTerminal {
	return &bestJSONTerminal{upstream: upstream}
}

func (bt *bestJSONTerminal) Next() (RouteItem, bool) {
	if !bt.drained {
		bt.drain()
	}
	return RouteItem{}, false
}

func (bt *bestJSONTerminal) drain() {
	bt.drained = true

	type bestResult struct {
		Family   string         `json:"family"`
		Prefix   string         `json:"prefix"`
		BestPeer string         `json:"best-peer"`
		Attrs    map[string]any `json:"attributes,omitempty"`
	}

	results := make([]bestResult, 0)
	for {
		item, ok := bt.upstream.Next()
		if !ok {
			break
		}
		br := bestResult{
			Family:   item.Family,
			Prefix:   item.Prefix,
			BestPeer: item.Peer,
		}
		if item.InEntry != nil {
			attrs := make(map[string]any)
			enrichRouteMapFromEntry(attrs, item.InEntry)
			if len(attrs) > 0 {
				br.Attrs = attrs
			}
		}
		results = append(results, br)
	}

	bt.meta.Count = len(results)
	data, _ := json.Marshal(map[string]any{"best-path": results})
	bt.meta.JSON = string(data)
}

func (bt *bestJSONTerminal) Meta() PipelineMeta {
	if !bt.drained {
		bt.drain()
	}
	return bt.meta
}
