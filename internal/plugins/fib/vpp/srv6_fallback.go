// Design: docs/architecture/fib/fib-depth-4-srv6.md -- durable IP/SRv6 transitions.
package fibvpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// srv6Fallback proves a successful ordinary API installation, not ownership
// inferred from a matching dump. VPP dumps expose the best FIB source, not an
// API-source route hidden by SR steering. An unconfirmed mutation is ambiguous.
type srv6Fallback struct {
	Prefix    netip.Prefix `json:"prefix"`
	Table     uint32       `json:"table"`
	NextHop   string       `json:"next-hop"`
	MPLS      bool         `json:"mpls"`
	Confirmed bool         `json:"confirmed"`
}

// reserveState MUST cover the entire transition, including temporary records,
// before the first write. Replacements and withdrawals of existing keys consume
// no slots. Reusing the RPC contract keeps every admitted namespace enumerable.
func (b *govppSRv6Backend) reserveState(growth int) error {
	used := len(b.policies) + len(b.routes) + len(b.fallbacks)
	if growth > rpc.StateListMax-used {
		return fmt.Errorf("SRv6 ownership capacity: %d keys plus %d required exceeds RPC limit %d", used, growth, rpc.StateListMax)
	}
	return nil
}

func (b *govppSRv6Backend) steeringGrowth(key srv6RouteKey, sid netip.Addr) int {
	growth := 0
	if b.routes[key] == nil {
		growth++
	}
	if b.bySID[sid] == nil {
		growth++
	}
	return growth
}

func (b *govppSRv6Backend) saveFallback(fallback *srv6Fallback) error {
	data, err := json.Marshal(fallback)
	if err != nil {
		return fmt.Errorf("encode SRv6 fallback ownership: %w", err)
	}
	key := srv6RouteKey{prefix: fallback.Prefix, table: fallback.Table}
	if err := b.store.WriteKey(srv6PrefixStateKey("fallback", key), data); err != nil {
		b.ready = false
		return fmt.Errorf("persist SRv6 fallback ownership: %w", err)
	}
	b.fallbacks[key] = fallback
	return nil
}

func (b *govppSRv6Backend) removeFallback(key srv6RouteKey) error {
	if b.fallbacks[key] == nil {
		return nil
	}
	if err := b.store.RemoveKey(srv6PrefixStateKey("fallback", key)); err != nil {
		b.ready = false
		return fmt.Errorf("remove SRv6 fallback ownership: %w", err)
	}
	delete(b.fallbacks, key)
	return nil
}

// checkpointSRv6Fallback records ordinary ownership already established by a
// successful consumer operation BEFORE steering can hide that route. Admission
// accounts for this record and every policy/steering record together.
func (f *fibVPP) checkpointSRv6Fallback(key srv6RouteKey, sid netip.Addr) error {
	b, ok := f.srv6Backend.(*govppSRv6Backend)
	if !ok {
		return nil
	}
	old, ordinary := f.installed[key]
	mpls := key.table == f.srv6TableID && f.mplsInstalled[key.prefix.String()]
	growth := b.steeringGrowth(key, sid)
	if (ordinary || mpls) && b.fallbacks[key] == nil {
		growth++
	}
	if err := b.reserveState(growth); err != nil {
		return err
	}
	if !ordinary && !mpls {
		return nil
	}
	if b.fallbacks[key] != nil {
		return nil
	}
	return b.saveFallback(&srv6Fallback{Prefix: key.prefix, Table: key.table,
		NextHop: old.nextHop, MPLS: mpls, Confirmed: true})
}

// beginSRv6Fallback persists intent before creating ordinary forwarding. Known
// API rejection can roll intent back; uncertain transport results cannot.
// The previous record is nil when no durable backend or prior record exists.
// On success, the caller MUST pass it to finishSRv6Fallback for rollback.
func (f *fibVPP) beginSRv6Fallback(c *incomingChange, key srv6RouteKey, previous **srv6Fallback) error {
	*previous = nil
	b, ok := f.srv6Backend.(*govppSRv6Backend)
	if !ok {
		return nil
	}
	*previous = b.fallbacks[key]
	growth := 0
	if *previous == nil {
		growth = 1
	}
	if err := b.reserveState(growth); err != nil {
		return err
	}
	pending := &srv6Fallback{Prefix: key.prefix, Table: key.table,
		NextHop: c.NextHop.String(), MPLS: len(c.Labels) != 0}
	return b.saveFallback(pending)
}

// finishSRv6Fallback MUST receive the previous record from beginSRv6Fallback.
// With no durable backend, only the forwarding mutation result is returned.
func (f *fibVPP) finishSRv6Fallback(key srv6RouteKey, old *srv6Fallback, mutationErr error) error {
	b, ok := f.srv6Backend.(*govppSRv6Backend)
	if !ok {
		return mutationErr
	}
	if mutationErr != nil {
		if errors.Is(mutationErr, errVPPMutationUncertain) {
			b.ready = false
			return fmt.Errorf("unconfirmed IP mutation for %s table %d: %w", key.prefix, key.table, mutationErr)
		}
		if old != nil {
			return errors.Join(mutationErr, b.saveFallback(old))
		}
		return errors.Join(mutationErr, b.removeFallback(key))
	}
	confirmed := *b.fallbacks[key]
	confirmed.Confirmed = true
	return b.saveFallback(&confirmed)
}
