// Design: docs/architecture/rib/unified-locrib.md -- selected-route projection.
// Related: nhresolver.go -- the shared recursive next-hop reader.

package sysrib

import (
	"encoding/json"
	"fmt"
	"math"
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/store"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectedProjection holds only the daemon's selected values. Its zero value
// and nil pointer are unavailable. The owner MUST serialize Apply with reads.
// A reset discards both tables and requires a new Open; an empty completed
// table remains synchronized. No candidate admission or selection runs here.
type selectedProjection struct {
	session  string
	epoch    string
	snapshot string
	cursor   uint64
	families map[family.Family]struct{}
	staging  projectionTable
	current  projectionTable
}

type projectionTable map[family.Family]*store.Store[*selectedProjectionPath]

// selectedProjectionPath retains metadata the existing local LPM path cannot
// represent. Callbacks MUST NOT mutate or retain its referenced slices.
type selectedProjectionPath struct {
	path    locrib.Path
	members []projectionMemberMetadata
}

type projectionMemberMetadata struct {
	source redistevents.ProtocolID
	labels []uint32
	sid    netip.Addr
}

type projectionKey struct {
	family family.Family
	prefix netip.Prefix
}

func newSelectedProjection(open rpc.RouteViewOpenOutput) (*selectedProjection, error) {
	if open.Session == "" {
		return nil, fmt.Errorf("routing view has no session; reopen the system routing view")
	}
	if open.Epoch == "" {
		return nil, fmt.Errorf("routing view has no epoch; reopen the system routing view")
	}
	if open.Snapshot == "" {
		return nil, fmt.Errorf("routing view has no snapshot; reopen the system routing view")
	}
	if open.Cursor != 0 {
		return nil, fmt.Errorf("routing view opens at cursor %d instead of zero; reopen the system routing view", open.Cursor)
	}
	p := &selectedProjection{session: open.Session, epoch: open.Epoch, snapshot: open.Snapshot, families: make(map[family.Family]struct{}), staging: make(projectionTable)}
	// Stage 2 follows the shared registration seal. Resolve names once here;
	// page admission compares numeric families and never registers wire input.
	for _, name := range family.RegisteredFamilyNames() {
		fam, ok := family.LookupFamily(name)
		if !ok {
			return nil, fmt.Errorf("registered routing family %q disappeared; restart the routing view", name)
		}
		p.families[fam] = struct{}{}
	}
	return p, nil
}

// Apply validates and owns the whole page before changing either table. Callers
// MUST finish Apply before resolving any returned dependent prefix. Snapshot
// records stay private until Complete; the final result names the complete
// selected table, not only the last page. A failed page preserves all state.
func (p *selectedProjection) Apply(page *rpc.RouteViewReadOutput) ([]projectionKey, error) {
	if p == nil {
		return nil, fmt.Errorf("routing view is unavailable; open the system routing view")
	}
	if page == nil {
		return nil, fmt.Errorf("routing view page is missing; read the system routing view again")
	}
	if page.Session != p.session {
		return nil, fmt.Errorf("routing view page belongs to another session; reopen the system routing view")
	}
	if page.Epoch != p.epoch {
		return nil, fmt.Errorf("routing view epoch changed; reopen the system routing view")
	}
	if p.session == "" {
		return nil, fmt.Errorf("routing view is not open; open the system routing view")
	}
	if p.staging == nil && p.current == nil {
		return nil, fmt.Errorf("routing view was reset; reopen the system routing view")
	}
	if len(page.Entries) > rpc.RouteViewRecordsMax {
		return nil, fmt.Errorf("routing view page exceeds %d records; request a smaller page", rpc.RouteViewRecordsMax)
	}
	if err := p.validateSequence(page); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		return nil, fmt.Errorf("routing view page cannot be encoded; read the system routing view again: %w", err)
	}
	if len(encoded) > rpc.RouteViewResultBudget() {
		return nil, fmt.Errorf("routing view page exceeds the reply frame; request a smaller page")
	}
	if page.Phase == rpc.RouteViewPhaseReset {
		p.staging = nil
		p.current = nil
		p.cursor = page.Cursor
		return nil, nil
	}
	ops := make([]projectionOperation, len(page.Entries))
	for i := range page.Entries {
		if err := p.prepare(&page.Entries[i], &ops[i]); err != nil {
			return nil, err
		}
	}
	table := p.current
	if page.Phase == rpc.RouteViewPhaseSnapshot {
		table = p.staging
	}
	keys := make([]projectionKey, 0, len(ops))
	for i := range ops {
		op := &ops[i]
		index := table[op.key.family]
		if op.remove {
			if index != nil {
				index.Delete(op.key.prefix)
			}
		} else {
			if index == nil {
				index = store.NewStore[*selectedProjectionPath](op.key.family)
				table[op.key.family] = index
			}
			index.Insert(op.key.prefix, op.value)
		}
		keys = append(keys, op.key)
	}
	p.cursor = page.Cursor
	if page.Phase == rpc.RouteViewPhaseSnapshot {
		if !page.Complete {
			return nil, nil
		}
		p.current = p.staging
		p.staging = nil
		keys = keys[:0]
		p.Visit(func(key projectionKey, _ *selectedProjectionPath) bool { keys = append(keys, key); return true })
	}
	return keys, nil
}

func (p *selectedProjection) validateSequence(page *rpc.RouteViewReadOutput) error {
	if page.RejectedPublication != 0 {
		if page.Phase != rpc.RouteViewPhaseReset {
			return fmt.Errorf("routing view publication proof is not a reset; read a valid reset before reopening the system routing view")
		}
		if page.Snapshot != "" {
			return fmt.Errorf("routing view publication reset proof names a snapshot; read a valid reset before reopening the system routing view")
		}
	}
	if page.Snapshot != "" {
		if page.Snapshot != p.snapshot {
			return fmt.Errorf("routing view page names another snapshot; reopen the system routing view")
		}
	}
	advance := uint64(len(page.Entries))
	switch page.Phase {
	case rpc.RouteViewPhaseSnapshot:
		if p.staging == nil {
			return fmt.Errorf("routing view snapshot already completed; read its next delta")
		}
		if page.Snapshot != p.snapshot {
			return fmt.Errorf("routing view snapshot changed; reopen the system routing view")
		}
		if page.Complete {
			advance++
		}
	case rpc.RouteViewPhaseDelta:
		if !p.Synchronized() {
			return fmt.Errorf("routing view snapshot is incomplete; finish it before reading deltas")
		}
		if !page.Complete {
			return fmt.Errorf("routing view delta lacks completion; read the system routing view again")
		}
	case rpc.RouteViewPhaseReset:
		if len(page.Entries) != 0 {
			return fmt.Errorf("routing view reset carries records; reopen the system routing view")
		}
		if page.Complete {
			return fmt.Errorf("routing view reset claims completion; reopen the system routing view")
		}
		if page.Cursor < p.cursor {
			return fmt.Errorf("routing view reset cursor %d precedes %d; reopen the system routing view", page.Cursor, p.cursor)
		}
		return nil
	case rpc.RouteViewPhaseUnspecified:
		return fmt.Errorf("routing view phase is missing; read the system routing view again")
	default: // A Direct caller can supply an unknown numeric phase.
		return fmt.Errorf("routing view phase %d is unknown; read the system routing view again", page.Phase)
	}
	if advance > math.MaxUint64-p.cursor {
		return fmt.Errorf("routing view cursor exhausted; reopen the system routing view")
	}
	if page.Cursor != p.cursor+advance {
		return fmt.Errorf("routing view cursor %d does not follow %d by %d records; reopen the system routing view", page.Cursor, p.cursor, advance)
	}
	return nil
}

func (p *selectedProjection) Synchronized() bool {
	if p == nil {
		return false
	}
	return p.current != nil
}

// Cursor is the consumed frontier, including private snapshot pages. Callers
// MUST also check Synchronized before publishing or replaying any output.
func (p *selectedProjection) Cursor() uint64 { return p.cursor }

// Inspect borrows one selected value for the duration of fn. The callback MUST
// NOT mutate or retain the value or its slices, and MUST NOT call Apply.
func (p *selectedProjection) Inspect(fam family.Family, prefix netip.Prefix, fn func(*selectedProjectionPath)) bool {
	if !p.Synchronized() {
		return false
	}
	index := p.current[fam]
	if index == nil {
		return false
	}
	value, found := index.Lookup(prefix)
	if !found {
		return false
	}
	fn(value)
	return true
}

// Visit borrows each selected value under the same obligations as Inspect.
func (p *selectedProjection) Visit(fn func(projectionKey, *selectedProjectionPath) bool) {
	if !p.Synchronized() {
		return
	}
	for fam, index := range p.current {
		keepGoing := true
		index.Iterate(func(prefix netip.Prefix, value *selectedProjectionPath) bool {
			keepGoing = fn(projectionKey{family: fam, prefix: prefix}, value)
			return keepGoing
		})
		if !keepGoing {
			return
		}
	}
}

// LPM returns a selected path with read-only borrowed slices. Unavailable and
// empty tables both miss; callers MUST use Synchronized to distinguish them.
func (p *selectedProjection) LPM(fam family.Family, addr netip.Addr) (locrib.Path, netip.Prefix, bool) {
	if !p.Synchronized() {
		return locrib.Path{}, netip.Prefix{}, false
	}
	index := p.current[fam]
	if index == nil {
		return locrib.Path{}, netip.Prefix{}, false
	}
	value, prefix, found := index.LookupLPM(addr)
	if !found {
		return locrib.Path{}, netip.Prefix{}, false
	}
	return value.path, prefix, true
}
