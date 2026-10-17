// Design: docs/architecture/resolve.md -- Clearing a resolver cache
// Related: types.go -- DeclareRegistrationInput.ResolverCaches, the Stage 1 declaration
//
// The resolver-cache-clear callback. A plugin that holds its own PeeringDB,
// IRR or Team Cymru client declares the caches it owns at Stage 1, and the
// engine asks it to clear them when an operator runs clear <cache> cache. The
// plugin answers for the clients it currently uses, which is what makes the
// count exact: a client it replaced is no longer its own and is not counted.

package rpc

import "slices"

// MethodResolverCacheClear asks a plugin to clear one of the resolver caches
// it declared in DeclareRegistrationInput.ResolverCaches.
const MethodResolverCacheClear = "ze-plugin-callback:resolver-cache-clear"

// The resolver caches a plugin can declare, spelled as the noun of the clear
// command that empties them (clear peeringdb cache, clear irr cache, clear
// cymru cache).
const (
	ResolverCachePeeringDB = "peeringdb"
	ResolverCacheIRR       = "irr"
	ResolverCacheCymru     = "cymru"
)

// The two clears a ResolverCacheClearInput asks for. The spelling is the
// action the operator's answer reports.
const (
	ResolverCacheClearAll    = "clear-all"
	ResolverCacheDeleteEntry = "delete-entry"
)

// resolverCacheNames is the one declaration of the resolver caches a plugin
// can declare, in the order a refusal lists them. KnownResolverCache and the
// Stage 1 refusal both read it, so a cache added here is accepted and named
// with no second edit.
var resolverCacheNames = []string{ResolverCachePeeringDB, ResolverCacheIRR, ResolverCacheCymru}

// ResolverCacheNames returns a copy of the resolver caches a plugin can
// declare, in the order a refusal lists them.
func ResolverCacheNames() []string {
	return slices.Clone(resolverCacheNames)
}

// KnownResolverCache reports whether name is a resolver cache a plugin can
// declare. The engine refuses any other name at Stage 1, because a misspelled
// declaration would otherwise leave the plugin's cache out of every clear with
// nothing said.
func KnownResolverCache(name string) bool {
	return slices.Contains(resolverCacheNames, name)
}

// ResolverCacheClearInput is the input for ze-plugin-callback:resolver-cache-clear.
//
// Action is ResolverCacheClearAll, which empties the cache, or
// ResolverCacheDeleteEntry, which removes the entries of Key. Key is empty for
// clear-all, and for delete-entry it is the key the engine already parsed: a
// decimal AS number for peeringdb, an AS-SET name for irr, an address in its
// canonical text for cymru.
type ResolverCacheClearInput struct {
	Cache  string `json:"cache"`
	Action string `json:"action"`
	Key    string `json:"key,omitempty"`
}

// ResolverCacheClearOutput is the output for ze-plugin-callback:resolver-cache-clear.
// Removed counts the cache entries the clear removed from the plugin's own
// clients: every entry for clear-all, the entries of the key for delete-entry.
type ResolverCacheClearOutput struct {
	Removed int `json:"removed"`
}
