// Design: docs/guide/graceful-restart.md -- families may skip conventional GR.
// Related: real_rib_test.go -- registered RIB engine lifecycle fixture.
package gr

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestLLGROmittedGRFamilyRetainsRealRIB checks immediate LLGR for a received
// family omitted from GR, alongside an independently timed conventional-GR
// family. It drives both event paths and observes the registered RIB engine.
// RFC 9494 Section 4.2: "If the Graceful Restart Capability that was received
// does not list all AFIs/SAFIs supported by the session, then the GR Restart
// Time shall be deemed zero for those AFIs/SAFIs that are not listed."
//
// RFC 9494 Section 4.2: "After the session goes down, and before the session is
// re-established, the stale routes for an AFI/SAFI MUST be retained."
// RFC 9494 Section 4.2: "For each AFI/SAFI for which it has received a nonzero
// Long-Lived Stale Time, the helper router MUST start a timer for that
// Long-Lived Stale Time."
// RFC requirement: RFC9494-4.2-1 positive -- on both event paths, an exchanged LLGR family omitted from received GR retains its eligible received route immediately, independently of a conventional-GR sibling.
// RFC requirement: RFC9494-4.2-2 positive -- the omitted family's received LLST starts at DOWN and expires at its original boundary, while the conventional family's LLST follows its own GR period.
// MUTATION: Build staleFamilies only from the received GR tuples; the omitted family's retained route disappears.
func TestLLGROmittedGRFamilyRetainsRealRIB(t *testing.T) {
	for _, path := range rfc9494Paths {
		for _, conventional := range []bool{false, true} {
			name := path.name + "/empty-gr"
			if conventional {
				name = path.name + "/mixed-gr-llgr"
			}
			t.Run(name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					gp, rib := newGRWithRealRIB(t)
					gp.peerCaps[testPeer] = testCap(6)
					if conventional {
						gp.peerCaps[testPeer] = testCap(6, famIPv6)
						rib.command("request bgp rib inject", testPeer, "ipv6/unicast", "2001:db8::/32", "nexthop", "::1")
						require.Len(t, rib.routes("2001:db8::/32"), 1)
					}
					gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
						{Family: family.IPv4Unicast, ForwardState: true, LLST: 2},
						{Family: family.IPv6Unicast, ForwardState: true, LLST: 4},
					}}
					declareLocalLLGR(gp, family.IPv4Unicast, family.IPv6Unicast)
					rib.received("18c63364", grReceivedAttrs)
					rib.received("18c63365", grReceivedAttrs+"c00804ffff0007")

					// RFC 9494 Section 4.2: omitted GR families have Restart Time zero.
					rfc9494Down(gp, path.structured)
					synctest.Wait()
					rib.down()
					rib.requireStale("198.51.100.0/24", 2)
					require.Contains(t, rib.communities("198.51.100.0/24"), uint32(0xffff0006))
					require.Empty(t, rib.routes("198.51.101.0/24"), "NO_LLGR is purged at immediate LLGR entry")
					require.True(t, gp.state.familyRetained(testPeer, family.IPv4Unicast))
					if conventional {
						rib.requireStale("2001:db8::/32", 1)
						require.NotContains(t, rib.communities("2001:db8::/32"), uint32(0xffff0006))
					}

					// The omitted family's LLST starts at DOWN, not at the other
					// family's six-second Restart Time. Advance only virtual time.
					<-time.After(2*time.Second - time.Nanosecond)
					synctest.Wait()
					rib.requireStale("198.51.100.0/24", 2)
					<-time.After(time.Nanosecond)
					synctest.Wait()
					require.Empty(t, rib.routes("198.51.100.0/24"))
					require.False(t, gp.state.familyRetained(testPeer, family.IPv4Unicast))
					if !conventional {
						return
					}
					rib.requireStale("2001:db8::/32", 1)
					require.True(t, gp.state.familyRetained(testPeer, family.IPv6Unicast))

					<-time.After(4*time.Second - time.Nanosecond)
					synctest.Wait()
					rib.requireStale("2001:db8::/32", 1)
					<-time.After(time.Nanosecond)
					synctest.Wait()
					rib.requireStale("2001:db8::/32", 2)
					require.Contains(t, rib.communities("2001:db8::/32"), uint32(0xffff0006))
					<-time.After(4*time.Second - time.Nanosecond)
					synctest.Wait()
					rib.requireStale("2001:db8::/32", 2)
					<-time.After(time.Nanosecond)
					synctest.Wait()
					require.Empty(t, rib.routes("2001:db8::/32"))
					require.False(t, gp.state.peerActive(testPeer))
				})
			})
		}
	}
}

// TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry keeps the omitted
// family's LLST running across another family's GR expiry, checking the
// original expiry against received RIB state.
// RFC 9494 Section 4.2: "If a Long-Lived Stale Time timer is running for routes
// with a given AFI/SAFI received from a peer, it MUST NOT be updated (other than
// by manual operator intervention) until the peer has established and
// synchronized a new session."
//
// RFC requirement: RFC9494-4.2-1 positive -- an omitted family's real received route remains LLGR-stale across its sibling's GR expiry and survives until its own original LLST deadline.
// RFC requirement: RFC9494-4.2-2 positive -- a running omitted-family LLST expires at DOWN plus eight seconds, not eight seconds after the sibling enters LLGR.
// MUTATION: Suppress handleLLSTExpired; the received route remains past its original deadline.
func TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry(t *testing.T) {
	for _, path := range rfc9494Paths {
		t.Run(path.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				gp, rib := newGRWithRealRIB(t)
				gp.peerCaps[testPeer] = testCap(6, famIPv6)
				gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
					{Family: family.IPv4Unicast, ForwardState: true, LLST: 8},
					{Family: family.IPv6Unicast, ForwardState: true, LLST: 4},
				}}
				declareLocalLLGR(gp, family.IPv4Unicast, family.IPv6Unicast)
				rib.received("18c63364", grReceivedAttrs)
				rib.command("request bgp rib inject", testPeer, "ipv6/unicast", "2001:db8::/32", "nexthop", "::1")
				// RFC 9494 Section 4.2.
				rfc9494Down(gp, path.structured)
				synctest.Wait()
				rib.down()
				rib.requireStale("198.51.100.0/24", 2)
				rib.requireStale("2001:db8::/32", 1)
				gp.state.mu.Lock()
				timer := gp.state.peers[testPeer].llgrFamilies[family.IPv4Unicast]
				gp.state.mu.Unlock()
				require.NotNil(t, timer)

				<-time.After(6 * time.Second)
				synctest.Wait()
				rib.requireStale("198.51.100.0/24", 2)
				rib.requireStale("2001:db8::/32", 2)
				<-time.After(2*time.Second - time.Nanosecond)
				synctest.Wait()
				rib.requireStale("198.51.100.0/24", 2)
				<-time.After(time.Nanosecond)
				synctest.Wait()
				require.Empty(t, rib.routes("198.51.100.0/24"))
				rib.requireStale("2001:db8::/32", 2)
				<-time.After(2 * time.Second)
				synctest.Wait()
				require.Empty(t, rib.routes("2001:db8::/32"))
				require.False(t, gp.state.peerActive(testPeer))
			})
		})
	}
}

// TestLLGROmittedGRFamilyWithoutLLSTIsRemoved is the negative control: a family
// omitted from GR cannot inherit another family's Restart Time, and received
// LLGR alone cannot enable retention without the matching local declaration.
// RFC 9494 Section 4.2: "If both are zero, none of these procedures would apply,
// only those of the base BGP specification [RFC4271] (although EoR would still
// be used as detailed in [RFC4724])."
//
// RFC requirement: RFC9494-4.2-1 negative -- a family omitted from GR with zero received LLST or no matching local LLGR declaration is not retained on either event path.
// RFC requirement: RFC9494-4.2-2 negative -- neither zero received LLST nor an unexchanged LLGR family installs an LLST timer for the omitted family.
// MUTATION: Honor the remote LLGR tuple without the matching local declaration; the not-exchanged control retains its route and arms a timer.
func TestLLGROmittedGRFamilyWithoutLLSTIsRemoved(t *testing.T) {
	for _, path := range rfc9494Paths {
		for _, local := range []bool{false, true} {
			name := path.name + "/not-exchanged"
			if local {
				name = path.name + "/zero-llst"
			}
			t.Run(name, func(t *testing.T) {
				gp, rib := newGRWithRealRIB(t)
				gp.peerCaps[testPeer] = testCap(6, famIPv6)
				llst := uint32(2)
				if local {
					llst = 0
					declareLocalLLGR(gp, family.IPv4Unicast)
				}
				gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
					{Family: family.IPv4Unicast, ForwardState: true, LLST: llst},
				}}
				rib.received("18c63364", grReceivedAttrs)
				// RFC 9494 Sections 4.2 and 5.
				rfc9494Down(gp, path.structured)
				rib.down()
				require.Empty(t, rib.routes("198.51.100.0/24"))
				require.False(t, gp.state.familyRetained(testPeer, family.IPv4Unicast))
				gp.state.mu.Lock()
				var timer *time.Timer
				if state := gp.state.peers[testPeer]; state != nil {
					timer = state.llgrFamilies[family.IPv4Unicast]
				}
				gp.state.mu.Unlock()
				require.Nil(t, timer, "an omitted family without an enabled nonzero LLST has no LLST timer")
			})
		}
	}
}

// TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry blocks only
// the immediate family's readvertisement RPC and observes its sibling's real
// RIB through conventional GR expiry and the sibling's LLST deadline.
// RFC 9494 Section 4.2: "The interval for which they are retained is limited
// by the sum of the Restart Time in the received Graceful Restart Capability
// and the Long-Lived Stale Time in the received Long-Lived Graceful Restart
// Capability."
//
// RFC requirement: RFC9494-4.2-1 positive -- while immediate-family readvertisement is blocked, the conventional sibling remains retained until its original GR-plus-LLST deadline and is then purged from the real RIB.
// RFC requirement: RFC9494-4.2-2 positive -- blocked immediate-family readvertisement cannot postpone the sibling's LLST entry at its GR deadline or its eventual timer expiry.
// MUTATION: Fire immediate-family callbacks before startRestartTimer; the conventional sibling remains at GR level 1 beyond its deadline.
func TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry(t *testing.T) {
	for _, path := range rfc9494Paths {
		t.Run(path.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				gp, rib := newGRWithRealRIB(t)
				gp.peerCaps[testPeer] = testCap(1, famIPv6)
				gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
					{Family: family.IPv4Unicast, ForwardState: true, LLST: 10},
					{Family: family.IPv6Unicast, ForwardState: true, LLST: 2},
				}}
				declareLocalLLGR(gp, family.IPv4Unicast, family.IPv6Unicast)
				rib.received("18c63364", grReceivedAttrs)
				rib.command("request bgp rib inject", testPeer, "ipv6/unicast", "2001:db8::/32", "nexthop", "::1")
				entered := make(chan struct{})
				release := make(chan struct{})
				finished := make(chan struct{})
				gp.dispatchHook = func(command string, args ...string) {
					if command == "clear bgp rib out" {
						if len(args) == 2 && args[1] == "ipv4/unicast" {
							close(entered)
							<-release
						}
					}
					rib.command(command, args...)
				}
				go func() {
					defer close(finished)
					// RFC 9494 Section 4.2.
					rfc9494Down(gp, path.structured)
				}()
				defer func() {
					close(release)
					<-finished
				}()
				<-entered
				synctest.Wait()
				rib.requireStale("198.51.100.0/24", 2)
				rib.requireStale("2001:db8::/32", 1)

				<-time.After(time.Second - time.Nanosecond)
				synctest.Wait()
				rib.requireStale("2001:db8::/32", 1)
				<-time.After(time.Nanosecond)
				synctest.Wait()
				rib.requireStale("198.51.100.0/24", 2)
				atEntry := rib.routes("2001:db8::/32")
				atEntryCommunities := rib.communities("2001:db8::/32")
				<-time.After(2*time.Second - time.Nanosecond)
				synctest.Wait()
				beforeExpiry := rib.routes("2001:db8::/32")
				<-time.After(time.Nanosecond)
				synctest.Wait()
				atExpiry := rib.routes("2001:db8::/32")
				rib.requireStale("198.51.100.0/24", 2)
				require.Len(t, atEntry, 1)
				require.Equal(t, float64(2), atEntry[0]["stale-level"], "immediate-family RPC must not postpone the sibling's GR deadline")
				require.Contains(t, atEntryCommunities, uint32(0xffff0006))
				require.Len(t, beforeExpiry, 1, "the sibling must survive until its LLST deadline")
				require.Equal(t, float64(2), beforeExpiry[0]["stale-level"])
				require.Empty(t, atExpiry, "immediate-family RPC must not postpone the sibling's LLST expiry")
			})
		})
	}
}
