// Design: docs/architecture/plugin/rib-storage-design.md -- forwarded-wire recovery admission.
// Related: rib_commands.go, rib_recovery.go -- shared candidate snapshots and recovery entry points.
package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
)

// TestRecoveryAdmissionKeepsWireCandidates compares the same retained path in
// the forwarding election and both existing wire-recovery entry points. Its
// real empty Loc-RIB cannot resolve the next hop. Recovery still supplies the
// source-owned bytes; ordinary forwarding must continue excluding the path.
func TestRecoveryAdmissionKeepsWireCandidates(t *testing.T) {
	for _, tc := range []struct {
		name string
		fam  family.Family
		nlri []byte
	}{
		{"cidr", family.IPv4Unicast, []byte{24, 198, 51, 100}},
		{"vpn-key", family.Family{AFI: 1, SAFI: 128}, []byte{112, 0, 6, 0x41, 0, 0, 0, 0, 0, 0, 0, 1, 198, 51, 100}},
	} {
		for _, published := range []bool{true, false} {
			transport := "command"
			if published {
				transport = "published"
			}
			t.Run(tc.name+"/"+transport, func(t *testing.T) {
				r, loc := nextHopReachabilityRIB(t)
				recoveryTestCleanup(t, r)
				source := netip.MustParseAddr("192.0.2.11")
				recoveryAdmissionReceived(r, source, tc.fam, tc.nlri)
				require.False(t, igpcost.Resolve(loc, source).Resolved)
				r.peerMu.RLock()
				ordinary := r.gatherCandidatesLocked(tc.fam, tc.nlri, false)
				r.peerMu.RUnlock()
				defer releaseCandidates(ordinary)
				require.Empty(t, ordinary, "wire recovery must not relax forwarding-table feasibility")

				request := ribevents.RecoveryRequest{Destination: netip.MustParseAddr("192.0.2.20"), Family: tc.fam,
					Keys: []ribevents.RecoveryKey{recoveryKey(tc.nlri)}}
				outcomes := recoveryAdmissionLookup(t, r, request, published)
				require.NoError(t, ribevents.CheckRecoveryAnswer(&request, outcomes))
				require.Len(t, outcomes, 1)
				require.False(t, outcomes[0].Withdraw, "a forwarded-wire replacement does not propose a local forwarding install")
				require.Len(t, outcomes[0].Survivors, 1)
				survivor := outcomes[0].Survivors[0]
				require.Equal(t, source, survivor.Source)
				require.Equal(t, uint64(81), survivor.MessageID)
				require.Equal(t, tc.nlri, survivor.NLRI)
				require.NotEmpty(t, survivor.Attributes)
				nextHop := source.AsSlice()
				if tc.fam != family.IPv4Unicast {
					nextHop = append(make([]byte, 8), nextHop...)
				}
				require.Equal(t, nextHop, survivor.NextHop, "the received next hop travels with the replacement")
			})
		}
	}
}

// TestRecoveryAdmissionPreservesCandidateGuards keeps policy, self-next-hop,
// destination, departed-source and no-survivor exclusions on both recovery
// entry points. Omitting local install feasibility is not unconditional admission.
func TestRecoveryAdmissionPreservesCandidateGuards(t *testing.T) {
	for _, name := range []string{"absent", "excluded", "down", "destination", "self-next-hop", "source-policy"} {
		for _, published := range []bool{true, false} {
			transport := "command"
			if published {
				transport = "published"
			}
			t.Run(name+"/"+transport, func(t *testing.T) {
				r, _ := nextHopReachabilityRIB(t)
				recoveryTestCleanup(t, r)
				source := netip.MustParseAddr("192.0.2.11")
				request := ribevents.RecoveryRequest{Destination: netip.MustParseAddr("192.0.2.20"), Family: family.IPv4Unicast,
					Keys: []ribevents.RecoveryKey{recoveryKey([]byte{24, 198, 51, 100})}}
				if name != "absent" {
					recoveryAdmissionReceived(r, source, request.Family, request.Keys[0].NLRI)
				}
				switch name {
				case "excluded":
					request.Keys[0].Excluded = []netip.Addr{source}
				case "down":
					r.peerUp[source] = false
				case "destination":
					request.Destination = source
				case "self-next-hop":
					self := []netip.Addr{source}
					r.selfNextHops.Store(&self)
				case "source-policy":
					r.bgpPeers[source].ModifyFamilyEntry(request.Family, request.Keys[0].NLRI,
						func(entry *storage.RouteEntry) { entry.SourcePolicyEpoch = 1 })
					r.bgpPeers[source].DenySourcePolicyThrough(1)
				}
				outcomes := recoveryAdmissionLookup(t, r, request, published)
				require.NoError(t, ribevents.CheckRecoveryAnswer(&request, outcomes))
				require.Len(t, outcomes, 1)
				require.True(t, outcomes[0].Withdraw)
				require.Empty(t, outcomes[0].Survivors)
			})
		}
	}
}

// recoveryAdmissionReceived supplies one valid retained route for each gather
// shape. The VPNv4 value includes a label, RD and prefix; its next-hop RD is zero.
func recoveryAdmissionReceived(r *RIBManager, source netip.Addr, fam family.Family, raw []byte) {
	routes := storage.NewPeerRIB(source.String())
	r.bgpPeers[source], r.peerUp[source] = routes, true
	attrs := nativeSentAttrs()
	if fam == family.IPv4Unicast {
		attrs = appendAttr(attrs, byte(attribute.AttrNextHop), 0x40, source.AsSlice())
	} else {
		reach := []byte{0, 1, 128, 12, 0, 0, 0, 0, 0, 0, 0, 0}
		reach = append(reach, source.AsSlice()...)
		reach = append(reach, 0)
		reach = append(reach, raw...)
		attrs = appendAttr(attrs, byte(attribute.AttrMPReachNLRI), 0x80, reach)
	}
	routes.Insert(fam, attrs, raw)
	routes.ModifyFamilyEntry(fam, raw, func(entry *storage.RouteEntry) { entry.MsgID = 81 })
}

// recoveryAdmissionLookup exercises either the published callback or the hidden
// command parser plus its JSON result representation. The publication MUST be
// closed before the next test may install another owner.
func recoveryAdmissionLookup(t *testing.T, r *RIBManager, request ribevents.RecoveryRequest, published bool) []ribevents.RecoveryOutcome {
	t.Helper()
	if published {
		require.Nil(t, ribevents.RecoveryProvider(), "this fixture must own its publication")
		owner := ribevents.PublishRecovery(r.recoveryRoutes)
		defer owner.Close()
		require.Same(t, owner, ribevents.RecoveryProvider())
		outcomes, err := ribevents.RecoveryProvider().Lookup(request)
		require.NoError(t, err)
		return outcomes
	}
	registerBuiltinCommands()
	status, answer, err := r.handleCommand("request bgp rib recovery", "", ribevents.AppendRecoveryArgs(nil, &request))
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	encoded, err := json.Marshal(answer)
	require.NoError(t, err)
	var outcomes []ribevents.RecoveryOutcome
	require.NoError(t, json.Unmarshal(encoded, &outcomes))
	return outcomes
}
