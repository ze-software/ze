// Design: docs/architecture/plugin/rib-storage-design.md — best-path pipeline for bgp rib show best commands
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_pipeline.go — iterator pipeline for show commands (scope, filters, terminals)
// Related: rib_commands.go — command handling and JSON responses
// Related: bestpath.go — best-path selection (gatherCandidates, SelectBest)
package rib

import (
	"encoding/json"
	"fmt"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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

// bestRouteKey builds the lookup key used by the reason terminal's
// candidates-by-prefix map. Must match the key format used by newBestSource
// when populating that map.
func bestRouteKey(familyS, prefixS string) string {
	return familyS + "|" + prefixS
}

// newBestSource builds the per-prefix best-path item list. When
// stashCandidates is non-nil, the full candidate slice for every yielded
// item is written into it keyed by bestRouteKey(family, prefix). This lets
// the "reason" terminal re-run the decision process with narration without
// re-querying gatherCandidates under a second lock acquisition.
func newBestSource(r *RIBManager, selector string, stashCandidates map[string][]*Candidate) *bestSource {
	// Collect all unique (family, nlriKey) across matching peers.
	type routeKey struct {
		fam     family.Family
		nlriKey string
		familyS string
		prefixS string
	}
	seen := make(map[string]routeKey) // "familyStr|nlriKey" → routeKey

	// Caller bestPipeline holds r.peerMu.RLock across this function; the
	// ribInPool iteration below is protected by that outer lock.
	for peer, peerRIB := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		peerRIB.Iterate(func(fam family.Family, nlriBytes []byte, _ storage.RouteEntry) bool {
			fStr := formatFamily(fam)
			pStr := formatNLRIAsPrefix(fam, nlriBytes, peerRIB.IsAddPath(fam))
			key := fStr + "|" + string(nlriBytes)
			if _, ok := seen[key]; !ok {
				seen[key] = routeKey{fam: fam, nlriKey: string(nlriBytes), familyS: fStr, prefixS: pStr}
			}
			return true
		})
	}

	// Snapshot the multipath config once per call. The atomic fields are
	// written only at Stage 2 OnConfigure and rarely after, so a single
	// load is race-free and cheap.
	multipathMax := r.maximumPaths.Load()
	relaxASPath := r.relaxASPath.Load()

	// For each unique prefix, gather candidates and select best (plus any
	// multipath siblings when bgp/multipath/maximum-paths > 1).
	var items []RouteItem
	for _, rk := range seen {
		// bestPipeline holds r.peerMu.RLock across this call; use the
		// Locked variant to avoid a recursive RLock that would deadlock
		// against a pending writer (Go sync.RWMutex docs).
		candidates := r.gatherCandidatesLocked(rk.fam, []byte(rk.nlriKey))
		best, siblings := SelectMultipath(candidates, multipathMax, relaxASPath)
		if best == nil {
			continue
		}

		item := RouteItem{
			Peer:      best.PeerAddr,
			Family:    rk.fam,
			Prefix:    rk.prefixS,
			Direction: rpc.DirectionReceived,
		}

		// Attach the pool entry from the winning peer for attribute access.
		// Caller bestPipeline holds r.peerMu.RLock; this ribInPool read is
		// protected by that outer lock.
		if peerRIB := r.ribInPool[best.PeerAddr]; peerRIB != nil {
			if entry, ok := peerRIB.Lookup(rk.fam, []byte(rk.nlriKey)); ok {
				item.HasInEntry = true
				item.InEntry = entry
			}
		}

		// Populate MultipathPeers with sibling peer addresses so the output
		// terminal can render the full ECMP set. nil when multipath is off.
		if len(siblings) > 0 {
			peers := make([]string, len(siblings))
			for i, s := range siblings {
				peers[i] = s.PeerAddr
			}
			item.MultipathPeers = peers
		}

		// Stash candidates for the reason terminal if requested. The map is
		// keyed by the same (familyS, prefixS) pair that the terminal can
		// reconstruct from RouteItem at drain time.
		if stashCandidates != nil {
			stashCandidates[bestRouteKey(rk.familyS, rk.prefixS)] = candidates
		}

		items = append(items, item)
	}

	// Sort by family then prefix for stable output.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Family != items[j].Family {
			return family.FamilyLess(items[i].Family, items[j].Family)
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
// Called by handleCommand for "bgp rib show best" with optional filter/terminal stages.
// Returns JSON string result with "best-path" top-level key.
func (r *RIBManager) bestPipeline(selector string, args []string) string {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	stages, errMsg := parseBestPipelineArgs(args)
	if errMsg != "" {
		data, _ := json.Marshal(map[string]any{"error": errMsg})
		return string(data)
	}

	// The reason terminal needs access to every candidate (not just the
	// winner) for every surviving prefix. Pre-allocate a stash map that
	// bestSource populates at construction; the reason terminal reads it at
	// drain time, keyed by the human-readable (family, prefix) pair.
	// For other terminals the map stays nil so bestSource skips the extra
	// bookkeeping.
	var candidatesByKey map[string][]*Candidate
	if hasReasonTerminal(stages) {
		candidatesByKey = make(map[string][]*Candidate)
	}

	source := newBestSource(r, selector, candidatesByKey)

	// Apply non-reason filter/terminal stages. The reason terminal is handled
	// explicitly after this loop because it needs the stashed candidates.
	var current PipelineIterator = source
	for _, stage := range stages {
		if stage.kind == bestTerminalReason {
			continue
		}
		current = stage.apply(current)
	}

	// Reason terminal: drive a specialized drain that consults the stash.
	if hasReasonTerminal(stages) {
		rt := newBestReasonTerminal(current, candidatesByKey)
		return rt.Meta().JSON
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

// bestTerminalReason is the keyword that activates the cmd-9 "reason"
// terminal. It lives local to rib_pipeline_best.go because "reason" only
// makes sense in the best-path pipeline -- the generic scoped pipeline
// does not compute per-prefix candidates.
const bestTerminalReason = "reason"

// hasReasonTerminal reports whether any stage is the reason terminal.
func hasReasonTerminal(stages []pipelineStage) bool {
	for _, s := range stages {
		if s.terminal && s.kind == bestTerminalReason {
			return true
		}
	}
	return false
}

// parseBestPipelineArgs parses args for bgp rib show best (no scope keyword, filters + terminals only).
// Returns (stages, errorMessage).
// Validates ordering: filters must precede terminals, and at most one terminal is allowed.
//
// In addition to the generic terminalKeywords accepted across all pipelines,
// this parser also accepts bestTerminalReason ("reason") which is specific to
// the best-path pipeline -- it reports WHY a particular path won the per-
// prefix decision process.
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

		if terminalKeywords[keyword] || keyword == bestTerminalReason {
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

// --- Best-path reason terminal ---

// bestReasonTerminal drains the filtered best-path items and, for each
// surviving prefix, re-runs the decision process with narration via
// SelectBestExplain. The stashed candidate slices come from newBestSource.
// Output JSON shape:
//
//	{"best-path-reason": [{"family","prefix","winner-peer","steps":[{"step","winner","reason"}]}]}
type bestReasonTerminal struct {
	upstream        PipelineIterator
	candidatesByKey map[string][]*Candidate
	meta            PipelineMeta
	drained         bool
}

func newBestReasonTerminal(upstream PipelineIterator, candidatesByKey map[string][]*Candidate) *bestReasonTerminal {
	return &bestReasonTerminal{upstream: upstream, candidatesByKey: candidatesByKey}
}

// Next is present so bestReasonTerminal satisfies PipelineIterator, but the
// terminal materializes the entire explanation at drain time -- Next always
// reports end-of-stream after drain.
func (rt *bestReasonTerminal) Next() (RouteItem, bool) {
	if !rt.drained {
		rt.drain()
	}
	return RouteItem{}, false
}

func (rt *bestReasonTerminal) Meta() PipelineMeta {
	if !rt.drained {
		rt.drain()
	}
	return rt.meta
}

// reasonStep is the JSON shape for a single pairwise comparison inside an
// explanation entry.
type reasonStep struct {
	Step       string `json:"step"`
	Incumbent  string `json:"incumbent"`
	Challenger string `json:"challenger"`
	Winner     string `json:"winner"`
	Reason     string `json:"reason"`
}

// reasonEntry is the JSON shape for a per-prefix explanation.
type reasonEntry struct {
	Family     string       `json:"family"`
	Prefix     string       `json:"prefix"`
	WinnerPeer string       `json:"winner-peer"`
	Candidates []string     `json:"candidates"` // peer addresses in original order
	Steps      []reasonStep `json:"steps"`
}

func (rt *bestReasonTerminal) drain() {
	rt.drained = true

	entries := make([]reasonEntry, 0)
	for {
		item, ok := rt.upstream.Next()
		if !ok {
			break
		}
		candidates := rt.candidatesByKey[bestRouteKey(item.Family.String(), item.Prefix)]
		exp := SelectBestExplain(candidates)
		if exp == nil {
			continue // defensive: prefix reached the terminal but has no candidates
		}

		entry := reasonEntry{
			Family:     item.Family.String(),
			Prefix:     item.Prefix,
			WinnerPeer: exp.Winner.PeerAddr,
			Candidates: make([]string, len(exp.Candidates)),
			Steps:      make([]reasonStep, len(exp.Steps)),
		}
		for i, c := range exp.Candidates {
			entry.Candidates[i] = c.PeerAddr
		}
		for i, s := range exp.Steps {
			entry.Steps[i] = reasonStep{
				Step:       s.Step.String(),
				Incumbent:  exp.Candidates[s.IncumbentIdx].PeerAddr,
				Challenger: exp.Candidates[s.ChallengerIdx].PeerAddr,
				Winner:     exp.Candidates[s.WinnerIdx].PeerAddr,
				Reason:     s.Reason,
			}
		}
		entries = append(entries, entry)
	}

	rt.meta.Count = len(entries)
	data, _ := json.Marshal(map[string]any{"best-path-reason": entries})
	rt.meta.JSON = string(data)
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
		Family         string         `json:"family"`
		Prefix         string         `json:"prefix"`
		BestPeer       string         `json:"best-peer"`
		MultipathPeers []string       `json:"multipath-peers,omitempty"` // cmd-3: ECMP siblings (primary excluded)
		Attrs          map[string]any `json:"attributes,omitempty"`
	}

	results := make([]bestResult, 0)
	for {
		item, ok := bt.upstream.Next()
		if !ok {
			break
		}
		br := bestResult{
			Family:         item.Family.String(),
			Prefix:         item.Prefix,
			BestPeer:       item.Peer,
			MultipathPeers: item.MultipathPeers,
		}
		if item.HasInEntry {
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
