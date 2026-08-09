// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- RFC 3101 sec 2.5 Type 7/Type 5 preference.
// RFC: rfc/short/rfc3101.md -- sec 2.5 (Type 7 P=1 > Type 5 > Type 7 P=0)

package spf

// External-LSA source preference (RFC 3101 sec 2.5): when the same external prefix is
// reachable via more than one LSA, a Type 7 with the P-bit set is preferred over a Type
// 5, which is preferred over a Type 7 with the P-bit clear. Lower value = preferred; it
// is the PRIMARY external key, ahead of the sec 16.4 E1/E2 cost.
const (
	prefType7P1 uint8 = 0
	prefType5   uint8 = 1
	prefType7P0 uint8 = 2
)

// ExternalPrefType5 is the RFC 3101 sec 2.5 source preference an external reader stamps on
// an AS-External (OSPFv2 Type 5 / OSPFv3 AS-External) record. ExternalPrefType7P1 /
// ExternalPrefType7P0 are the NSSA Type-7 variants the reader stamps from the LSA's P-bit
// (P=1 is preferred over a Type 5, which is preferred over P=0).
const (
	ExternalPrefType5   = prefType5
	ExternalPrefType7P1 = prefType7P1
	ExternalPrefType7P0 = prefType7P0
)
