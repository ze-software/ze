// Related: storetrim.go -- the spawn seam the external wiring test swaps

package scratch

// StoreTrimSpawn lets the external test package, which drives the real le root
// handler and so cannot be package scratch, record the scratch trim spawn instead
// of starting a child of the test binary.
var StoreTrimSpawn = &storeTrimSpawn

// TrimSpawn names the spawn seam's type for the same external test.
type TrimSpawn = trimSpawn
