// Design: docs/architecture/rib/unified-locrib.md -- selected-feed prefix identity.

package sysrib

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
)

// overflowKey names one prefix retained by the current selected-view feed.
type overflowKey struct {
	fam    family.Family
	prefix netip.Prefix
}
