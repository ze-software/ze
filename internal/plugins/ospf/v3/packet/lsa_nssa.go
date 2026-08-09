// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 NSSA-LSA body codec (reuses the external body).
// RFC: rfc/short/rfc5340.md (§A.4.8 NSSA-LSA)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// The NSSA-LSA (Type 0x2007) body is byte-identical to the AS-External-LSA (Type
// 0x4005) body (RFC 5340 §A.4.8): the two differ only in LS Type and flooding
// scope (NSSA is area scope, U-bit clear; AS-External is AS scope). Both decode
// and encode through ExternalLSA. The NSSA "propagate" P-bit is NOT a header
// Options bit in OSPFv3; it lives in the prefix's PrefixOptions field
// (types.OptPrefixP, 0x08), set so an NSSA ABR re-advertises the prefix as a
// Type 5 AS-External-LSA.

// NSSAPropagate reports whether an NSSA-LSA's prefix carries the P-bit
// (types.OptPrefixP) that asks the NSSA ABR to translate it to a Type 5 LSA.
func NSSAPropagate(l ExternalLSA) bool { return l.Prefix.Options.Has(types.OptPrefixP) }
