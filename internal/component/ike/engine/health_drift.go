// Design: plan/spec-ipsec-dataplane-inspection.md -- kernel dataplane read surface
// Related: health.go -- checkIPsecHealth, which folds this signal in

package engine

import (
	"sort"

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

// driftingPeers names every peer whose Child SA the engine counts as installed
// and whose SPI the kernel SAD does not hold.
//
// The second result says whether the kernel could be READ. When it is false the
// first is empty and means nothing: no backend, an unprivileged process, or a
// backend that cannot enumerate all land there. A caller that treated that as
// "no drift" would report healthy on the strength of a question nobody asked,
// which is the exact false green this spec exists to remove
// (ai/rules/evidence.md).
//
// The comparison runs in ONE direction. An SPI the kernel holds that the engine
// does not name is not drift: RFC 7296 Section 2.8 keeps the old and the new
// Child SA alive together until the old one is deleted, so a rekey window
// legitimately holds both.
func driftingPeers() (peers []string, known bool) {
	sas, err := driftSAD()
	if err != nil {
		return nil, false
	}

	inKernel := make(map[uint32]bool, len(sas))
	for i := range sas {
		inKernel[sas[i].SPI] = true
	}

	// PeerInfo is large, so the map is indexed rather than range-copied.
	infos := PeerInfoMap()
	for name := range infos {
		info := infos[name]
		if !info.HasChild {
			continue
		}
		missing := (info.ChildInSPI != 0 && !inKernel[info.ChildInSPI]) ||
			(info.ChildOutSPI != 0 && !inKernel[info.ChildOutSPI])
		if missing {
			peers = append(peers, name)
		}
	}
	sort.Strings(peers)
	return peers, true
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
