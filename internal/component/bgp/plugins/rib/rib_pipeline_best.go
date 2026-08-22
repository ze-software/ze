// Design: docs/architecture/plugin/rib-storage-design.md — best-path pipeline for show bgp rib best commands,
//
//	and the retained read that lets its walk write with no lock held
//
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_pipeline.go — iterator pipeline for show commands (scope, filters, terminals)
// Related: rib_commands.go — command handling and JSON responses
// Related: bestpath.go — best-path selection (gatherCandidates, SelectBest)
package rib

import (
	"encoding/json"
	"iter"
	"sort"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// --- Best-path source ---

// bestEntry is one best-path result: the item a consumer reads, and where its
// pool entry is re-resolved when the item is pulled.
//
// The two are apart because the item travels and the reference does not. A
// RouteItem that carried a live pool handle from construction to the row that
// writes it would be reading attributes nothing kept alive, since a route
// removed in between releases its handles under the storage lock alone
// (PeerRIB.LookupRetained).
type bestEntry struct {
	item    RouteItem
	peerRIB *storage.PeerRIB // the winning peer's RIB, nil when it has gone
	nlri    []byte           // the key of the winning route inside that RIB
}

// bestSource iterates over best-path results (one winner per prefix).
// Gathers unique (family, nlri) keys across matching peers, selects best per prefix,
// and yields the winning RouteItem with pool entry from the best peer.
//
// Construction reads the peer-keyed maps, so the caller MUST hold at least
// RLock on RIBManager for newBestSource. The DRAIN needs no such lock: Next
// re-resolves each item's pool entry and retains it.
//
// The reference covers the item Next LAST handed over and nothing else, because
// Next gives the previous one back on every pull. At most one is held at a time,
// which is what makes a walk of a million rows affordable. A stage that HOLDS
// items past the pull that produced them is therefore outside that cover and
// MUST take a reference of its own: `last N` is that stage, and applyBestStages
// is where it is given one.
//
// The caller MUST call release when it stops pulling, whatever stopped it, so
// the last reference does not outlive the walk.
type bestSource struct {
	entries []bestEntry
	idx     int
	count   int

	// held is the pool entry the item Next last returned refers to. It is
	// retained inside the storage lock and given back by the next pull or by
	// release, so a row is encoded from attributes that cannot be freed under it.
	held      storage.RouteEntry
	heldValid bool
}

// bestRouteKey builds the lookup key used by the reason terminal's
// candidates-by-prefix map. Must match the key format used by newBestSource
// when populating that map.
func bestRouteKey(familyS, prefixS string) string {
	var tb textbuf.Buffer
	return tb.Str(familyS).Byte('|').Str(prefixS).String()
}

// newBestSource builds the per-prefix best-path item list. When
// stashCandidates is non-nil, the full candidate slice for every yielded
// item is written into it keyed by bestRouteKey(family, prefix). This lets
// the "reason" terminal re-run the decision process with narration without
// re-querying gatherCandidates under a second lock acquisition.
func newBestSource(r *RIBManager, selectorStr string, stashCandidates map[string][]*Candidate) *bestSource {
	sel := selector.ParseDefault(selectorStr)
	// Collect all unique (family, nlriKey) across matching peers.
	type routeKey struct {
		fam     family.Family
		nlriKey string
		familyS string
		prefixS string
	}
	seen := make(map[string]routeKey) // "familyStr|nlriKey" → routeKey

	// Caller bestPipeline holds r.peerMu.RLock across this function; the
	// bgpPeers / ribInPool iterations below are protected by that outer lock.
	collect := func(peerRIB *storage.PeerRIB) {
		// Read the ADD-PATH flags before the iteration, for the reason
		// PeerRIB.IsAddPath states: asking inside the callback takes the read
		// lock the iteration already holds, and a writer arriving between the
		// two wedges both.
		addPath := peerRIB.AddPathFamilies()
		peerRIB.IterateSorted(func(fam family.Family, nlriBytes []byte, _ storage.RouteEntry) bool {
			fStr := formatFamily(fam)
			pStr := formatNLRIAsPrefix(fam, nlriBytes, addPath[fam])
			var tb textbuf.Buffer
			key := tb.Str(fStr).Byte('|').Str(string(nlriBytes)).String()
			if _, ok := seen[key]; !ok {
				seen[key] = routeKey{fam: fam, nlriKey: string(nlriBytes), familyS: fStr, prefixS: pStr}
			}
			return true
		})
	}
	for peer, peerRIB := range r.bgpPeers {
		if !sel.Matches(peer) {
			continue
		}
		collect(peerRIB)
	}
	for _, protoPeers := range r.ribInPool {
		for peer, peerRIB := range protoPeers {
			if !sel.MatchesPeerKey(peer) {
				continue
			}
			collect(peerRIB)
		}
	}

	// Snapshot the multipath config once per call. The atomic fields are
	// written only at Stage 2 OnConfigure and rarely after, so a single
	// load is race-free and cheap.
	multipathMax := r.maximumPaths.Load()
	relaxASPath := r.relaxASPath.Load()

	// For each unique prefix, gather candidates and select best (plus any
	// multipath siblings when bgp/multipath/maximum-paths > 1).
	var entries []bestEntry
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

		// Record where the winning peer's pool entry is, rather than the entry
		// itself. Next resolves and retains it when the item is pulled, so the
		// attributes a row is built from are read under a reference and never
		// from a handle this construction happened to see.
		// Caller bestPipeline holds r.peerMu.RLock; this bgpPeers read is
		// protected by that outer lock.
		peerRIB := r.bgpPeers[best.PeerIP]

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

		entries = append(entries, bestEntry{item: item, peerRIB: peerRIB, nlri: []byte(rk.nlriKey)})
	}

	// Sort by family then prefix for stable output.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].item.Family != entries[j].item.Family {
			return family.FamilyLess(entries[i].item.Family, entries[j].item.Family)
		}
		return entries[i].item.Prefix < entries[j].item.Prefix
	})

	return &bestSource{entries: entries}
}

// Next resolves the item's pool entry and RETAINS it, so every consumer
// downstream -- a filter reading an AS path, the row that writes the answer --
// dereferences attributes a reference keeps alive rather than a handle the RIB
// may already have released.
//
// The reference of the previous item is given back here, so a filter that drops
// an item costs nothing and at most one entry is held at a time. The last one is
// given back by release, which the caller owes on every way out.
//
// A route withdrawn between construction and this pull resolves to nothing, and
// the row then carries the prefix and its winner with no attributes. That is the
// price of not holding a lock across the answer, and it is a state only a
// concurrent withdrawal reaches.
func (s *bestSource) Next() (RouteItem, bool) {
	s.release()
	if s.idx >= len(s.entries) {
		return RouteItem{}, false
	}
	entry := s.entries[s.idx]
	s.idx++
	s.count++

	item := entry.item
	if entry.peerRIB != nil {
		if held, ok := entry.peerRIB.LookupRetained(item.Family, entry.nlri); ok {
			s.held, s.heldValid = held, true
			item.HasInEntry = true
			item.InEntry = held
		}
	}
	return item, true
}

// release gives back the pool entry the last pulled item refers to. It is safe
// to call more than once and safe to call when nothing was ever pulled, so the
// caller can defer it beside the construction it pairs with.
func (s *bestSource) release() {
	if !s.heldValid {
		return
	}
	s.held.Release()
	s.heldValid = false
}

func (s *bestSource) Meta() PipelineMeta {
	return PipelineMeta{Count: s.count}
}

// --- Best-path pipeline builder ---

// bestPathEnvelopeKey is the envelope the best-path rows are answered under. It
// is the key the whole-document form carried, so a walk that ends inside the
// buffering window collapses to the object this command has always produced
// (CollapseRecords, pkg/plugin/rpc/collapse.go).
const bestPathEnvelopeKey = "best-path"

// bestPipeline builds and executes a pipeline from best-path source.
// Called by handleCommand for "show bgp rib best" with optional filter/terminal stages.
//
// With no terminal stage the answer is the WALK: the rows are handed over one at
// a time, so a table of a million best paths reaches the operator without the
// daemon holding it as one document, and no socket write happens while peerMu is
// held (bestPathRows).
//
// A terminal stage folds the walk into one number or one document here, so those
// answers are built before the answer opens and the read lock spans the drain.
// Either way the pool entry each row reads is retained by bestSource.Next and
// given back by source.release, which the deferred call below owes on every way
// out.
func (r *RIBManager) bestPipeline(selector string, args []string) any {
	pipeSelector, stages, errMsg := parseBestPipelineArgs(args)
	if errMsg != "" {
		return map[string]any{"error": errMsg}
	}
	if pipeSelector != "" {
		selector = pipeSelector
	}

	if !hasTerminal(stages) {
		return sdk.Records{Key: bestPathEnvelopeKey, Rows: r.bestPathRows(selector, stages)}
	}

	var candidatesByKey map[string][]*Candidate
	if hasReasonTerminal(stages) {
		candidatesByKey = make(map[string][]*Candidate)
	}

	// A terminal builds its whole answer here, so the read lock spans the drain
	// as it always has: nothing is written to a transport under it.
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()
	source := newBestSource(r, selector, candidatesByKey)
	defer source.release()

	current, releaseStages := applyBestStages(source, stages)
	defer releaseStages()

	// Reason terminal: drive a specialized drain that consults the stash.
	if hasReasonTerminal(stages) {
		rt := newBestReasonTerminal(current, candidatesByKey)
		return json.RawMessage(rt.Meta().JSON)
	}

	// Execute terminal — drain it and return metadata
	meta := current.Meta()
	if meta.JSON != "" {
		if hasTerminalKind(stages, "graph") {
			return meta.JSON
		}
		return json.RawMessage(meta.JSON)
	}

	// count terminal
	return map[string]any{"count": meta.Count}
}

// bestPathRows is the walk `show bgp rib best` answers with when no terminal
// folds it into something else.
//
// WHERE THE LOCK BOUNDARY IS. peerMu is taken to read the peer-keyed maps the
// item list is built from and given back before the first row is handed over.
// The answer writer appends each row and writes its line INSIDE the yield below
// (WriteRecordAnswer, pkg/plugin/rpc/answer_write.go), so a lock held across
// that yield would be a lock held across a socket write. This function holds
// none: peerMu is given back on the line after it is taken, and the storage lock
// is taken and given back inside bestSource.Next.
//
// WHAT CROSSES THE BOUNDARY. Bytes, and nothing else. Each row is encoded into a
// buffer this walk owns and the writer copies it into the line it is building,
// so no pool handle, no RIB pointer and no map reference outlives the row. The
// attributes those bytes are read from are covered by a reference bestSource.Next
// takes inside the storage lock, which is what makes the dereference safe with
// no lock held at all.
//
// Nothing here starts a goroutine, for the answer or for a row
// (ai/rules/goroutine-lifecycle.md). The walk runs on the goroutine serving the
// command, and it ends when the rows run out or when the consumer stops it,
// which is how `| first 10` bounds a read of a million best paths.
func (r *RIBManager) bestPathRows(selectorStr string, stages []pipelineStage) iter.Seq[rpc.RowRecord] {
	return func(yield func(rpc.RowRecord) bool) {
		r.peerMu.RLock()
		source := newBestSource(r, selectorStr, nil)
		r.peerMu.RUnlock()

		defer source.release()

		current, releaseStages := applyBestStages(source, stages)
		defer releaseStages()

		row := newBestRowWriter()
		for {
			item, ok := current.Next()
			if !ok {
				return
			}
			if !yield(row.record(item)) {
				return
			}
		}
	}
}

// applyBestStages builds the stage chain over a bestSource and reports the
// release that chain owes.
//
// bestSource.Next gives an item's pool reference back on the NEXT pull, so the
// reference covers the item last handed over and nothing else. A stage that
// HOLDS items past that pull therefore needs a reference of its own, and `last
// N` is the one stage that holds them: lastFilter.drain pulls the whole walk
// into a ring and emits from it afterwards. Without this, every row `show bgp
// rib best last N` produces but the final one is encoded from handles the RIB
// may already have released, and in a release build a released slot is
// re-interned with other bytes (PeerRIB.LookupRetained).
//
// The caller MUST call the returned release on every way out, beside
// bestSource.release. It is bounded by N, which is what makes the ring's
// references affordable where retaining the whole walk would not be.
// A stage is recognized as a holder by its TYPE rather than by its keyword, so
// the keyword lives in one place (pipelineStage.apply). A buffering stage added
// later that is not a lastFilter is invisible here, which is why lastFilter's
// retain field states the obligation on its own side of the pair.
func applyBestStages(source *bestSource, stages []pipelineStage) (pipelineIterator, func()) {
	var current pipelineIterator = source
	var holders []*lastFilter
	for _, stage := range stages {
		if stage.kind == bestTerminalReason {
			continue
		}
		current = stage.apply(current)
		if held, isHolder := current.(*lastFilter); isHolder {
			held.retain = true
			holders = append(holders, held)
		}
	}
	return current, func() {
		for _, held := range holders {
			held.release()
		}
	}
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

// parseBestPipelineArgs parses args for show bgp rib best (no scope keyword, filters + terminals only).
// Returns (peerSelector, stages, errorMessage).
// Validates ordering: filters must precede terminals, and at most one terminal is allowed.
//
// In addition to the generic terminalKeywords accepted across all pipelines,
// this parser also accepts bestTerminalReason ("reason") which is specific to
// the best-path pipeline -- it reports WHY a particular path won the per-
// prefix decision process.
func parseBestPipelineArgs(args []string) (string, []pipelineStage, string) {
	selector := ""
	var stages []pipelineStage
	i := 0
	sawTerminal := false
	for i < len(args) {
		keyword := args[i]
		if keyword == "peer" {
			if sawTerminal {
				return "", nil, "filter after terminal: peer"
			}
			if selector != "" {
				return "", nil, "duplicate peer filter"
			}
			i++
			if i >= len(args) {
				return "", nil, "peer requires a value"
			}
			selector = args[i]
			i++
			continue
		}

		if filterKeywords[keyword] {
			if sawTerminal {
				var tb textbuf.Buffer
				return "", nil, tb.Str("filter after terminal: ").Str(keyword).String()
			}
			i++
			if i >= len(args) {
				var tb2 textbuf.Buffer
				return "", nil, tb2.Str(keyword).Str(" requires a value").String()
			}
			if keyword == filterPath {
				if errMsg := validatePathPattern(args[i]); errMsg != "" {
					return "", nil, errMsg
				}
			}
			stages = append(stages, pipelineStage{kind: keyword, arg: args[i]})
			i++
			continue
		}

		if terminalKeywords[keyword] || keyword == bestTerminalReason {
			if sawTerminal {
				return "", nil, "multiple terminals not allowed"
			}
			sawTerminal = true
			stages = append(stages, pipelineStage{kind: keyword, terminal: true})
			i++
			continue
		}

		var tb3 textbuf.Buffer
		return "", nil, tb3.Str("unknown keyword: ").Str(keyword).String()
	}
	return selector, stages, ""
}

// --- Best-path reason terminal ---

// bestReasonTerminal drains the filtered best-path items and, for each
// surviving prefix, re-runs the decision process with narration via
// SelectBestExplain. The stashed candidate slices come from newBestSource.
// Output JSON shape:
//
//	{"best-path-reason": [{"family","prefix","winner-peer","steps":[{"step","winner","reason"}]}]}
type bestReasonTerminal struct {
	upstream        pipelineIterator
	candidatesByKey map[string][]*Candidate
	meta            PipelineMeta
	drained         bool
}

func newBestReasonTerminal(upstream pipelineIterator, candidatesByKey map[string][]*Candidate) *bestReasonTerminal {
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

// --- Best-path row ---

// bestResult is one row of the best-path answer. The field ORDER is the order
// the bytes carry, so moving a field here moves it in every answer this command
// produces, on the wire and in the document a bounded walk collapses to.
type bestResult struct {
	Family         string         `json:"family"`
	Prefix         string         `json:"prefix"`
	BestPeer       string         `json:"best-peer"`
	MultipathPeers []string       `json:"multipath-peers,omitempty"` // cmd-3: ECMP siblings (primary excluded)
	Attrs          map[string]any `json:"attributes,omitempty"`
}

// bestRowCapacity is the room one encoded best-path row is given before the
// buffer has to grow. A row carries two addresses, a prefix and the handful of
// attributes enrichRouteMapFromEntry renders, so 512 bytes holds a common one
// and the buffer keeps whatever width the widest row of the walk grew it to.
const bestRowCapacity = 512

// bestRowWriter builds one best-path row at a time and hands its bytes to the
// answer writer.
//
// ONE writer serves a whole walk. The writer appends a row into its own buffer
// before the yield that carried it returns and keeps no reference to it
// (rpc.Row), so this buffer is refilled in place for every row and a walk of a
// million rows allocates no row.
type bestRowWriter struct {
	buf     []byte
	result  bestResult
	encoder *json.Encoder
}

func newBestRowWriter() *bestRowWriter {
	w := &bestRowWriter{buf: make([]byte, 0, bestRowCapacity)}
	w.encoder = json.NewEncoder(w)
	return w
}

// Write is how the row's encoder reaches the buffer the row is read from. It
// satisfies io.Writer for that one use and is not part of the row contract.
func (w *bestRowWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

// AppendTo appends the row built by the last record call. It is rpc.Row, the
// contract the answer writer takes every row through.
func (w *bestRowWriter) AppendTo(buf []byte) []byte {
	return append(buf, w.buf...)
}

// record builds the row for item and reports it as the record the walk yields.
//
// The value is encoded through one json.Encoder rather than marshaled into a
// slice of its own, so the row's bytes land in the buffer this writer owns.
// Encoder and Marshal write the same bytes for the same value, which is what
// keeps a streamed row and the document a bounded walk collapses to identical.
//
// A value that cannot be encoded is reported as a REJECTED row rather than
// ending the answer, for the reason a row too wide for one line is: refusing one
// row must not cost the operator the rows around it. The cause goes to the log,
// because a fault payload reaches an operator.
func (w *bestRowWriter) record(item RouteItem) rpc.RowRecord {
	w.result = bestResultFor(item)
	w.buf = w.buf[:0]
	if err := w.encoder.Encode(&w.result); err != nil {
		logger().Warn("best-path row could not be encoded",
			"family", item.Family.String(), "prefix", item.Prefix, "error", err)
		var tb textbuf.Buffer
		tb.Str(`{"message":"best-path row could not be encoded","family":`).
			Quoted(item.Family.String()).
			Str(`,"prefix":`).
			Quoted(item.Prefix).
			Byte('}')
		return rpc.RowRecord{Fault: rpc.RawRow(tb.String())}
	}
	// Encode ends the value with a newline; a record line is the value alone.
	w.buf = w.buf[:len(w.buf)-1]
	return rpc.RowRecord{Item: w}
}

// bestResultFor is the value one best-path item renders to. It reads the item's
// pool entry, so it MUST run while that entry is retained (bestSource.Next).
func bestResultFor(item RouteItem) bestResult {
	br := bestResult{
		Family:         item.Family.String(),
		Prefix:         item.Prefix,
		BestPeer:       item.Peer,
		MultipathPeers: item.MultipathPeers,
	}
	if !item.HasInEntry {
		return br
	}
	attrs := make(map[string]any)
	enrichRouteMapFromEntry(attrs, item.InEntry)
	if len(attrs) > 0 {
		br.Attrs = attrs
	}
	return br
}
