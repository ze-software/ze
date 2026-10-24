// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin structured delivery
// Related: rib.go — insertPoolNLRIs, the pool receive path
// Related: rib_structured.go — the structured receive path

package rib

import (
	"github.com/ze-software/ze/internal/core/family"
)

// unstorableFamilyKey names one peer's family that the RIB cannot store.
type unstorableFamilyKey struct {
	peer   string
	family family.Family
}

// reportUnstorableFamily says that routes of fam from peer reached the RIB
// with no nlrisplit splitter to store them, so they are not stored, selected
// or re-sent while the session stays up. A Debug line left that invisible at
// the default level. It warns once per peer and family: a family the RIB
// cannot store stays unstorable for the life of the process, and a line per
// UPDATE would flood the log.
//
// The set it records into is bounded by the configured peers times the
// registered families. Safe for concurrent use.
func (r *RIBManager) reportUnstorableFamily(peer string, fam family.Family) {
	key := unstorableFamilyKey{peer: peer, family: fam}
	if _, reported := r.unstorableFamilies.LoadOrStore(key, struct{}{}); reported {
		return
	}
	logger().Warn("bgp-rib: routes of a negotiated family are not stored: no NLRI splitter is "+
		"registered for it, so they are not selected, shown or re-sent; load the plugin that "+
		"decodes the family, or remove it from the peer's `family` list",
		"peer", peer, "family", fam.String())
}
