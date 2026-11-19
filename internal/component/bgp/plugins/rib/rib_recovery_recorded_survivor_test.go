// Design: docs/architecture/plugin/rib-storage-design.md -- recovery retains original admitted provenance.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRecoveryUnrecordedWinnerCannotStarveRecordedSurvivor puts a passive
// forwarding observation ahead of a genuinely leased path in the same rank.
// A bounded recovery answer must spend its one slot on the recorded survivor,
// carrying the original receipt; when that path is denied, passive input must
// not become authority merely because it is the last candidate left.
func TestRecoveryUnrecordedWinnerCannotStarveRecordedSurvivor(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, published := range []bool{false, true} {
			rail := "wire/command"
			if native {
				rail = "native/command"
			}
			if published {
				rail += "/published"
			}
			t.Run(rail, func(t *testing.T) {
				r, calls := sourceLeaseTestRIB(t, native)
				source, _ := sourceIdentityPeers()
				raw := []byte{24, 10, 77, 1}
				source.state(t, r, false, rpc.SessionStateUp, rpc.ReasonAttached)
				source.receive(t, r, false, raw, 100)
				require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, source.receipt))
				const passiveOwner uint64 = 1772
				passiveAddress := netip.MustParseAddr("192.0.2.177")
				passive, err := storage.NewAddressPeerRIB(passiveAddress)
				require.NoError(t, err)
				passive.Insert(family.IPv4Unicast, unicastAttrs([4]byte{10, 0, 0, 1}, 30, 300), raw)
				r.peerMu.Lock()
				r.bgpPeers[passiveOwner] = passive
				r.peerMeta[passiveOwner] = &peerMetadata{Address: passiveAddress}
				r.peerUp[passiveOwner] = true
				candidates := r.gatherPrefixCandidatesForLocked(family.IPv4Unicast,
					netip.MustParsePrefix("10.77.1.0/24"), candidateAdmissionRecovery)
				r.peerMu.Unlock()
				require.Len(t, candidates, 2, "both paths must reach the actual recovery gather")
				winner := SelectBest(candidates)
				require.NotNil(t, winner)
				require.Equal(t, passiveAddress, winner.PeerIP, "the passive path must outrank the useful survivor before admission")
				require.Equal(t, candidateOriginUnrecorded, winner.origin)
				require.Zero(t, winner.PeerOwner)
				require.Zero(t, winner.entry.SourceReceipt)
				releaseCandidates(candidates)

				request := ribevents.RecoveryRequest{Destination: netip.MustParseAddr("192.0.2.250"),
					DestinationOwner: 1999, Family: family.IPv4Unicast, Limit: 1,
					Keys: []ribevents.RecoveryKey{{NLRI: raw}}}
				outcomes := recoveryAdmissionLookup(t, r, request, published)
				require.NoError(t, ribevents.CheckRecoveryAnswer(&request, outcomes))
				require.Len(t, outcomes, 1)
				require.False(t, outcomes[0].Withdraw)
				require.Len(t, outcomes[0].Survivors, 1)
				survivor := outcomes[0].Survivors[0]
				require.Equal(t, source.owner, survivor.SourceOwner)
				require.Equal(t, netip.MustParseAddr(sourceIdentityAddress), survivor.Source)
				require.Equal(t, source.receipt, survivor.SourceReceipt)
				require.Equal(t, source.message, survivor.MessageID)
				require.Equal(t, uint64(5), survivor.SourcePolicyEpoch)
				require.Equal(t, raw, survivor.NLRI)
				require.Equal(t, []byte{10, 0, 0, 1}, survivor.NextHop)
				require.NotEmpty(t, survivor.Attributes)
				require.Zero(t, calls.count(rpc.SourceLeaseAcquire, 0), "passive storage must not acquire source authority")

				r.peerMu.Lock()
				r.bgpPeers[source.owner].DenySourcePolicyThrough(5)
				r.peerMu.Unlock()
				outcomes = recoveryAdmissionLookup(t, r, request, published)
				require.NoError(t, ribevents.CheckRecoveryAnswer(&request, outcomes))
				require.Len(t, outcomes, 1)
				require.True(t, outcomes[0].Withdraw, "an unrecorded observation alone cannot authorize recovery")
				require.Empty(t, outcomes[0].Survivors)
				require.Zero(t, calls.count(rpc.SourceLeaseRelease, source.receipt), "recovery lookup does not destructively retire a useful stored source")
			})
		}
	}
}
