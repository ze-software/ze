// Design: docs/architecture/ike/ipsec-dataplane-inspection.md -- kernel dataplane read surface
// Related: health.go -- checkIPsecHealth, which folds this signal in

package engine

import (
	"errors"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// driftSAD reads the kernel SAD for the drift comparison. A variable so a test
// can supply a fixture: the comparison is the thing under test, not netlink.
var driftSAD = func() ([]dataplane.SAInfo, error) {
	dp := dataplane.Get()
	if dp == nil {
		return nil, dataplane.ErrNotRegistered
	}
	return dp.ListSAs(0)
}

// dataplaneObservation tracks publications and backend writes. The mutex is never
// held across a dump or while acquiring a peer lock.
var dataplaneObservation struct {
	sync.Mutex
	generation uint64
	writers    uint64
}

func dataplaneChanged() {
	dataplaneObservation.Lock()
	dataplaneObservation.generation++
	dataplaneObservation.Unlock()
}

// beginDataplaneWrite MUST be paired with deferred endDataplaneWrite.
func beginDataplaneWrite() {
	dataplaneObservation.Lock()
	dataplaneObservation.generation++
	dataplaneObservation.writers++
	dataplaneObservation.Unlock()
}

// endDataplaneWrite MUST run after each beginDataplaneWrite, including failed writes.
func endDataplaneWrite() {
	dataplaneObservation.Lock()
	dataplaneObservation.generation++
	dataplaneObservation.writers--
	dataplaneObservation.Unlock()
}

func dataplaneGeneration() (uint64, bool) {
	dataplaneObservation.Lock()
	defer dataplaneObservation.Unlock()
	return dataplaneObservation.generation, dataplaneObservation.writers == 0
}

// setChildRemoving runs within a guarded backend write. Successful reinstallation
// clears the flag; a replacement Child SA starts with it clear.
func setChildRemoving(child *ChildSA, removing bool) {
	dataplaneObservation.Lock()
	child.dataplaneRemoving = removing
	dataplaneObservation.Unlock()
}

var errDataplaneChanging = errors.New("cannot obtain a consistent Child SA generation; dataplane state is unknown")

// DataplaneSnapshot joins one peer generation with one SAD dump. Peers remains
// available on error for engine-belief displays; SAs must only be used on success.
type DataplaneSnapshot struct {
	Peers map[string]PeerInfo
	SAs   []dataplane.SAInfo
}

// ObserveDataplane reads belief and kernel state without blocking an installation
// on netlink readback. Any publication or backend write across the read invalidates
// the observation, even if the replacement reuses the previous SPI.
func ObserveDataplane() (DataplaneSnapshot, error) {
	generation, idle := dataplaneGeneration()
	observation := DataplaneSnapshot{Peers: PeerInfoMap()}
	if !idle {
		return observation, errDataplaneChanging
	}
	for name := range observation.Peers {
		if observation.Peers[name].childRemoving {
			return observation, errDataplaneChanging
		}
	}
	sas, err := driftSAD()
	if err != nil {
		return observation, err
	}
	after, idle := dataplaneGeneration()
	if !idle || generation != after {
		return observation, errDataplaneChanging
	}
	observation.SAs = sas
	return observation, nil
}

func driftingPeers() (peers []string, known bool) {
	observation, err := ObserveDataplane()
	if err != nil {
		return nil, false
	}
	return driftingPeersFrom(observation), true
}

// driftingPeersFrom compares only expected identities. Old SAs may coexist with
// replacements during rekey (RFC 7296 Section 2.8).
func driftingPeersFrom(observation DataplaneSnapshot) []string {
	inKernel := make(map[dataplane.SAIdentity]bool, len(observation.SAs))
	for i := range observation.SAs {
		sa := &observation.SAs[i]
		inKernel[dataplane.IdentityOf(sa.SPI, sa.Dst, sa.Proto, sa.IfID)] = true
	}
	var peers []string
	for name := range observation.Peers {
		info := observation.Peers[name]
		if !info.HasChild {
			continue
		}
		missing := (info.ChildInSPI != 0 && !inKernel[info.ChildInID]) ||
			(info.ChildOutSPI != 0 && !inKernel[info.ChildOutID])
		if missing {
			peers = append(peers, name)
		}
	}
	slices.Sort(peers)
	return peers
}

// driftDetail renders the health message for a drifting set. The peer names are
// IN the message because a health status with no subject leaves an operator to
// find the peer themselves.
func driftDetail(peers []string) string {
	var b textbuf.Buffer
	b.Str("ipsec dataplane drift: the kernel does not hold the child SA of ")
	for i, name := range peers {
		if i > 0 {
			b.Str(", ")
		}
		b.Quoted(name)
	}
	return b.String()
}
