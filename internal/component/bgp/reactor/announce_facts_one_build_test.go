// Design: reactor_api_batch.go -- announceFacts, the group key whose equality this file proves still groups.
// Related: announce_facts_partition_test.go -- the other half: peers that differ are split.
// Related: announce_metrics_test.go -- announceFakeRegistry, the counter this file reads.
package reactor

import (
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnnounceFactsIdenticalPeersShareOneBuild proves the grouping still does
// its job: two peers equal in every announceFacts field land in ONE group and
// are served by ONE build.
//
// A build leaves no mark on the wire that says how many builds produced it, so
// the test counts builds where a build IS counted: a batch too large for the
// build buffer is refused by buildBatchAnnounceUpdate, and every refusal moves
// ze_bgp_announce_dropped_oversize_total once (logAnnounceTooLarge). With
// update groups disabled each peer builds on its own, so the same announce
// counts two; that case shows the counter counts builds rather than announces.
// With update groups enabled the two peers share a key, so it counts one.
//
// VALIDATES: peers identical in every announceFacts field share one group and
// one build.
// PREVENTS: a key holding a per-peer value no build reads (an address, a
// pointer), which would split every group and silently turn the update-group
// optimization into one build per peer.
func TestAnnounceFactsIdenticalPeersShareOneBuild(t *testing.T) {
	for _, tc := range []struct {
		name   string
		groups bool
		builds int
	}{
		{name: "groups disabled builds per peer", groups: false, builds: 2},
		{name: "groups enabled builds once", groups: true, builds: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetAnnounceMetrics(t)
			reg := &announceFakeRegistry{}
			setAnnounceMetricsRegistry(reg)

			// The two peers differ only in their address and router ID, and
			// neither is a field of announceFacts.
			left, _ := newGroupUpdatesPeer(t, "10.0.0.8", "absent")
			right, _ := newGroupUpdatesPeer(t, "10.0.0.9", "absent")
			adapter := groupUpdatesReactor([]*Peer{left, right}, tc.groups)

			err := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), oneBuildOversizeBatch(), plugin.OperatorSender())
			require.ErrorIs(t, err, errAnnounceTooLarge, "the build must refuse the batch, which is what the counter counts")

			require.NotNil(t, reg.vec, "a refused build must reach the counter")
			builds := 0
			for _, counter := range reg.vec.counters {
				builds += counter.n
			}
			assert.Equal(t, tc.builds, builds)
		})
	}
}

// oneBuildOversizeBatch is an IPv4 unicast batch of 1000 host routes. At five
// wire octets each (one length octet, four address octets) the NLRI block is
// 5000 octets, past the 4096-octet build slot, so every build of it is refused.
func oneBuildOversizeBatch() bgptypes.NLRIBatch {
	const hosts = 1000
	nlris := make([]nlri.NLRI, 0, hosts)
	base := netip.MustParseAddr("10.1.0.0").As4()
	for i := range hosts {
		addr := base
		addr[2] = byte(i >> 8)
		addr[3] = byte(i)
		nlris = append(nlris, nlri.NewINET(family.IPv4Unicast, netip.PrefixFrom(netip.AddrFrom4(addr), 32), 0))
	}
	return bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   nlris,
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("1.1.1.1")),
	}
}
