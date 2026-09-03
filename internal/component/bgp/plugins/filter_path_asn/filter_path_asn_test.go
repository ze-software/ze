// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
package filter_path_asn

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestHandleFilterUpdateUnknownListRejects covers AC-22.
//
// A chain naming a list this plugin does not hold is a config the operator
// believes is filtering. Accepting would pass every route the list exists to
// stop, and nothing anywhere would say so.
//
// VALIDATES: AC-22.
// PREVENTS: a lookup miss reading as an empty list, which is the guard-shaped
// zero ai/rules/principles.md forbids.
func TestHandleFilterUpdateUnknownListRejects(t *testing.T) {
	configureFrom(t, `        reject-asn NO-TRANSIT {
            indirect [ 3356 ]
        }`)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NOT-CONFIGURED", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
		Update: updateFor(sequence(65001, 65002)),
	})

	assert.Equal(t, sdk.FilterReject, out.Action, "an unknown list name fails closed")
}

// TestHandleFilterUpdateBeforeConfigureRejects covers AC-23.
//
// A filter-update that arrives before any configure delivery has the same cost
// as an unknown list and takes the same answer.
//
// VALIDATES: AC-23.
// PREVENTS: the store's nil pointer reading as "no list rejects this", which
// would open every attached session for the whole startup window.
func TestHandleFilterUpdateBeforeConfigureRejects(t *testing.T) {
	held := listsByName.Load()
	t.Cleanup(func() { listsByName.Store(held) })
	listsByName.Store(nil)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
		Update: updateFor(sequence(65001, 65002)),
	})

	assert.Equal(t, sdk.FilterReject, out.Action, "no configure delivery yet fails closed")
}
