package pool

// Attributes is the global pool for BGP path attributes.
//
// Routes store handles referencing this pool instead of copying
// attribute bytes. Identical attributes across routes share storage,
// reducing memory usage by 80-90% for route reflector scenarios.
//
// Usage:
//
//	h := pool.Attributes.Intern(attrBytes)
//	defer pool.Attributes.Release(h)
//	data, err := pool.Attributes.Get(h)
var Attributes = NewWithIdx(0, 1<<20) // idx=0, 1MB initial

// NLRI is the global pool for BGP NLRI (Network Layer Reachability Information).
//
// Routes store handles referencing this pool for zero-copy NLRI forwarding.
// Identical NLRI across routes share storage.
//
// Usage:
//
//	h := pool.NLRI.Intern(nlriBytes)
//	defer pool.NLRI.Release(h)
//	data, err := pool.NLRI.Get(h)
var NLRI = NewWithIdx(1, 1<<18) // idx=1, 256KB initial
