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
//	data := pool.Attributes.Get(h)
var Attributes = NewPool(PoolConfig{
	InitialBufferSize: 1 << 20, // 1MB initial
	ExpectedEntries:   10000,   // ~10K unique attribute sets
})

// NLRI is the global pool for BGP NLRI (Network Layer Reachability Information).
//
// Routes store handles referencing this pool for zero-copy NLRI forwarding.
// Identical NLRI across routes share storage.
//
// Usage:
//
//	h := pool.NLRI.Intern(nlriBytes)
//	defer pool.NLRI.Release(h)
//	data := pool.NLRI.Get(h)
var NLRI = NewPool(PoolConfig{
	InitialBufferSize: 1 << 18, // 256KB initial (NLRI typically smaller)
	ExpectedEntries:   50000,   // More NLRI entries than attribute sets
})
