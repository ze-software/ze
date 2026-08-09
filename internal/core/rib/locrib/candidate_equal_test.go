// Design: docs/architecture/rib/unified-locrib.md -- Path.Equal label handling (F1) test
package locrib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: F1 -- Path.Equal compares the MPLS label stack, so a relabel (same
// next hop, new label) is detected as a change and propagated to the FIB rather
// than suppressed.
func TestPathEqualComparesLabels(t *testing.T) {
	base := Path{Source: 1, NextHop: netip.MustParseAddr("10.0.0.2"), Labels: []uint32{100}}

	same := base
	assert.True(t, base.Equal(same), "identical paths are equal")

	relabel := base
	relabel.Labels = []uint32{200}
	assert.False(t, base.Equal(relabel), "a relabel must be detected as a change")

	unlabeled := Path{Source: 1, NextHop: netip.MustParseAddr("10.0.0.2")}
	assert.False(t, base.Equal(unlabeled), "adding/removing labels is a change")
}
