// Design: docs/architecture/ospf/ospfv3-1-types.md -- OSPFv3 leaf value-type package.

// Package types holds the OSPFv3 (RFC 5340) leaf value types shared by every later
// OSPFv3 child spec: Router ID, Area ID, Instance ID, Interface ID, Link State ID, the
// 16-bit LS Type with embedded flooding scope, the comparable LSA key, sequence numbers,
// ages, the 24-bit Options field, IPv6 prefix length/options, and metrics.
//
// It is pure value code -- no sockets, timers, goroutines, config loading, plugin
// lifecycle, LSDB maps, or route installation -- and imports no OSPFv2 or other runtime
// package (enforced by TestOSPFv3TypesNoRuntimeImports). OSPFv2 and OSPFv3 share Router
// ID and LSA concepts, but OSPFv3's wider LS Type, Instance ID, Interface ID, 24-bit
// Options, and IPv6 prefix encoding are different enough that sharing would leak detail.
package types
