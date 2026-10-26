// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- virtual-IP pools and the lease registry
// RFC: rfc/short/rfc3948.md -- locally unique inner addresses (Section 5.1)
// Related: apply.go -- applyConfig reloads the pools on every configuration
// Related: table.go -- the SATable carries this state to every peer session

package engine

import (
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/eap"
)

// virtualIPState is the engine's virtual-IP state: one lease registry that lives as long as
// the engine, and the current generation of pool descriptors. The registry is the only
// uniqueness authority (RFC 3948 Section 5.1), and it outlives every pool edit, so a pool
// rebuilt on reload can never hand a live address to a second peer.
//
// Safe for concurrent use. The registry carries its own mutex, and pools is an atomic
// pointer that only applyConfig, the engine's one writer, stores; peer sessions load it.
type virtualIPState struct {
	leases *eap.Registry
	pools  atomic.Pointer[virtualIPPools]
}

// virtualIPPools is one immutable generation of pool descriptors, with the configuration
// each was built from so the next reload can tell an edited pool from an unedited one.
type virtualIPPools struct {
	byName map[string]*eap.Pool
	cfg    map[string]ipsec.VirtualIPPool
}

// newVirtualIPState returns the state with an empty registry and no pools.
func newVirtualIPState() *virtualIPState {
	return &virtualIPState{leases: eap.NewRegistry()}
}

// pool returns the current descriptor of the pool named name.
func (v *virtualIPState) pool(name string) (*eap.Pool, bool) {
	gen := v.pools.Load()
	if gen == nil {
		return nil, false
	}
	p, ok := gen.byName[name]
	return p, ok
}

// reload publishes the pools of ra as the next generation. A pool whose configuration is
// unchanged keeps its descriptor, and so its rotating cursor; an edited one is rebuilt from
// what the operator wrote, DNS included, because the DNS servers are sent to a client too.
// The registry is not touched: a live lease outside every range after an edit stays held
// until its IKE SA ends. A pool that fails to build is left out and named at Warn, so a
// peer bound to it is refused an address rather than served from a stale range; commit
// validation refuses such a pool first, so this is reached only by a config that bypassed it.
//
// MUST be called only from applyConfig, the engine's single writer.
func (v *virtualIPState) reload(ra *ipsec.RemoteAccessConfig, log *slog.Logger) {
	next := &virtualIPPools{
		byName: make(map[string]*eap.Pool),
		cfg:    make(map[string]ipsec.VirtualIPPool),
	}
	if ra == nil {
		v.pools.Store(next)
		return
	}
	prev := v.pools.Load()
	for _, name := range slices.Sorted(maps.Keys(ra.Pools)) {
		cfg := ra.Pools[name]
		if prev != nil {
			if old, ok := prev.byName[name]; ok && reflect.DeepEqual(prev.cfg[name], cfg) {
				next.byName[name] = old
				next.cfg[name] = cfg
				continue
			}
		}
		pool, err := eap.NewPool(name, cfg.Range, cfg.Range6, cfg.DNS)
		if err != nil {
			log.Warn("ike: virtual IP pool not built, so peers bound to it get no address; "+
				"correct the pool under vpn ipsec remote-access pool and commit again",
				"pool", name, "error", err)
			continue
		}
		next.byName[name] = pool
		next.cfg[name] = cfg
		log.Info("ike: virtual IP pool ready", "pool", name, "range", cfg.Range, "range6", cfg.Range6)
	}
	v.pools.Store(next)
}

// boundVirtualIPPool returns the pool this peer is bound to. bound is false for a peer with
// no virtual-ip-pool leaf. For a bound peer, ok is false when the pool cannot be resolved,
// which is a state only a defect or a bypassed commit check reaches; the caller MUST then
// refuse the Child SA rather than install the peer's self-chosen inner address.
func (ps *PeerSession) boundVirtualIPPool() (pool *eap.Pool, bound, ok bool) {
	name := ps.peerCfg.VirtualIPPoolName()
	if name == "" {
		return nil, false, false
	}
	if ps.virtualIP == nil {
		return nil, true, false
	}
	pool, ok = ps.virtualIP.pool(name)
	return pool, true, ok
}
