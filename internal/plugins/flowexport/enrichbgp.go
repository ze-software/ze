// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- BGP RIB enrichment wiring
// Related: enrich/enricher.go -- atomic radix tree the builder feeds
// Related: enrich/radix.go -- prefix-to-AS tree rebuilt from RIB updates

package flowexport

import (
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/plugins/flowexport/enrich"
	"github.com/ze-software/ze/pkg/ze"
)

// bgpEnrichBuilder subscribes to BGP RIB best-path changes and maintains
// the prefix-to-AS radix tree consumed by the Enricher.
//
// The RIB best-change event (ribevents.BestChangeEntry) carries the prefix,
// next-hop, origin AS, and full AS_PATH of the winning route, so the tree is
// populated with all of them. Full-table replay events omit the AS data (the
// stored best-path record does not keep the AS_PATH handle); those entries are
// corrected by the next incremental change.
//
// Updates accumulate into a map (cheap, incremental). A long-lived worker
// rebuilds the immutable radix tree at most once per rebuildInterval when
// the map is dirty, then swaps it atomically into the Enricher so per-flow
// readers never see a partial tree.
type bgpEnrichBuilder struct {
	enricher *enrich.Enricher

	mu    sync.Mutex
	table map[netip.Prefix]enrich.ASEntry
	dirty bool

	stopCh  chan struct{}
	doneCh  chan struct{}
	started bool
	unsub   func()
}

// enrichRebuildInterval debounces tree rebuilds. Each rebuild reconstructs the
// whole radix tree from the prefix table, which is O(N) allocations (~tens of
// MB for a full BGP table). BGP convergence settles over tens of seconds, so a
// few-second debounce trades minor enrichment staleness for a large drop in GC
// pressure under route churn. An incremental/persistent-trie rebuild is the
// longer-term follow-up (see docs/architecture/flowexport/flow-export-2-flow-records.md).
const enrichRebuildInterval = 5 * time.Second

func newBGPEnrichBuilder(e *enrich.Enricher) *bgpEnrichBuilder {
	return &bgpEnrichBuilder{
		enricher: e,
		table:    make(map[netip.Prefix]enrich.ASEntry),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// applyBatch folds one best-change batch into the prefix table. Withdrawals
// remove the prefix; adds and updates set its next-hop. Marks the table
// dirty so the next rebuild tick republishes the tree.
func (b *bgpEnrichBuilder) applyBatch(batch *ribevents.BestChangeBatch) {
	if batch == nil || len(batch.Changes) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range batch.Changes {
		ch := &batch.Changes[i]
		if ch.Action == ribevents.BestChangeWithdraw {
			delete(b.table, ch.Prefix)
			continue
		}
		b.table[ch.Prefix] = enrich.ASEntry{
			AS:      ch.OriginAS,
			NextHop: ch.NextHop,
			ASPath:  ch.ASPath,
		}
	}
	b.dirty = true
}

// rebuild constructs a fresh radix tree from the current table and swaps it
// into the Enricher. No-op when the table is not dirty.
func (b *bgpEnrichBuilder) rebuild() {
	b.mu.Lock()
	if !b.dirty {
		b.mu.Unlock()
		return
	}
	tree := enrich.NewRadixTree()
	for prefix, entry := range b.table {
		tree.Insert(prefix, entry)
	}
	b.dirty = false
	b.mu.Unlock()

	b.enricher.UpdateTree(tree)
}

// Start subscribes to RIB best-change events and launches the rebuild
// worker. Safe to call once; Stop releases both.
func (b *bgpEnrichBuilder) Start(eb ze.EventBus) {
	b.started = true
	if eb != nil {
		b.unsub = ribevents.BestChange.Subscribe(eb, b.applyBatch)
	}
	go func() {
		defer close(b.doneCh)
		ticker := time.NewTicker(enrichRebuildInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.rebuild()
			case <-b.stopCh:
				return
			}
		}
	}()
}

// Stop unsubscribes from the bus and stops the rebuild worker, waiting for the
// goroutine to exit so teardown is complete on return (matches the sampling and
// conntrack worker contract).
func (b *bgpEnrichBuilder) Stop() {
	if b.unsub != nil {
		b.unsub()
		b.unsub = nil
	}
	close(b.stopCh)
	if b.started {
		<-b.doneCh
	}
}
