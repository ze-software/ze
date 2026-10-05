// Related: storetrim.go -- the spawn seam the external wiring test swaps

package scratch

// StoreTrimSpawn lets the external test package, which drives the real le root
// handler and so cannot be package scratch, record the store-trim spawn instead
// of starting a child of the test binary.
var StoreTrimSpawn = &storeTrimSpawn
