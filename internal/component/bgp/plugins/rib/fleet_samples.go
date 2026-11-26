// Design: docs/architecture/fleet-config.md -- BGP RIB fleet metric sample producer
// Related: rib_commands.go -- status() inbound path counting and command registration
// Related: rib.go -- commandDecls() Stage 1 declaration

package rib

import (
	"github.com/ze-software/ze/pkg/fleet"
)

// cmdFleetSourceBGPRIB is the hidden read-only command queried by the managed
// fleet sampler across both internal and forked transports.
const cmdFleetSourceBGPRIB = "show fleet source bgp-rib"

// totalRoutesIn returns the total current inbound paths held by this RIBManager
// across every BGP peer Adj-RIB-In and every non-BGP inbound pool, matching the
// exact source of routes-in in RIBManager.status.
func (r *RIBManager) totalRoutesIn() int {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	total := 0
	for _, peerRIB := range r.bgpPeers {
		if peerRIB != nil {
			total += peerRIB.Len()
		}
	}
	for _, protoPeers := range r.ribInPool {
		for _, peerRIB := range protoPeers {
			if peerRIB != nil {
				total += peerRIB.Len()
			}
		}
	}
	return total
}

// fleetSourceSnapshot returns the bounded fleet.SourceSnapshot carrying the
// current inbound route count held by this RIB instance.
func (r *RIBManager) fleetSourceSnapshot() fleet.SourceSnapshot {
	return fleet.SourceSnapshot{
		Owner: fleet.SampleOwnerBGPRIB,
		Readings: []fleet.SourceReading{
			{
				Figure: fleet.FigureRoutesHeld,
				Unit:   fleet.SampleUnitCount,
				Value:  float64(r.totalRoutesIn()),
			},
		},
	}
}

// fleetSourceCommand answers show fleet source bgp-rib for both internal and
// forked bgp-rib executions.
func (r *RIBManager) fleetSourceCommand(_ string, _ []string) (string, any, error) {
	return statusDone, r.fleetSourceSnapshot(), nil
}
