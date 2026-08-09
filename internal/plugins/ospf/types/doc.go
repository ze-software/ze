// Design: docs/architecture/ospf/ospf-1-types.md -- OSPFv2 leaf domain value types
// Related: routerid.go -- Router ID fixed-width identifier
// Related: lsakey.go -- LSDB key tuple
// Related: checksum.go -- OSPFv2 checksum algorithms

// Package types contains pure OSPFv2 value types and checksum algorithms.
//
// The package is intentionally a leaf: no network I/O, timers, goroutines, or
// imports from OSPF runtime packages, IS-IS, BGP-LS, or other protocol engines.
package types
