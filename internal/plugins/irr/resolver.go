// Design: docs/architecture/irr-owner.md -- the one IRR and PeeringDB client set
// Related: owner.go -- the worker, the only caller
//
// The owner is the only holder of an IRR whois client and a PeeringDB client
// for the data Ze enforces. Resolution, the per-family last-known-good merge
// and the empty-answer rule are the shared PrefixStore's, run here without
// persistence: the owner stores each answer itself, as a dataset.

package irr

import (
	"context"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// resolver is what the worker asks of the network. The production one is a
// storeResolver; a test hands the owner one that answers without a socket.
// It is called from the worker goroutine only.
type resolver interface {
	// refresh fetches asSet from the server and merges the answer into the
	// last-known-good copy (store.PrefixStore.Refresh). It never asks
	// PeeringDB: discover does, and the worker refreshes the set it chose.
	refresh(ctx context.Context, asSet string) (*store.CachedEntry, error)
	// seed installs a stored dataset as the copy the next answer merges into.
	seed(entry store.CachedEntry) error
	// forget drops the copy of asSet, after `clear irr` removed it.
	forget(asSet string)
	// discover answers the AS-SET PeeringDB names for asn: its first set, or
	// the literal AS<asn> when PeeringDB holds none. An error means PeeringDB
	// did not answer, which decides nothing.
	discover(ctx context.Context, asn uint32) (string, error)
	// useClients points the resolver at cfg's source address and PeeringDB
	// URL, keeping its copies: the server is unchanged.
	useClients(cfg ownerConfig)
	// clearCache empties one client cache for `clear irr|peeringdb cache`.
	clearCache(input rpc.ResolverCacheClearInput) (int, error)
}

// storeResolver is the production resolver over one server.
type storeResolver struct {
	prefixes *store.PrefixStore
	pdb      *peeringdb.PeeringDB
}

// newStoreResolver builds the clients cfg names. A new server gets a new
// resolver, so an answer from it never merges into another server's copy.
func newStoreResolver(cfg ownerConfig) resolver {
	whois, pdb := newClients(cfg)
	return &storeResolver{prefixes: store.New(whois, pdb), pdb: pdb}
}

// newClients builds the whois client of cfg's server and source address and
// the PeeringDB client of its URL.
func newClients(cfg ownerConfig) (*irr.IRR, *peeringdb.PeeringDB) {
	whois := irr.NewIRR(cfg.server)
	if cfg.sourceAddress != "" {
		whois.SetSourceAddress(cfg.sourceAddress)
	}
	return whois, peeringdb.NewPeeringDB(cfg.peeringDBURL)
}

func (r *storeResolver) refresh(ctx context.Context, asSet string) (*store.CachedEntry, error) {
	return r.prefixes.Refresh(ctx, asSet)
}

func (r *storeResolver) seed(entry store.CachedEntry) error {
	return r.prefixes.Seed(entry)
}

func (r *storeResolver) forget(asSet string) {
	r.prefixes.Purge(asSet)
}

func (r *storeResolver) discover(ctx context.Context, asn uint32) (string, error) {
	sets, err := r.pdb.LookupASSet(ctx, asn)
	if err != nil {
		return "", err
	}
	if len(sets) == 0 {
		return literalASSet(asn), nil
	}
	return store.ASSetName(sets[0])
}

func (r *storeResolver) useClients(cfg ownerConfig) {
	whois, pdb := newClients(cfg)
	r.prefixes.UseClients(whois, pdb)
	r.pdb = pdb
}

func (r *storeResolver) clearCache(input rpc.ResolverCacheClearInput) (int, error) {
	return r.prefixes.ClearResolverCache(input)
}

// literalASSet is the set queried for an AS number PeeringDB names none for:
// the aut-num itself, AS<asn>.
func literalASSet(asn uint32) string {
	return "AS" + textbuf.StringUint32(asn)
}
