// Design: docs/guide/flowspec-protected-router.md -- replay admission follows registered FlowSpec identity

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/core/family"
)

// TestFlowSpecReplayRecognizesRegisteredFamilies proves that replay admission
// follows the registered SAFI, including an AFI outside the original IPv4/IPv6
// name list. The control is a registered unicast family; an unknown name must
// refuse instead of being treated as a unicast-only peer.
func TestFlowSpecReplayRecognizesRegisteredFamilies(t *testing.T) {
	added, err := family.RegisterFamilyBatch([]family.FamilyRegistration{
		{AFI: 65500, SAFI: family.SAFIFlowSpec, AFIName: "replay-test", SAFIName: "flow"},
	})
	require.NoError(t, err)
	t.Cleanup(func() { family.UnregisterFamilyBatch(added) })
	for _, test := range []struct {
		name     string
		flowSpec bool
		unknown  bool
	}{
		{name: "replay-test/flow", flowSpec: true},
		{name: "ipv4/unicast"},
		{name: "unregistered/flow", unknown: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer := &reactor.PeerSettings{PrefixMaximum: map[string]uint32{test.name: 100}}
			flowSpec, err := hasFlowSpecFamily(peer)
			if test.unknown {
				require.ErrorContains(t, err, test.name)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.flowSpec, flowSpec)
		})
	}
}
