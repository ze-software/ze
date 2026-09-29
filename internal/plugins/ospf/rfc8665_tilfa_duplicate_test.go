// Design: docs/architecture/ospf/ospf-ext-6-ti-lfa.md -- the TI-LFA SRResolver.
// Related: sr_tilfa.go -- srTILFAResolver.PrefixSIDLabel, the second reader of the received
// Prefix-SIDs these tests drive.
//
// VALIDATES: RFC 8665 section 5 on the TI-LFA repair path: a Prefix-SID that its router
// advertises more than once for the same prefix, topology and algorithm is ignored there too.
// PREVENTS: a TI-LFA repair stack built from a Prefix-SID the RFC says to ignore.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC8665-5-6 positive -- a router that advertises ONE Prefix-SID (absolute
// label 20009) for its node prefix 10.0.0.9/32 is resolved by the TI-LFA resolver to 20009.
// RFC requirement: RFC8665-5-6 negative -- once the same router advertises a second Prefix-SID
// for the same prefix, topology and algorithm (in a second Extended Prefix LSA), the TI-LFA
// resolver ignores both and resolves no label for the router.
func TestRFC8665TILFAIgnoresDuplicatePrefixSIDs(t *testing.T) {
	// Goal: the duplicate verdict reaches the TI-LFA reader, not only the installer. Method:
	// an absolute-label SID needs no SRGB, so the only reason to resolve nothing is the
	// duplicate.
	srTestReset(t)
	eng, _ := newRedistEngine(t, extOrigCfg)
	adv := types.RouterID{10, 0, 0, 9}
	sid := sr.PrefixSID{Flags: sr.SIDFlags{V: true, L: true}, IsLabel: true, Label: 20009}
	resolver := srTILFAResolver{e: eng}

	srInstallPrefixSIDs(t, eng, adv, types.BackboneArea, 1, sid)
	if label, ok := resolver.PrefixSIDLabel(adv); !ok || label != 20009 {
		t.Fatalf("one Prefix-SID: label = %d (ok %v), want 20009", label, ok)
	}

	srInstallPrefixSIDs(t, eng, adv, types.BackboneArea, 2, sid)
	if label, ok := resolver.PrefixSIDLabel(adv); ok {
		t.Fatalf("two Prefix-SIDs for one prefix, topology and algorithm: TI-LFA resolved label %d, want none (all MUST be ignored)", label)
	}
}
